# Suppression des variables globales de flags — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Supprimer les 4 variables globales de flags dans `commands.go` en les remplaçant par un `globalFlags` struct injecté via `context.WithValue` dans un `Before` hook.

**Architecture:** `BeforeFunc` de urfave/cli/v3 retourne `(context.Context, error)` — on injecte `globalFlags` dans le contexte avant chaque action. `flagsFromCtx(ctx)` lit la struct. Les fonctions helper `filterComponentResults`, `filterVarsResults`, `printComponentResults` reçoivent `globalFlags` en paramètre. Après ce refactoring, les actions peuvent être testées en construisant directement un `context.WithValue` sans invoquer de CLI réelle.

**Tech Stack:** Go 1.24, `github.com/urfave/cli/v3 v3.8.0` (`BeforeFunc` = `func(context.Context, *Command) (context.Context, error)`).

---

### Task 1: Refactoring des variables globales — `commands.go`

**Goal:** Supprimer les 4 globales et câbler `globalFlags` via le contexte dans tout `commands.go`.

**Files:**
- Modify: `cmd/gg-version/commands.go`

**Acceptance Criteria:**
- [ ] Les 4 variables globales `configPath`, `repoPath`, `componentFlag`, `rootFlag` sont supprimées
- [ ] `globalFlags`, `contextKey`, `flagsFromCtx` sont définis dans `commands.go`
- [ ] `Run()` utilise un `Before` hook pour injecter `globalFlags` dans le contexte
- [ ] Aucun flag n'a de champ `Destination`
- [ ] `filterComponentResults`, `filterVarsResults`, `printComponentResults` acceptent `globalFlags` en 3ème paramètre
- [ ] `go build ./cmd/gg-version` réussit
- [ ] `go test ./...` PASS

**Verify:** `go build ./cmd/gg-version && go test ./...` → build OK, tous les tests PASS

**Steps:**

- [ ] **Step 1 : Remplacer le bloc `var` par les nouveaux types**

Supprimer le bloc `var` (lignes 20-25) et ajouter juste après les imports :

```go
// contextKey is the unexported key type for storing globalFlags in a context.
type contextKey struct{}

// globalFlags holds the values of the CLI global flags, populated by the Before hook.
type globalFlags struct {
	Config    string
	Repo      string
	Component string
	Root      bool
	Vars      map[string]string // parsed from --var name=value
}

// flagsFromCtx retrieves the globalFlags from a context populated by the Before hook.
// Returns safe defaults if the context does not contain the flags (e.g., in tests).
func flagsFromCtx(ctx context.Context) globalFlags {
	if f, ok := ctx.Value(contextKey{}).(globalFlags); ok {
		return f
	}
	return globalFlags{Config: ".gg-version.yml", Repo: "."}
}
```

- [ ] **Step 2 : Mettre à jour `Run()` — ajouter `Before`, supprimer `Destination`**

Remplacer le début de la fonction `Run()` (la définition des flags globaux et l'ouverture de `cli.Command`) par la version ci-dessous. La liste des sous-commandes (`Commands: []*cli.Command{...}`) reste inchangée.

```go
func Run(version string) error {
	cmd := &cli.Command{
		Version:                    version,
		EnableShellCompletion:      true,
		ShellCompletionCommandName: "completion",
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			f := globalFlags{
				Config:    cmd.String("config"),
				Repo:      cmd.String("repo"),
				Component: cmd.String("component"),
				Root:      cmd.Bool("root"),
				Vars:      parseVarFlags(cmd.StringSlice("var")),
			}
			return context.WithValue(ctx, contextKey{}, f), nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: ".gg-version.yml",
				Usage: "path to the configuration file",
			},
			&cli.StringFlag{
				Name:  "repo",
				Value: ".",
				Usage: "path to the git repository",
			},
			&cli.StringFlag{
				Name:  "component",
				Usage: "filter output to a single component (use with components defined in config)",
			},
			&cli.BoolFlag{
				Name:  "root",
				Usage: "show only the root version, ignoring components",
			},
			&cli.StringSliceFlag{
				Name:  "var",
				Usage: "extra template variable as name=value (repeatable)",
			},
		},
		// Commands: [...] — unchanged
```

- [ ] **Step 3 : Mettre à jour `currentCmd`**

```go
func currentCmd(ctx context.Context, cmd *cli.Command) error {
	flags := flagsFromCtx(ctx)
	if flags.Component != "" && flags.Root {
		return fmt.Errorf("--component and --root are mutually exclusive")
	}

	cfg, err := config.Load(flags.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	project, err := gitpkg.NewProject(flags.Repo, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}
	if project.IsShallow() {
		fmt.Fprintln(os.Stderr, "warning: shallow clone detected — computed version may be underestimated")
	}
	strategy := semverstrategy.NewStrategy(cfg.Semver)

	results, err := strategy.AllCurrent(project, flags.Vars, cfg)
	if err != nil {
		return fmt.Errorf("computing current version: %w", err)
	}
	for i := range results {
		if !results[i].Tagged {
			results[i].Version = ""
		}
	}
	return printComponentResults(results, cmd.String("format"), flags)
}
```

- [ ] **Step 4 : Mettre à jour `nextCmd`**

```go
func nextCmd(ctx context.Context, cmd *cli.Command) error {
	flags := flagsFromCtx(ctx)
	if flags.Component != "" && flags.Root {
		return fmt.Errorf("--component and --root are mutually exclusive")
	}

	cfg, err := config.Load(flags.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	project, err := gitpkg.NewProject(flags.Repo, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}
	if project.IsShallow() {
		fmt.Fprintln(os.Stderr, "warning: shallow clone detected — computed version may be underestimated")
	}
	strategy := semverstrategy.NewStrategy(cfg.Semver)

	results, err := strategy.AllCurrent(project, flags.Vars, cfg)
	if err != nil {
		return fmt.Errorf("computing next version: %w", err)
	}
	return printComponentResults(results, cmd.String("format"), flags)
}
```

- [ ] **Step 5 : Mettre à jour `lastCmd`**

```go
func lastCmd(ctx context.Context, cmd *cli.Command) error {
	flags := flagsFromCtx(ctx)
	if flags.Component != "" && flags.Root {
		return fmt.Errorf("--component and --root are mutually exclusive")
	}

	cfg, err := config.Load(flags.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	project, err := gitpkg.NewProject(flags.Repo, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}
	strategy := semverstrategy.NewStrategy(cfg.Semver)

	results, err := strategy.AllLast(project, cfg)
	if err != nil {
		return fmt.Errorf("computing last version: %w", err)
	}
	return printComponentResults(results, cmd.String("format"), flags)
}
```

- [ ] **Step 6 : Mettre à jour `envCmd`**

```go
func envCmd(ctx context.Context, cmd *cli.Command) error {
	flags := flagsFromCtx(ctx)
	if flags.Component != "" && flags.Root {
		return fmt.Errorf("--component and --root are mutually exclusive")
	}

	format := cmd.String("format")

	project, err := gitpkg.NewProject(flags.Repo, "")
	cfg, cfgErr := config.Load(flags.Config)

	if err != nil || cfgErr != nil {
		// Not a git repo or no config: populate only var namespace
		varVars := map[string]interface{}{}
		for k, v := range flags.Vars {
			varVars[k] = v
		}
		vars := map[string]interface{}{
			"semver": map[string]interface{}{},
			"git":    map[string]interface{}{},
			"regex":  map[string]interface{}{},
			"var":    varVars,
		}
		return printVars(vars, format)
	}

	strategy := semverstrategy.NewStrategy(cfg.Semver)
	allResults, err := strategy.AllVars(project, flags.Vars, cfg)
	if err != nil {
		return fmt.Errorf("computing vars: %w", err)
	}

	// Warn on stderr if any component has a truncated history (shallow clone)
	for _, r := range allResults {
		if gitVars, ok := r.Vars["git"].(map[string]interface{}); ok {
			if truncated, ok := gitVars["Truncated"].(bool); ok && truncated {
				fmt.Fprintln(os.Stderr, "warning: shallow clone — history is truncated, computed version may be underestimated")
				break
			}
		}
	}

	filtered := filterVarsResults(allResults, flags)

	if flags.Component != "" && len(filtered) == 0 {
		return fmt.Errorf("component %q not found in config", flags.Component)
	}

	// Single unnamed result (no components): plain vars output
	if len(filtered) == 1 && filtered[0].Name == "" {
		return printVars(filtered[0].Vars, format)
	}

	if format == "json" {
		out := map[string]interface{}{}
		for _, r := range filtered {
			out[r.Name] = r.Vars
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling vars to JSON: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	// plain multi-component: prefix each key with component name
	for _, r := range filtered {
		prefix := r.Name + "."
		if r.Name == "" {
			prefix = ""
		}
		for _, ns := range []string{"semver", "git", "regex", "var"} {
			nsVars, ok := r.Vars[ns].(map[string]interface{})
			if !ok {
				continue
			}
			keys := make([]string, 0, len(nsVars))
			for k := range nsVars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("%s%s.%s=%v\n", prefix, ns, k, nsVars[k])
			}
		}
	}
	return nil
}
```

- [ ] **Step 7 : Mettre à jour `configCmd`**

```go
func configCmd(ctx context.Context, cmd *cli.Command) error {
	flags := flagsFromCtx(ctx)
	format := cmd.String("format")
	if format != "yaml" && format != "json" {
		return fmt.Errorf("unknown format %q: must be yaml or json", format)
	}

	source := flags.Config
	if _, err := os.Stat(flags.Config); os.IsNotExist(err) {
		source = "default"
	}

	cfg, err := config.Load(flags.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if format == "json" {
		out := struct {
			Source     string                            `json:"_source"`
			Semver     config.SemverConfig               `json:"semver"`
			Components map[string]config.ComponentConfig `json:"components,omitempty"`
		}{
			Source:     source,
			Semver:     cfg.Semver,
			Components: cfg.Components,
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling config to JSON: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config to YAML: %w", err)
	}
	var comment string
	if source == "default" {
		comment = "# default config\n"
	} else {
		comment = fmt.Sprintf("# config from: %s\n", source)
	}
	fmt.Print(comment + string(b))
	return nil
}
```

- [ ] **Step 8 : Mettre à jour `componentsCmd`**

```go
func componentsCmd(ctx context.Context, cmd *cli.Command) error {
	flags := flagsFromCtx(ctx)
	format := cmd.String("format")
	if format != "plain" && format != "json" {
		return fmt.Errorf("unknown format %q: must be plain or json", format)
	}

	cfg, err := config.Load(flags.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Components) == 0 {
		fmt.Println("(no components defined)")
		return nil
	}

	type componentInfo struct {
		Path       string `json:"path"`
		TagScope   string `json:"tag_scope"`
		TagPattern string `json:"tag_pattern"`
	}
	infoMap := map[string]componentInfo{}
	names := make([]string, 0, len(cfg.Components))
	for name := range cfg.Components {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		comp := cfg.Components[name]
		scope := comp.TagScope
		if scope == "" {
			scope = name
		}
		pattern := scope + "/" + cfg.Semver.TagPrefix + "*"
		infoMap[name] = componentInfo{
			Path:       comp.Path,
			TagScope:   scope,
			TagPattern: pattern,
		}
	}

	if format == "json" {
		b, err := json.MarshalIndent(infoMap, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling to JSON: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	// Calculate dynamic column widths
	maxNameLen, maxPathLen := 0, 0
	for _, name := range names {
		if len(name) > maxNameLen {
			maxNameLen = len(name)
		}
		if len(infoMap[name].Path) > maxPathLen {
			maxPathLen = len(infoMap[name].Path)
		}
	}

	for _, name := range names {
		info := infoMap[name]
		fmt.Printf("%-*s path=%-*s tag=%s\n", maxNameLen, name, maxPathLen, info.Path, info.TagPattern)
	}
	return nil
}
```

- [ ] **Step 9 : Mettre à jour `tagCmd`**

```go
func tagCmd(ctx context.Context, cmd *cli.Command) error {
	flags := flagsFromCtx(ctx)
	if flags.Component != "" && flags.Root {
		return fmt.Errorf("--component and --root are mutually exclusive")
	}

	cfg, err := config.Load(flags.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	p, err := gitpkg.NewProject(flags.Repo, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}
	if p.IsShallow() {
		fmt.Fprintln(os.Stderr, "warning: shallow clone detected — computed version may be underestimated")
	}
	strategy := semverstrategy.NewStrategy(cfg.Semver)
	results, err := strategy.AllCurrent(p, flags.Vars, cfg)
	if err != nil {
		return fmt.Errorf("computing versions: %w", err)
	}
	filtered := filterComponentResults(results, flags)

	dryRun := cmd.Bool("dry-run")
	push := cmd.Bool("push")
	msgFlag := cmd.String("message")

	hash, err := p.CommitHash()
	if err != nil {
		return fmt.Errorf("getting commit hash: %w", err)
	}
	shortHash := hash[:7]
	created := 0

	for _, r := range filtered {
		if r.Tagged {
			fmt.Fprintf(os.Stderr, "already tagged as %s, skipping\n", r.Version)
			continue
		}
		tagName := r.Version
		if tagName == "" {
			continue
		}
		msg := msgFlag
		if msg == "" {
			msg = "chore: release " + tagName
		}
		if dryRun {
			fmt.Printf("would create tag %s on %s\n", tagName, shortHash)
			continue
		}
		if err := p.CreateTag(tagName, msg); err != nil {
			return fmt.Errorf("creating tag %s: %w", tagName, err)
		}
		fmt.Printf("created tag %s on %s\n", tagName, shortHash)
		created++
	}

	if push && !dryRun && created > 0 {
		if err := p.PushTags(); err != nil {
			return fmt.Errorf("pushing tags: %w", err)
		}
		fmt.Printf("pushed %d tag(s) to origin\n", created)
	}
	return nil
}
```

- [ ] **Step 10 : Mettre à jour `lintCmd`**

```go
func lintCmd(ctx context.Context, cmd *cli.Command) error {
	flags := flagsFromCtx(ctx)
	if flags.Component != "" && flags.Root {
		return fmt.Errorf("--component and --root are mutually exclusive")
	}

	cfg, err := config.Load(flags.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	p, err := gitpkg.NewProject(flags.Repo, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}

	tagPrefix, filterCfg, err := lintFilterConfig(cfg, flags.Component, flags.Root)
	if err != nil {
		return err
	}

	f := semverstrategy.NewSemverFormat(tagPrefix, nil)
	lastTag, err := p.LastTag(f)
	if err != nil {
		return fmt.Errorf("finding last tag: %w", err)
	}

	if lastTag == "0.0.0" {
		return nil
	}

	all, truncated, err := p.CommitSinceTag(lastTag)
	if err != nil {
		return fmt.Errorf("reading commits since %s: %w", lastTag, err)
	}
	if truncated {
		fmt.Fprintln(os.Stderr, "warning: shallow clone — commit history is truncated, lint results may be incomplete")
	}
	var commits []*object.Commit
	if len(all) > 1 {
		commits = all[:len(all)-1]
	}

	commits = semverstrategy.FilterCommits(commits, p.CommitFiles, filterCfg)

	violations := semverstrategy.LintCommits(commits, cfg.Semver.ConventionalCommits)
	if len(violations) == 0 {
		return nil
	}

	fmt.Fprintf(os.Stderr, "%d commit(s) do not follow Conventional Commits since %s:\n",
		len(violations), lastTag)
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  %s %q\n", v.Hash, v.Subject)
	}
	return cli.Exit("", 1)
}
```

- [ ] **Step 11 : Mettre à jour les helpers — `filterComponentResults`, `filterVarsResults`, `printComponentResults`**

```go
// filterComponentResults filters results based on globalFlags.
func filterComponentResults(results []semverstrategy.ComponentResult, flags globalFlags) []semverstrategy.ComponentResult {
	if flags.Root {
		for _, r := range results {
			if r.Name == "@root" || r.Name == "" {
				return []semverstrategy.ComponentResult{r}
			}
		}
		return results[:1]
	}
	if flags.Component != "" {
		for _, r := range results {
			if r.Name == flags.Component {
				return []semverstrategy.ComponentResult{r}
			}
		}
		return nil
	}
	return results
}

func filterVarsResults(results []semverstrategy.ComponentVarsResult, flags globalFlags) []semverstrategy.ComponentVarsResult {
	if flags.Root {
		for _, r := range results {
			if r.Name == "@root" || r.Name == "" {
				return []semverstrategy.ComponentVarsResult{r}
			}
		}
		return results[:1]
	}
	if flags.Component != "" {
		for _, r := range results {
			if r.Name == flags.Component {
				return []semverstrategy.ComponentVarsResult{r}
			}
		}
		return nil
	}
	return results
}

// printComponentResults prints version results to stdout in the requested format.
func printComponentResults(results []semverstrategy.ComponentResult, format string, flags globalFlags) error {
	if format != "plain" && format != "json" {
		return fmt.Errorf("unknown format %q: must be plain or json", format)
	}

	filtered := filterComponentResults(results, flags)

	if flags.Component != "" && len(filtered) == 0 {
		return fmt.Errorf("component %q not found in config", flags.Component)
	}

	// Single version: non-monorepo (no name) or filtered by --component/--root
	if len(filtered) == 1 && (filtered[0].Name == "" || flags.Component != "" || flags.Root) {
		if format == "json" {
			b, err := json.Marshal(filtered[0].Version)
			if err != nil {
				return fmt.Errorf("marshaling version to JSON: %w", err)
			}
			fmt.Println(string(b))
			return nil
		}
		fmt.Println(filtered[0].Version)
		return nil
	}

	// Multiple named results (monorepo without filter)
	if format == "json" {
		out := make(map[string]string, len(filtered))
		for _, r := range filtered {
			out[r.Name] = r.Version
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling versions to JSON: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	// Calculate dynamic column width for component names
	maxLen := 0
	for _, r := range filtered {
		if len(r.Name) > maxLen {
			maxLen = len(r.Name)
		}
	}

	for _, r := range filtered {
		fmt.Printf("%-*s %s\n", maxLen, r.Name, r.Version)
	}
	return nil
}
```

- [ ] **Step 12 : Vérifier build et tests**

```bash
go build ./cmd/gg-version && go test ./...
```

Résultat attendu :
```
ok  gover/config
ok  gover/git
ok  gover/strategy/semver
```

Si le build échoue : chercher les références restantes aux 4 globales avec `grep -n "configPath\|repoPath\|componentFlag\|rootFlag" cmd/gg-version/commands.go` — elles doivent toutes être remplacées.

- [ ] **Step 13 : Vérifier l'absence des globales**

```bash
grep -n "^var\b\|configPath\|repoPath\|componentFlag\|rootFlag\|Destination:" cmd/gg-version/commands.go
```

Résultat attendu : aucune ligne.

- [ ] **Step 14 : Commit**

```bash
git add cmd/gg-version/commands.go
git commit -m "refactor: replace global flag vars with globalFlags struct via context"
```

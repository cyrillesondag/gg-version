# env Command & Variable Namespacing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ajouter `Strategy.Vars()`, migrer les templates vers la syntaxe namespaceée (`{{ .semver.LastTag }}`), ajouter `--var` sur toutes les commandes et implémenter la commande `env`.

**Architecture:** `Strategy.Vars()` construit un `map[string]interface{}` à deux niveaux (semver/git/regex/var) ; `Current()` est refactorisé pour appeler `Vars()` dans le cas pre-release ; la commande `env` appelle `Vars()` directement avec `DefaultConfig`.

**Tech Stack:** Go, `urfave/cli/v3`, `go-git/v5`, `text/template`, `encoding/json`

---

### Task 1: Vars() method, Current() refactor, breaking template syntax change

**Goal:** Ajouter `Strategy.Vars()`, refactoriser `Current()` pour l'utiliser, et migrer tous les templates vers la syntaxe `{{ .semver.LastTag }}`.

**Files:**
- Modify: `strategy/semver/semver.go`
- Modify: `strategy/semver/semver_test.go`
- Modify: `config/config.go`

**Acceptance Criteria:**
- [ ] `Strategy.Vars(p GitProject, extra map[string]string) (map[string]interface{}, error)` existe et retourne les 4 namespaces
- [ ] `vars["semver"]` contient `LastTag`, `CommitCount`, `ShortHash`
- [ ] `vars["git"]` contient `Branch` (nom court, sans `refs/heads/`)
- [ ] `vars["regex"]` contient les groupes nommés du pattern de branche
- [ ] `vars["var"]` contient les paires extra passées en paramètre
- [ ] `Current()` accepte `extra map[string]string` en deuxième paramètre
- [ ] `Current()` appelle `Vars()` pour le cas pre-release
- [ ] `DefaultConfig()` utilise `{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}`
- [ ] `go test ./strategy/semver/ ./config/ -v` → PASS

**Verify:** `/usr/local/go/bin/go test ./strategy/semver/ ./config/ -v` → tous PASS

**Steps:**

- [ ] **Step 1 : Écrire les tests qui échouent dans `strategy/semver/semver_test.go`**

Ajouter après `TestCompare`, avant les helpers in-memory. Mettre à jour `mainConfig()` et les appels à `Current()` en même temps pour garder la cohérence :

```go
// ── Updated mainConfig ───────────────────────────────────────────────────────

func mainConfig() config.SemverConfig {
	return config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: "^refs/heads/main$", Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}"},
		},
	}
}
```

Ajouter les tests `Vars` après les helpers `fakeProject` :

```go
// ── Vars tests ────────────────────────────────────────────────────────────────

func TestVars_semverNamespace(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	createCommit(t, repo)
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	semverVars, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	if semverVars["LastTag"] != "1.2.3" {
		t.Errorf("expected LastTag 1.2.3, got %v", semverVars["LastTag"])
	}
	if semverVars["CommitCount"] != 2 {
		t.Errorf("expected CommitCount 2, got %v", semverVars["CommitCount"])
	}
	hash, ok := semverVars["ShortHash"].(string)
	if !ok || len(hash) != 7 {
		t.Errorf("expected ShortHash to be a 7-char string, got %v", semverVars["ShortHash"])
	}
}

func TestVars_gitNamespace(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/feature/foo")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	gitVars, ok := vars["git"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"git\"] to be a map")
	}
	if gitVars["Branch"] != "feature/foo" {
		t.Errorf("expected Branch feature/foo, got %v", gitVars["Branch"])
	}
}

func TestVars_regexNamespace(t *testing.T) {
	repo := newRepo(t)
	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: `^refs/heads/release/(?P<major>\d+)\.x$`, Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .semver.LastTag }}-dev.{{ .semver.CommitCount }}"},
		},
	}
	p := newFakeProject(t, repo, "refs/heads/release/1.x")
	s := semverstrategy.NewStrategy(cfg)

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	regexVars, ok := vars["regex"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"regex\"] to be a map")
	}
	if regexVars["major"] != "1" {
		t.Errorf("expected regex.major=1, got %v", regexVars["major"])
	}
}

func TestVars_varNamespace(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, map[string]string{"env": "prod", "team": "platform"})
	if err != nil {
		t.Fatal(err)
	}
	varVars, ok := vars["var"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"var\"] to be a map")
	}
	if varVars["env"] != "prod" {
		t.Errorf("expected var.env=prod, got %v", varVars["env"])
	}
	if varVars["team"] != "platform" {
		t.Errorf("expected var.team=platform, got %v", varVars["team"])
	}
}
```

Mettre à jour `TestLastRespectsMajorConstraint` (format inline) :

```go
// Dans TestLastRespectsMajorConstraint, remplacer :
{Pattern: ".*", Release: false, Format: "{{ .LastTag }}-dev.{{ .CommitCount }}"},
// par :
{Pattern: ".*", Release: false, Format: "{{ .semver.LastTag }}-dev.{{ .semver.CommitCount }}"},
```

Mettre à jour les appels `s.Current(p)` → `s.Current(p, nil)` dans tous les tests strategy existants.

- [ ] **Step 2 : Vérifier que les tests échouent**

```bash
/usr/local/go/bin/go test ./strategy/semver/ -v -run TestVars
```

Résultat attendu : FAIL avec `s.Vars undefined` (méthode inexistante).

- [ ] **Step 3 : Implémenter `Vars()` et mettre à jour `Current()` dans `strategy/semver/semver.go`**

Ajouter `Vars()` après `Last()` :

```go
// Vars returns all template variables as a nested map, grouped by namespace:
//   - "semver": LastTag, CommitCount, ShortHash
//   - "git":    Branch (short name, without refs/heads/)
//   - "regex":  named captures from the matching branch pattern
//   - "var":    key=value pairs from extra
func (s Strategy) Vars(p GitProject, extra map[string]string) (map[string]interface{}, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return nil, fmt.Errorf("getting branch name: %w", err)
	}

	_, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
	f := NewSemverFormat(s.cfg.TagPrefix, constraints)

	lastTag, err := p.LastTag(f)
	if err != nil {
		return nil, err
	}

	effectiveLastTag := lastTag
	if lastTag == "0.0.0" {
		effectiveLastTag = s.cfg.Initial
	}

	commitCount := 0
	if lastTag != "0.0.0" {
		tagged, err := p.IsHeadTagged(lastTag)
		if err != nil {
			return nil, err
		}
		if !tagged {
			commits, err := p.CommitSinceTag(lastTag)
			if err != nil {
				return nil, err
			}
			commitCount = len(commits) - 1
		}
	}

	commitHashFull, err := p.CommitHash()
	if err != nil {
		return nil, err
	}
	shortHash := commitHashFull
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	shortBranch := strings.TrimPrefix(branchName, "refs/heads/")

	regexVars := map[string]interface{}{}
	for k, v := range captures {
		regexVars[k] = v
	}

	varVars := map[string]interface{}{}
	for k, v := range extra {
		varVars[k] = v
	}

	return map[string]interface{}{
		"semver": map[string]interface{}{
			"LastTag":     effectiveLastTag,
			"CommitCount": commitCount,
			"ShortHash":   shortHash,
		},
		"git": map[string]interface{}{
			"Branch": shortBranch,
		},
		"regex": regexVars,
		"var":   varVars,
	}, nil
}
```

Remplacer la signature et le corps de `Current()` :

```go
// Current returns the version at HEAD:
//   - Exact tag if HEAD is a tagged commit
//   - Last tag if on a release branch (untagged HEAD)
//   - Rendered format template if on a pre-release branch
//   - cfg.Initial if no tag exists at all
//
// extra is a map of key=value pairs injected into the "var" template namespace.
func (s Strategy) Current(p GitProject, extra map[string]string) (string, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return "", fmt.Errorf("getting branch name: %w", err)
	}

	branchCfg, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
	f := NewSemverFormat(s.cfg.TagPrefix, constraints)

	lastTag, err := p.LastTag(f)
	if err != nil {
		return "", err
	}

	if lastTag == "0.0.0" {
		return s.cfg.Initial, nil
	}

	tagged, err := p.IsHeadTagged(lastTag)
	if err != nil {
		return "", err
	}
	if tagged {
		return lastTag, nil
	}

	if branchCfg.Release {
		return lastTag, nil
	}

	// Pre-release branch: build vars and render template
	vars, err := s.Vars(p, extra)
	if err != nil {
		return "", err
	}
	return renderTemplate(branchCfg.Format, vars)
}
```

- [ ] **Step 4 : Mettre à jour `DefaultConfig()` dans `config/config.go`**

```go
// Remplacer dans DefaultConfig() :
Format: "{{ .LastTag }}-{{ .Branch }}.{{ .CommitCount }}",
// par :
Format: "{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}",
```

- [ ] **Step 5 : Vérifier que tous les tests passent**

```bash
/usr/local/go/bin/go test ./strategy/semver/ ./config/ -v
```

Résultat attendu : tous PASS.

- [ ] **Step 6 : Commit**

```bash
git add strategy/semver/semver.go strategy/semver/semver_test.go config/config.go
git commit -m "feat: add Strategy.Vars() with namespaced template variables (breaking change)"
```

---

### Task 2: env command and --var flag in CLI

**Goal:** Ajouter la commande `env` avec `--format plain|json`, et le flag `--var name=value` sur toutes les sous-commandes.

**Files:**
- Modify: `command/commands.go`

**Acceptance Criteria:**
- [ ] `gg-version current --var foo=bar` fonctionne (exit 0)
- [ ] `gg-version last --var foo=bar` fonctionne (exit 0)
- [ ] `gg-version env` affiche des lignes `namespace.key=value` (format plain)
- [ ] `gg-version env --format json` affiche du JSON valide avec 4 clés de premier niveau
- [ ] `gg-version env --var env=prod` inclut `var.env=prod` dans la sortie
- [ ] `go build ./...` → succès
- [ ] `go test ./...` → tous PASS

**Verify:** `/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./...` → PASS

**Steps:**

- [ ] **Step 1 : Réécrire `command/commands.go`**

```go
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"

	"gover/config"
	gitpkg "gover/git"
	semverstrategy "gover/strategy/semver"
)

var (
	configPath string
	repoPath   string
)

func Run() error {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Value:       ".gg-version.yml",
				Destination: &configPath,
				Usage:       "path to the configuration file",
			},
			&cli.StringFlag{
				Name:        "repo",
				Value:       ".",
				Destination: &repoPath,
				Usage:       "path to the git repository",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "current",
				Usage: "print the current version at HEAD",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "var",
						Usage: "extra template variable as name=value (repeatable)",
					},
				},
				Action: currentCmd,
			},
			{
				Name:  "last",
				Usage: "print the last valid semver tag reachable from HEAD",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "var",
						Usage: "extra template variable as name=value (repeatable)",
					},
				},
				Action: lastCmd,
			},
			{
				Name:  "env",
				Usage: "print all template variables available for version formatting",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "var",
						Usage: "extra template variable as name=value (repeatable)",
					},
					&cli.StringFlag{
						Name:  "format",
						Value: "plain",
						Usage: "output format: plain or json",
					},
				},
				Action: envCmd,
			},
		},
	}

	return cmd.Run(context.Background(), os.Args)
}

func currentCmd(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	project, err := gitpkg.NewProject(repoPath, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}

	extra := parseVarFlags(cmd.StringSlice("var"))
	strategy := semverstrategy.NewStrategy(cfg.Semver)
	version, err := strategy.Current(project, extra)
	if err != nil {
		return fmt.Errorf("computing current version: %w", err)
	}

	fmt.Println(version)
	return nil
}

func lastCmd(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	project, err := gitpkg.NewProject(repoPath, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}

	strategy := semverstrategy.NewStrategy(cfg.Semver)
	version, err := strategy.Last(project)
	if err != nil {
		return fmt.Errorf("computing last version: %w", err)
	}

	fmt.Println(version)
	return nil
}

func envCmd(ctx context.Context, cmd *cli.Command) error {
	extra := parseVarFlags(cmd.StringSlice("var"))

	project, err := gitpkg.NewProject(".", "")

	var vars map[string]interface{}
	if err != nil {
		// Not a git repo: populate only var namespace, leave others empty
		varVars := map[string]interface{}{}
		for k, v := range extra {
			varVars[k] = v
		}
		vars = map[string]interface{}{
			"semver": map[string]interface{}{"LastTag": "", "CommitCount": 0, "ShortHash": ""},
			"git":    map[string]interface{}{"Branch": ""},
			"regex":  map[string]interface{}{},
			"var":    varVars,
		}
	} else {
		strategy := semverstrategy.NewStrategy(config.DefaultConfig().Semver)
		vars, err = strategy.Vars(project, extra)
		if err != nil {
			return fmt.Errorf("computing vars: %w", err)
		}
	}

	return printVars(vars, cmd.String("format"))
}

// parseVarFlags parses a slice of "name=value" strings into a map.
func parseVarFlags(rawVars []string) map[string]string {
	extra := map[string]string{}
	for _, v := range rawVars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			extra[parts[0]] = parts[1]
		}
	}
	return extra
}

// printVars prints vars to stdout in the requested format (plain or json).
func printVars(vars map[string]interface{}, format string) error {
	if format == "json" {
		out, err := json.MarshalIndent(vars, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling vars to JSON: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}
	// plain (default): one line per variable, format namespace.key=value
	for _, ns := range []string{"semver", "git", "regex", "var"} {
		nsVars, ok := vars[ns].(map[string]interface{})
		if !ok {
			continue
		}
		keys := make([]string, 0, len(nsVars))
		for k := range nsVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s.%s=%v\n", ns, k, nsVars[k])
		}
	}
	return nil
}
```

- [ ] **Step 2 : Vérifier que le build et les tests passent**

```bash
/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./... -v
```

Résultat attendu : build succès, tous les tests PASS.

- [ ] **Step 3 : Vérifier la commande `env` manuellement sur le repo courant**

```bash
/usr/local/go/bin/go run . env
```

Résultat attendu : lignes du style `git.Branch=main`, `semver.CommitCount=0`, `semver.LastTag=0.1.0`, `semver.ShortHash=<7 chars>`.

```bash
/usr/local/go/bin/go run . env --format json
```

Résultat attendu : JSON avec les clés `semver`, `git`, `regex`, `var`.

```bash
/usr/local/go/bin/go run . env --var env=prod
```

Résultat attendu : ligne `var.env=prod` dans la sortie.

- [ ] **Step 4 : Commit**

```bash
git add command/commands.go
git commit -m "feat: add env command and --var flag on all commands"
```

# AllLint — Correction du bypass Strategy dans lintCmd

**Goal:** Déplacer la logique de routing monorepo de `lintCmd` vers l'interface `Strategy` en ajoutant `AllLint`, et réduire `lintCmd` à une délégation pure.

**Architecture:** `AllLint` suit le pattern exact de `AllCurrent` — traversal unique de l'historique git via `buildSharedHistory`, routing par composant dans `strategy/semver`, `ComponentLintResult` par composant. `lintCmd` appelle `strategy.AllLint(p, cfg)` et ne contient plus aucune logique métier.

**Tech Stack:** Go 1.24, go-git/v5, urfave/cli/v3, infrastructure `sharedHistory` et `FilterCommits`/`LintCommits` existantes.

---

## Problème

La commande `lint` contourne l'interface `Strategy` et reconstitue manuellement la logique de routing monorepo (composant → tag prefix → filtre) dans `lintFilterConfig` (~30 lignes dans `commands.go`). Cette logique est dupliquée de `AllCurrent`/`AllVars` dans `component.go`. Toute modification de la sémantique d'un composant (ex. `TagScope` configurable sur `@root`) doit être propagée à deux endroits — risque de divergence silencieuse.

---

## Interface `Strategy` — nouveau contrat

Dans `strategy/semver/semver.go` :

```go
// ComponentLintResult holds the lint result for one entity (component or root).
type ComponentLintResult struct {
    Name       string       // "" = non-monorepo, "@root" = root, sinon nom du composant
    Violations []LintResult // commits dont le sujet ne respecte pas le format CC
    Truncated  bool         // true si l'historique est tronqué (shallow clone)
}

type Strategy interface {
    Current(p GitProject, extra map[string]string) (string, error)
    Last(p GitProject) (string, error)
    Vars(p GitProject, extra map[string]string) (map[string]interface{}, error)
    AllCurrent(p GitProject, extra map[string]string, cfg config.Config) ([]ComponentResult, error)
    AllLast(p GitProject, cfg config.Config) ([]ComponentResult, error)
    AllVars(p GitProject, extra map[string]string, cfg config.Config) ([]ComponentVarsResult, error)
    AllLint(p GitProject, cfg config.Config) ([]ComponentLintResult, error)
}
```

---

## Implémentation `AllLint` dans `component.go`

### Non-monorepo (`cfg.Components` vide)

```go
func (s semverStrategy) AllLint(p GitProject, cfg config.Config) ([]ComponentLintResult, error) {
    if len(cfg.Components) == 0 {
        filterCfg := FilterConfig{
            ExcludePaths:  s.cfg.IgnorePaths,
            IgnoreCommits: s.cfg.IgnoreCommits,
        }
        violations, truncated, err := s.lintWithPrefix(p, s.cfg.TagPrefix, filterCfg)
        if err != nil {
            return nil, err
        }
        return []ComponentLintResult{{Name: "", Violations: violations, Truncated: truncated}}, nil
    }

    // Monorepo : traversal unique partagé.
    hist, err := buildSharedHistory(p)
    if err != nil {
        return nil, err
    }

    branchName, err := p.BranchName()
    if err != nil {
        return nil, fmt.Errorf("getting branch name: %w", err)
    }
    _, captures := s.matchBranch(branchName)
    constraints := versionConstraints(captures)

    // @root : exclut tous les chemins de composants.
    rootFilter := FilterConfig{
        ExcludePaths:  append(append([]string{}, s.cfg.IgnorePaths...), allComponentPaths(cfg.Components)...),
        IgnoreCommits: s.cfg.IgnoreCommits,
    }
    rootViolations, rootTruncated, err := s.lintFromHistory(hist, s.cfg.TagPrefix, constraints, rootFilter, cfg.Semver.ConventionalCommits)
    if err != nil {
        return nil, err
    }
    results := []ComponentLintResult{{Name: "@root", Violations: rootViolations, Truncated: rootTruncated}}

    for _, name := range sortedComponentNames(cfg.Components) {
        comp := cfg.Components[name]
        tagPrefix := ResolveTagPrefix(name, comp, s.cfg.TagPrefix)
        compFilter := FilterConfig{
            IncludePaths:  []string{comp.Path},
            ExcludePaths:  s.cfg.IgnorePaths,
            IgnoreCommits: s.cfg.IgnoreCommits,
        }
        violations, truncated, err := s.lintFromHistory(hist, tagPrefix, constraints, compFilter, cfg.Semver.ConventionalCommits)
        if err != nil {
            return nil, err
        }
        results = append(results, ComponentLintResult{Name: name, Violations: violations, Truncated: truncated})
    }
    return results, nil
}
```

### Helpers internes

```go
// lintWithPrefix est la variante non-monorepo : appelle p.LastTag + p.CommitSinceTag.
func (s semverStrategy) lintWithPrefix(p GitProject, tagPrefix string, filterCfg FilterConfig) ([]LintResult, bool, error) {
    f := NewSemverFormat(tagPrefix, nil)
    lastTag, err := p.LastTag(f)
    if err != nil {
        return nil, false, err
    }
    if lastTag == "0.0.0" {
        return nil, false, nil
    }
    tagged, err := p.IsHeadTagged(lastTag)
    if err != nil {
        return nil, false, err
    }
    if tagged {
        return nil, false, nil
    }
    all, truncated, err := p.CommitSinceTag(lastTag)
    if err != nil {
        return nil, false, fmt.Errorf("commits since %s: %w", lastTag, err)
    }
    var commits []*object.Commit
    if len(all) > 1 {
        commits = all[:len(all)-1]
    }
    commits = FilterCommits(commits, p.CommitFiles, filterCfg)
    violations := LintCommits(commits, s.cfg.ConventionalCommits)
    return violations, truncated, nil
}

// lintFromHistory est la variante monorepo : utilise un sharedHistory pré-fetchée.
func (s semverStrategy) lintFromHistory(
    hist *sharedHistory,
    tagPrefix string,
    constraints map[string]string,
    filterCfg FilterConfig,
    ccCfg config.ConventionalCommitsConfig,
) ([]LintResult, bool, error) {
    f := NewSemverFormat(tagPrefix, constraints)
    lastTag, tagIdx := hist.findLastTag(f)
    if lastTag == "0.0.0" {
        return nil, false, nil
    }
    rawCommits, truncated := hist.commitsSince(tagIdx)
    commits := FilterCommits(rawCommits, nil, filterCfg) // files func not needed for monorepo path
    violations := LintCommits(commits, ccCfg)
    return violations, truncated, nil
}
```

**Note :** `FilterCommits` appelle `files(c)` dès que `IncludePaths` ou `ExcludePaths` est non-vide. `lintFromHistory` doit donc recevoir une référence à `p.CommitFiles`. Passer `p GitProject` à `lintFromHistory` (comme `varsCoreFromHistory`) est l'approche la plus simple. La signature devient :

```go
func (s semverStrategy) lintFromHistory(
    p GitProject,
    hist *sharedHistory,
    tagPrefix string,
    constraints map[string]string,
    filterCfg FilterConfig,
    ccCfg config.ConventionalCommitsConfig,
) ([]LintResult, bool, error)
```

---

## Refactoring `lintCmd` dans `commands.go`

### Avant

```go
func lintCmd(ctx context.Context, cmd *cli.Command) error {
    // ... ~50 lignes : lintFilterConfig, p.LastTag, p.CommitSinceTag, FilterCommits, LintCommits
}

func lintFilterConfig(cfg config.Config, component string, root bool) (string, FilterConfig, error) {
    // ~30 lignes de logique de routing composant→tagPrefix→filtre
}
```

### Après

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

    strategy := semverstrategy.NewStrategy(cfg.Semver)
    results, err := strategy.AllLint(p, cfg)
    if err != nil {
        return fmt.Errorf("lint: %w", err)
    }

    // Filtrer par --component ou --root si spécifié.
    results = filterLintResults(results, flags.Component, flags.Root, len(cfg.Components) > 0)

    // Vérifier truncation et afficher warning.
    for _, r := range results {
        if r.Truncated {
            fmt.Fprintln(os.Stderr, "warning: shallow clone — commit history is truncated, lint results may be incomplete")
            break
        }
    }

    // Collecter et afficher les violations.
    var allViolations []semverstrategy.LintResult
    var lastTag string // pour le message d'erreur — extrait du premier résultat non-vide
    for _, r := range results {
        allViolations = append(allViolations, r.Violations...)
    }

    if len(allViolations) == 0 {
        return nil
    }

    fmt.Fprintf(os.Stderr, "%d commit(s) do not follow Conventional Commits:\n", len(allViolations))
    for _, v := range allViolations {
        fmt.Fprintf(os.Stderr, "  %s %q\n", v.Hash, v.Subject)
    }
    _ = lastTag
    return cli.Exit("", 1)
}

// filterLintResults applique --component / --root sur les résultats AllLint.
func filterLintResults(results []semverstrategy.ComponentLintResult, component string, root bool, isMonorepo bool) []semverstrategy.ComponentLintResult {
    if !isMonorepo {
        return results // non-monorepo : un seul résultat, Name="", pas de filtre
    }
    if component != "" {
        for _, r := range results {
            if r.Name == component {
                return []semverstrategy.ComponentLintResult{r}
            }
        }
        return nil // composant non trouvé — lintCmd gère l'erreur en amont
    }
    if root {
        for _, r := range results {
            if r.Name == "@root" {
                return []semverstrategy.ComponentLintResult{r}
            }
        }
    }
    return results // pas de filtre — tous les composants
}
```

**Note :** `lintFilterConfig` est entièrement supprimée. `filterLintResults` remplace uniquement la logique de présentation (filtre par nom), pas la logique métier.

---

## Message d'erreur — composant introuvable

Avec le nouveau design, si `--component unknown` est passé, `AllLint` retourne normalement (il ne connaît pas le filtre `--component`), et `filterLintResults` retourne `nil`. `lintCmd` doit détecter ce cas :

```go
if flags.Component != "" && isMonorepo {
    found := false
    for _, r := range results {
        if r.Name == flags.Component {
            found = true
            break
        }
    }
    if !found {
        return fmt.Errorf("component %q not found in config", flags.Component)
    }
}
```

Cette vérification se fait **après** `AllLint`, avant `filterLintResults`.

---

## Tests à écrire

### `strategy/semver/component_test.go` — `AllLint`

```go
// TestAllLint_NonMonorepo_NoViolations
// TestAllLint_NonMonorepo_WithViolations
// TestAllLint_NonMonorepo_NoTag
// TestAllLint_Monorepo_SingleTraversal (CommitHistory appelé 1 seule fois)
// TestAllLint_Monorepo_RootViolation
// TestAllLint_Monorepo_ComponentViolation
// TestAllLint_Monorepo_AlphabeticalOrder
// TestAllLint_Monorepo_Truncated
```

Les tests utilisent l'infrastructure existante : `fakeProject` / repos in-memory avec `createCommit`/`createTag`.

### `cmd/gg-version/commands_test.go` (si existant) — `lintCmd`

```go
// Vérifier que lintCmd délègue à AllLint et ne contient plus de logique métier.
// Tests comportementaux : exit 0 si clean, exit 1 si violations, warning si truncated.
```

---

## Fichiers impactés

| Fichier | Modification |
|---|---|
| `strategy/semver/semver.go` | Ajouter `ComponentLintResult` + `AllLint` à l'interface `Strategy` |
| `strategy/semver/component.go` | Implémenter `AllLint`, `lintWithPrefix`, `lintFromHistory` |
| `strategy/semver/component_test.go` | Tests `AllLint_*` |
| `cmd/gg-version/commands.go` | Refactorer `lintCmd`, supprimer `lintFilterConfig`, ajouter `filterLintResults` |

---

## Breaking changes

Aucun changement de comportement observable pour l'utilisateur. La sortie stderr et les exit codes restent identiques.

---

## Invariants à préserver

- `--component` et `--root` restent mutuellement exclusifs
- Warning shallow clone affiché sur stderr quand `Truncated: true`
- Exit 1 si au moins 1 violation, exit 0 sinon
- `IgnorePaths` et `IgnoreCommits` de la config sont respectés (via `FilterConfig` construit dans `AllLint`)
- Ordre des résultats : `@root` en premier, puis composants par ordre alphabétique (invariant de `AllCurrent`)

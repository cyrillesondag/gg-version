# Commande `lint` — Design Spec

**Goal:** Ajouter `gg-version lint` pour signaler les commits qui ne respectent pas le format Conventional Commits depuis le dernier tag.

**Architecture:** Une fonction `LintCommits` dans `strategy/semver/conventional.go` retourne la liste des commits non-CC. La commande `lintCmd` dans `commands.go` orchestre `CommitSinceTag` → `FilterCommits` → `LintCommits` et affiche les violations sur stderr avec exit code 1.

**Tech Stack:** Go 1.24, go-git/v5, urfave/cli/v3, infrastructure CC existante.

---

## Problème

Les commits non-CC (`"WIP fix auth"`, `"temp"`, merges automatiques GitHub) déclenchent un bump de patch implicite sans que l'équipe en soit informée. Il n'existe aucun moyen de valider la qualité des messages de commits avant que l'historique ne soit gravé.

---

## Commande

```
gg-version [global flags] lint
```

Pas de flags supplémentaires. Supporte les flags globaux `--component`, `--root`, `--repo`, `--config`.

---

## Comportement

### Succès — aucune violation

```bash
$ gg-version lint
$ echo $?
0
```

Sortie vide (silencieux comme `git diff`), exit 0.

### Violations trouvées

```bash
$ gg-version lint
3 commit(s) do not follow Conventional Commits since v1.2.0:
  a1b2c3d "WIP fix auth"
  def4567 "temp"
  890abcd "Merge pull request #42"
$ echo $?
1
```

Sortie sur **stderr**, exit **1**.

### HEAD exactement sur un tag (aucun commit à analyser)

```bash
$ gg-version lint
$ echo $?
0
```

Rien à valider, exit 0.

### Aucun tag dans le dépôt

Analyse tous les commits depuis le début du dépôt. Comportement identique : exit 0 si tous CC, exit 1 sinon.

---

## Définition d'une violation

Un commit est une **violation** si son sujet (première ligne du message) **ne correspond pas** au pattern `format` de `conventional_commits` dans la config. Exemples avec le format par défaut `^\w+(?:\(.+\))?!?:` :

| Sujet | Résultat |
|---|---|
| `feat(auth): add OAuth2` | ✓ valide |
| `fix: correct null pointer` | ✓ valide |
| `chore(ci): update actions` | ✓ valide (type inconnu mais format CC respecté) |
| `WIP fix auth` | ✗ violation |
| `Merge pull request #42` | ✗ violation |
| `temp` | ✗ violation |

Les commits CC de type inconnu (ne contribuant pas à un bump) sont **valides** pour `lint` — seul le format compte.

---

## Plage analysée

Commits depuis le dernier tag valide jusqu'à HEAD, via `CommitSinceTag`. Même plage que celle utilisée par le calcul de version. Les règles `ignore_paths` et `ignore_commits` de la config s'appliquent via `FilterCommits`.

---

## Monorepo

Sans `--component` ni `--root`, `lint` analyse les commits `@root` (tous les commits filtrés par `ExcludePaths` de tous les composants). Avec `--component api`, analyse uniquement les commits qui touchent le chemin de `api`. Avec `--root`, identique au comportement sans flag en mode non-monorepo.

---

## Couche logique

### `strategy/semver/conventional.go`

```go
// LintResult holds a commit that violates Conventional Commits format.
type LintResult struct {
    Hash    string // 7-char short hash
    Subject string // first line of commit message
}

// LintCommits returns the commits whose subject does not match the CC format.
// If cfg.Format is empty or invalid, all commits pass (no violations).
// FilterCommits should be applied before calling LintCommits.
func LintCommits(commits []*object.Commit, cfg config.ConventionalCommitsConfig) []LintResult {
    formatRe := compilePattern(cfg.Format)
    var violations []LintResult
    for _, c := range commits {
        subject := strings.SplitN(strings.TrimRight(c.Message, "\n"), "\n", 2)[0]
        if formatRe == nil || !formatRe.MatchString(subject) {
            violations = append(violations, LintResult{
                Hash:    c.Hash.String()[:7],
                Subject: subject,
            })
        }
    }
    return violations
}
```

### `cmd/gg-version/commands.go`

```go
func lintCmd(ctx context.Context, cmd *cli.Command) error {
    if componentFlag != "" && rootFlag {
        return fmt.Errorf("--component and --root are mutually exclusive")
    }
    cfg, err := config.Load(configPath)
    if err != nil {
        return fmt.Errorf("loading config: %w", err)
    }
    p, err := gitpkg.NewProject(repoPath, "")
    if err != nil {
        return fmt.Errorf("opening repo: %w", err)
    }

    strategy := semverstrategy.NewStrategy(cfg.Semver)
    // Determine tag prefix and filter config for the selected component
    tagPrefix, filterCfg := strategy.LintConfig(componentFlag, rootFlag, cfg)

    f := semverstrategy.NewSemverFormat(tagPrefix, nil)
    lastTag, err := p.LastTag(f)
    if err != nil {
        return fmt.Errorf("finding last tag: %w", err)
    }
    all, _, err := p.CommitSinceTag(lastTag)
    if err != nil {
        return fmt.Errorf("reading commits: %w", err)
    }
    commits := semverstrategy.FilterCommits(all, nil, filterCfg)
    violations := semverstrategy.LintCommits(commits, cfg.Semver.ConventionalCommits)

    if len(violations) == 0 {
        return nil
    }

    tagLabel := lastTag
    if tagLabel == "" {
        tagLabel = "(beginning of history)"
    }
    fmt.Fprintf(os.Stderr, "%d commit(s) do not follow Conventional Commits since %s:\n",
        len(violations), tagLabel)
    for _, v := range violations {
        fmt.Fprintf(os.Stderr, "  %s %q\n", v.Hash, v.Subject)
    }
    return cli.Exit("", 1)
}
```

**Note sur `LintConfig` :** méthode helper sur `Strategy` qui retourne `(tagPrefix string, filterCfg FilterConfig)` selon `componentFlag`/`rootFlag`/`cfg` — factorise la logique de sélection de composant déjà présente dans `component.go`.

---

## Tests

### `strategy/semver/conventional_test.go`

```go
func TestLintCommits_clean(t *testing.T) {
    // commits avec sujets CC valides → violations vide
}

func TestLintCommits_violations(t *testing.T) {
    // commits avec sujets non-CC → retourne LintResult avec hash+subject corrects
}

func TestLintCommits_nilFormat(t *testing.T) {
    // cfg.Format vide → aucune violation (formatRe nil)
}
```

---

## Fichiers

| Fichier | Modification |
|---|---|
| `strategy/semver/conventional.go` | Ajouter `LintResult` + `LintCommits` |
| `strategy/semver/conventional_test.go` | `TestLintCommits_*` |
| `strategy/semver/semver.go` | Ajouter méthode helper `LintConfig` sur `Strategy` |
| `cmd/gg-version/commands.go` | Enregistrer `lint` + implémenter `lintCmd` |
| `docs/reference.md` | Ajouter section `lint` |

---

## Breaking changes

Aucun. La commande `lint` est additive.

---

## Limitations connues

- Pas de `--fix` automatique des messages de commits.
- Les merges automatiques (`Merge pull request #42`) sont toujours signalés — à ignorer via `ignore_commits` si gênant.
- Pas de validation des footers (`BREAKING CHANGE:` mal formé non détecté).

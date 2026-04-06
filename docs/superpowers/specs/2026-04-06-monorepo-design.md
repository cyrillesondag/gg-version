# Monorepo Support Design

**Date:** 2026-04-06

## Goal

Permettre à `gg-version` de gérer plusieurs composants indépendants dans un monorepo, chacun avec sa propre version calculée depuis l'historique git, ses propres tags scopés, et un filtre de chemin.

---

## Context

Jusqu'ici `gg-version` opère sur un seul dépôt à la fois. Cette feature ajoute le concept de **composant** : une unité versionnée indépendamment au sein du même repo git, identifiée par un glob de chemin et un scope de tag.

---

## Section 1 : Configuration

### Structure YAML

```yaml
semver:
  tag_prefix: "v"
  initial: "0.1.0"
  branches:
    - pattern: "^refs/heads/main$"
      release: true
    - pattern: ".*"
      release: false
      format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
  conventional_commits:
    format: '^\w+(?:\(.+\))?!?:'
    major: ['^\w+(?:\(.+\))?!:', 'BREAKING[- ]CHANGE:']
    minor: ['^feat(?:\(.+\))?:']
    patch: ['^fix(?:\(.+\))?:']
  ignore_paths:
    - "*.md"
    - "docs/**"
  ignore_commits:
    - "abc1234"

components:
  api:
    path: "api/**"
  web:
    path: "web/**"
  shared:
    path: "{api,web}/shared/**"
    tag_scope: "shared"       # surcharge optionnelle du scope
```

### Résolution du scope de tag

- Scope = `tag_scope` si défini, sinon = nom de la clé dans `components`
- Tag complet = `{scope}/{tag_prefix}{version}` → `api/v1.2.3`
- Séparateur fixe `/`

### Sémantique des globs

| Champ | Condition d'inclusion |
|---|---|
| `components.*.path` | ≥ 1 fichier modifié du commit matche le glob |
| `semver.ignore_paths` | **Tous** les fichiers modifiés du commit matchent ≥ 1 pattern → commit exclu |

### Exclusion par commit

`ignore_commits` : liste de préfixes SHA. Un commit est exclu si son SHA complet commence par l'un des préfixes. Le format court (7 caractères, comme `git log --oneline`) est supporté.

```
strings.HasPrefix(commitFullSHA, entry)
```

### Structs Go

```go
type ComponentConfig struct {
    Path     string `yaml:"path"`      // glob, ex: "api/**"
    TagScope string `yaml:"tag_scope"` // optionnel, défaut = nom de la clé
}

type SemverConfig struct {
    TagPrefix           string                    `yaml:"tag_prefix"`
    Initial             string                    `yaml:"initial"`
    Branches            []BranchConfig            `yaml:"branches"`
    ConventionalCommits ConventionalCommitsConfig `yaml:"conventional_commits"`
    IgnorePaths         []string                  `yaml:"ignore_paths"`
    IgnoreCommits       []string                  `yaml:"ignore_commits"`
}

type Config struct {
    Semver     SemverConfig               `yaml:"semver"`
    Components map[string]ComponentConfig `yaml:"components"`
}
```

---

## Section 2 : Interface CLI

### Comportement selon la config

**Sans composants définis** → comportement identique à aujourd'hui, sortie inchangée.

**Avec composants définis** → `@root` apparaît automatiquement en premier, suivi des composants :

```bash
gg-version current
# @root   1.5.0
# api     1.3.0
# web     2.0.0

gg-version last
# @root   1.4.0
# api     1.2.3
# web     1.9.0
```

### Flag `--component`

Filtre la sortie à un seul composant. Avec `--format plain`, retourne la valeur seule (compatible scripts CI) :

```bash
gg-version current --component api
# api     1.3.0

gg-version current --component api --format plain
# 1.3.0
```

### Flag `--root`

Supprime les composants et n'affiche que `@root`. La sortie est identique au comportement sans composants (rétrocompatible) :

```bash
gg-version current --root
# 1.5.0

gg-version last --root
# 1.4.0
```

`--root` et `--component` sont mutuellement exclusifs.

### Commande `components`

Liste les composants définis dans la config (`@root` non listé, il est implicite) :

```bash
gg-version components
# api     path=api/**               tag=api/v*
# web     path=web/**               tag=web/v*
# shared  path={api,web}/shared/**  tag=shared/v*

gg-version components --format json
```

```json
{
  "api":    { "path": "api/**",               "tag_scope": "api",    "tag_pattern": "api/v*" },
  "web":    { "path": "web/**",               "tag_scope": "web",    "tag_pattern": "web/v*" },
  "shared": { "path": "{api,web}/shared/**",  "tag_scope": "shared", "tag_pattern": "shared/v*" }
}
```

---

## Section 3 : Architecture

### Fichiers modifiés

| Fichier | Changement |
|---|---|
| `config/config.go` | `ComponentConfig`, `IgnorePaths`, `IgnoreCommits` dans `SemverConfig`, `Config.Components` |
| `config/config_test.go` | Tests des nouvelles structs |
| `git/git.go` | Nouvelle méthode `CommitFiles` sur `Project` |
| `git/git_test.go` | Tests de `CommitFiles` |
| `strategy/semver/conventional.go` | `FilterCommits` — filtrage par chemin, ignore_paths, ignore_commits |
| `strategy/semver/conventional_test.go` | Tests de `FilterCommits` |
| `strategy/semver/semver.go` | Gestion multi-composants, calcul `@root` |
| `strategy/semver/semver_test.go` | Tests multi-composants |
| `command/commands.go` | Flags `--component`, `--root`, commande `components`, sortie multi-lignes |

### Couche git — `CommitFiles`

Extension de l'interface `GitProject` :

```go
// CommitFiles returns the list of files changed in the given commit
// relative to its first parent. For the initial commit, returns all files.
CommitFiles(c *object.Commit) ([]string, error)
```

Implémentation via `commit.Stats()` de `go-git`. Utilisée par `FilterCommits` pour évaluer les globs de chemin.

### Filtrage des commits — `FilterCommits`

Nouvelle fonction dans `strategy/semver/conventional.go` :

```go
type FilterConfig struct {
    IncludePaths  []string // glob — commit inclus si ≥1 fichier matche (nil = pas de filtre)
    ExcludePaths  []string // glob — commit exclu si tous fichiers matchent
    IgnoreCommits []string // préfixes SHA
}

func FilterCommits(
    commits []*object.Commit,
    files   func(*object.Commit) ([]string, error),
    cfg     FilterConfig,
) []*object.Commit
```

**Logique par commit (ordre d'évaluation) :**
1. SHA du commit commence par une entrée `IgnoreCommits` → exclu
2. `IncludePaths` défini ET aucun fichier modifié ne matche → exclu
3. Tous les fichiers modifiés matchent ≥ 1 pattern `ExcludePaths` → exclu
4. Sinon → inclus

### Calcul `@root` — exclusion automatique des composants

Quand des composants sont définis, `@root` reçoit dans `ExcludePaths` :
- `semver.ignore_paths`
- Les `path` globs de tous les composants

```
@root.ExcludePaths = ["*.md", "docs/**", "api/**", "web/**"]
@root.IncludePaths = []  // pas de filtre positif
```

Un commit touchant **uniquement** `api/handler.go` → exclu de `@root`.
Un commit touchant `api/handler.go` ET `go.mod` → inclus dans `@root` ET dans `api`.

### Sortie multi-composants

Les commandes `current`, `last`, `env` itèrent sur `[@root, ...components]` quand `Config.Components` est non vide, sauf si `--root` ou `--component` est spécifié.

---

## Section 4 : Tests

### `config/config_test.go`
- `TestLoadComponents` — charge un YAML avec `components`, vérifie `Path` et `TagScope`
- `TestLoadIgnorePaths` — vérifie `ignore_paths` et `ignore_commits`
- `TestDefaultConfig_noComponents` — `DefaultConfig()` retourne `Components` vide

### `git/git_test.go`
- `TestCommitFiles_singleFile` — commit touchant un fichier → `["foo.txt"]`
- `TestCommitFiles_multipleFiles` — commit touchant plusieurs fichiers
- `TestCommitFiles_initialCommit` — premier commit sans parent

### `strategy/semver/conventional_test.go`
- `TestFilterCommits_includePath` — commit `api/x.go` inclus, `web/y.go` exclu
- `TestFilterCommits_excludeAll` — commit dont tous les fichiers matchent `*.md` → exclu
- `TestFilterCommits_excludePartial` — commit `api/x.go` + `README.md` → inclus
- `TestFilterCommits_ignoreCommitSHA` — SHA complet → exclu
- `TestFilterCommits_ignoreCommitShortSHA` — SHA court 7 chars → exclu
- `TestFilterCommits_rootExcludesComponents` — commit uniquement `api/x.go` exclu de `@root`

### `strategy/semver/semver_test.go`
- `TestVars_withComponent` — composant `api`, tag `api/v1.2.3`, commits filtrés
- `TestCurrent_componentMinorBump` — `feat:` dans `api/` → `api/v1.3.0`
- `TestCurrent_rootExcludesComponentPaths` — commit `api/` only → pas de bump `@root`
- `TestCurrent_rootIncludesSharedCommit` — commit `go.mod` + `api/x.go` → bump `@root` ET `api`

### `command/commands_test.go`
- `TestComponentsCommand` — liste les composants définis
- `TestCurrentWithComponentFlag` — `--component api` filtre la sortie
- `TestCurrentWithRootFlag` — `--root` retourne version globale seule
- `TestCurrentMultiComponent` — sortie multi-lignes `@root`, `api`, `web`

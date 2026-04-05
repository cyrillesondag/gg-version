# Conventional Commits Version Calculation Design

**Date:** 2026-04-05

## Goal

Calculer la version courante (`current`) en analysant les commits entre le dernier tag et HEAD via la norme Conventional Commits, plutôt que d'exposer uniquement le dernier tag. `last` reste inchangé.

---

## Context

Ceci est la **feature B** d'un plan en deux parties :
- **A** (livré) : enrichissement des vars avec composants semver + date
- **B** (ce doc) : calcul de version par Conventional Commits

---

## Section 1 : Architecture et configuration

### Fichiers

| Fichier | Action |
|---------|--------|
| `config/config.go` | Ajouter `ConventionalCommitsConfig` dans `SemverConfig` |
| `strategy/semver/conventional.go` | **Nouveau** — parsing CC + calcul de bump |
| `strategy/semver/conventional_test.go` | **Nouveau** — tests unitaires du parsing |
| `strategy/semver/semver.go` | Mise à jour de `Vars()` et `Current()` |

### Configuration

```yaml
semver:
  conventional_commits:
    format: "^\\w+(?:\\(.+\\))?!?:"    # détecte un sujet au format CC
    major:
      - "^\\w+(?:\\(.+\\))?!:"         # feat!: ou feat(scope)!:
      - "BREAKING[- ]CHANGE:"          # footer token (sans ancre ^)
    minor:
      - "^feat(?:\\(.+\\))?:"
    patch:
      - "^fix(?:\\(.+\\))?:"
```

**Règles d'interprétation des patterns :**
- Patterns `major`/`minor`/`patch` : regex appliquées ligne par ligne sur le message de commit
- `format` : détecte si le sujet suit la syntaxe CC (`type[(scope)][!]:`)
- `major[0]` : pattern sujet — ancre `^` correcte (préfixe de ligne)
- `major[1]` : footer token — pas d'ancre `^` (peut apparaître n'importe où dans une ligne du body)
- `BREAKING CHANGE:` et `BREAKING-CHANGE:` sont tous deux reconnus

**Go structs :**

```go
type ConventionalCommitsConfig struct {
    Format string   `yaml:"format"`
    Major  []string `yaml:"major"`
    Minor  []string `yaml:"minor"`
    Patch  []string `yaml:"patch"`
}

// dans SemverConfig :
type SemverConfig struct {
    TagPrefix           string                    `yaml:"tag_prefix"`
    Initial             string                    `yaml:"initial"`
    Branches            []BranchConfig            `yaml:"branches"`
    ConventionalCommits ConventionalCommitsConfig `yaml:"conventional_commits"`
}
```

### Comportement de `Current()`

| Situation | Valeur retournée |
|-----------|-----------------|
| HEAD est taggé | tag (= `last`) |
| Pas de tag | `cfg.Initial` |
| Commits depuis le tag, branche release | version CC-calculée (`semver.Semver`) |
| Commits depuis le tag, branche non-release | `renderTemplate(format, vars)` |
| Commits tous non-bump (`chore:`, etc.) | `last` (pas d'incrément) |

---

## Section 2 : Logique de calcul — `strategy/semver/conventional.go`

### Fonctions

```go
// analyzeBump scanne les commits et retourne le niveau de bump :
// 0 = pas de bump, 1 = patch, 2 = minor, 3 = major
func analyzeBump(commits []*object.Commit, cfg config.ConventionalCommitsConfig) int

// bumpVersion applique le niveau de bump sur lastTag et retourne la nouvelle version
func bumpVersion(lastTag, prefix string, level int) string
```

### Logique de `analyzeBump` — pour chaque commit

1. Sépare le message en `subject` (1ère ligne) et `footer` (lignes après la première ligne vide)
2. Teste le **subject** :
   - Matche un pattern `major`/`minor`/`patch` → niveau correspondant
   - Matche `format` mais aucun pattern de bump → **aucune contribution** (type CC non mappé : `chore:`, `docs:`, type inconnu, etc.)
   - Ne matche **pas** `format` → **patch par défaut** (message non-CC)
3. Teste chaque ligne du **footer** contre tous les patterns (notamment `BREAKING[- ]CHANGE:` → major)
4. Retient le niveau le plus élevé pour ce commit

**Résultat global** = niveau le plus élevé parmi tous les commits.

| Niveau max | Comportement |
|------------|-------------|
| 0 (tous no-bump) | `semver.Semver == semver.LastVersion` — pas d'incrément |
| 1 (patch) | bump patch |
| 2 (minor) | bump minor, patch → 0 |
| 3 (major) | bump major, minor → 0, patch → 0 |

### `HasNonConventionalCommits`

`true` si au moins un commit a un sujet qui ne matche pas le pattern `format` (message libre, non-CC). Indique que le patch par défaut a été appliqué à cause d'un commit non-conforme.

### Mise à jour de `Vars()`

- Les commits récupérés pour `CommitCount` sont passés à `analyzeBump`
- `bumpVersion` calcule `semver.Semver`
- Si aucun commit (HEAD taggé ou pas de tag) → `semver.Semver == semver.LastVersion`
- Les composants de `Semver` (`Major`, `Minor`, `Patch`, `PreRelease`) sont extraits via `gosemver.NewVersion()`

### Mise à jour de `Current()`

- `Vars()` est appelé avant le check `branchCfg.Release`
- Branche release → retourne `vars["semver"]["Semver"]` au lieu de `lastTag`
- Branche non-release → le template a accès à `{{ .semver.Semver }}`

---

## Section 3 : Namespace des variables de template

### `git`

```
git.Branch        — nom court de la branche (sans refs/heads/)
git.Date          — date du jour UTC (YYYY-MM-DD)
git.LastTag       — valeur brute du dernier tag git (ex: v1.2.3-rc.1)
git.Hash          — hash complet du commit HEAD
git.ShortHash     — 7 premiers caractères du hash
git.CommitCount   — nombre de commits depuis le dernier tag
```

### `semver`

```
semver.Semver      — version CC-calculée, propre (ex: 1.3.0)
semver.Major       — composant major de Semver
semver.Minor       — composant minor de Semver
semver.Patch       — composant patch de Semver
semver.PreRelease  — composant pre-release de Semver (vide pour versions CC)

semver.LastVersion    — dernier tag parsé sans préfixe (ex: 1.2.3)
semver.LastMajor      — composant major de LastVersion
semver.LastMinor      — composant minor de LastVersion
semver.LastPatch      — composant patch de LastVersion
semver.LastPreRelease — composant pre-release de LastVersion

semver.IsBreakingChange          — true si bump = major détecté par CC
semver.IsPreRelease              — true si branche Release: false
semver.HasNonConventionalCommits — true si ≥1 commit non-CC (patch par défaut appliqué)
```

### `regex`

Captures nommées du pattern de branche (ex: `(?P<major>\d+)`).

### `var`

Paires `key=value` injectées via le flag `--var`.

---

## Section 4 : Tests

### `strategy/semver/conventional_test.go` (nouveau)

- `TestAnalyzeBump_feat` → minor
- `TestAnalyzeBump_fix` → patch
- `TestAnalyzeBump_breakingExclamation` → `feat!:` et `feat(scope)!:` → major
- `TestAnalyzeBump_breakingFooter` → `BREAKING CHANGE:` et `BREAKING-CHANGE:` dans le body → major
- `TestAnalyzeBump_noneType` → `chore:`, `docs:` → pas de bump (CC non mappé)
- `TestAnalyzeBump_nonCC` → message libre → patch par défaut
- `TestAnalyzeBump_allNone` → tous `chore:` → pas de bump
- `TestAnalyzeBump_mixed` → `fix:` + `feat:` → minor (le plus haut gagne)
- `TestAnalyzeBump_noCommits` → pas de commits → pas de bump

### `strategy/semver/semver_test.go`

- `TestCurrent_releaseWithFeat` → minor bump sur branche release
- `TestCurrent_releaseWithFix` → patch bump
- `TestCurrent_releaseWithBreaking` → major bump
- `TestCurrent_releaseAllNone` → `current == last` (pas de bump)
- `TestCurrent_preReleaseUsesNextVersion` → template utilise `semver.Semver`

### `config/config_test.go`

- `TestDefaultConfig_conventionalCommits` → patterns par défaut présents (`format`, `major`, `minor`, `patch`)

---

## Fichiers modifiés

| Fichier | Action |
|---------|--------|
| `config/config.go` | Ajout `ConventionalCommitsConfig` dans `SemverConfig` + defaults |
| `strategy/semver/conventional.go` | Nouveau — `analyzeBump`, `bumpVersion` |
| `strategy/semver/conventional_test.go` | Nouveau — 9 tests unitaires CC |
| `strategy/semver/semver.go` | Mise à jour `Vars()` + `Current()` |

### Breaking changes sur les templates existants

Le namespace `semver` est réorganisé. Les templates existants qui utilisent `{{ .semver.LastTag }}` doivent migrer vers `{{ .git.LastTag }}`. Les templates qui utilisaient `{{ .semver.CommitCount }}` ou `{{ .semver.ShortHash }}` migrent vers `{{ .git.CommitCount }}` et `{{ .git.ShortHash }}`.

Le format par défaut dans `config.go` doit être mis à jour :

```go
// avant
Format: "{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}"

// après
Format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
```

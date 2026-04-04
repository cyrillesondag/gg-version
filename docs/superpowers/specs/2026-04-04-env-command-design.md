# env Command & Variable Namespacing Design

**Date:** 2026-04-04

## Goal

Add an `env` command that displays all template variables available for version formatting, introduce namespaced variables (`semver.*`, `git.*`, `regex.*`, `var.*`), and allow custom variables via `--var name=value` on all commands.

---

## Section 1 : Architecture et namespacing des variables

### Variable namespaces

Toutes les variables de template sont organisées en 4 namespaces distincts :

| Namespace | Source | Exemples |
|-----------|--------|---------|
| `semver` | Calculs de version | `LastTag`, `CommitCount`, `ShortHash` |
| `git` | Métadonnées git | `Branch` |
| `regex` | Groupes nommés du pattern de branche | `major`, `minor`, tout groupe `(?P<name>...)` |
| `var` | Paramètres `--var name=value` | Clés arbitraires passées par l'utilisateur |

### Structure de données

Les variables forment un `map[string]interface{}` à deux niveaux :

```go
map[string]interface{}{
    "semver": map[string]interface{}{
        "LastTag":     "1.2.3",
        "CommitCount": 4,
        "ShortHash":   "abc1234",
    },
    "git": map[string]interface{}{
        "Branch": "feature/foo",
    },
    "regex": map[string]interface{}{
        "major": "1",  // si pattern contient (?P<major>\d+)
    },
    "var": map[string]interface{}{
        "env": "prod",  // passé via --var env=prod
    },
}
```

### Syntaxe de template

```
{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}+{{ .semver.ShortHash }}
```

### Méthode centrale

Une nouvelle méthode `Vars()` dans `Strategy` centralise la construction des variables :

```go
func (s Strategy) Vars(p GitProject, extra map[string]string) (map[string]interface{}, error)
```

La commande `env` et la méthode `Current()` appellent toutes les deux `Vars()`.

---

## Section 2 : Changements dans `strategy/semver`

### Méthode `Vars()`

```go
func (s Strategy) Vars(p GitProject, extra map[string]string) (map[string]interface{}, error)
```

Retourne le map à deux niveaux décrit en Section 1. `extra` alimente le namespace `var`.

### Refactoring de `Current()`

`Current()` est refactorisé pour appeler `Vars()` puis `renderTemplate()`. La logique de construction des variables ne vit plus qu'à un seul endroit.

### Breaking change sur les templates

Les noms de variables dans les configs `.gg-version.yaml` changent :

| Avant | Après |
|-------|-------|
| `{{ .LastTag }}` | `{{ .semver.LastTag }}` |
| `{{ .Branch }}` | `{{ .git.Branch }}` |
| `{{ .CommitCount }}` | `{{ .semver.CommitCount }}` |
| `{{ .ShortHash }}` | `{{ .semver.ShortHash }}` |
| `{{ .major }}` (capture) | `{{ .regex.major }}` |

Le `DefaultConfig` est mis à jour. Pas de compatibilité ascendante — version précoce du projet.

---

## Section 3 : Changements CLI

### Flag `--var` sur toutes les commandes

`current`, `last`, et `env` acceptent tous `--var name=value` (flag répétable) :

```bash
gg-version current --var env=prod
gg-version last --var env=prod
gg-version env --var env=prod --format json
```

Les valeurs sont parsées et passées comme `extra map[string]string` à `Vars()`.

### Nouvelle commande `env`

```bash
gg-version env [--format plain|json] [--var name=value]...
```

- **Pas de `--repo` ni `--config`** : toujours `.` + `DefaultConfig`
- Si `.` n'est pas un repo git, les variables `git.*` et `semver.*` qui dépendent du repo affichent des valeurs vides (`""` / `0`) — pas d'erreur fatale
- **`--format plain`** (défaut) : une ligne par variable, format `semver.LastTag=1.2.3`
- **`--format json`** : objet JSON à deux niveaux `{"semver": {"LastTag": "1.2.3"}, ...}`
- Affiche **toutes** les variables disponibles pour le templating

### Organisation des flags

- Flags globaux (`--repo`, `--config`) : niveau racine, uniquement sur `current` et `last`
- `--var` : flag local répété sur chaque sous-commande (`current`, `last`, `env`)
- `--format` : uniquement sur `env`

---

## Section 4 : Tests

### `strategy/semver/semver_test.go` — nouveaux tests

- `TestVars_semverNamespace` : `LastTag`, `CommitCount`, `ShortHash` présents dans `.semver`
- `TestVars_gitNamespace` : `Branch` présent dans `.git`
- `TestVars_regexNamespace` : branch pattern avec `(?P<major>\d+)` → `.regex.major`
- `TestVars_varNamespace` : `extra = map[string]string{"env": "prod"}` → `.var.env`
- `TestCurrent_withNestedTemplate` : template `{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}` produit le bon résultat
- `TestCurrent_breakingChange` : ancien template plat `{{ .LastTag }}` produit une chaîne vide (comportement documenté)

### Approche de test

Les tests `strategy/semver` restent en in-memory via `fakeProject`. Pas de dépendance disque.

---

## Fichiers modifiés

| Fichier | Action |
|---------|--------|
| `strategy/semver/semver.go` | Ajout `Vars()`, refactoring `Current()`, mise à jour `DefaultConfig` template |
| `strategy/semver/semver_test.go` | Nouveaux tests `Vars`, mise à jour templates existants |
| `command/commands.go` | Ajout commande `env`, flag `--var` sur toutes les commandes, flag `--format` sur `env` |
| `config/config.go` | Mise à jour `DefaultConfig` avec nouveaux noms de variables |

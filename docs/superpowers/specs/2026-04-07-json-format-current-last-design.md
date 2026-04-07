# `--format json` sur `current` et `last` — Design Spec

**Goal:** Ajouter un flag `--format plain|json` aux commandes `current` et `last`, cohérent avec le pattern existant de `env` et `components`.

**Architecture:** Un seul fichier modifié — `cmd/gg-version/commands.go`. Ajout du flag sur les deux commandes, passage du format à `printComponentResults`.

**Tech Stack:** Go 1.24, urfave/cli/v3, encoding/json (déjà importé).

---

## Problème

`current` et `last` sont les commandes les plus utilisées en CI mais leur sortie n'est pas structurée. En mode monorepo, la sortie tabulaire (`%-12s`) n'est pas parseable proprement sans `awk`. Aucun moyen d'intégrer proprement le résultat dans un pipeline JSON.

---

## Design

### Flag ajouté

Identique au pattern de `env` et `components` :

```go
&cli.StringFlag{
    Name:  "format",
    Value: "plain",
    Usage: "output format: plain or json",
},
```

Ajouté dans la liste `Flags` de `currentCmd` et `lastCmd`.

### Modification de `printComponentResults`

Signature avant :
```go
func printComponentResults(results []semverstrategy.ComponentResult) error
```

Signature après :
```go
func printComponentResults(results []semverstrategy.ComponentResult, format string) error
```

Les deux appels existants passent `cmd.String("format")`.

### Comportement JSON

| Cas | Sortie |
|---|---|
| Repo simple (1 résultat sans nom) | `"v1.4.2"` — chaîne JSON brute |
| Filtré par `--component` ou `--root` | `"v0.5.1"` — même logique (1 résultat filtré) |
| Monorepo sans filtre | `{"@root":"v2.1.0","api":"v0.5.1","frontend":"v3.0.0"}` |

Implémentation :
- Cas chaîne : `json.Marshal(version)` → `"v1.4.2"`
- Cas objet : construire `map[string]string`, puis `json.MarshalIndent(m, "", "  ")`
- Format invalide : `return fmt.Errorf("unknown format %q: must be plain or json", format)`

### Comportement `plain` inchangé

Aucune modification du chemin `plain`. Régression impossible.

---

## Fichiers

| Fichier | Modification |
|---|---|
| `cmd/gg-version/commands.go` | Flag `--format` sur `currentCmd` et `lastCmd` ; `printComponentResults` avec paramètre `format` |

---

## Exemples

```bash
# Repo simple
gg-version current --format json
# "v1.4.2"

gg-version last --format json
# "v1.4.1"

# Monorepo
gg-version current --format json
# {
#   "@root": "v2.1.0",
#   "api": "v0.5.1",
#   "frontend": "v3.0.0"
# }

# Filtré
gg-version --component api current --format json
# "v0.5.1"

gg-version --root last --format json
# "v2.0.0"
```

---

## Breaking changes

Aucun. Le flag `--format` n'existait pas sur ces commandes. La valeur par défaut `plain` préserve le comportement existant.

---

## Tests

Pas de fichier de tests pour `cmd/gg-version` (hors scope de cette tâche). Vérification manuelle avec `go run ./cmd/gg-version current --format json` dans le dépôt local.

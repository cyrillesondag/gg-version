# Suppression des variables globales de flags — Design Spec

**Date:** 2026-04-10

## Problème

`cmd/gg-version/commands.go` expose 4 variables globales mutées par urfave/cli lors du parsing des flags :

```go
var (
    configPath    string
    repoPath      string
    componentFlag string
    rootFlag      bool
)
```

Ces globales rendent le code non-réentrant et les fonctions de commande impossibles à tester sans invoquer une vraie CLI. `filterComponentResults`, `filterVarsResults` et `printComponentResults` lisent également ces globales directement.

---

## Architecture

Un type `globalFlags` regroupe les 4 valeurs, plus les `--var` parsés :

```go
type globalFlags struct {
    Config    string
    Repo      string
    Component string
    Root      bool
    Vars      map[string]string // parsed from --var name=value
}
```

Un type opaque `contextKey struct{}` sert de clé de contexte (évite les collisions). Un `Before` hook sur le root command peuple la struct et l'injecte dans le contexte :

```go
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
```

Une fonction `flagsFromCtx(ctx context.Context) globalFlags` expose la lecture avec une valeur par défaut sûre si le contexte ne contient pas la clé.

---

## Changements par fonction

| Fonction | Changement |
|---|---|
| `Run()` | Suppression des 4 globales + `Destination`. Ajout du `Before` hook. |
| `currentCmd` | `flags := flagsFromCtx(ctx)` remplace les lectures de globales |
| `nextCmd` | idem |
| `lastCmd` | idem |
| `envCmd` | idem |
| `configCmd` | `flags.Config` remplace `configPath` |
| `componentsCmd` | `flags.Config` remplace `configPath` |
| `tagCmd` | `flags := flagsFromCtx(ctx)` |
| `lintCmd` | `flags := flagsFromCtx(ctx)` |
| `filterComponentResults(results, flags)` | Signature étendue — reçoit `globalFlags` |
| `filterVarsResults(results, flags)` | Signature étendue — reçoit `globalFlags` |
| `printComponentResults(results, format, flags)` | Signature étendue — reçoit `globalFlags` |
| `lintFilterConfig` | Signature inchangée |

Les `Destination` sur les 4 flags globaux sont supprimées. `parseVarFlags` est appelé une seule fois dans le `Before` hook.

---

## Testabilité

Après ce refactoring, les fonctions de commande peuvent être testées directement :

```go
ctx := context.WithValue(context.Background(), contextKey{}, globalFlags{
    Config: "testdata/.gg-version.yml",
    Repo:   tmpRepoPath,
})
err := currentCmd(ctx, &cli.Command{
    Flags: []cli.Flag{&cli.StringFlag{Name: "format", Value: "plain"}},
})
```

Aucune invocation CLI réelle n'est nécessaire. L'infrastructure de test pour #9 (tests sur cmd/gg-version) est ainsi prête.

---

## Tests de cette tâche

Ce refactoring est une réécriture interne sans changement de comportement observable.

**Critères d'acceptation :**
- Les 4 globales `configPath`, `repoPath`, `componentFlag`, `rootFlag` sont supprimées
- `globalFlags` et `flagsFromCtx` sont définis dans `commands.go`
- `filterComponentResults`, `filterVarsResults`, `printComponentResults` acceptent `globalFlags` en paramètre
- `go build ./cmd/gg-version` passe
- `go test ./...` passe

---

## Fichiers

- **Modify:** `cmd/gg-version/commands.go` — unique fichier modifié

# Strategy Interface — Design Spec

**Date:** 2026-04-10

## Problème

`Strategy` est une struct concrète exportée. `GitProject` est une interface mockable. L'asymétrie est incohérente : on ne peut pas substituer `Strategy` dans les tests ni fournir d'implémentation alternative sans modifier les call sites.

## Objectif

Rendre `Strategy` une interface publique dans `strategy/semver`. La struct existante devient `semverStrategy` (unexported). `NewStrategy` retourne l'interface. Aucun consommateur n'est impacté.

---

## Architecture

### Interface `Strategy` — `strategy/semver/semver.go`

Définie juste avant la struct actuelle :

```go
// Strategy computes semver versions from the git history.
// *semverStrategy satisfies this interface.
type Strategy interface {
    Current(p GitProject, extra map[string]string) (string, error)
    Last(p GitProject) (string, error)
    Vars(p GitProject, extra map[string]string) (map[string]interface{}, error)
    AllCurrent(p GitProject, extra map[string]string, cfg config.Config) ([]ComponentResult, error)
    AllLast(p GitProject, cfg config.Config) ([]ComponentResult, error)
    AllVars(p GitProject, extra map[string]string, cfg config.Config) ([]ComponentVarsResult, error)
}
```

### Struct `semverStrategy` (unexported)

La struct `Strategy` est renommée `semverStrategy`. Tous les receivers dans `semver.go` et `component.go` sont mis à jour. Les méthodes privées (`varsCore`, `varsCoreFromHistory`, `matchBranch`, `currentFromVars`) suivent le même renommage de receiver.

```go
type semverStrategy struct {
    cfg config.SemverConfig
}

// compile-time check
var _ Strategy = semverStrategy{}
```

### `NewStrategy`

Retourne l'interface :

```go
func NewStrategy(cfg config.SemverConfig) Strategy {
    return semverStrategy{cfg: cfg}
}
```

---

## Impact sur les fichiers

| Fichier | Changement |
|---|---|
| `strategy/semver/semver.go` | Ajout interface `Strategy` ; struct → `semverStrategy` ; receivers mis à jour ; garde de compilation |
| `strategy/semver/component.go` | Receivers `(s Strategy)` → `(s semverStrategy)` |
| `cmd/gg-version/commands.go` | **Aucun** — `NewStrategy` retourne toujours `semverstrategy.Strategy` |
| `strategy/semver/semver_test.go` | **Aucun** — `NewStrategy` retourne l'interface, appels inchangés |
| `strategy/semver/component_test.go` | **Aucun** |

---

## Tests

Aucun test nouveau. Les tests existants valident toutes les méthodes publiques. `go test ./...` doit passer sans modification de test.

---

## Fichiers

- **Modify:** `strategy/semver/semver.go` — interface + renommage struct + receivers + garde
- **Modify:** `strategy/semver/component.go` — receivers uniquement

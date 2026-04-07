# Shallow Clones — Design Spec

**Goal:** Détecter les shallow clones et signaler explicitement quand l'historique est tronqué, au lieu de retourner silencieusement une version sous-estimée.

**Architecture:** Deux modifications composées — détection shallow dans `git.Project` (via `IsShallow()`) et détection de troncature dans `CommitSinceTag` (via un bool `truncated` retourné). La couche git ne touche pas stderr ; la CLI décide d'afficher l'avertissement.

**Tech Stack:** Go 1.24, go-git/v5 (`Storer.Shallow()`, `Storer.SetShallow()`).

---

## Problème

Quasiment toutes les pipelines CI modernes utilisent `git clone --depth=N`. Si le tag de référence est au-delà de la profondeur du clone, `CommitSinceTag` retourne une liste tronquée sans erreur ni avertissement — ce qui peut produire `0.1.0` alors que le vrai dernier tag est `v3.2.1`. Le problème est entièrement silencieux.

---

## Design

### 1. `IsShallow() bool` sur `git.Project`

```go
// git/git.go
func (p Project) IsShallow() bool {
    hashes, err := p.repo.Storer.Shallow()
    return err == nil && len(hashes) > 0
}
```

`Storer.Shallow()` retourne la liste des commits graftés (ceux dont les parents ont été artificiellement coupés). Non vide = shallow clone.

Ajouté à l'interface `GitProject` dans `strategy/semver/semver.go` :
```go
IsShallow() bool
```

Exposé dans `varsCore` comme variable de template :
```go
"git": map[string]interface{}{
    // ... variables existantes ...
    "IsShallow": p.IsShallow(),
}
```

### 2. `CommitSinceTag` — détection de troncature

Signature avant :
```go
CommitSinceTag(tag string) ([]*object.Commit, error)
```

Signature après :
```go
CommitSinceTag(tag string) ([]*object.Commit, bool, error)
```

Le bool `truncated` est `true` quand l'itérateur épuise l'historique sans avoir trouvé le commit du tag.

Implémentation :
```go
found := false
err = iter.ForEach(func(c *object.Commit) error {
    history = append(history, c)
    if c.Hash == ancestor.Hash {
        found = true
        return storer.ErrStop
    }
    return nil
})
return history, !found, err
```

L'interface `GitProject` est mise à jour en conséquence.

### 3. Avertissement stderr dans la CLI

Dans `commands.go`, après chaque appel qui consomme `CommitSinceTag` (via `varsCore`), si `truncated` est détecté, écrire sur stderr :

```
warning: shallow clone — history is truncated, computed version may be underestimated
```

L'avertissement est émis une seule fois par invocation, même en mode monorepo avec plusieurs composants. La commande continue et retourne son résultat avec code de sortie `0`.

**Important :** `truncated` remonte de `varsCore` jusqu'à `commands.go`. Cela implique que `varsCore` retourne un résultat enrichi ou un bool supplémentaire. Voir section Propagation ci-dessous.

### 4. Propagation de `truncated` jusqu'à la CLI

`varsCore` retourne actuellement `(map[string]interface{}, error)`. Pour propager `truncated`, deux options :

**Option retenue — champ dans le résultat :** ajouter `git.Truncated` (bool) comme variable de template supplémentaire dans le map retourné. La CLI lit `result["git"].(map[string]interface{})["Truncated"]` pour décider d'avertir.

Avantage : pas de changement de signature de `varsCore`. La valeur est aussi disponible dans les templates via `{{ .git.Truncated }}`.

---

## Fichiers

| Fichier | Modification |
|---|---|
| `git/git.go` | Ajouter `IsShallow() bool` ; modifier `CommitSinceTag` → `(..., bool, error)` |
| `git/git_test.go` | `TestIsShallow`, `TestCommitSinceTag_truncated` |
| `strategy/semver/semver.go` | `GitProject` interface : `IsShallow() bool`, `CommitSinceTag` mise à jour ; `varsCore` : `git.IsShallow`, `git.Truncated` |
| `strategy/semver/semver_test.go` | `fakeProject.IsShallow()` stub ; `fakeProject.CommitSinceTag` mise à jour |
| `strategy/semver/component.go` | Mise à jour des appels à `CommitSinceTag` |
| `cmd/gg-version/commands.go` | Avertissement stderr si `git.Truncated` est `true` |
| `docs/reference.md` | Ajouter `git.IsShallow` et `git.Truncated` au tableau des variables |

---

## Variables de template ajoutées

### Namespace `git`

| Variable | Type | Description |
|---|---|---|
| `git.IsShallow` | bool | `true` si le dépôt est un clone superficiel |
| `git.Truncated` | bool | `true` si l'historique a été tronqué avant d'atteindre le tag de référence |

---

## Tests

### `git/git_test.go`

**`TestIsShallow`**
- Repo normal (pas de graft) → `IsShallow()` retourne `false`
- Repo avec `Storer.SetShallow([]plumbing.Hash{hash})` → `IsShallow()` retourne `true`

**`TestCommitSinceTag_truncated`**
- Historique C0 → C1 → C2, tag sur C0
- Simuler shallow : `Storer.SetShallow([]plumbing.Hash{C1.Hash})` (C1 devient la coupure)
- `CommitSinceTag("v1.0.0")` depuis C2 → `truncated = true`, liste = [C2, C1]
- Même historique sans shallow → `truncated = false`, liste = [C2, C1, C0]

### `strategy/semver/semver_test.go`

- `fakeProject.IsShallow()` retourne `false` par défaut
- `fakeProject.CommitSinceTag` retourne `(commits, false, nil)` par défaut
- Test `TestVars_shallow` : `fakeProject` avec `IsShallow() = true` → `git.IsShallow = true` dans le map

---

## Comportement en monorepo

`AllCurrent`, `AllLast`, `AllVars` appellent `varsCore` pour chaque composant. L'avertissement stderr est émis une seule fois si au moins un composant a `git.Truncated = true`. La variable `git.Truncated` est exposée par composant.

---

## Breaking changes

| Changement | Impact |
|---|---|
| `CommitSinceTag` retourne 3 valeurs | Interne seulement — `GitProject` interface, `fakeProject` dans les tests |
| `git.IsShallow` et `git.Truncated` ajoutés | Additif — aucun template existant cassé |

Aucun breaking change utilisateur.

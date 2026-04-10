# Factorisation de l'historique git — Design Spec

**Date:** 2026-04-10

## Problème

En mode monorepo, `AllCurrent`, `AllLast` et `AllVars` appellent `varsCore` (ou `lastWithPrefix`) une fois par composant plus une fois pour `@root`. Chaque appel déclenche indépendamment :

- `p.LastTag(f)` — itère sur **tous les tags** du repo et, pour chaque tag valide, remonte l'historique entier pour vérifier l'ancêtre (`isAncestor`) : complexité O(K × N) avec K tags et N commits.
- `p.CommitSinceTag(tag)` — remonte les commits de HEAD jusqu'au commit tagué : O(N).

Sur un repo avec 5 composants : 6 appels à `LastTag` + 6 appels à `CommitSinceTag` = 12 traversées indépendantes du même graphe de commits.

C'est une optimisation préventive — il n'y a pas de plainte aujourd'hui — mais l'algorithme est clairement sous-optimal.

---

## Architecture

**Approche retenue : traversée unique, filtrage en mémoire.**

Une seule traversée git construit un snapshot `sharedHistory` de tout l'historique depuis HEAD, chaque commit étant annoté avec les tags qui pointent sur lui. Les N composants font ensuite leurs opérations (trouver le dernier tag, trancher la liste, filtrer par chemin) en mémoire sur ce snapshot.

Le chemin **non-monorepo** (`len(cfg.Components) == 0`) reste inchangé — il n'a pas de problème N+1 et ne nécessite aucune modification.

**Complexité après optimisation :** O(K + N) pour la traversée initiale + O(N) par composant pour le filtrage en mémoire, au lieu de O(K × N) × (N_composants + 1).

---

## Couche git : `CommitHistory()`

### Nouveau type

```go
// CommitWithTags groups a commit with the tag names that point directly to it.
// Tags is empty for most commits.
type CommitWithTags struct {
    Commit *object.Commit
    Tags   []string
}
```

### Nouvelle méthode

```go
// CommitHistory returns all commits reachable from HEAD in topological order
// (HEAD first, ancestors later), each annotated with the tag names pointing to it.
// Annotated tags are resolved to their target commit before matching.
func (p Project) CommitHistory() ([]CommitWithTags, error)
```

**Implémentation en deux passes :**

1. `repo.Tags()` → pour chaque tag, résoudre via `getCommitFromTag` (existant) → construire `map[plumbing.Hash][]string` (hash du commit → noms des tags)
2. `object.NewCommitPreorderIter(p.head, nil, nil)` → pour chaque commit, annoter avec les tags du map → retourner `[]CommitWithTags`

### Ajout dans l'interface `GitProject`

```go
CommitHistory() ([]CommitWithTags, error)
```

Le type `CommitWithTags` est défini dans le package `git` et importé dans `strategy/semver`. Ce cross-package import est déjà établi : `strategy/semver` importe `github.com/go-git/go-git/v5/plumbing/object` pour `*object.Commit`. L'ajout de `gover/git` pour `git.CommitWithTags` suit le même pattern.

### Stub dans `fakeProject`

```go
type fakeProject struct {
    // ... champs existants ...
    commitHistory []git.CommitWithTags
}

func (f fakeProject) CommitHistory() ([]git.CommitWithTags, error) {
    return f.commitHistory, nil
}
```

---

## Couche strategy : `sharedHistory`

Défini dans `strategy/semver/component.go` (pas de nouveau fichier).

```go
type sharedHistory struct {
    items       []git.CommitWithTags
    indexByHash map[plumbing.Hash]int // commit hash → position dans items
}

func buildSharedHistory(p GitProject) (*sharedHistory, error) {
    items, err := p.CommitHistory()
    if err != nil {
        return nil, err
    }
    idx := make(map[plumbing.Hash]int, len(items))
    for i, item := range items {
        idx[item.Commit.Hash] = i
    }
    return &sharedHistory{items: items, indexByHash: idx}, nil
}
```

### `findLastTag`

Parcours linéaire des items (HEAD en premier). Premier tag valide trouvé = tag le plus proche. Tiebreaker `Compare` si plusieurs tags sur le même commit (identique au comportement actuel de `LastTag`).

```go
// findLastTag returns the nearest ancestor tag valid for f, and its index in items.
// Returns ("0.0.0", -1) when no valid tag is found.
func (h *sharedHistory) findLastTag(f format.VersionFormat) (string, int)
```

### `commitsSince`

```go
// commitsSince returns commits strictly before the tagged commit (items[:tagIdx]).
// truncated is true when tagIdx == -1 (shallow clone: tag not in accessible history).
func (h *sharedHistory) commitsSince(tagIdx int) (commits []*object.Commit, truncated bool)
```

Quand `tagIdx == -1` : retourne tous les items accessibles avec `truncated = true` — comportement identique à l'actuel `CommitSinceTag` en shallow clone.

---

## `varsCoreFromHistory`

Variante interne de `varsCore` qui accepte `*sharedHistory` et `tagIdx int` au lieu d'appeler `p.LastTag` et `p.CommitSinceTag`. Toute la logique CC/bump/template/namespaces reste identique.

```go
func (s Strategy) varsCoreFromHistory(
    p GitProject,
    extra map[string]string,
    hist *sharedHistory,
    tagPrefix string,
    filterCfg FilterConfig,
) (map[string]interface{}, error)
```

Étapes internes :
1. `hist.findLastTag(NewSemverFormat(tagPrefix, constraints))` → `(lastTag, tagIdx)`
2. `hist.commitsSince(tagIdx)` → `(rawCommits, truncated)`
3. `FilterCommits(rawCommits, p.CommitFiles, filterCfg)` → commits filtrés
4. Suite identique à `varsCore` : `AnalyzeBump`, `BumpVersion`, construction des namespaces

---

## Refactoring de `AllCurrent`, `AllLast`, `AllVars`

Les trois méthodes suivent le même pattern :

```
si len(cfg.Components) == 0 → chemin existant inchangé
sinon →
    hist := buildSharedHistory(p)      // 1 traversée
    pour chaque composant (+ @root) →
        varsCoreFromHistory(hist, ...)  // en mémoire
```

`varsCore` reste inchangé — appelé uniquement par le chemin non-monorepo (`Vars`, `Current`, `Last` simples).

### `AllLast`

`AllLast` n'a pas besoin de `varsCoreFromHistory` complet. Il utilise directement `hist.findLastTag(f)` + résolution de `cfg.Initial`, identique à `lastWithPrefix` mais sur le snapshot.

---

## Tests

### `git/git_test.go` — `TestCommitHistory`

Repo in-memory, 4 commits, 2 tags (1 léger, 1 annoté). Vérifications :
- Longueur de la slice = 4
- Commits dans l'ordre topologique (HEAD en premier)
- Tag léger annoté sur le bon commit
- Tag annoté résolu et annoté sur le bon commit
- Commits sans tag : `Tags == nil` ou slice vide

### `strategy/semver/semver_test.go`

Stub `fakeProject.CommitHistory` retournant une slice prédéfinie. Les tests existants restent valides (non-monorepo ne touche pas `CommitHistory`).

### `strategy/semver/component_test.go` (tests existants)

Les tests existants sur `AllCurrent` passent sans modification — comportement identique. Ajout de `TestAllCurrent_singleTraversal` : un `fakeProject` qui compte les appels à `CommitHistory` vérifie qu'avec 2 composants, `CommitHistory` est appelé exactement 1 fois.

---

## Fichiers

| Fichier | Changement |
|---|---|
| `git/git.go` | Ajouter `CommitWithTags` + `CommitHistory()` |
| `git/git_test.go` | Ajouter `TestCommitHistory` |
| `strategy/semver/semver.go` | Ajouter `CommitHistory()` dans `GitProject` |
| `strategy/semver/semver_test.go` | Stub `CommitHistory` dans `fakeProject` |
| `strategy/semver/component.go` | Ajouter `sharedHistory`, `buildSharedHistory`, `varsCoreFromHistory` ; refactorer `AllCurrent`/`AllLast`/`AllVars` |

Aucun nouveau fichier. Aucun changement à `cmd/gg-version/commands.go` ni à `config/`.

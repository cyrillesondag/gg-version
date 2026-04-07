# `current` / `next` — Séparation sémantique — Design Spec

**Goal:** Clarifier la sémantique de `current` : retourner uniquement le tag existant quand HEAD est tagué (chaîne vide sinon), et ajouter `next` qui reprend le comportement actuel de calcul de version.

**Architecture:** Ajouter `Tagged bool` à `ComponentResult` dans la couche strategy. `AllCurrent` positionne ce champ. `currentCmd` filtre l'affichage selon `Tagged`. `nextCmd` est une copie de l'actuel `currentCmd`, inchangé.

**Tech Stack:** Go 1.24, urfave/cli/v3, go-git/v5.

---

## Problème

Sur un commit non tagué sur `main`, `gg-version current` retourne `v1.3.0` — une version calculée qui n'existe pas encore comme tag. C'est en réalité la version *suivante*. La sémantique de "version courante" suggère le tag qui existe déjà dans le dépôt.

---

## Design

### 1. `Tagged bool` dans `ComponentResult`

```go
// strategy/semver/semver.go
type ComponentResult struct {
    Name    string
    Version string
    Tagged  bool // true si HEAD est exactement sur ce tag
}
```

### 2. `AllCurrent` positionne `Tagged`

Dans `strategy/semver/component.go`, après avoir calculé la version pour chaque composant, `AllCurrent` vérifie si HEAD est tagué via `IsHeadTagged`. Si `true` → `Tagged = true`.

Le champ `Version` est toujours la version calculée — c'est la CLI qui décide quoi afficher selon `Tagged`.

### 3. `current` — comportement modifié

- HEAD tagué → affiche le tag (comme aujourd'hui)
- HEAD non tagué → affiche `""` (chaîne vide), exit `0`
- En monorepo sans filtre : affiche tous les composants, version vide pour les non-tagués
- `--format json` : `{"@root": "v2.1.0", "api": ""}` — tous les composants inclus

### 4. `next` — nouveau command

Comportement identique à l'actuel `current` : calcule la version même si HEAD non tagué. Supporte `--format json`. Implémenté en réutilisant exactement la même logique que `currentCmd` avant modification, en appelant `strategy.AllCurrent` et en affichant `r.Version` (sans filtrage par `Tagged`).

---

## Fichiers

| Fichier | Modification |
|---|---|
| `strategy/semver/semver.go` | Ajouter `Tagged bool` à `ComponentResult` |
| `strategy/semver/component.go` | `AllCurrent` positionne `Tagged` via `IsHeadTagged` |
| `strategy/semver/semver_test.go` | Mettre à jour les références à `ComponentResult`, ajouter `TestAllCurrent_tagged` |
| `cmd/gg-version/commands.go` | `currentCmd` filtre par `Tagged` ; ajouter `nextCmd` |
| `docs/reference.md` | Documenter `next`, mettre à jour `current` |

---

## Comportement détaillé

### `current`

| État HEAD | Sortie plain | Sortie JSON |
|---|---|---|
| Tagué `v1.4.2` | `v1.4.2` | `"v1.4.2"` |
| Non tagué | `` (vide) | `""` |
| Monorepo, @root tagué, api non tagué | `@root v2.1.0` / `api ` | `{"@root":"v2.1.0","api":""}` |

### `next`

Identique à l'actuel `current` — retourne toujours la version calculée, qu'elle existe comme tag ou non.

---

## Tests

### `strategy/semver/semver_test.go`

- Mettre à jour tous les `ComponentResult{Name: ..., Version: ...}` en `ComponentResult{Name: ..., Version: ..., Tagged: ...}`
- Ajouter `TestAllCurrent_tagged` :
  - HEAD sur tag → `Tagged = true`, `Version = lastTag`
  - HEAD non tagué → `Tagged = false`, `Version = nextVersion`

---

## Breaking changes

| Changement | Impact |
|---|---|
| `current` retourne `""` si HEAD non tagué | **Breaking** — les pipelines CI utilisant `current` pour la version calculée doivent migrer vers `next` |
| `ComponentResult` : `Tagged bool` ajouté | Interne seulement |
| `next` command ajoutée | Additif |

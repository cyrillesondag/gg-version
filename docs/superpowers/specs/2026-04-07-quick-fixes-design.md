# Quick Fixes — Design Spec

**Goal:** Corriger cinq problèmes indépendants à fort impact identifiés dans IMPROVEMENTS.md : restructuration du point d'entrée binaire, `--var` global, dates de commit reproductibles, tiebreaker déterministe dans `LastTag`, et flag `--version`.

**Architecture:** Un plan séquentiel unique. La restructuration `cmd/gg-version` est faite en premier car elle touche le point d'entrée que les tâches suivantes modifient également. Les tâches 3 à 5 sont indépendantes entre elles une fois le point d'entrée stabilisé.

**Tech Stack:** Go 1.24, urfave/cli/v3, go-git/v5, go-semver.

---

## Fix 1 — Restructuration `cmd/gg-version/`

### Problème

`go install` produit un binaire nommé `gover` (dernier segment du module path) au lieu de `gg-version`. `types.go` à la racine contient du dead code (`type Version string`, `type Commit struct`) jamais référencé.

### Design

Créer `cmd/gg-version/` et y déplacer le contenu de `command/` :

```
cmd/
  gg-version/
    main.go        ← ancien main.go (point d'entrée, injection ldflags)
    commands.go    ← ancien command/commands.go (logique CLI complète)
```

- Le répertoire `command/` est supprimé.
- `types.go` est supprimé (dead code).
- `format/calver.go` est supprimé (struct vide non utilisée).
- Le module reste `gover` dans `go.mod` — les chemins d'import internes (`gover/config`, `gover/git`, etc.) sont inchangés.
- `cmd/gg-version/commands.go` déclare `package main` (pas `package command`). L'appel dans `main.go` devient un appel direct à `Run()` dans le même package.

Un `Makefile` minimal est ajouté à la racine :

```makefile
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build
build:
	go build -ldflags "-X main.Version=$(VERSION)" -o gg-version ./cmd/gg-version
```

### Fichiers

- Créer : `cmd/gg-version/main.go`
- Créer : `cmd/gg-version/commands.go` (contenu de `command/commands.go`)
- Créer : `Makefile`
- Supprimer : `command/commands.go`
- Supprimer : `main.go` (racine)
- Supprimer : `types.go`
- Supprimer : `format/calver.go`

### Acceptance criteria

- `go build ./cmd/gg-version` produit un binaire fonctionnel
- `go test ./...` passe (tous les imports internes résolus)
- Le répertoire `command/` n'existe plus
- `types.go` et `format/calver.go` n'existent plus

---

## Fix 2 — `--var` en flag global

### Problème

`--var` est déclaré localement sur `current` et `env`. `gg-version --var foo=bar current` échoue. L'asymétrie avec `--component` et `--root` (flags globaux) est surprenante.

### Design

Déplacer la déclaration `--var` dans la liste des flags globaux du `cli.Command` racine. Supprimer les deux déclarations locales sur `current` et `env`.

Dans chaque action, remplacer `cmd.StringSlice("var")` par `cmd.Root().StringSlice("var")`.

```go
// flags globaux — cmd/gg-version/commands.go
&cli.StringSliceFlag{
    Name:  "var",
    Usage: "extra template variable as name=value (repeatable)",
},
```

**Breaking change :** `gg-version current --var foo=bar` ne fonctionne plus. Seul `gg-version --var foo=bar current` est valide. Documenté dans la doc utilisateur.

### Fichiers

- Modifier : `cmd/gg-version/commands.go`
- Modifier : `docs/reference.md` (section flags globaux)
- Modifier : `docs/how-to.md` (exemples `--var`)

### Acceptance criteria

- `gg-version --var env=prod current` fonctionne
- `gg-version current --var env=prod` retourne une erreur (flag inconnu)
- `gg-version --var foo=bar env` fonctionne
- `cmd.Root().StringSlice("var")` utilisé dans toutes les actions

---

## Fix 3 — `git.AuthorDate` et `git.CommitterDate`

### Problème

`git.Date` retourne `time.Now()` — la date du build, pas du commit. Deux builds sur le même commit à deux jours d'intervalle produisent des versions différentes. Non reproductible.

### Design

Supprimer `git.Date`. Ajouter deux nouvelles variables dans le namespace `git` :

| Variable | Source |
|---|---|
| `git.AuthorDate` | `commit.Author.When.UTC().Format("2006-01-02")` |
| `git.CommitterDate` | `commit.Committer.When.UTC().Format("2006-01-02")` |

Ajouter `CommitDate() (authorDate, committerDate time.Time, error)` à l'interface `GitProject` dans `strategy/semver/semver.go`. L'implémenter sur `git.Project` dans `git/git.go`.

```go
// git/git.go
func (p Project) CommitDate() (time.Time, time.Time, error) {
    return p.head.Author.When, p.head.Committer.When, nil
}
```

Dans `varsCore`, remplacer l'appel `time.Now()` par un appel à `p.CommitDate()`.

`fakeProject` dans `semver_test.go` reçoit un stub `CommitDate()` renvoyant des dates fixes pour les tests.

**Breaking change :** `{{ .git.Date }}` cesse de fonctionner dans les templates. Documenté.

### Fichiers

- Modifier : `strategy/semver/semver.go` (interface `GitProject` + `varsCore`)
- Modifier : `git/git.go` (implémentation `CommitDate`)
- Modifier : `strategy/semver/semver_test.go` (stub `fakeProject`)
- Modifier : `docs/reference.md` (tableau variables `git`)
- Modifier : `docs/how-to.md` (exemples mentionnant `git.Date`)

### Acceptance criteria

- `gg-version env` affiche `git.AuthorDate` et `git.CommitterDate`, pas `git.Date`
- Deux exécutions sur le même commit produisent les mêmes dates
- `git.Date` n'apparaît plus dans la sortie de `env`
- `TestCommitDate` passe dans `git/git_test.go`
- `fakeProject.CommitDate()` compilé et stub fonctionnel

---

## Fix 4 — `Compare` comme tiebreaker dans `LastTag`

### Problème

Quand deux tags sont topologiquement équidistants de HEAD (branches parallèles mergées), `LastTag` retient le dernier vu selon l'ordre d'itération non garanti de `go-git`. Le résultat est non déterministe.

### Design

Dans `git/git.go`, après avoir établi qu'un tag candidat n'est pas ancêtre du tag courant retenu (ni l'inverse), utiliser `f.Compare` comme tiebreaker : garder le tag à la version sémantiquement la plus élevée.

```go
candidateIsNewer, _ := isAncestor(tagCommit, lastTagCommit)
if candidateIsNewer {
    lastTag = tagRef
    lastTagCommit = tagCommit
    return nil
}

// Ni l'un ni l'autre n'est ancêtre → tags équidistants → tiebreaker sémantique
isLastNewer, _ := isAncestor(lastTagCommit, tagCommit)
if !isLastNewer {
    cmp, err := f.Compare(tagRef.Name().Short(), lastTag.Name().Short())
    if err == nil && cmp > 0 {
        lastTag = tagRef
        lastTagCommit = tagCommit
    }
}
```

Comportement inchangé pour les cas normaux (un seul tag le plus proche).

### Fichiers

- Modifier : `git/git.go` (fonction `LastTag`)
- Modifier : `git/git_test.go` (test `TestLastTag_tiebreaker`)

### Acceptance criteria

- `TestLastTag_tiebreaker` : deux tags équidistants → le plus élevé sémantiquement est retenu
- `TestLastTag_tiebreaker` : même résultat quelle que soit l'ordre de création des tags
- Tous les tests `git/` existants passent

---

## Fix 5 — `--version` via ldflags

### Problème

`gg-version --version` retourne une erreur. Un outil de versioning qui ne peut pas afficher sa propre version est difficile à déboguer en production.

### Design

Dans `cmd/gg-version/main.go`, déclarer une variable `Version` injectable par ldflags et la passer à `Run()` :

```go
var Version = "dev"

func main() {
    if err := Run(Version); err != nil {
        log.Fatal(err)
    }
}
```

Dans `cmd/gg-version/commands.go`, `Run(version string)` configure la version sur le `cli.Command` racine :

```go
func Run(version string) error {
    cmd := &cli.Command{
        Version: version,
        // ...
    }
    return cmd.Run(context.Background(), os.Args)
}
```

`urfave/cli/v3` expose automatiquement `--version` / `-v` quand `Version` est renseigné.

Build avec version injectée :
```bash
go build -ldflags "-X main.Version=1.2.3" -o gg-version ./cmd/gg-version
gg-version --version
# gg-version version 1.2.3
```

Sans ldflags :
```bash
gg-version --version
# gg-version version dev
```

### Fichiers

- Modifier : `cmd/gg-version/main.go`
- Modifier : `cmd/gg-version/commands.go` (signature `Run`)
- Modifier : `Makefile` (injection `VERSION`)

### Acceptance criteria

- `gg-version --version` affiche `gg-version version dev` sans ldflags
- `gg-version --version` affiche la version injectée avec ldflags
- `make build && ./gg-version --version` affiche un tag git ou `dev`

---

## Ordre d'implémentation

1. **Fix 1** — restructuration `cmd/gg-version/` (prérequis pour tous les autres)
2. **Fix 2** — `--var` global (modifie `commands.go`)
3. **Fix 3** — `git.AuthorDate` / `git.CommitterDate` (modifie `semver.go` et `git.go`)
4. **Fix 4** — tiebreaker `Compare` dans `LastTag` (modifie `git.go`)
5. **Fix 5** — `--version` (modifie `main.go` et `commands.go`)

## Breaking changes

| Changement | Impact |
|---|---|
| `git.Date` supprimé | Templates utilisant `{{ .git.Date }}` cessent de fonctionner |
| `--var` uniquement global | `gg-version current --var foo=bar` retourne une erreur |
| `go install` path | `go install .../cmd/gg-version@latest` (anciennement `go install ...@latest`) |

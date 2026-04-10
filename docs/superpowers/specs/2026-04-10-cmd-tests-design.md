# Tests de `cmd/gg-version` — Design Spec

**Date:** 2026-04-10

## Problème

`cmd/gg-version` n'a aucun test. Les bugs de parsing de flags, d'exclusion mutuelle `--component/--root`, de filtrage par composant et de formatage de sortie ne sont testés nulle part. Après le refactoring #10 (globalFlags via contexte), les fonctions de commande sont directement testables sans invoquer une vraie CLI.

---

## Architecture

**Fichier :** `cmd/gg-version/commands_test.go`, `package main`.

**Approche :** appel direct des fonctions de commande (`currentCmd`, `nextCmd`, etc.) avec un `context.Context` portant les `globalFlags` construits à la main. Repos git créés on-disk dans `t.TempDir()` via go-git (pas de binaire git requis).

**Dépendances :** déjà dans `go.mod` — `github.com/go-git/go-git/v5`, `gopkg.in/yaml.v3`, `gover/config`.

---

## Infrastructure de test

Six helpers partagés définis en haut du fichier :

```go
// newOnDiskRepo crée un repo go-git dans t.TempDir() avec un commit initial
// et retourne (chemin, *gogit.Repository).
func newOnDiskRepo(t *testing.T) (string, *gogit.Repository)

// addCommit crée un commit (fichier vide unique) dans le worktree.
func addCommit(t *testing.T, r *gogit.Repository, msg string) plumbing.Hash

// addTag crée un tag léger sur un commit donné.
func addTag(t *testing.T, r *gogit.Repository, hash plumbing.Hash, name string)

// testCtx retourne un context.Context avec les globalFlags injectés.
func testCtx(flags globalFlags) context.Context

// testCmd retourne un *cli.Command minimal avec un StringFlag "format".
func testCmd(format string) *cli.Command

// captureOutput redirige os.Stdout pendant fn() et retourne ce qui a été écrit.
func captureOutput(t *testing.T, fn func()) string

// writeConfig écrit cfg en YAML dans t.TempDir() et retourne le chemin du fichier.
func writeConfig(t *testing.T, cfg config.Config) string
```

`captureOutput` utilise `os.Pipe()` + goroutine de lecture pour capturer stdout sans affecter stderr.

---

## Cas de test par commande

### `currentCmd`
- HEAD tagué → version affichée sur stdout
- HEAD non tagué → sortie vide (current ne montre que les tags posés)
- `--component` + `--root` → erreur
- `format=json` → sortie JSON valide

### `nextCmd`
- HEAD non tagué → version calculée affichée
- `--component` + `--root` → erreur
- `format=json` → sortie JSON valide

### `lastCmd`
- Avec un tag → dernier tag affiché
- Sans tag → `"0.0.0"` affiché
- `--component` + `--root` → erreur

### `envCmd`
- Retourne les namespaces `semver`, `git`, `regex`, `var`
- `format=json` → JSON avec les 4 namespaces
- Composant inconnu avec `--component` → erreur

### `configCmd`
- `format=yaml` → sortie YAML contenant les clés de config
- `format=json` → sortie JSON contenant `_source`
- Format inconnu → erreur

### `componentsCmd`
- Sans composants → `"(no components defined)"`
- Avec un composant dans la config → ligne de tableau
- `format=json` → JSON avec le composant

### `lintCmd`
- HEAD exactement sur un tag (aucun commit à linter) → `nil` (exit 0)
- Commits CC valides depuis le tag → `nil` (exit 0)
- Commit non-CC depuis le tag → `cli.Exit("", 1)`

### `tagCmd`
- `--dry-run` → message "would create tag" sur stdout, aucun tag créé
- HEAD déjà tagué → message "already tagged" sur stderr, pas d'erreur
- `--component` + `--root` → erreur

---

## Organisation du fichier

```
cmd/gg-version/commands_test.go
├── imports
├── helpers (newOnDiskRepo, addCommit, addTag, testCtx, testCmd, captureOutput, writeConfig)
├── TestCurrentCmd (4 sous-tests)
├── TestNextCmd (3 sous-tests)
├── TestLastCmd (3 sous-tests)
├── TestEnvCmd (3 sous-tests)
├── TestConfigCmd (3 sous-tests)
├── TestComponentsCmd (3 sous-tests)
├── TestLintCmd (3 sous-tests)
└── TestTagCmd (3 sous-tests)
```

Total estimé : ~25 sous-tests dans un seul fichier, cohérent avec le style du projet.

---

## Fichiers

- **Create:** `cmd/gg-version/commands_test.go` — unique fichier créé

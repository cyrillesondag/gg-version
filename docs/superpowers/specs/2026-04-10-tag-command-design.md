# Commande `tag` — Design Spec

**Goal:** Ajouter `gg-version tag` pour créer un tag annoté sur HEAD avec la version `next` calculée, avec options `--push`, `--dry-run` et `--message`.

**Architecture:** Deux nouvelles méthodes sur `git.Project` (`CreateTag`, `PushTags`) encapsulant go-git ; une action `tagCmd` dans la CLI qui utilise `strategy.AllCurrent` pour calculer les versions et `git.Project` pour créer les tags. Cohérent avec l'architecture existante.

**Tech Stack:** Go 1.24, go-git/v5 (`CreateTag`, `Push`), urfave/cli/v3, golang.org/x/crypto/ssh (SSH agent).

---

## Problème

`gg-version` calcule la prochaine version mais ne peut pas créer le tag correspondant. Chaque pipeline doit dupliquer la logique `git tag $(gg-version next) && git push --tags`, rendant l'outil incomplet pour un workflow de release.

---

## Commande

```
gg-version tag [--push] [--dry-run] [--message <msg>]
```

Supporte les flags globaux `--component` et `--root`.

### Flags

| Flag | Défaut | Description |
|---|---|---|
| `--push` | false | Pushe les tags créés vers origin après création |
| `--dry-run` | false | Affiche ce qui serait fait sans créer de tag |
| `--message <msg>` | `"chore: release <version>"` | Message du tag annoté |

---

## Comportement

### Flux d'exécution

1. Charger la config et ouvrir le dépôt (identique aux autres commandes)
2. Calculer les versions `next` via `strategy.AllCurrent(p, extra, cfg)`
3. Filtrer par `--component` / `--root` si présents
4. Pour chaque résultat :
   - `tagName = r.Version` (ex: `v2.2.0`, `api/v0.5.2`)
   - Si `--dry-run` : afficher `would create tag <name> on <shortHash>`, continuer
   - Sinon : `p.CreateTag(tagName, message)` → erreur si le tag existe déjà
5. Si `--push` et pas `--dry-run` : `p.PushTags()`

### Sortie plain

```
created tag v2.2.0 on a1b2c3d
created tag api/v0.5.2 on a1b2c3d
pushed 2 tag(s) to origin
```

### Mode dry-run

```
would create tag v2.2.0 on a1b2c3d
would create tag api/v0.5.2 on a1b2c3d
```

### Erreur si tag existe déjà

```
error: tag v2.2.0 already exists
```

La commande s'arrête à la première erreur (pas de rollback partiel).

### Message par défaut

Si `--message` n'est pas fourni :
```
chore: release v2.2.0
```
Un message distinct est généré par composant si plusieurs tags sont créés.

---

## Couche git

### `git.Project.CreateTag`

```go
// CreateTag creates an annotated tag pointing to HEAD with the given message.
// Returns an error if the tag already exists.
func (p Project) CreateTag(name, message string) error {
    sig := &object.Signature{
        Name:  "gg-version",
        Email: "gg-version@local",
        When:  time.Now(),
    }
    _, err := p.repo.CreateTag(name, p.head.Hash, &gogit.CreateTagOptions{
        Message: message,
        Tagger:  sig,
    })
    return err
}
```

`go-git` retourne une erreur `ErrTagExists` si le tag existe déjà — propagée telle quelle.

### `git.Project.PushTags`

```go
// PushTags pushes all local tags to the "origin" remote.
// Uses SSH agent for authentication; falls back to nil auth for HTTPS.
func (p Project) PushTags() error {
    auth, _ := ssh.NewSSHAgentAuth("git")
    return p.repo.Push(&gogit.PushOptions{
        RemoteName: "origin",
        RefSpecs:   []config.RefSpec{"refs/tags/*:refs/tags/*"},
        Auth:       auth,
    })
}
```

Si `ssh.NewSSHAgentAuth` échoue (pas d'agent SSH), `auth` est `nil` — go-git essaie sans authentification, ce qui fonctionne pour les remotes HTTPS avec credential manager OS.

### Interface `GitProject`

Ajouter dans `strategy/semver/semver.go` :

```go
CreateTag(name, message string) error
PushTags() error
```

### `fakeProject` dans les tests

Stubs no-op retournant `nil` :

```go
func (fp *fakeProject) CreateTag(name, message string) error { return nil }
func (fp *fakeProject) PushTags() error                      { return nil }
```

---

## CLI

### Enregistrement dans `Run()`

```go
{
    Name:  "tag",
    Usage: "create a semver tag on HEAD for the calculated next version",
    Flags: []cli.Flag{
        &cli.BoolFlag{
            Name:  "push",
            Usage: "push created tags to origin after creation",
        },
        &cli.BoolFlag{
            Name:  "dry-run",
            Usage: "print what would be done without creating tags",
        },
        &cli.StringFlag{
            Name:  "message",
            Usage: "annotated tag message (default: \"chore: release <version>\")",
        },
    },
    Action: tagCmd,
},
```

### Action `tagCmd`

```go
func tagCmd(ctx context.Context, cmd *cli.Command) error {
    cfg, err := config.Load(configPath)
    if err != nil {
        return fmt.Errorf("loading config: %w", err)
    }
    p, err := gitpkg.NewProject(repoPath, "")
    if err != nil {
        return fmt.Errorf("opening repo: %w", err)
    }
    strategy := semverstrategy.New(cfg.Semver)
    extra := parseVarFlags(cmd.Root().StringSlice("var"))
    results, err := strategy.AllCurrent(p, extra, cfg)
    if err != nil {
        return fmt.Errorf("computing versions: %w", err)
    }
    filtered := filterComponentResults(results)

    dryRun := cmd.Bool("dry-run")
    push := cmd.Bool("push")
    msgFlag := cmd.String("message")
    shortHash := p.CommitHash()[:7]
    created := 0

    for _, r := range filtered {
        tagName := r.Version
        if tagName == "" {
            continue // composant non tagué, version vide (mode current)
        }
        msg := msgFlag
        if msg == "" {
            msg = "chore: release " + tagName
        }
        if dryRun {
            fmt.Printf("would create tag %s on %s\n", tagName, shortHash)
            continue
        }
        if err := p.CreateTag(tagName, msg); err != nil {
            return fmt.Errorf("creating tag %s: %w", tagName, err)
        }
        fmt.Printf("created tag %s on %s\n", tagName, shortHash)
        created++
    }

    if push && !dryRun && created > 0 {
        if err := p.PushTags(); err != nil {
            return fmt.Errorf("pushing tags: %w", err)
        }
        fmt.Printf("pushed %d tag(s) to origin\n", created)
    }
    return nil
}
```

---

## Tests

### `git/git_test.go`

**`TestCreateTag`**
- Créer un repo en mémoire avec un commit
- Appeler `p.CreateTag("v1.0.0", "chore: release v1.0.0")`
- Vérifier que le tag est résolvable via `repo.Tag("v1.0.0")`
- Vérifier que c'est un tag annoté (non nil `Message`)

**`TestCreateTag_alreadyExists`**
- Créer le tag `v1.0.0` deux fois
- Vérifier que la deuxième invocation retourne une erreur non-nil

### `strategy/semver/semver_test.go`

Stubs `fakeProject.CreateTag` et `fakeProject.PushTags` — pas de test de comportement car la logique est dans la couche git.

---

## Fichiers

| Fichier | Modification |
|---|---|
| `git/git.go` | Ajouter `CreateTag(name, message string) error` et `PushTags() error` |
| `git/git_test.go` | `TestCreateTag`, `TestCreateTag_alreadyExists` |
| `strategy/semver/semver.go` | `GitProject` : ajouter `CreateTag`, `PushTags` ; `fakeProject` stubs |
| `strategy/semver/semver_test.go` | Stubs `fakeProject.CreateTag` et `fakeProject.PushTags` |
| `cmd/gg-version/commands.go` | Enregistrer `tagCmd` + implémenter l'action |
| `docs/reference.md` | Ajouter section `tag` |

---

## Breaking changes

Aucun. La commande `tag` est additive.

---

## Limitations connues

- `PushTags` pousse **tous** les tags locaux (pas seulement ceux créés dans cette invocation). Comportement standard de `git push --tags`.
- L'authentification HTTPS avancée (token, `.netrc`) dépend du credential manager OS ; non testée explicitement.
- Pas de rollback si la création de tags échoue à mi-chemin en monorepo — les tags déjà créés restent.

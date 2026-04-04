# Design : Stratégie Semver pour gg-version

**Date :** 2026-04-04  
**Scope :** Implémentation de la stratégie semver uniquement — commandes `current` et `last`

---

## Contexte

`gg-version` est un CLI Go qui calcule la version d'un projet à partir de son historique git et de ses tags. La couche `git/` (lecture des tags, commits) est déjà implémentée. Ce design couvre l'implémentation de la stratégie semver et le câblage des commandes CLI.

---

## Commandes

Deux commandes sont dans le scope de cette implémentation :

- **`current`** — retourne la version au HEAD
- **`last`** — retourne le dernier tag semver valide atteignable depuis HEAD

Les commandes `next`, `previous` et `release` sont supprimées pour l'instant.

### Comportement de `current`

1. Si HEAD est sur un commit tagué → retourner le tag exact (ex: `1.4.2`)
2. Si la branche est une branche de **release** → retourner le dernier tag tel quel
3. Si la branche est une branche de **pre-release** → calculer une version via le template `format` configuré

### Comportement de `last`

Retourne le dernier tag semver valide atteignable depuis HEAD. Respecte les contraintes de version extraites du nom de la branche (voir section Configuration).

---

## Architecture

```
gover/
├── main.go
├── command/
│   └── commands.go          # handlers Current(), Last() — câblage CLI uniquement
├── config/
│   └── config.go            # parsing du .gg-version.yaml → struct Config
├── strategy/
│   └── semver/
│       ├── semver.go        # Strategy + SemverFormat (fusionnés)
│       └── semver_test.go   # tests avec repos in-memory
├── format/
│   └── format.go            # interface VersionFormat uniquement
└── git/
    └── git.go               # existant — LastTag(VersionFormat), CommitSinceTag(), BranchName()
```

**Flux de données :**
```
CLI args → command/ → config.Load() → strategy/semver.Strategy → git.Project → résultat string
```

`format/semver.go` est supprimé. `SemverFormat` migre dans `strategy/semver/semver.go`. L'interface `VersionFormat` reste dans `format/format.go`.

**Note importante :** `LastTag()` accepte un `VersionFormat` en paramètre (au lieu de le lire depuis `Project`). Ceci est nécessaire car les contraintes de version dépendent du nom de la branche, qui n'est connu qu'à l'exécution. `git.Project` n'a donc plus besoin du `VersionFormat` à la construction.

---

## Configuration

### Fichier `.gg-version.yaml`

```yaml
semver:
  tag_prefix: "v"
  initial: "0.1.0"
  branches:
    - pattern: "^refs/heads/main$"
      release: true
    - pattern: "^refs/heads/release/(?P<major>\\d+)\\.x$"
      release: true
    - pattern: "^refs/heads/release/(?P<major>\\d+)\\.(?P<minor>\\d+)\\.x$"
      release: true
    - pattern: "^refs/heads/feature/(?P<name>.+)$"
      release: false
      format: "{{ .LastTag }}-{{ .name }}.{{ .CommitCount }}"
    - pattern: ".*"
      release: false
      format: "{{ .LastTag }}-{{ .Branch }}.{{ .CommitCount }}"
```

### Règles de matching des branches

- Les patterns sont évalués dans l'ordre — premier match gagne
- Toute branche ne matchant aucun pattern est traitée comme pre-release avec le format par défaut
- Les groupes nommés `(?P<major>\d+)`, `(?P<minor>\d+)`, `(?P<patch>\d+)` extraits du pattern servent à deux fins :
  1. **Contrainte de version** (si `release: true`) — `LastTag()` filtre les tags qui ne respectent pas les composants capturés
  2. **Variable de template** (si `release: false`) — disponibles dans le template `format`

### Variables disponibles dans le template `format`

| Variable | Description |
|---|---|
| `{{ .LastTag }}` | Dernier tag valide (ex: `1.4.2`) |
| `{{ .Branch }}` | Nom court de la branche (ex: `feature-foo`) |
| `{{ .CommitCount }}` | Nombre de commits depuis le dernier tag |
| `{{ .ShortHash }}` | Hash court du commit HEAD |
| `{{ .name }}` | Tout groupe nommé capturé par le pattern (ex: `(?P<name>.+)`) |

### Struct Go (`config/config.go`)

```go
type Config struct {
    Semver SemverConfig `yaml:"semver"`
}

type SemverConfig struct {
    TagPrefix string         `yaml:"tag_prefix"`
    Initial   string         `yaml:"initial"`
    Branches  []BranchConfig `yaml:"branches"`
}

type BranchConfig struct {
    Pattern string `yaml:"pattern"`
    Release bool   `yaml:"release"`
    Format  string `yaml:"format"` // ignoré si release: true
}
```

Si le fichier de config est absent, des valeurs par défaut sont appliquées :
- `tag_prefix: ""`
- `initial: "0.1.0"`
- Un pattern catch-all pre-release avec format `"{{ .LastTag }}-{{ .Branch }}.{{ .CommitCount }}"`

---

## Package `strategy/semver`

### Interface publique

```go
type Strategy struct {
    config config.SemverConfig
}

func NewStrategy(cfg config.SemverConfig) Strategy

// Last retourne le dernier tag semver valide atteignable depuis HEAD,
// en respectant les contraintes de version extraites du pattern de branche.
func (s Strategy) Last(project *git.Project) (string, error)

// Current retourne la version au HEAD selon l'algorithme décrit ci-dessus.
func (s Strategy) Current(project *git.Project) (string, error)
```

### `SemverFormat` (migré depuis `format/semver.go`)

```go
type SemverFormat struct {
    Prefix      string
    Constraints map[string]string // groupes nommés extraits du pattern (ex: {"major": "1"})
}

func (s SemverFormat) IsValid(version string) bool
func (s SemverFormat) Compare(version1, version2 string) (int, error)
```

`IsValid` vérifie que le tag est un semver valide ET que ses composants respectent les contraintes extraites (ex: si `major="1"`, le tag `2.0.0` est rejeté).

### Algorithme de `Current`

1. Appeler `project.BranchName()` → matcher contre les patterns `branches` dans l'ordre
2. Extraire les groupes nommés du pattern matché
3. Construire un `SemverFormat` avec le `tag_prefix` et les contraintes extraites
4. Appeler `project.LastTag(semverFormat)` — le format est passé en paramètre
5. Vérifier si HEAD == commit du tag (HEAD tagué)
6. Si HEAD tagué → retourner le tag
7. Si branche release → retourner le dernier tag
8. Si branche pre-release → compter les commits via `project.CommitSinceTag(tag)`, exécuter le template Go avec les variables fixes + groupes nommés → retourner la version calculée

### Algorithme de `Last`

1. Appeler `project.BranchName()` → matcher le pattern, extraire les contraintes
2. Construire un `SemverFormat` avec le `tag_prefix` et les contraintes extraites
3. Appeler `project.LastTag(semverFormat)` → retourner le résultat

---

## Câblage CLI (`command/commands.go`)

```go
func Current(ctx context.Context, cmd *cli.Command) error {
    cfg, err := config.Load(configPath)
    project, err := git.NewProject(repoPath, "")
    strategy := semver.NewStrategy(cfg.Semver)
    version, err := strategy.Current(project)
    fmt.Println(version)
    return nil
}

func Last(ctx context.Context, cmd *cli.Command) error {
    cfg, err := config.Load(configPath)
    project, err := git.NewProject(repoPath, "")
    strategy := semver.NewStrategy(cfg.Semver)
    version, err := strategy.Last(project)
    fmt.Println(version)
    return nil
}
```

**Flags CLI :**

| Flag | Défaut | Description |
|---|---|---|
| `--config` | `.gg-version.yml` | Chemin vers le fichier de configuration |
| `--repo` | `.` | Chemin vers le dépôt Git |

---

## Tests

Les tests dans `strategy/semver/semver_test.go` suivent le même pattern que `git/git_test.go` : repos entièrement in-memory via `go-git` + `go-billy/memfs`. Pas de dépendance au système de fichiers réel ni à une installation Git.

**Scénarios à couvrir :**

- HEAD sur un commit tagué → `current` retourne le tag exact
- HEAD sur une branche release non taguée → `current` retourne le dernier tag
- HEAD sur une branche pre-release → `current` retourne la version formatée
- Branche `release/1.x` avec contrainte major=1 → `last` ignore les tags `2.x.x`
- Aucun tag trouvé → `last` retourne `initial` (`0.1.0`)
- Config absente → valeurs par défaut appliquées

---

## Ce qui est hors scope

- Stratégie `increment`
- Commandes `next`, `previous`, `release`
- Flag `--strategy` (semver est la seule stratégie pour l'instant)
- Flag `--format` (output format plain/json)
- Flag `--dry-run`
- Écriture de tags dans le repo

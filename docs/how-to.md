# Guides pratiques

Ces guides répondent à des objectifs précis. Choisissez celui qui correspond à votre situation.

---

## Utiliser un préfixe `v` sur les tags

Par défaut, aucun préfixe n'est utilisé. Pour que `gg-version` reconnaisse les tags du style `v1.2.3` :

```yaml
# .gg-version.yml
semver:
  tag_prefix: "v"
```

```bash
git tag v1.0.0
gg-version current
# → v1.0.0

# Après un commit feat:
gg-version current
# → v1.1.0
```

---

## Personnaliser le format des versions de pré-release

Sur les branches qui ne sont pas des releases, la version est rendue via un template Go. Par défaut : `{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}`.

Pour inclure le hash court :

```yaml
semver:
  tag_prefix: "v"
  branches:
    - pattern: "main"
      release: true
    - pattern: ".*"
      release: false
      format: "{{ .semver.Semver }}-{{ .git.ShortHash }}"
```

```bash
# Sur la branche feat/login, après un feat:
gg-version current
# → 1.3.0-a1b2c3d
```

Autre exemple — inclure la date :

```yaml
format: "{{ .semver.Semver }}-{{ .git.AuthorDate }}.{{ .git.CommitCount }}"
```

```bash
gg-version current
# → 1.3.0-2026-04-06.7
```

---

## Configurer des branches de release supplémentaires

Par défaut, seule `main` est une branche de release. Pour ajouter `master` et les branches `release/*` :

```yaml
semver:
  tag_prefix: "v"
  branches:
    - pattern: "main"
      release: true
    - pattern: "master"
      release: true
    - pattern: "release/.*"
      release: true
    - pattern: ".*"
      release: false
      format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
```

Sur `release/1.x`, la version sera un semver plein (ex : `v1.4.2`), pas un pré-release.

---

## Extraire des informations du nom de branche

Les patterns de branche sont des expressions régulières avec capture nommée. Les captures sont disponibles dans `{{ .regex.<nom> }}`.

Exemple : extraire le numéro de ticket depuis `feat/PROJ-123-my-feature` :

```yaml
semver:
  branches:
    - pattern: "main"
      release: true
    - pattern: "feat/(?P<ticket>[A-Z]+-[0-9]+)-.*"
      release: false
      format: "{{ .semver.Semver }}-{{ .regex.ticket }}.{{ .git.CommitCount }}"
    - pattern: ".*"
      release: false
      format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
```

```bash
# Sur la branche feat/PROJ-123-login
gg-version current
# → 1.3.0-PROJ-123.4
```

---

## Passer des variables personnalisées au template

`--var name=value` injecte des variables accessibles via `{{ .var.name }}` :

```bash
gg-version --var env=staging --var region=eu-west current
```

Avec le format :

```yaml
format: "{{ .semver.Semver }}-{{ .var.env }}-{{ .git.ShortHash }}"
```

```
1.3.0-staging-a1b2c3d
```

Utile pour inclure des métadonnées de build sans modifier la configuration.

---

## Ignorer certains chemins de fichiers

Pour exclure les commits qui ne touchent que la documentation de l'analyse de version :

```yaml
semver:
  tag_prefix: "v"
  ignore_paths:
    - "docs/**"
    - "*.md"
    - ".github/**"
```

Un commit qui modifie uniquement `docs/tutorial.md` ne contribue pas au calcul de version. Un commit qui modifie à la fois `docs/tutorial.md` et `src/api.go` est conservé (tous les fichiers doivent correspondre pour qu'un commit soit ignoré).

---

## Ignorer des commits spécifiques par SHA

Pour exclure un commit précis (hotfix de CI, commit de merge automatique…) :

```yaml
semver:
  ignore_commits:
    - "a1b2c3d"      # préfixe court suffit
    - "deadbeef12"
```

```bash
# Vérifier l'effet
gg-version env | grep CommitCount
# git.CommitCount=4   ← le commit ignoré n'est pas compté
```

---

## Utiliser gg-version dans un monorepo

Pour des projets où plusieurs composants sont versionnés indépendamment dans le même dépôt :

```yaml
# .gg-version.yml
semver:
  tag_prefix: "v"

components:
  api:
    path: "api/**"
  frontend:
    path: "frontend/**"
  shared:
    path: "shared/**"
    tag_scope: "libs"   # les tags seront libs/v1.0.0 au lieu de shared/v1.0.0
```

```bash
gg-version current
# @root        v2.1.0
# api          v0.5.0
# frontend     v3.2.1
# shared       v1.0.0
```

Chaque composant est versionné à partir des commits qui touchent son répertoire. `@root` représente tout ce qui ne touche pas un composant déclaré.

Filtrer sur un seul composant :

```bash
gg-version current --component api
# → v0.5.0

gg-version current --root
# → v2.1.0
```

Lister les composants et leurs patterns de tags :

```bash
gg-version components
# api          path=api/**                      tag=api/v*
# frontend     path=frontend/**                 tag=frontend/v*
# shared       path=shared/**                   tag=libs/v*
```

---

## Intégrer gg-version dans une pipeline CI

### GitHub Actions

```yaml
- name: Compute version
  id: version
  run: echo "value=$(gg-version current)" >> $GITHUB_OUTPUT

- name: Build
  run: docker build -t myapp:${{ steps.version.outputs.value }} .
```

### GitLab CI

```yaml
compute-version:
  script:
    - export APP_VERSION=$(gg-version current)
    - echo "APP_VERSION=$APP_VERSION" >> build.env
  artifacts:
    reports:
      dotenv: build.env
```

### Makefile

```makefile
VERSION := $(shell gg-version current)

.PHONY: build
build:
	go build -ldflags="-X main.version=$(VERSION)" ./...
```

---

## Injecter la version dans un binaire Go

```bash
gg-version current
# v1.4.2
```

```makefile
VERSION := $(shell gg-version current)

build:
	go build -ldflags="-X main.Version=$(VERSION)" -o myapp .
```

```go
// main.go
var Version = "dev"

func main() {
    fmt.Println("Version:", Version)
}
```

```bash
make build && ./myapp
# Version: v1.4.2
```

---

## Déboguer le calcul de version

Quand la version affichée vous surprend, inspectez toutes les variables :

```bash
gg-version env
```

```
git.AuthorDate=2026-04-06
git.Branch=main
git.CommitCount=3
git.CommitterDate=2026-04-06
git.Hash=abc1234def5678...
git.LastTag=v1.2.0
git.ShortHash=abc1234
semver.HasNonConventionalCommits=false
semver.IsBreakingChange=false
semver.IsPreRelease=false
semver.LastMajor=1
semver.LastMinor=2
semver.LastPatch=0
semver.LastVersion=1.2.0
semver.Major=1
semver.Minor=3
semver.Patch=0
semver.Semver=1.3.0
```

En format JSON pour un parsing CI :

```bash
gg-version env --format json
```

```json
{
  "git": {
    "AuthorDate": "2026-04-06",
    "Branch": "main",
    "CommitCount": 3,
    "CommitterDate": "2026-04-06",
    "Hash": "abc1234def5678...",
    "LastTag": "v1.2.0",
    "ShortHash": "abc1234"
  },
  "semver": {
    "Major": "1",
    "Minor": "3",
    "Patch": "0",
    "Semver": "1.3.0",
    ...
  }
}
```

Vérifier la configuration effective (utile pour diagnostiquer un fichier `.gg-version.yml` mal interprété) :

```bash
gg-version config
# config from: .gg-version.yml
# semver:
#   tag_prefix: v
#   ...
```

---

## Travailler sur un dépôt distant ou dans un sous-répertoire

```bash
# Dépôt dans un autre répertoire
gg-version --repo /path/to/other-project current

# Fichier de config dans un emplacement non standard
gg-version --config config/versioning.yml current

# Les deux combinés
gg-version --repo ../backend --config ../backend/.gg-version.yml current
```

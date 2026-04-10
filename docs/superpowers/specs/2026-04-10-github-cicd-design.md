# GitHub Repository & CI/CD Workflow

**Goal:** Créer le repo GitHub, configurer un pipeline CI/CD complet (build, test, release, security, updates) pour le projet `gg-version`.

**Architecture:** 4 fichiers de workflow GitHub Actions à responsabilité unique + GoReleaser pour la publication multi-plateforme. Déclenchement release sur tag `v*.*.*`. Mise à jour des dépendances via Dependabot.

**Tech Stack:** Go 1.24, GitHub Actions, GoReleaser v2, golangci-lint, govulncheck, CodeQL, Dependabot.

---

## Prérequis : mise à jour du module path

Le `go.mod` déclare `module gover` (chemin local). Ce chemin doit devenir `github.com/<user>/<repo>` avant toute publication.

**Étapes :**
1. Renommer le module dans `go.mod` : `module github.com/<user>/<repo>`
2. Mettre à jour tous les imports internes dans les packages Go (`cmd/gg-version`, `config`, `format`, `git`, `strategy/semver`)
3. `go build ./...` + `go test ./...` → doivent passer

Le placeholder `<user>/<repo>` sera remplacé par le nom réel du compte et du repository GitHub.

---

## Fichiers créés / modifiés

| Fichier | Action | Rôle |
|---|---|---|
| `go.mod` | Modify | Mise à jour du module path |
| `.goreleaser.yml` | Create | Configuration GoReleaser |
| `.github/workflows/ci.yml` | Create | Build + test sur chaque push/PR |
| `.github/workflows/release.yml` | Create | Publication GoReleaser sur tag `v*` |
| `.github/workflows/security.yml` | Create | govulncheck + CodeQL |
| `.github/dependabot.yml` | Create | Mises à jour automatiques |

---

## `.goreleaser.yml`

```yaml
version: 2

project_name: gg-version

before:
  hooks:
    - go mod tidy

builds:
  - id: gg-version
    binary: gg-version
    main: ./cmd/gg-version
    ldflags:
      - -s -w -X main.Version={{.Version}}
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - id: default
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        formats: [zip]
    formats: [tar.gz]

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  use: git
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
      - Merge pull request
      - Merge branch

release:
  github:
    owner: "{{ .Env.GITHUB_REPOSITORY_OWNER }}"
    name: "{{ .ProjectName }}"
  draft: false
  prerelease: auto
```

---

## `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    name: Test (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-14, windows-latest]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: Run tests
        run: go test ./...

  build:
    name: Build (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-14, windows-latest]
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: Build
        run: go build -ldflags "-X main.Version=dev" ./cmd/gg-version

  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

---

## `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags:
      - "v*.*.*"

permissions:
  contents: write

jobs:
  release:
    name: GoReleaser
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## `.github/workflows/security.yml`

```yaml
name: Security

on:
  push:
    branches: [main]
  schedule:
    - cron: "0 3 * * 1"   # CodeQL : hebdomadaire, lundi 3h UTC
    - cron: "0 3 * * *"   # govulncheck : quotidien, 3h UTC

jobs:
  govulncheck:
    name: govulncheck
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - name: Run govulncheck
        run: govulncheck ./...

  codeql:
    name: CodeQL
    runs-on: ubuntu-latest
    permissions:
      security-events: write
      actions: read
      contents: read
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - uses: github/codeql-action/init@v3
        with:
          languages: go
      - uses: github/codeql-action/autobuild@v3
      - uses: github/codeql-action/analyze@v3
```

---

## `.github/dependabot.yml`

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
```

---

## Flux de release

```
git tag v1.2.3
git push origin v1.2.3
    → release.yml déclenché
    → GoReleaser compile 9 binaires (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
    → GitHub Release créée avec archives + checksums.txt + changelog CC
```

---

## Invariants à préserver

- `ci.yml` s'exécute sur chaque push/PR — les releases ne doivent jamais bypasser les tests
- `GITHUB_TOKEN` est le seul secret nécessaire (fourni automatiquement par GitHub Actions)
- `fetch-depth: 0` obligatoire dans `release.yml` et `build` jobs (GoReleaser et `git describe` en ont besoin)
- `windows/arm64` exclu de GoReleaser (cross-compilation Go non supportée de façon fiable)
- Le schedule `security.yml` utilise deux crons séparés : govulncheck quotidien, CodeQL hebdomadaire

---

## Breaking changes

- Le module path `gover` change — tout import interne doit être mis à jour
- Aucun changement de comportement observable pour les utilisateurs finaux du binaire

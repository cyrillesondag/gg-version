# gg-version

> A Git-aware CLI that computes your project's version from its commit history — no manual tagging required.

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)

---

## Overview

`gg-version` is a higly configurable CLI tool used to manage automatic projects versionning by using conventionnal commits and semver.

The version computation is based on the working directory, commit history, git tags, and branch names.

Key design principles:
- **No side effects by default** - `gg-version` never writes tags, commits, or files unless explicitly asked.
- **CI-friendly** - machine-readable output, exit codes, and `--dry-run` support.
- **Monorepo compatible** – Supports multiple repositories in the same workspace. 

---

## Features

| Feature | Command | Description |
|---|---|---|
| Current version | `current` | Resolves the version at `HEAD` based on existing tags |
| Next version | `next` | Computes the next version to release |
| Dry-run | `--dry-run` | Previews the computed version without any side effect |
| Strategy selection | `--strategy` | Chooses the versioning strategy (`semver`, `increment`, …) |

---

## Installation

### Via `go install`

```bash
go install github.com/<your-username>/gg-version@latest
```

### From source

```bash
git clone https://github.com/<your-username>/gg-version.git
cd gg-version
go build -o gg-version ./cmd/gg-version
```

### Binary releases

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/<your-username>/gg-version/releases) page.

---

## Usage

```
gg-version <command> [flags]
```

### Commands

#### `current`

Resolves and prints the version at `HEAD`.

```bash
gg-version current
# → 1.4.2
```

If `HEAD` does not point to a tagged commit, the version is derived from the last reachable tag and the strategy in use.

#### `next`

Computes the next version that should be released based on the commits since the last tag.

```bash
gg-version next
# → 1.5.0
```

#### Global flags

| Flag | Default | Description |
|---|---|---|
| `--strategy` | `semver` | Versioning strategy to apply (`semver`, `increment`) |
| `--repo` | `.` | Path to the Git repository |
| `--dry-run` | `false` | Print the result without performing any write operation |
| `--format` | `plain` | Output format: `plain`, `json` |
| `--config` | `.gg-version.yaml` | Path to a configuration file |

---

## Versioning Strategies

### `semver` (default)

Follows the [Semantic Versioning 2.0.0](https://semver.org) specification.
Commit messages must conform to the [Conventional Commits](https://www.conventionalcommits.org) format.

| Commit type | Version bump |
|---|---|
| `fix:` | PATCH → `1.4.2` → `1.4.3` |
| `feat:` | MINOR → `1.4.2` → `1.5.0` |
| `feat!:` or `BREAKING CHANGE:` | MAJOR → `1.4.2` → `2.0.0` |

```bash
gg-version next --strategy semver
# → 2.0.0
```

### `increment`

Ignores commit message conventions. Simply increments a numeric counter based on the number of commits since the last tag.

```bash
gg-version next --strategy increment
# → 42
```

The tag format can be customized in the configuration file (e.g., `build-42`, `v42`).

### Extensibility

The strategy system is designed to be open. Additional strategies can be implemented by satisfying the `Strategy` interface:

```go
type Strategy interface {
    // Current resolves the version for the given commit.
    Current(repo *git.Repository, head *object.Commit) (string, error)
    // Next computes the next version from the last tag and commits since.
    Next(repo *git.Repository, last *semver.Version, commits []*object.Commit) (string, error)
}
```

---

## Configuration

`gg-version` looks for a `.gg-version.yaml` file at the root of the repository (or at the path given by `--config`).

```yaml
# .gg-version.yaml

# Strategy to use: semver | increment
strategy: semver

# For the "increment" strategy: format of the version string.
# Supports Go template syntax. Available variables: .Count, .Hash
increment:
  format: "build-{{ .Count }}"

# For the "semver" strategy: initial version when no tag is found.
semver:
  initial: "0.1.0"
  # Prefix to use when looking for / creating tags (e.g., "v" → looks for "v1.0.0")
  tag_prefix: "v"
```

Command-line flags always take precedence over the configuration file.

---

## Examples

```bash
# Show current version using semver strategy
gg-version current --strategy semver

# Preview the next version without side effects
gg-version next --dry-run

# Use the increment strategy on a specific repository
gg-version next --strategy increment --repo /path/to/repo

# Output as JSON (useful in CI pipelines)
gg-version next --format json
# → {"version":"1.5.0","strategy":"semver","commits_since_last_tag":3}
```

### CI integration example (GitLab CI)

```yaml
compute-version:
  image: golang:1.22
  script:
    - go install github.com/<your-username>/gg-version@latest
    - export APP_VERSION=$(gg-version next --format plain)
    - echo "Next version: $APP_VERSION"
  artifacts:
    reports:
      dotenv: version.env
```

---

## Development

### Prerequisites

- Go 1.22+
- A local Git repository to test against

### Running tests

```bash
go test ./...
```

### Running locally

```bash
go run ./cmd/gg-version next --dry-run
```

---

## Contributing

Contributions are welcome! Feel free to open an issue or a pull request.

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-strategy`)
3. Commit your changes following Conventional Commits
4. Open a Pull Request

If you are implementing a new strategy, please add:
- The strategy implementation under `internal/strategy/`
- Unit tests with representative commit history fixtures
- Documentation in this README

---

## License

MIT © [Your Name](https://github.com/<your-username>)
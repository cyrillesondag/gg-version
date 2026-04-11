# gg-version

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/cyrillesondag/gg-version/badge)](https://securityscorecards.dev/viewer/?uri=github.com/cyrillesondag/gg-version)

> Automatically computes your project version from Git history and Conventional Commits — without ever writing to the repository.

---

## Installation

```bash
go install github.com/cyrillesondag/gg-version@latest
```

Or from source:

```bash
git clone https://github.com/cyrillesondag/gg-version.git
cd gg-version
go build -o gg-version .
```

---

## Quick Start

```bash
# Computed version at HEAD
gg-version next
# → v1.4.2

# Last tagged version
gg-version last
# → v1.4.1

# All template variables available for formatting
gg-version env
```

Without a configuration file, `gg-version` works with sensible defaults.

---

## Documentation

| Document | When to read |
|---|---|
| [Tutorial](docs/tutorial.md) | Getting started — follow a step-by-step example |
| [How-to guides](docs/how-to.md) | You have a specific goal (monorepo, CI, custom format…) |
| [Reference](docs/reference.md) | Looking for a flag, config option, or template variable |
| [Concepts](docs/explanation.md) | You want to understand how it works internally |

---

## How It Works in One Sentence

`gg-version` walks the Git history from HEAD, finds the last reachable semver tag, analyses the commits in between using Conventional Commits, and derives the current version — without ever creating a tag or modifying the repository.

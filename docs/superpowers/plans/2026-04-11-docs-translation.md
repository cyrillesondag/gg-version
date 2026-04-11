# Documentation Translation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Translate all five user-facing Markdown documentation files from French to English, fixing stale `gg-version current` references and the `yourorg` placeholder URL in the same pass.

**Architecture:** Five independent tasks, one per file. Each task reads the French source, writes the English replacement in-place, verifies with grep, and commits. No code changes — documentation only.

**Tech Stack:** Markdown, bash grep

---

## File Map

| File | Action |
|---|---|
| `README.md` | Translate + fix `current`→`next` + fix `yourorg`→`cyrillesondag` |
| `docs/tutorial.md` | Translate + fix `current`→`next` throughout |
| `docs/how-to.md` | Translate + fix `current`→`next` throughout |
| `docs/reference.md` | Translate + remove stale migration note about removed `current` command |
| `docs/explanation.md` | Translate + rename `current`→`next` in conceptual section |

---

### Task 1: Translate README.md

**Goal:** Replace the French README with an English version, fixing the `current` command reference and the `yourorg` placeholder URL.

**Files:**
- Modify: `README.md`

**Acceptance Criteria:**
- [ ] All prose is in English
- [ ] `go install` URL uses `github.com/cyrillesondag/gg-version`
- [ ] `git clone` URL uses `github.com/cyrillesondag/gg-version`
- [ ] Quick-start shows `gg-version next`, not `gg-version current`
- [ ] No French words remain

**Verify:**
```bash
grep -i "current\b" README.md        # must return 0 lines
grep "yourorg" README.md             # must return 0 lines
grep "cyrillesondag" README.md       # must return 2 lines (go install + git clone)
```

**Steps:**

- [ ] **Step 1: Write the English README**

Write the following content to `README.md` (overwrite completely):

```markdown
# gg-version

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
```

- [ ] **Step 2: Verify**

```bash
grep -ic "current" README.md        # expect 0
grep "yourorg" README.md            # expect 0 lines
grep "cyrillesondag" README.md      # expect 2 lines
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: translate README to English"
```

---

### Task 2: Translate docs/tutorial.md

**Goal:** Replace the French tutorial with an English version. Replace all `gg-version current` with `gg-version next` and update the surrounding explanations.

**Files:**
- Modify: `docs/tutorial.md`

**Acceptance Criteria:**
- [ ] All prose is in English
- [ ] No `gg-version current` occurrences remain
- [ ] `VERSION=$(gg-version next)` in the script example
- [ ] Step numbering and cross-links intact

**Verify:**
```bash
grep "gg-version current" docs/tutorial.md   # must return 0 lines
```

**Steps:**

- [ ] **Step 1: Write the English tutorial**

Write the following content to `docs/tutorial.md` (overwrite completely):

````markdown
# Tutorial: your first automatic version

In this tutorial you will configure `gg-version` on a real Git repository and get your first automatically computed version number. No prior knowledge is required.

**What you will have at the end:** a command usable in your CI pipeline that prints `v1.2.3` (or the appropriate version) on every build.

---

## Prerequisites

- `gg-version` installed (`go install` or downloaded binary)
- A Git repository with at least one commit

---

## Step 1 — Check that gg-version sees your repository

From the root of your repository:

```bash
gg-version next
```

Expected result if you have no tags:

```
0.1.0
```

This is the default initial version. The tool works out of the box with no configuration.

---

## Step 2 — Create your first version tag

`gg-version` reads existing Git tags. Create your starting point:

```bash
git tag v1.0.0
```

Verify that the tool recognises it:

```bash
gg-version next
```

```
v1.0.0
```

`gg-version` confirms that HEAD points to a tagged commit — the computed version is exactly that tag.

---

## Step 3 — Add commits and observe the evolution

Make a few commits after the tag. Use the Conventional Commits format:

```bash
echo "change" >> README.md
git add README.md
git commit -m "fix: correct typo in README"

echo "feature" >> feature.txt
git add feature.txt
git commit -m "feat: add feature.txt"
```

Run again:

```bash
gg-version next
```

```
v1.1.0
```

`gg-version` analysed the two commits:
- `fix:` → patch bump
- `feat:` → minor bump (overrides the patch)

The result is `v1.1.0`, the next tag that should be created.

---

## Step 4 — Create a configuration file

Without configuration, the tag prefix is empty. Most projects use `v`. Create `.gg-version.yml` at the root:

```yaml
semver:
  tag_prefix: "v"
  initial: "0.1.0"
```

Verify the loaded configuration:

```bash
gg-version config
```

```yaml
# config from: .gg-version.yml
semver:
  tag_prefix: "v"
  initial: "0.1.0"
  branches:
    - pattern: main
      release: true
      format: ""
    - pattern: .*
      release: false
      format: '{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}'
  ...
```

---

## Step 5 — Observe the behaviour on a feature branch

Create a branch:

```bash
git checkout -b feat/my-feature
echo "work" >> work.txt
git add work.txt
git commit -m "feat: add work"
```

```bash
gg-version next
```

```
1.2.0-feat/my-feature.1
```

On a branch that is not `main`, `gg-version` generates a pre-release identifier with the branch name and the commit count.

---

## Step 6 — Use the version in a script

```bash
VERSION=$(gg-version next)
echo "Building version $VERSION"
docker build -t myapp:$VERSION .
```

On `main`:
```
Building version v1.2.0
```

On a feature branch:
```
Building version 1.2.0-feat/my-feature.1
```

---

## Summary

You have learned to:

1. Get a version with no configuration
2. Anchor a version with a Git tag
3. Observe how Conventional Commits evolve the version
4. Create a minimal configuration file
5. See the difference between a release branch and a feature branch
6. Capture the version in a CI script

**Next step:** see the [how-to guides](how-to.md) for specific scenarios, or the [reference](reference.md) for the full list of options.
````

- [ ] **Step 2: Verify**

```bash
grep "gg-version current" docs/tutorial.md   # expect 0 lines
```

- [ ] **Step 3: Commit**

```bash
git add docs/tutorial.md
git commit -m "docs: translate tutorial to English"
```

---

### Task 3: Translate docs/how-to.md

**Goal:** Replace the French how-to guides with an English version. Replace all `gg-version current` with `gg-version next`.

**Files:**
- Modify: `docs/how-to.md`

**Acceptance Criteria:**
- [ ] All prose is in English
- [ ] No `gg-version current` occurrences remain
- [ ] All section anchors work (no broken relative links)

**Verify:**
```bash
grep "gg-version current" docs/how-to.md   # must return 0 lines
```

**Steps:**

- [ ] **Step 1: Write the English how-to guide**

Write the following content to `docs/how-to.md` (overwrite completely):

````markdown
# How-to guides

These guides answer specific goals. Pick the one that matches your situation.

---

## Use a `v` prefix on tags

By default no prefix is used. To make `gg-version` recognise tags like `v1.2.3`:

```yaml
# .gg-version.yml
semver:
  tag_prefix: "v"
```

```bash
git tag v1.0.0
gg-version next
# → v1.0.0

# After a feat: commit:
gg-version next
# → v1.1.0
```

---

## Customise the pre-release version format

On branches that are not releases, the version is rendered via a Go template. Default: `{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}`.

To include the short hash:

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
# On branch feat/login, after a feat: commit:
gg-version next
# → 1.3.0-a1b2c3d
```

Another example — include the date:

```yaml
format: "{{ .semver.Semver }}-{{ .git.AuthorDate }}.{{ .git.CommitCount }}"
```

```bash
gg-version next
# → 1.3.0-2026-04-06.7
```

---

## Configure additional release branches

By default only `main` is a release branch. To add `master` and `release/*` branches:

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

On `release/1.x`, the version will be a full semver (e.g. `v1.4.2`), not a pre-release.

---

## Extract information from the branch name

Branch patterns are Go regular expressions with named captures. Captures are available as `{{ .regex.<name> }}`.

Example: extract the ticket number from `feat/PROJ-123-my-feature`:

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
# On branch feat/PROJ-123-login
gg-version next
# → 1.3.0-PROJ-123.4
```

---

## Pass custom variables to the template

`--var name=value` injects variables accessible as `{{ .var.name }}`:

```bash
gg-version --var env=staging --var region=eu-west next
```

With format:

```yaml
format: "{{ .semver.Semver }}-{{ .var.env }}-{{ .git.ShortHash }}"
```

```
1.3.0-staging-a1b2c3d
```

Useful for including build metadata without modifying the configuration.

---

## Ignore certain file paths

To exclude commits that only touch documentation from version analysis:

```yaml
semver:
  tag_prefix: "v"
  ignore_paths:
    - "docs/**"
    - "*.md"
    - ".github/**"
```

A commit that modifies only `docs/tutorial.md` does not contribute to the version. A commit that modifies both `docs/tutorial.md` and `src/api.go` is kept (all files must match for a commit to be excluded).

---

## Ignore specific commits by SHA

To exclude a specific commit (CI hotfix, automatic merge commit…):

```yaml
semver:
  ignore_commits:
    - "a1b2c3d"      # short prefix is enough
    - "deadbeef12"
```

```bash
# Check the effect
gg-version env | grep CommitCount
# git.CommitCount=4   ← the ignored commit is not counted
```

---

## Use gg-version in a monorepo

For projects where multiple components are versioned independently in the same repository:

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
    tag_scope: "libs"   # tags will be libs/v1.0.0 instead of shared/v1.0.0
```

```bash
gg-version next
# @root        v2.1.0
# api          v0.5.0
# frontend     v3.2.1
# shared       v1.0.0
```

Each component is versioned from commits that touch its directory. `@root` represents everything that does not touch any declared component.

Filter to a single component:

```bash
gg-version next --component api
# → v0.5.0

gg-version next --root
# → v2.1.0
```

List components and their tag patterns:

```bash
gg-version components
# api          path=api/**                      tag=api/v*
# frontend     path=frontend/**                 tag=frontend/v*
# shared       path=shared/**                   tag=libs/v*
```

---

## Integrate gg-version in a CI pipeline

### GitHub Actions

```yaml
- name: Compute version
  id: version
  run: echo "value=$(gg-version next)" >> $GITHUB_OUTPUT

- name: Build
  run: docker build -t myapp:${{ steps.version.outputs.value }} .
```

### GitLab CI

```yaml
compute-version:
  script:
    - export APP_VERSION=$(gg-version next)
    - echo "APP_VERSION=$APP_VERSION" >> build.env
  artifacts:
    reports:
      dotenv: build.env
```

### Makefile

```makefile
VERSION := $(shell gg-version next)

.PHONY: build
build:
	go build -ldflags="-X main.version=$(VERSION)" ./...
```

---

## Inject the version into a Go binary

```bash
gg-version next
# v1.4.2
```

```makefile
VERSION := $(shell gg-version next)

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

## Debug version computation

When the displayed version surprises you, inspect all variables:

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

In JSON format for CI parsing:

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
    "Semver": "1.3.0"
  }
}
```

Check the effective configuration (useful to diagnose a misread `.gg-version.yml`):

```bash
gg-version config
# config from: .gg-version.yml
# semver:
#   tag_prefix: v
#   ...
```

---

## Work with a remote repository or a subdirectory

```bash
# Repository in another directory
gg-version --repo /path/to/other-project next

# Config file in a non-standard location
gg-version --config config/versioning.yml next

# Both combined
gg-version --repo ../backend --config ../backend/.gg-version.yml next
```
````

- [ ] **Step 2: Verify**

```bash
grep "gg-version current" docs/how-to.md   # expect 0 lines
```

- [ ] **Step 3: Commit**

```bash
git add docs/how-to.md
git commit -m "docs: translate how-to guides to English"
```

---

### Task 4: Translate docs/reference.md

**Goal:** Replace the French reference with an English version. Remove the stale migration note about the removed `current` command (that note is itself obsolete).

**Files:**
- Modify: `docs/reference.md`

**Acceptance Criteria:**
- [ ] All prose is in English
- [ ] The stale note `> **Note :** La commande \`current\` a été supprimée...` is removed
- [ ] All tables, flags, and template variable descriptions are translated

**Verify:**
```bash
grep -i "commande\|affiche\|défaut\|comportement\|depuis\|aucun" docs/reference.md   # expect 0 lines
grep "current.*supprimée\|current.*removed" docs/reference.md                        # expect 0 lines
```

**Steps:**

- [ ] **Step 1: Write the English reference**

Write the following content to `docs/reference.md` (overwrite completely):

````markdown
# Reference

---

## Commands

### `next`

Prints the computed version at HEAD — whether or not it exists as a tag.

```
gg-version [global flags] next
```

**Behaviour:**
- HEAD is tagged → prints that tag
- HEAD not tagged, release branch → prints the computed next tag (`tag_prefix + semver`)
- HEAD not tagged, pre-release branch → renders the branch `format` template
- No tag found → prints `initial`

**Examples:**

```bash
gg-version next
# v1.5.0  (computed version even if HEAD is not tagged)

gg-version next --format json
# "v1.5.0"

# With a template variable
gg-version --var env=staging next
# (uses {{ .var.env }} in the format template)

# In a monorepo — all components with computed version
gg-version next
# @root        v2.2.0
# api          v0.6.0

gg-version next --format json
# {
#   "@root": "v2.2.0",
#   "api": "v0.6.0"
# }

# Filter to one component
gg-version next --component api
# v0.6.0

gg-version next --root
# v2.2.0
```

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--format <plain\|json>` | `plain` | Output format |

---

### `last`

Prints the last semver tag reachable from HEAD.

```
gg-version [global flags] last
```

**Behaviour:**
- Walks all ancestors of HEAD, filters valid tags by `tag_prefix`, returns the topologically closest one.
- No tag found → prints `initial`.
- Unlike `next`, `last` does not analyse commits: it returns the tag as-is, without computing a bump.

**Examples:**

```bash
gg-version last
# v1.4.1

# In a monorepo
gg-version last
# @root        v2.0.0
# api          v0.4.0
# frontend     v2.9.0

gg-version last --component frontend
# v2.9.0
```

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--format <plain\|json>` | `plain` | Output format |

---

### `env`

Prints all available template variables.

```
gg-version [global flags] env [--format plain|json]
```

**Behaviour:**
- In monorepo mode, prints variables for each component prefixed by its name.
- If the repository or config is inaccessible, prints only `var.*` variables.

**Examples:**

```bash
# Default format (plain)
gg-version env
# git.AuthorDate=2026-04-06
# git.Branch=main
# git.CommitCount=5
# git.CommitterDate=2026-04-06
# git.Hash=abc1234def5678901234567890abcdef12345678
# git.LastTag=v1.2.0
# git.ShortHash=abc1234
# semver.HasNonConventionalCommits=false
# semver.IsBreakingChange=false
# semver.IsPreRelease=false
# semver.LastMajor=1
# semver.LastMinor=2
# semver.LastPatch=0
# semver.LastPreRelease=
# semver.LastVersion=1.2.0
# semver.Major=1
# semver.Minor=3
# semver.Patch=0
# semver.PreRelease=
# semver.Semver=1.3.0

# JSON format
gg-version env --format json

# With a custom variable
gg-version --var buildno=42 env
# var.buildno=42

# In a monorepo (plain)
gg-version env
# @root.git.Branch=main
# @root.semver.Semver=2.1.0
# api.git.LastTag=api/v0.5.0
# api.semver.Semver=0.5.1
# ...

# In a monorepo (JSON)
gg-version env --format json
# {
#   "@root": { "git": {...}, "semver": {...} },
#   "api": { "git": {...}, "semver": {...} }
# }

gg-version env --component api --format json
```

---

### `config`

Prints the effective configuration (defaults merged with `.gg-version.yml`).

```
gg-version [global flags] config [--format yaml|json]
```

**Examples:**

```bash
gg-version config
# config from: .gg-version.yml
# semver:
#   tag_prefix: v
#   initial: 0.1.0
#   branches: ...

gg-version config --format json
# {
#   "_source": ".gg-version.yml",
#   "semver": { ... },
#   "components": { ... }
# }

# When no config file exists
gg-version config
# # default config
# semver:
#   tag_prefix: ""
#   ...
```

---

### `components`

Lists the components defined in the configuration (monorepo).

```
gg-version [global flags] components [--format plain|json]
```

**Examples:**

```bash
gg-version components
# api          path=api/**                      tag=api/v*
# frontend     path=frontend/**                 tag=frontend/v*

gg-version components --format json
# {
#   "api": {
#     "path": "api/**",
#     "tag_scope": "api",
#     "tag_pattern": "api/v*"
#   },
#   "frontend": {
#     "path": "frontend/**",
#     "tag_scope": "frontend",
#     "tag_pattern": "frontend/v*"
#   }
# }

# With no components defined
gg-version components
# (no components defined)
```

---

### `tag`

Creates an annotated tag on HEAD with the computed `next` version. In a monorepo, creates one tag per component.

```
gg-version [global flags] tag [--push] [--dry-run] [--message <msg>]
```

**Examples:**

```bash
gg-version tag
# created tag v2.2.0 on a1b2c3d

gg-version tag --dry-run
# would create tag v2.2.0 on a1b2c3d

gg-version tag --push
# created tag v2.2.0 on a1b2c3d
# pushed 1 tag(s) to origin

gg-version tag --message "release: sprint 42"
# created tag v2.2.0 on a1b2c3d
```

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--push` | false | Push tags to origin after creation (SSH agent) |
| `--dry-run` | false | Print what would happen without creating a tag |
| `--message <msg>` | `"chore: release <version>"` | Annotated tag message |

**Monorepo behaviour:**

Without `--component` or `--root`, creates a tag for each component and for `@root`:

```bash
gg-version tag --dry-run
# would create tag v2.2.0 on a1b2c3d
# would create tag api/v0.5.2 on a1b2c3d
# would create tag frontend/v3.1.0 on a1b2c3d
```

With `--component api`: creates only the `api` component tag.
With `--root`: creates only the `@root` tag.

**Errors:**

If a tag already exists, the command returns an error and stops:

```
error: creating tag v2.2.0: tag already exists
```

---

### `lint`

Verifies that commits since the last tag follow the Conventional Commits format. Returns exit 1 if violations are found.

```
gg-version [global flags] lint
```

**Behaviour:**
- No violations → empty output, exit 0
- Violations found → list on stderr, exit 1
- HEAD is exactly on a tag (no commits to analyse) → exit 0
- No tag in the repository → exit 0 (no baseline)

**Examples:**

```bash
# No violations
gg-version lint
echo $?  # 0

# Violations found
gg-version lint
# 2 commit(s) do not follow Conventional Commits since v1.2.0:
#   a1b2c3d "WIP fix auth"
#   def4567 "Merge pull request #42 from foo/bar"
echo $?  # 1

# In a monorepo — lint only the api component commits
gg-version lint --component api

# In a monorepo — lint only @root commits
gg-version lint --root
```

**Violation rule:** a commit is flagged if its subject (first line) does not match the `format` pattern of `conventional_commits` in the config. Commits with an unknown CC type (`chore:`, `style:`) are **valid** — only the format matters.

**CI integration:**

```yaml
- name: Lint commits
  run: gg-version lint
```

---

## Global flags

These flags apply to all commands and are placed before the command name.

| Flag | Default | Description |
|---|---|---|
| `--config <path>` | `.gg-version.yml` | Path to the configuration file |
| `--repo <path>` | `.` | Path to the Git repository |
| `--component <name>` | _(none)_ | Filter output to one component (monorepo) |
| `--root` | `false` | Show only the `@root` component (monorepo) |
| `--var <name=value>` | _(none)_ | Extra template variable (repeatable) |

`--component` and `--root` are mutually exclusive.

> **Shell note:** `@root` contains `@`, a special character in some shell contexts. Use `--root` (preferred) or quote the value: `--component '@root'`.

```bash
gg-version --repo /path/to/project --config /path/to/.gg-version.yml next
gg-version --component api next
gg-version --root last
gg-version --component '@root' last  # equivalent to --root
```

---

## Configuration file

Default location: `.gg-version.yml` at the repository root. If the file does not exist, defaults apply.

### Full schema

```yaml
semver:
  # Expected prefix on Git tags. Example: "v" for tags like v1.2.3.
  # Default: "" (no prefix)
  tag_prefix: "v"

  # Version returned when no tag is found.
  # Default: "0.1.0"
  initial: "0.1.0"

  # Branch rules. Evaluated in order — the first match is used.
  branches:
    - pattern: "main"        # Go regular expression
      release: true          # true → release version (no template)
      # format is ignored when release: true

    - pattern: "release/(?P<major>[0-9]+)\\.x"
      release: true

    - pattern: ".*"
      release: false
      # Go template. Available variables: {{ .semver.* }}, {{ .git.* }},
      # {{ .regex.* }}, {{ .var.* }}
      format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"

  # Conventional Commits detection rules (Go regular expressions).
  conventional_commits:
    # General CC format. A commit that matches but is not in any
    # major/minor/patch list → no bump (BumpNone).
    format: '^\w+(?:\(.+\))?!?:'

    # Patterns that trigger a MAJOR bump.
    major:
      - '^\w+(?:\(.+\))?!:'      # feat!: or fix!:
      - 'BREAKING[- ]CHANGE:'    # footer BREAKING CHANGE:

    # Patterns that trigger a MINOR bump.
    minor:
      - '^feat(?:\(.+\))?:'

    # Patterns that trigger a PATCH bump.
    patch:
      - '^fix(?:\(.+\))?:'

  # Paths to exclude from version computation (doublestar globs).
  # A commit is ignored if ALL its modified files match at least one pattern.
  # A mixed commit (docs + code) is not ignored.
  # Default: []
  ignore_paths:
    - "docs/**"
    - "*.md"
    - ".github/**"

  # Commit SHAs to ignore (short prefixes accepted).
  # Default: []
  ignore_commits:
    - "abc1234"
    - "deadbeef"

# Components for monorepos. Optional.
components:
  api:
    # Glob of files belonging to this component.
    path: "api/**"
    # Tag scope. Default: the key name ("api").
    # Tags for this component will be api/v1.2.3.
    tag_scope: "api"

  frontend:
    path: "frontend/**"
    # Without tag_scope, tags will be frontend/v1.2.3.
```

---

## Template variables

Available in the branch `format` field and via `gg-version env`.

### Namespace `semver`

| Variable | Type | Description |
|---|---|---|
| `semver.Semver` | string | Version computed by CC (e.g. `1.3.0`) |
| `semver.Major` | string | Major component of the computed version |
| `semver.Minor` | string | Minor component of the computed version |
| `semver.Patch` | string | Patch component of the computed version |
| `semver.PreRelease` | string | Pre-release of the computed version (usually empty) |
| `semver.LastVersion` | string | Last tag version (without prefix, e.g. `1.2.0`) |
| `semver.LastMajor` | string | Major of the last tag |
| `semver.LastMinor` | string | Minor of the last tag |
| `semver.LastPatch` | string | Patch of the last tag |
| `semver.LastPreRelease` | string | Pre-release of the last tag |
| `semver.IsBreakingChange` | bool | `true` if at least one MAJOR commit since the last tag |
| `semver.IsPreRelease` | bool | `true` if the current branch is not a release branch |
| `semver.HasNonConventionalCommits` | bool | `true` if at least one commit does not follow the CC format |

### Namespace `git`

| Variable | Type | Description |
|---|---|---|
| `git.Branch` | string | Current branch name |
| `git.AuthorDate` | string | Author date of the HEAD commit (format `2006-01-02`) |
| `git.CommitterDate` | string | Committer date of the HEAD commit (format `2006-01-02`) |
| `git.LastTag` | string | Last tag found (with prefix, e.g. `v1.2.0`), empty if none |
| `git.Hash` | string | Full hash of the HEAD commit |
| `git.ShortHash` | string | First 7 characters of the hash |
| `git.CommitCount` | int | Number of commits since the last tag |
| `git.IsShallow` | bool | `true` if the repository is a shallow clone (`git clone --depth=N`) |
| `git.Truncated` | bool | `true` if history was truncated before reaching the reference tag |

### Namespace `regex`

Named captures extracted from the matching branch pattern. Example with `(?P<ticket>[A-Z]+-[0-9]+)`:

| Variable | Description |
|---|---|
| `regex.ticket` | Captured value of the named group `ticket` |

### Namespace `var`

Variables injected via `--var name=value` on the command line:

```bash
gg-version --var env=staging --var buildno=42 next
```

Accessible as `{{ .var.env }}` and `{{ .var.buildno }}`.

---

## Tags in a monorepo

When components are defined, each component has its own tag namespace:

| Component | `tag_scope` | `tag_prefix` | Tag format |
|---|---|---|---|
| `api` | `api` (default) | `v` | `api/v1.2.3` |
| `frontend` | `frontend` (default) | `v` | `frontend/v1.2.3` |
| `shared` | `libs` (explicit) | `v` | `libs/v0.9.0` |

`@root` uses the global `tag_prefix` with no scope:

| Component | Tag format |
|---|---|
| `@root` | `v2.1.0` |

---

## Conventional Commits rules

Bump priority for each commit:

| Criterion | Bump |
|---|---|
| Subject or footer matches a `major` pattern | MAJOR |
| Subject matches a `minor` pattern | MINOR |
| Subject matches a `patch` pattern | PATCH |
| Subject matches the CC `format` but no pattern | None |
| Subject does not match the CC `format` | PATCH (+ `HasNonConventionalCommits=true`) |

The highest bump among all commits since the last tag determines the computed version.

---

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Error (repository not found, invalid config, unknown flag…) or `lint` violations found |

> **`lint` note:** `gg-version lint` returns `1` when non-CC commits are detected — this is the normal result of a quality check, not a tool error. Infrastructure errors (missing config, repository not found) also return `1`, but with an error message on stderr.

---

## Shell completion

`gg-version` supports native shell completion for bash, zsh, fish, and PowerShell via the `completion` sub-command.

### Installation

**bash**
```bash
echo 'source <(gg-version completion bash)' >> ~/.bashrc
source ~/.bashrc
```

**zsh**
```bash
echo 'source <(gg-version completion zsh)' >> ~/.zshrc
source ~/.zshrc
```

**fish**
```bash
gg-version completion fish > ~/.config/fish/completions/gg-version.fish
```

**PowerShell**
```powershell
gg-version completion pwsh >> $PROFILE
```

### Generating the script

```bash
gg-version completion bash # bash script
gg-version completion zsh  # zsh script
gg-version completion fish # fish script
gg-version completion pwsh # PowerShell script
```
````

- [ ] **Step 2: Verify**

```bash
grep -i "commande\|affiche\|défaut\|comportement\|depuis\|aucun" docs/reference.md   # expect 0 lines
grep "current.*supprimée\|Note.*current" docs/reference.md                           # expect 0 lines
```

- [ ] **Step 3: Commit**

```bash
git add docs/reference.md
git commit -m "docs: translate reference to English"
```

---

### Task 5: Translate docs/explanation.md

**Goal:** Replace the French explanation document with an English version. Rename the conceptual section from `last` vs `current` to `last` vs `next`, updating examples accordingly.

**Files:**
- Modify: `docs/explanation.md`

**Acceptance Criteria:**
- [ ] All prose is in English
- [ ] Section "Why `last` and `next` are different" uses `gg-version next` in examples (not `gg-version current`)
- [ ] No French words remain

**Verify:**
```bash
grep "gg-version current" docs/explanation.md   # must return 0 lines
grep -i "pourquoi\|depuis\|commit\|aucun\|dernier" docs/explanation.md   # must return 0 lines (legitimate uses of these as English words are fine, but none should appear as French)
```

**Steps:**

- [ ] **Step 1: Write the English explanation**

Write the following content to `docs/explanation.md` (overwrite completely):

````markdown
# Concepts: how gg-version computes versions

This document explains the reasoning behind how `gg-version` works. Read it if you want to understand *why* the tool behaves the way it does, not just *how* to use it.

---

## The fundamental principle: read without writing

`gg-version` never creates a tag, never makes a commit, never modifies any file. It reads the Git history and computes what the version *should* be — the decision to create a tag remains entirely yours.

This separation is deliberate. The tool can be run at any time with no side effects, making it ideal in CI: the same call produces the same result whether you are checking locally or inside a pipeline.

---

## How the version is computed

The computation happens in three phases.

### Phase 1: find the last tag

`gg-version` walks all ancestors of HEAD and identifies valid tags (those that start with the configured `tag_prefix` and contain a valid semver). It keeps the **topologically closest** tag — not the most recent in time, but the closest in the commit graph.

If multiple tags are at equal distance, the semantically highest one is kept.

If no tag is found, the `initial` value is used as the base (default `0.1.0`).

### Phase 2: analyse the commits in between

`gg-version` retrieves all commits between the last tag and HEAD (the tag itself excluded). It analyses each commit message against the Conventional Commits rules:

- The **subject** (first line) is tested against the `major`, `minor`, `patch` patterns in that order.
- **Footers** (lines after the first blank line) are also tested — this is where `BREAKING CHANGE:` is recognised.
- A commit that does not follow the CC format is treated as a patch (and sets `HasNonConventionalCommits=true`).
- A CC commit of an unknown type (e.g. `docs:`, `chore:`) contributes `BumpNone` — it is recognised but does not increment the version.

The final bump level is the maximum across all analysed commits.

### Phase 3: produce the version

The bump is applied to the last tag's version to get the computed semver (e.g. `1.3.0`). Then:

**If HEAD is exactly on a tag** → the version is that tag, without computation.

**If the branch is a release branch** (`release: true`) → the version is `tag_prefix + semver` (e.g. `v1.3.0`). This is the version you should tag.

**If the branch is a pre-release branch** (`release: false`) → the `format` template is rendered with all available variables. The resulting version identifies the build without claiming to be a release.

---

## Releases vs pre-releases

The `release: true / false` distinction is central.

A **release branch** (`main`, `master`, `release/x.y`…) produces clean versions ready to be tagged: `v1.3.0`. These versions are stable and mean "this code is ready to ship".

A **pre-release branch** (feature, hotfix, develop…) produces build identifiers: `1.3.0-feat/login.5`. These versions allow tracing a precise build without polluting the stable version namespace.

The `format` template is only rendered on pre-release branches. On a release branch it is ignored.

---

## Why `last` and `next` are different

`gg-version last` answers: *"What is the last tag that was created?"*

`gg-version next` answers: *"What version does this code represent?"*

On an untagged commit on `main` with `feat:` commits since `v1.2.0`, the answers are:

```bash
gg-version last     # v1.2.0  — the last existing tag
gg-version next     # v1.3.0  — the version this code should have
```

`last` is useful for checking what was shipped. `next` is useful for naming what is going to be shipped.

---

## How path filtering works (monorepo)

When `ignore_paths` or components are configured, commits are filtered before the CC analysis.

The exclusion rule is intentionally strict: a commit is ignored only if **all** its modified files match an exclusion pattern. A commit that touches both `docs/README.md` and `src/api.go` is not ignored — it contributes to the version even though documentation is excluded.

This rule avoids false negatives: it is better to over-count a commit than to miss it and produce a version that under-estimates the real change.

For components, the logic is symmetric: a commit belongs to a component if at least one of its files matches the component's `path` (partial inclusion).

---

## Component isolation in a monorepo

Each component lives in its own tag namespace (`{scope}/{prefix}{version}`) and its own commit namespace (filtered by `path`).

`@root` is the implicit component that represents "everything that does not touch a declared component". Its commits are those that belong to no component. This is useful for versioning global configuration, deployment scripts, or any shared code that does not deserve its own component.

Important consequence: a commit that touches two components (`api/` and `frontend/`) counts for both. This is not a bug — a shared change must be reflected in the version of both components.

---

## Go templates

Version formats use the standard Go template syntax (`text/template`). A few useful reminders:

```
{{ .semver.Major }}                       → raw value
{{ printf "%02d" .git.CommitCount }}      → numeric formatting
```

Templates have access to all variables from the four namespaces: `semver`, `git`, `regex`, `var`. A missing variable produces an empty string without error.
````

- [ ] **Step 2: Verify**

```bash
grep "gg-version current" docs/explanation.md   # expect 0 lines
```

- [ ] **Step 3: Commit**

```bash
git add docs/explanation.md
git commit -m "docs: translate explanation to English"
```

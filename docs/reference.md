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
- HEAD not tagged, pre-release branch → renders the branch `version_format` template
- No tag found → prints `initial`

**Examples:**

```bash
gg-version next
# v1.5.0  (computed version even if HEAD is not tagged)

gg-version next --format json
# "v1.5.0"

# With a template variable
gg-version --var env=staging next
# (uses {{ .var.env }} in the version_format template)

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
    - pattern: "main"        # Go regular expression; no version_format = release branch
    - pattern: "master"
    - pattern: "release/(?P<major>[0-9]+)\\.x"
      # Restrict tag search to tags matching the rendered wildcard semver.
      # Available: {{ .regex.* }} (branch captures), {{ .var.* }} (--var flags).
      # Format: N.N.N where each component is an integer or x (wildcard).
      constraint: "{{ .regex.major }}.x.x"

    - pattern: ".*"
      # Go template rendered as the version suffix (after tag_prefix).
      # Available variables: {{ .semver.* }}, {{ .git.* }}, {{ .regex.* }}, {{ .var.* }}
      # Absent or empty = release branch: output is tag_prefix + {{ .semver.Semver }}
      version_format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"

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

  # vars: default values for the .var.* template namespace.
  # Each value is a Go template rendered with .env.* (OS env vars) available.
  # CLI --var flags override config vars. The "default" pipe function is available.
  # Default: {}
  vars:
    stream: '{{ .env.STREAM | default "1" }}'
    build:  "{{ .env.CI_BUILD_NUMBER }}"
    env:    "prod"

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

Available in the branch `version_format` field and via `gg-version env`.

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
| `semver.IsPreRelease` | bool | `true` if the current branch has a non-empty `version_format` |
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

Config-level defaults can be set under `semver.vars:` — CLI `--var` always wins.

### OS environment variables (`.env.*`)

All OS environment variables are available as `{{ .env.<NAME> }}` in `version_format`, `constraint`, and `vars:` values.

```
# .env.* — all OS environment variables. {{ .env.BUILD_NUMBER }}, etc.
# Absent variables render as empty string. Use | default for fallbacks.
```

Example:

```yaml
semver:
  vars:
    stream: '{{ .env.STREAM | default "1" }}'
  branches:
    - pattern: "main"
      version_format: "{{ .semver.Semver }}+{{ .env.CI_BUILD_NUMBER }}"
```

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

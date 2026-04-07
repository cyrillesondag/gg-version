# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./cmd/gg-version
make build   # injects VERSION from git describe

# Run all tests
go test ./...

# Run tests for a specific package
go test ./git/ -v -run TestNoTag
go test ./strategy/semver/ -v -run TestFilterCommits

# Run the tool locally
go run ./cmd/gg-version current
go run ./cmd/gg-version --repo /path/to/other-repo current
```

## Architecture

`gover` (binary name: `gg-version`) is a Git-aware CLI that computes a project's version from its commit history and tags — without writing anything back to the repo.

### Package overview

```
cmd/gg-version/
  main.go                  — entry point; var Version injected by ldflags
  commands.go              — CLI definition (urfave/cli/v3), commands and flags
config/config.go           — config file loading and defaults
format/format.go           — VersionFormat interface
git/git.go                 — git layer (Project struct)
strategy/semver/
  semver.go                — Strategy, SemverFormat, varsCore
  conventional.go          — Conventional Commits analysis, FilterCommits
  component.go             — AllCurrent, AllLast, AllVars for monorepo
```

### `cmd/gg-version` package

`Run()` (in `commands.go`) builds the `urfave/cli/v3` command tree and runs it. Both files use `package main`.

**Global flags** (apply to all subcommands):
- `--config` (default `.gg-version.yml`) — config file path
- `--repo` (default `.`) — git repository path
- `--component <name>` — filter output to one component (monorepo)
- `--root` — show only `@root` (monorepo); mutually exclusive with `--component`
- `--var <name=value>` — extra template variable (repeatable)

**Commands:**
- `current` — version at HEAD
- `last` — last semver tag reachable from HEAD
- `env [--format plain|json]` — all template variables
- `config [--format yaml|json]` — effective configuration
- `components [--format plain|json]` — list monorepo components

### `config` package

`config.Load(path)` reads `.gg-version.yml`; returns `DefaultConfig()` if the file is absent.

```go
type Config struct {
    Semver     SemverConfig               `yaml:"semver"`
    Components map[string]ComponentConfig `yaml:"components"`
}

type SemverConfig struct {
    TagPrefix           string
    Initial             string
    Branches            []BranchConfig
    ConventionalCommits ConventionalCommitsConfig
    IgnorePaths         []string   // globs; commit excluded if ALL files match
    IgnoreCommits       []string   // SHA prefixes to skip entirely
}

type ComponentConfig struct {
    Path     string  // doublestar glob, e.g. "api/**"
    TagScope string  // tag prefix scope; defaults to component name
}
```

### `format` package

`VersionFormat` interface used by `git.Project.LastTag` to filter tags:

```go
type VersionFormat interface {
    IsValid(version string) bool
    Compare(version1, version2 string) (int, error)
}
```

`SemverFormat` (in `strategy/semver/semver.go`) implements this interface.

### `git` package

`Project` wraps a `go-git` repository and a HEAD commit. Key methods:

- `NewProject(path, sha string)` — opens an on-disk repo; `sha=""` resolves HEAD.
- `NewProjectFromRepo(repo, hash)` — for tests with in-memory repos.
- `LastTag(f VersionFormat)` — walks all reachable tags, filters by `f.IsValid`, returns the topologically closest ancestor. Returns `"0.0.0"` when none found.
- `IsHeadTagged(tag string)` — true if HEAD points to that tag's commit.
- `CommitSinceTag(tag string)` — commits from HEAD back to (and including) the tagged commit.
- `CommitFiles(c *object.Commit)` — files changed by a commit (all files for initial commit, stats diff otherwise).
- `BranchName()`, `CommitHash()` — metadata helpers.

### `strategy/semver` package

**`semver.go`**

`Strategy` holds `SemverConfig` and exposes:
- `Current(p, extra)` — version string at HEAD (single-entity, no components).
- `Last(p)` — last tag string.
- `Vars(p, extra)` — all template variables as `map[string]interface{}` with namespaces `semver`, `git`, `regex`, `var`.
- `varsCore(p, extra, tagPrefix, FilterConfig)` — internal parameterised implementation used by both `Vars` and the component methods; applies `FilterCommits` before CC analysis.

`SemverFormat` implements `format.VersionFormat` for semver tags with optional prefix and version constraints (used to restrict tag matching on version-locked branches).

`matchBranch(branchName)` returns the first matching `BranchConfig` and its named regex captures. Named captures from the branch pattern are available as `{{ .regex.<name> }}` in format templates.

**`conventional.go`**

`FilterCommits(commits, files, FilterConfig)` — filters a commit slice in four ordered steps:
1. SHA prefix in `IgnoreCommits` → excluded
2. `IncludePaths` set and no file matches → excluded
3. `ExcludePaths` set and **all** files match → excluded
4. Otherwise → included

Uses `github.com/bmatcuk/doublestar/v4` for glob matching (`**` support).

`AnalyzeBump(commits, ConventionalCommitsConfig)` — returns bump level (0=none, 1=patch, 2=minor, 3=major) and `hasNonCC` flag. Footer lines (after first blank line) are also tested for `BREAKING CHANGE:`.

`BumpVersion(lastTag, prefix, level)` — applies the bump and returns the new version string without prefix.

**`component.go`**

Monorepo layer on top of `varsCore`:

- `AllCurrent(p, extra, cfg)` — returns `[]ComponentResult` with `@root` first, then alphabetical components.
- `AllLast(p, cfg)` — same structure, using `lastWithPrefix` per component.
- `AllVars(p, extra, cfg)` — returns `[]ComponentVarsResult`.
- `ResolveTagPrefix(name, comp, globalPrefix)` — builds `"{scope}/{globalPrefix}"` (e.g. `"api/v"`).

When `cfg.Components` is empty, all three methods return a single result with `Name: ""` (backward-compatible with non-monorepo use).

`@root` excludes all component `Path` globs from its `FilterConfig.ExcludePaths`. Each component uses its own `Path` as `IncludePaths`.

### Testing approach

All tests use fully in-memory repos (`go-git` + `go-billy/memfs`) — no disk I/O, no real Git installation required.

Helper functions in `git/git_test.go`:
- `newRepo()` — creates an in-memory bare repo
- `projectAtHead(t, repo)` / `projectAtCommit(t, repo, hash)`
- `createCommit(t, repo, msg, files...)` — creates a commit with files
- `createTag(t, repo, hash, name)` / `createAnnotatedTag(...)`

`strategy/semver/semver_test.go` uses `fakeProject` (implements `GitProject`) to test strategy logic without a real repo.

### File naming conventions

- Config file: `.gg-version.yml`
- Worktree directory: `.worktrees/` (git-ignored)

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run tests
go test ./...

# Run a single test
go test ./git/ -run TestNoTag

# Run the tool locally (once main.go is wired up)
go run . <path-to-git-repo>
```

## Architecture

`gover` (binary name: `gg-version`) is a Git-aware CLI that computes a project's version from its commit history and tags — without writing anything back to the repo.

### Packages

- **`main` (root)** — entry point.

- **`command`** — defines the `Command` interface (`Run`) and `VersionCommand`, which implements `Command` and executes the version computation logic.

  - **`format`** — defines the `VersionFormat` interface (`IsValid`) and `SemVerFormat`,

- **`git/`** — core logic. `Project` wraps a `go-git` repository + a HEAD commit + a `VersionFormat`. Key methods:
  - `NewProject(path, sha, format)` — opens a real on-disk repo; `sha=""` resolves HEAD.
  - `LastTag()` — walks all reachable tags from HEAD, filters by `VersionFormat.IsValid`, picks the topologically closest ancestor tag. Returns `"0.0.0"` when no valid tag is found.
  - `CommitSinceTag(tag)` — returns commits from HEAD back to (and including) the tagged commit.
  - `BranchName()`, `CommitHash()` — metadata helpers.

### Intended design (from README, not yet implemented)

The README describes a `Strategy` interface:
```go
type Strategy interface {
    Current(repo *git.Repository, head *object.Commit) (string, error)
    Next(repo *git.Repository, last *semver.Version, commits []*object.Commit) (string, error)
}
```
Strategies (`semver`, `increment`) will live under `internal/strategy/`. The CLI (`current`, `next` commands) with `--strategy`, `--repo`, `--dry-run`, `--format`, `--config` flags is the planned interface.

### Testing approach

Tests in `git/git_test.go` use fully in-memory repos (`go-git` + `go-billy/memfs`) — no disk I/O, no real Git installation needed. Helper functions (`newRepo`, `projectAtHead`, `projectAtCommit`, `createCommit`, `createTag`, `createAnnotatedTag`) set up commit graphs for each scenario.

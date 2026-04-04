# Semver Strategy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the semver version strategy with `current` and `last` commands, branch-based release detection, configurable pre-release format templates, and version constraints extracted from branch name patterns.

**Architecture:** A `strategy/semver` package holds all semver logic (format validation + strategy computation), a `config` package parses `.gg-version.yaml`, and `git.Project` is decoupled from `VersionFormat` so branch-specific constraints can be injected at call time. The CLI wires everything together via `command/commands.go`.

**Tech Stack:** Go 1.25, `github.com/coreos/go-semver`, `go-git/v5`, `urfave/cli/v3`, `gopkg.in/yaml.v3`, `go-billy/v5/memfs` (tests)

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `git/git.go` | Remove Format from Project, LastTag(f) param, add IsHeadTagged, fix error returns |
| Modify | `git/git_test.go` | Update helpers and test calls to match new signatures |
| Create | `config/config.go` | Parse .gg-version.yaml → Config struct, DefaultConfig |
| Create | `config/config_test.go` | Test Load with file, missing file, and defaults |
| Create | `strategy/semver/semver.go` | SemverFormat (with constraints) + Strategy (Last, Current) |
| Create | `strategy/semver/semver_test.go` | Format validation and strategy tests with in-memory repos |
| Modify | `command/commands.go` | Implement Current/Last, add --repo flag, remove dead commands |
| Delete | `format/semver.go` | Replaced by strategy/semver/semver.go |

---

### Task 1: Decouple VersionFormat from git.Project

**Goal:** Remove `Format` from `Project` struct, pass `VersionFormat` to `LastTag()` as a parameter, add `IsHeadTagged()`, and fix `NewProject` to return errors instead of calling `log.Fatal`.

**Files:**
- Modify: `git/git.go`
- Modify: `git/git_test.go`

**Acceptance Criteria:**
- [ ] `Project` struct has no `Format` field
- [ ] `NewProject(path, sha string) (*Project, error)` — two params, returns error
- [ ] `LastTag(f format.VersionFormat) (string, error)` — format passed as param
- [ ] `IsHeadTagged(tag string) (bool, error)` method added
- [ ] All existing tests pass: `go test ./git/ -v`

**Verify:** `go test ./git/ -v` → all tests PASS

**Steps:**

- [ ] **Step 1: Write a failing test for IsHeadTagged**

Add to `git/git_test.go` (after `TestAnnotatedTag`):

```go
// TestIsHeadTagged: HEAD pointe directement sur un commit tagué → true.
func TestIsHeadTagged(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	p := projectAtHead(t, repo)

	tagged, err := p.IsHeadTagged("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !tagged {
		t.Fatal("expected HEAD to be tagged")
	}
}

// TestIsHeadTaggedFalse: HEAD est après le tag → false.
func TestIsHeadTaggedFalse(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tagged, err := p.IsHeadTagged("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if tagged {
		t.Fatal("expected HEAD not to be tagged")
	}
}
```

- [ ] **Step 2: Run to confirm it fails (won't compile)**

```bash
go test ./git/ -v -run TestIsHeadTagged
```
Expected: compilation error — `projectAtHead` signature mismatch and `IsHeadTagged` not defined.

- [ ] **Step 3: Rewrite git/git.go**

Replace the full content of `git/git.go` with:

```go
package git

import (
	"fmt"
	"gover/format"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

type Project struct {
	repo *git.Repository
	head *object.Commit
}

type Commit struct {
	LastTag string
	Message string
	Hash    string

	BranchName      string
	IsMainBranch    bool
	IsReleaseBranch bool

	lastCommits []Commit
}

func NewProject(path, sha string) (*Project, error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("opening repo at %s: %w", path, err)
	}

	var commit *object.Commit
	if sha == "" {
		head, err := r.Head()
		if err != nil {
			return nil, fmt.Errorf("reading HEAD: %w", err)
		}
		c, err := r.CommitObject(head.Hash())
		if err != nil {
			return nil, fmt.Errorf("reading HEAD commit: %w", err)
		}
		commit = c
	} else {
		rev, err := r.ResolveRevision(plumbing.Revision(sha))
		if err != nil {
			return nil, fmt.Errorf("resolving revision %s: %w", sha, err)
		}
		c, err := r.CommitObject(*rev)
		if err != nil {
			return nil, fmt.Errorf("reading commit for %s: %w", sha, err)
		}
		commit = c
	}

	return &Project{r, commit}, nil
}

// LastTag walks all reachable tags from HEAD, filters by f.IsValid, and returns
// the topologically closest ancestor tag. Returns "0.0.0" when no valid tag is found.
func (p Project) LastTag(f format.VersionFormat) (string, error) {
	repo := p.repo

	tagsRef, err := repo.Tags()
	if err != nil {
		return "", fmt.Errorf("listing tags: %w", err)
	}
	defer tagsRef.Close()

	var lastTag *plumbing.Reference
	var lastTagCommit *object.Commit

	err = tagsRef.ForEach(func(tagRef *plumbing.Reference) error {
		if !f.IsValid(tagRef.Name().Short()) {
			return nil
		}

		tagCommit, err := getCommitFromTag(repo, tagRef)
		if err != nil {
			return nil
		}

		reachable, err := isAncestor(p.head, tagCommit)
		if err != nil || !reachable {
			return nil
		}

		if lastTag == nil {
			lastTag = tagRef
			lastTagCommit = tagCommit
			return nil
		}

		candidateIsNewer, _ := isAncestor(tagCommit, lastTagCommit)
		if candidateIsNewer {
			lastTag = tagRef
			lastTagCommit = tagCommit
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	if lastTag == nil {
		return "0.0.0", nil
	}
	return lastTag.Name().Short(), nil
}

// IsHeadTagged reports whether the HEAD commit is the same commit pointed to by tag.
func (p Project) IsHeadTagged(tag string) (bool, error) {
	ref, err := p.repo.Tag(tag)
	if err != nil {
		return false, nil // tag not found
	}
	tagCommit, err := getCommitFromTag(p.repo, ref)
	if err != nil {
		return false, err
	}
	return p.head.Hash == tagCommit.Hash, nil
}

func (p Project) CommitSinceTag(tag string) ([]*object.Commit, error) {
	ref, err := p.repo.Tag(tag)
	if err != nil {
		return nil, fmt.Errorf("tag %q not found: %w", tag, err)
	}

	ancestor, err := getCommitFromTag(p.repo, ref)
	if err != nil {
		return nil, err
	}

	iter := object.NewCommitPreorderIter(p.head, nil, nil)
	defer iter.Close()

	var history []*object.Commit
	err = iter.ForEach(func(c *object.Commit) error {
		history = append(history, c)
		if c.Hash == ancestor.Hash {
			return storer.ErrStop
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return history, nil
}

func (p Project) BranchName() (string, error) {
	head, err := p.repo.Head()
	if err != nil {
		return "", err
	}
	return string(head.Name()), nil
}

func (p Project) CommitHash() (string, error) {
	return p.head.Hash.String(), nil
}

func getCommitFromTag(repo *git.Repository, tagRef *plumbing.Reference) (*object.Commit, error) {
	obj, err := repo.TagObject(tagRef.Hash())
	if err != nil {
		return repo.CommitObject(tagRef.Hash())
	}
	return obj.Commit()
}

func isAncestor(commit, ancestor *object.Commit) (bool, error) {
	if commit.Hash == ancestor.Hash {
		return true, nil
	}

	iter := object.NewCommitPreorderIter(commit, nil, nil)
	defer iter.Close()

	found := false
	err := iter.ForEach(func(c *object.Commit) error {
		if c.Hash == ancestor.Hash {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	if err != nil {
		return false, err
	}

	return found, nil
}
```

- [ ] **Step 4: Rewrite git/git_test.go**

Replace the full content of `git/git_test.go` with:

```go
package git

import (
	"gover/format"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/go-git/go-billy/v5/memfs"
)

func newRepo(t *testing.T) *git.Repository {
	t.Helper()
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	createCommit(t, repo)
	return repo
}

func projectAtHead(t *testing.T, repo *git.Repository) *Project {
	t.Helper()
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	return &Project{repo, commit}
}

func projectAtCommit(t *testing.T, repo *git.Repository, hash plumbing.Hash) *Project {
	t.Helper()
	commit, err := repo.CommitObject(hash)
	if err != nil {
		t.Fatal(err)
	}
	return &Project{repo, commit}
}

func createAnnotatedTag(t *testing.T, r *git.Repository, tag string) plumbing.Hash {
	t.Helper()
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	tagRef, err := r.CreateTag(tag, head.Hash(), &git.CreateTagOptions{
		Message: tag,
		Tagger: &object.Signature{
			Name:  "tagger",
			Email: "tagger@test.local",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return tagRef.Hash()
}

func createCommit(t *testing.T, r *git.Repository) plumbing.Hash {
	t.Helper()
	wt, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	rm, err := wt.Filesystem.Create("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, err = rm.Write([]byte("foo text"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = wt.Add("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	author := object.Signature{
		Name:  "go-git",
		Email: "go-git@fake.local",
		When:  time.Now(),
	}
	h, err := wt.Commit("test commit message", &git.CommitOptions{
		All:               true,
		Author:            &author,
		Committer:         &author,
		AllowEmptyCommits: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func createTag(t *testing.T, r *git.Repository, tag string) plumbing.Hash {
	t.Helper()
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	tagRef, err := r.CreateTag(tag, head.Hash(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return tagRef.Hash()
}

func semverFormat(prefix string) format.VersionFormat {
	return format.NewSemverFormat(prefix)
}

// TestNoTag: aucun tag dans le repo → "0.0.0".
func TestNoTag(t *testing.T) {
	repo := newRepo(t)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "0.0.0" {
		t.Fatalf("expected 0.0.0, got %s", tag)
	}
}

// TestSingleTag: un seul tag dans l'historique → retourné.
func TestSingleTag(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "1.0.0" {
		t.Fatalf("expected 1.0.0, got %s", tag)
	}
}

// TestBranchTags: le tag "1.2.0" créé sur my-branch ne doit PAS être visible
// depuis le commit qui existait avant la création de my-branch.
func TestBranchTags(t *testing.T) {
	f := semverFormat("")
	repo := newRepo(t)

	createTag(t, repo, "1.0.0")

	if err := repo.CreateBranch(&config.Branch{Name: "test"}); err != nil {
		t.Fatal("expected branch creation to succeed:", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	createCommit(t, repo)
	createTag(t, repo, "1.1.0")

	p := projectAtHead(t, repo)

	headRef, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}

	branchName := plumbing.NewBranchReferenceName("my-branch")
	branchRef := plumbing.NewHashReference(branchName, headRef.Hash())
	if err := repo.Storer.SetReference(branchRef); err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: branchName}); err != nil {
		t.Fatal(err)
	}

	createCommit(t, repo)
	createTag(t, repo, "1.2.0")

	tag, err := p.LastTag(f)
	if err != nil {
		t.Fatal(err)
	}

	if tag != "1.1.0" {
		t.Fatalf("expected 1.1.0, got %s", tag)
	}
}

// TestTagOnCurrentCommit: HEAD est directement tagué → le tag doit être retourné.
func TestTagOnCurrentCommit(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "2.0.0")
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %s", tag)
	}
}

// TestMultipleTagsMostRecentReturned: plusieurs tags → le plus récent ancêtre est retourné.
func TestMultipleTagsMostRecentReturned(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createTag(t, repo, "2.0.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %s", tag)
	}
}

// TestTagWithVPrefix: format avec préfixe "v".
func TestTagWithVPrefix(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "v1.5.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFormat("v"))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.5.0" {
		t.Fatalf("expected v1.5.0, got %s", tag)
	}
}

// TestInvalidTagsIgnored: les tags non-semver sont ignorés.
func TestInvalidTagsIgnored(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "release-candidate")
	createCommit(t, repo)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "1.0.0" {
		t.Fatalf("expected 1.0.0, got %s", tag)
	}
}

// TestAllTagsInvalidFallsBackToDefault: tous les tags invalides → "0.0.0".
func TestAllTagsInvalidFallsBackToDefault(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "latest")
	createTag(t, repo, "beta")
	createTag(t, repo, "release-1")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "0.0.0" {
		t.Fatalf("expected 0.0.0, got %s", tag)
	}
}

// TestAnnotatedTag: les tags annotés sont correctement résolus.
func TestAnnotatedTag(t *testing.T) {
	repo := newRepo(t)
	createAnnotatedTag(t, repo, "3.0.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "3.0.0" {
		t.Fatalf("expected 3.0.0, got %s", tag)
	}
}

// TestDescendantTagIgnored: un tag posé après HEAD ne doit pas apparaître.
func TestDescendantTagIgnored(t *testing.T) {
	repo := newRepo(t)
	headHash := createCommit(t, repo)
	createCommit(t, repo)
	createTag(t, repo, "1.0.0")

	p := projectAtCommit(t, repo, headHash)
	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "0.0.0" {
		t.Fatalf("expected 0.0.0, got %s", tag)
	}
}

// TestTagOnDivergentBranchIgnored: un tag sur une branche divergente est ignoré.
func TestTagOnDivergentBranchIgnored(t *testing.T) {
	repo := newRepo(t)
	masterHash := createCommit(t, repo)

	featureBranch := plumbing.NewBranchReferenceName("feature")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(featureBranch, masterHash)); err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: featureBranch}); err != nil {
		t.Fatal(err)
	}

	createCommit(t, repo)
	createTag(t, repo, "1.0.0")

	p := projectAtCommit(t, repo, masterHash)
	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "0.0.0" {
		t.Fatalf("expected 0.0.0, got %s", tag)
	}
}

// TestPrereleaseTagIgnoredWhenStableExists: le tag pre-release récent est retourné.
func TestPrereleaseTagIgnoredWhenStableExists(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createTag(t, repo, "2.0.0-alpha.1")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFormat(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "2.0.0-alpha.1" {
		t.Fatalf("expected 2.0.0-alpha.1, got %s", tag)
	}
}

// TestCommitSinceTagCount: CommitSinceTag doit retourner commits entre HEAD et tag inclus.
func TestCommitSinceTagCount(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	commits, err := p.CommitSinceTag("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(commits))
	}
}

// TestCommitSinceTagMessages: le premier commit est HEAD.
func TestCommitSinceTagMessages(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createCommit(t, repo)
	headHash := createCommit(t, repo)
	p := projectAtHead(t, repo)

	commits, err := p.CommitSinceTag("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least one commit")
	}
	if commits[0].Hash != headHash {
		t.Fatalf("first commit should be HEAD (%s), got %s", headHash, commits[0].Hash)
	}
}

// TestIsHeadTagged: HEAD pointe directement sur un commit tagué → true.
func TestIsHeadTagged(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	p := projectAtHead(t, repo)

	tagged, err := p.IsHeadTagged("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !tagged {
		t.Fatal("expected HEAD to be tagged")
	}
}

// TestIsHeadTaggedFalse: HEAD est après le tag → false.
func TestIsHeadTaggedFalse(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tagged, err := p.IsHeadTagged("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if tagged {
		t.Fatal("expected HEAD not to be tagged")
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./git/ -v
```
Expected: all tests PASS (including new `TestIsHeadTagged` and `TestIsHeadTaggedFalse`).

- [ ] **Step 6: Delete format/semver.go** — wait until Task 3 is complete. Skip for now.

- [ ] **Step 7: Commit**

```bash
git add git/git.go git/git_test.go
git commit -m "refactor(git): decouple VersionFormat from Project, add IsHeadTagged"
```

---

### Task 2: Create config package

**Goal:** Parse `.gg-version.yaml` into a typed `Config` struct; return `DefaultConfig` when the file is absent.

**Files:**
- Create: `config/config.go`
- Create: `config/config_test.go`

**Acceptance Criteria:**
- [ ] `config.Load(path)` returns `DefaultConfig` when file does not exist
- [ ] `config.Load(path)` parses a valid YAML file correctly
- [ ] `DefaultConfig()` returns sensible defaults (initial `0.1.0`, catch-all pre-release pattern)
- [ ] `go test ./config/ -v` → all tests PASS

**Verify:** `go test ./config/ -v` → PASS

**Steps:**

- [ ] **Step 1: Add yaml dependency**

```bash
go get gopkg.in/yaml.v3
```

Expected: `go.mod` and `go.sum` updated.

- [ ] **Step 2: Write failing tests**

Create `config/config_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"gover/config"
)

func TestDefaultConfigWhenFileMissing(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/.gg-version.yaml")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Semver.Initial != "0.1.0" {
		t.Fatalf("expected initial 0.1.0, got %s", cfg.Semver.Initial)
	}
	if cfg.Semver.TagPrefix != "" {
		t.Fatalf("expected empty tag_prefix, got %s", cfg.Semver.TagPrefix)
	}
	if len(cfg.Semver.Branches) == 0 {
		t.Fatal("expected at least one default branch pattern")
	}
}

func TestLoadValidYAML(t *testing.T) {
	content := `
semver:
  tag_prefix: "v"
  initial: "1.0.0"
  branches:
    - pattern: "^refs/heads/main$"
      release: true
    - pattern: ".*"
      release: false
      format: "{{ .LastTag }}-dev.{{ .CommitCount }}"
`
	dir := t.TempDir()
	path := filepath.Join(dir, ".gg-version.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Semver.TagPrefix != "v" {
		t.Fatalf("expected tag_prefix v, got %s", cfg.Semver.TagPrefix)
	}
	if cfg.Semver.Initial != "1.0.0" {
		t.Fatalf("expected initial 1.0.0, got %s", cfg.Semver.Initial)
	}
	if len(cfg.Semver.Branches) != 2 {
		t.Fatalf("expected 2 branch configs, got %d", len(cfg.Semver.Branches))
	}
	if !cfg.Semver.Branches[0].Release {
		t.Fatal("expected first branch to be release")
	}
	if cfg.Semver.Branches[1].Release {
		t.Fatal("expected second branch to be pre-release")
	}
	if cfg.Semver.Branches[1].Format != "{{ .LastTag }}-dev.{{ .CommitCount }}" {
		t.Fatalf("unexpected format: %s", cfg.Semver.Branches[1].Format)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gg-version.yaml")
	if err := os.WriteFile(path, []byte(":::invalid yaml:::"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}
```

- [ ] **Step 3: Run to confirm it fails**

```bash
go test ./config/ -v
```
Expected: compilation error — package `config` does not exist yet.

- [ ] **Step 4: Create config/config.go**

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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
	Format  string `yaml:"format"` // ignored when Release is true
}

// DefaultConfig returns a Config with sensible defaults.
// It is used when no .gg-version.yaml file is found.
func DefaultConfig() Config {
	return Config{
		Semver: SemverConfig{
			TagPrefix: "",
			Initial:   "0.1.0",
			Branches: []BranchConfig{
				{
					Pattern: ".*",
					Release: false,
					Format:  "{{ .LastTag }}-{{ .Branch }}.{{ .CommitCount }}",
				},
			},
		},
	}
}

// Load reads the YAML config file at path and returns a Config.
// If the file does not exist, DefaultConfig is returned without error.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("reading config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return cfg, nil
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./config/ -v
```
Expected: all 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add config/config.go config/config_test.go go.mod go.sum
git commit -m "feat(config): add YAML config loader with defaults"
```

---

### Task 3: Implement SemverFormat with version constraints

**Goal:** Implement `SemverFormat` in `strategy/semver` package — validates semver tags, supports a `Prefix` and `Constraints` map (`major`/`minor`/`patch`) that restrict which tag versions are accepted.

**Files:**
- Create: `strategy/semver/semver.go` (SemverFormat only, Strategy added in Task 4)
- Create: `strategy/semver/semver_test.go` (format tests only)

**Acceptance Criteria:**
- [ ] `SemverFormat.IsValid("1.0.0")` returns true for valid semver
- [ ] `SemverFormat.IsValid("not-a-version")` returns false
- [ ] `SemverFormat` with `Prefix: "v"` accepts `"v1.0.0"` and rejects `"1.0.0"`
- [ ] Constraint `major: "1"` accepts `"1.2.3"` and rejects `"2.0.0"`
- [ ] Constraint `major: "1", minor: "2"` accepts `"1.2.3"` and rejects `"1.3.0"`
- [ ] `SemverFormat` implements `format.VersionFormat` (compile-time check)
- [ ] `go test ./strategy/semver/ -v` → all tests PASS

**Verify:** `go test ./strategy/semver/ -v` → PASS

**Steps:**

- [ ] **Step 1: Write failing format tests**

Create `strategy/semver/semver_test.go`:

```go
package semver_test

import (
	"testing"

	semverstrategy "gover/strategy/semver"
)

func TestIsValidBasicSemver(t *testing.T) {
	f := semverstrategy.NewSemverFormat("", nil)
	cases := []struct {
		version string
		want    bool
	}{
		{"1.0.0", true},
		{"0.1.0", true},
		{"1.2.3", true},
		{"1.0.0-alpha.1", true},
		{"not-a-version", false},
		{"v1.0.0", false}, // no prefix configured
		{"", false},
	}
	for _, tc := range cases {
		got := f.IsValid(tc.version)
		if got != tc.want {
			t.Errorf("IsValid(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}

func TestIsValidWithPrefix(t *testing.T) {
	f := semverstrategy.NewSemverFormat("v", nil)
	if !f.IsValid("v1.0.0") {
		t.Error("expected v1.0.0 to be valid with prefix v")
	}
	if f.IsValid("1.0.0") {
		t.Error("expected 1.0.0 to be invalid without prefix v")
	}
}

func TestIsValidWithMajorConstraint(t *testing.T) {
	f := semverstrategy.NewSemverFormat("", map[string]string{"major": "1"})
	if !f.IsValid("1.0.0") {
		t.Error("expected 1.0.0 to be valid (major=1)")
	}
	if !f.IsValid("1.5.3") {
		t.Error("expected 1.5.3 to be valid (major=1)")
	}
	if f.IsValid("2.0.0") {
		t.Error("expected 2.0.0 to be invalid (major constraint is 1)")
	}
}

func TestIsValidWithMajorMinorConstraint(t *testing.T) {
	f := semverstrategy.NewSemverFormat("", map[string]string{"major": "1", "minor": "2"})
	if !f.IsValid("1.2.0") {
		t.Error("expected 1.2.0 to be valid (major=1,minor=2)")
	}
	if !f.IsValid("1.2.9") {
		t.Error("expected 1.2.9 to be valid (major=1,minor=2)")
	}
	if f.IsValid("1.3.0") {
		t.Error("expected 1.3.0 to be invalid (minor constraint is 2)")
	}
	if f.IsValid("2.2.0") {
		t.Error("expected 2.2.0 to be invalid (major constraint is 1)")
	}
}

func TestCompare(t *testing.T) {
	f := semverstrategy.NewSemverFormat("", nil)
	cmp, err := f.Compare("1.0.0", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if cmp >= 0 {
		t.Errorf("expected 1.0.0 < 2.0.0, got %d", cmp)
	}

	cmp, err = f.Compare("2.0.0", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if cmp <= 0 {
		t.Errorf("expected 2.0.0 > 1.0.0, got %d", cmp)
	}

	cmp, err = f.Compare("1.0.0", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if cmp != 0 {
		t.Errorf("expected 1.0.0 == 1.0.0, got %d", cmp)
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./strategy/semver/ -v
```
Expected: compilation error — package `strategy/semver` does not exist.

- [ ] **Step 3: Create strategy/semver/semver.go (SemverFormat only)**

```go
package semver

import (
	"fmt"
	"strconv"
	"strings"

	gosemver "github.com/coreos/go-semver/semver"

	"gover/format"
)

// SemverFormat validates semver tags, optionally requiring a prefix and
// enforcing version component constraints (major, minor, patch).
type SemverFormat struct {
	Prefix      string
	Constraints map[string]string // e.g. {"major": "1", "minor": "2"}
}

// NewSemverFormat creates a SemverFormat. Pass nil constraints for no restriction.
func NewSemverFormat(prefix string, constraints map[string]string) SemverFormat {
	if constraints == nil {
		constraints = map[string]string{}
	}
	return SemverFormat{Prefix: prefix, Constraints: constraints}
}

// IsValid returns true if version is a valid semver (with the configured prefix)
// and satisfies all component constraints.
func (s SemverFormat) IsValid(version string) bool {
	v, err := s.parse(version)
	if err != nil {
		return false
	}
	if maj, ok := s.Constraints["major"]; ok {
		if strconv.FormatInt(v.Major, 10) != maj {
			return false
		}
	}
	if min, ok := s.Constraints["minor"]; ok {
		if strconv.FormatInt(v.Minor, 10) != min {
			return false
		}
	}
	if patch, ok := s.Constraints["patch"]; ok {
		if strconv.FormatInt(v.Patch, 10) != patch {
			return false
		}
	}
	return true
}

// Compare returns 0 if equal, negative if version1 < version2, positive if version1 > version2.
func (s SemverFormat) Compare(version1, version2 string) (int, error) {
	v1, err := s.parse(version1)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: %w", version1, err)
	}
	v2, err := s.parse(version2)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: %w", version2, err)
	}
	return v1.Compare(*v2), nil
}

func (s SemverFormat) parse(version string) (*gosemver.Version, error) {
	trimmed := strings.TrimPrefix(version, s.Prefix)
	if trimmed == version && s.Prefix != "" {
		// Prefix was expected but not found
		return nil, fmt.Errorf("version %q does not start with prefix %q", version, s.Prefix)
	}
	return gosemver.NewVersion(trimmed)
}

// compile-time check that SemverFormat implements format.VersionFormat
var _ format.VersionFormat = SemverFormat{}
```

- [ ] **Step 4: Run tests**

```bash
go test ./strategy/semver/ -v
```
Expected: all format tests PASS.

- [ ] **Step 5: Commit**

```bash
git add strategy/semver/semver.go strategy/semver/semver_test.go
git commit -m "feat(semver): add SemverFormat with prefix and version constraints"
```

---

### Task 4: Implement Strategy — Last() and Current()

**Goal:** Add `Strategy` to `strategy/semver/semver.go` with `Last()` and `Current()` methods. Test using in-memory repos.

**Files:**
- Modify: `strategy/semver/semver.go` (add Strategy)
- Modify: `strategy/semver/semver_test.go` (add strategy tests)

**Acceptance Criteria:**
- [ ] `Last()` returns the last valid semver tag reachable from HEAD, respecting branch constraints
- [ ] `Last()` returns `cfg.Initial` when no tag is found
- [ ] `Current()` returns the exact tag when HEAD is tagged
- [ ] `Current()` returns the last tag on release branches (untagged HEAD)
- [ ] `Current()` returns a rendered format template on pre-release branches
- [ ] Branch with `major=1` constraint ignores `2.x.x` tags
- [ ] `go test ./strategy/semver/ -v` → all tests PASS

**Verify:** `go test ./strategy/semver/ -v` → PASS

**Steps:**

- [ ] **Step 1: Write failing strategy tests**

Add to `strategy/semver/semver_test.go` (after existing tests):

```go
package semver_test

import (
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"

	"gover/config"
	gitpkg "gover/git"
	semverstrategy "gover/strategy/semver"
)

// ── in-memory repo helpers ────────────────────────────────────────────────────

func newRepo(t *testing.T) *gogit.Repository {
	t.Helper()
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	createCommit(t, repo)
	return repo
}

func projectAtHead(t *testing.T, repo *gogit.Repository) *gitpkg.Project {
	t.Helper()
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	p, err := gitpkg.NewProject("", head.Hash().String())
	// NewProject opens a path-based repo; for tests we need to construct directly.
	// Use the exported test constructor instead.
	_ = p
	// Build directly since NewProject needs a path. Use helper:
	return projectFromRepo(t, repo, head.Hash())
}

func projectFromRepo(t *testing.T, repo *gogit.Repository, hash plumbing.Hash) *gitpkg.Project {
	t.Helper()
	p, err := gitpkg.NewProjectFromRepo(repo, hash)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func createCommit(t *testing.T, r *gogit.Repository) plumbing.Hash {
	t.Helper()
	wt, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	f, err := wt.Filesystem.Create("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("content"))
	_, _ = wt.Add("foo.txt")
	author := object.Signature{Name: "test", Email: "test@test.local", When: time.Now()}
	h, err := wt.Commit("test commit", &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func createTag(t *testing.T, r *gogit.Repository, tag string) {
	t.Helper()
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateTag(tag, head.Hash(), nil); err != nil {
		t.Fatal(err)
	}
}

func headHash(t *testing.T, r *gogit.Repository) plumbing.Hash {
	t.Helper()
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	return head.Hash()
}

// ── helpers to build strategy ─────────────────────────────────────────────────

func mainBranchConfig() config.SemverConfig {
	return config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: "^refs/heads/main$", Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .LastTag }}-{{ .Branch }}.{{ .CommitCount }}"},
		},
	}
}

// ── strategy tests ────────────────────────────────────────────────────────────

func TestLastReturnsInitialWhenNoTag(t *testing.T) {
	repo := newRepo(t)
	p := projectFromRepo(t, repo, headHash(t, repo))
	s := semverstrategy.NewStrategy(mainBranchConfig())

	version, err := s.Last(p)
	if err != nil {
		t.Fatal(err)
	}
	if version != "0.1.0" {
		t.Fatalf("expected 0.1.0 (initial), got %s", version)
	}
}

func TestLastReturnsLastTag(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	createCommit(t, repo)
	p := projectFromRepo(t, repo, headHash(t, repo))
	s := semverstrategy.NewStrategy(mainBranchConfig())

	version, err := s.Last(p)
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %s", version)
	}
}

func TestLastRespectsMajorConstraint(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createTag(t, repo, "2.0.0")
	createCommit(t, repo)

	p := projectFromRepo(t, repo, headHash(t, repo))

	// Branch pattern with major=1 constraint
	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: `^refs/heads/release/(?P<major>\d+)\.x$`, Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .LastTag }}-dev.{{ .CommitCount }}"},
		},
	}
	s := semverstrategy.NewStrategy(cfg)

	// Simulate being on refs/heads/release/1.x by using a project that reports that branch.
	// Since in-memory repos can't easily set branch names to arbitrary values,
	// we test the constraint logic directly via a mock branch name project.
	// Instead, test with Last() where we pass a project on the right branch.
	// Use projectWithBranch helper:
	p2 := projectFromRepoWithBranch(t, repo, headHash(t, repo), "refs/heads/release/1.x")
	version, err := s.Last(p2)
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.0.0" {
		t.Fatalf("expected 1.0.0 (major=1 constraint, ignores 2.0.0), got %s", version)
	}
}

func TestCurrentReturnsTagWhenHeadIsTagged(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.5.0")
	p := projectFromRepo(t, repo, headHash(t, repo))
	s := semverstrategy.NewStrategy(mainBranchConfig())

	version, err := s.Current(p)
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.5.0" {
		t.Fatalf("expected 1.5.0 (HEAD is tagged), got %s", version)
	}
}

func TestCurrentReturnsLastTagOnReleaseBranchUntagged(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	p := projectFromRepo(t, repo, headHash(t, repo))
	// Simulate main branch
	p2 := projectFromRepoWithBranch(t, repo, headHash(t, repo), "refs/heads/main")
	s := semverstrategy.NewStrategy(mainBranchConfig())

	_ = p
	version, err := s.Current(p2)
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.0.0" {
		t.Fatalf("expected 1.0.0 (release branch, untagged HEAD), got %s", version)
	}
}

func TestCurrentReturnsFormattedVersionOnPreReleaseBranch(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createCommit(t, repo)
	// Simulate feature branch (pre-release)
	p := projectFromRepoWithBranch(t, repo, headHash(t, repo), "refs/heads/feature/my-feat")
	s := semverstrategy.NewStrategy(mainBranchConfig())

	version, err := s.Current(p)
	if err != nil {
		t.Fatal(err)
	}
	// Expected: "1.0.0-feature/my-feat.2" (2 commits after tag, branch = short name)
	// Short branch = "feature/my-feat"
	expected := "1.0.0-feature/my-feat.2"
	if version != expected {
		t.Fatalf("expected %s, got %s", expected, version)
	}
}

func TestCurrentReturnsInitialWhenNoTag(t *testing.T) {
	repo := newRepo(t)
	p := projectFromRepoWithBranch(t, repo, headHash(t, repo), "refs/heads/main")
	s := semverstrategy.NewStrategy(mainBranchConfig())

	version, err := s.Current(p)
	if err != nil {
		t.Fatal(err)
	}
	if version != "0.1.0" {
		t.Fatalf("expected 0.1.0 (no tag, initial), got %s", version)
	}
}
```

Note: the tests above use `gitpkg.NewProjectFromRepo` — a constructor that takes an in-memory `*git.Repository` and a commit hash directly (bypassing `PlainOpen`). This must be added to `git/git.go` in the same step.

- [ ] **Step 2: Add NewProjectFromRepo and projectFromRepoWithBranch to git package**

Add to `git/git.go`:

```go
// NewProjectFromRepo builds a Project from an already-opened repository and a specific commit.
// Used in tests with in-memory repositories.
func NewProjectFromRepo(repo *git.Repository, hash plumbing.Hash) (*Project, error) {
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("reading commit %s: %w", hash, err)
	}
	return &Project{repo, commit}, nil
}
```

Also add `ProjectWithOverrideBranch` to support tests that need a specific branch name without a real checkout:

```go
// ProjectWithOverrideBranch wraps a Project and overrides the branch name returned by BranchName.
// Used in tests to simulate being on a named branch with an in-memory repo.
type ProjectWithOverrideBranch struct {
	*Project
	branch string
}

func NewProjectWithBranch(p *Project, branch string) *ProjectWithOverrideBranch {
	return &ProjectWithOverrideBranch{p, branch}
}

func (p *ProjectWithOverrideBranch) BranchName() (string, error) {
	return p.branch, nil
}
```

And update `strategy/semver` to accept an interface instead of `*git.Project`:

Actually, this requires a `GitProject` interface. Let me use a simpler approach: add `NewProjectFromRepo` to git package (public, usable in other package tests), and for branch override in tests use a wrapper type also in `git` package.

The cleaner approach for `Strategy` methods: accept a `GitProject` interface:

```go
// In strategy/semver/semver.go:
type GitProject interface {
    LastTag(f format.VersionFormat) (string, error)
    IsHeadTagged(tag string) (bool, error)
    CommitSinceTag(tag string) ([]*object.Commit, error)
    BranchName() (string, error)
    CommitHash() (string, error)
}
```

`*git.Project` satisfies this interface automatically. Tests can use a fake implementation.

Add to `git/git.go`:

```go
// NewProjectFromRepo builds a Project directly from an in-memory repository.
// Primarily used in tests.
func NewProjectFromRepo(repo *git.Repository, hash plumbing.Hash) (*Project, error) {
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("reading commit %s: %w", hash, err)
	}
	return &Project{repo, commit}, nil
}
```

- [ ] **Step 3: Rewrite strategy/semver/semver_test.go with the interface-based approach**

Replace the strategy test section with:

```go
// fakeProject is a test double for GitProject.
type fakeProject struct {
	repo   *gogit.Repository
	hash   plumbing.Hash
	branch string // overrides the branch name
}

func newFakeProject(t *testing.T, repo *gogit.Repository, branch string) *fakeProject {
	t.Helper()
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	return &fakeProject{repo: repo, hash: head.Hash(), branch: branch}
}

func newFakeProjectAt(repo *gogit.Repository, hash plumbing.Hash, branch string) *fakeProject {
	return &fakeProject{repo: repo, hash: hash, branch: branch}
}

func (fp *fakeProject) BranchName() (string, error) {
	return fp.branch, nil
}

func (fp *fakeProject) CommitHash() (string, error) {
	return fp.hash.String(), nil
}

func (fp *fakeProject) LastTag(f gitformat.VersionFormat) (string, error) {
	p, err := gitpkg.NewProjectFromRepo(fp.repo, fp.hash)
	if err != nil {
		return "", err
	}
	return p.LastTag(f)
}

func (fp *fakeProject) IsHeadTagged(tag string) (bool, error) {
	p, err := gitpkg.NewProjectFromRepo(fp.repo, fp.hash)
	if err != nil {
		return false, err
	}
	return p.IsHeadTagged(tag)
}

func (fp *fakeProject) CommitSinceTag(tag string) ([]*object.Commit, error) {
	p, err := gitpkg.NewProjectFromRepo(fp.repo, fp.hash)
	if err != nil {
		return nil, err
	}
	return p.CommitSinceTag(tag)
}
```

The full `strategy/semver/semver_test.go` (complete file — replaces what was written in Step 1):

```go
package semver_test

import (
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"

	"gover/config"
	gitformat "gover/format"
	gitpkg "gover/git"
	semverstrategy "gover/strategy/semver"
)

// ── SemverFormat tests ────────────────────────────────────────────────────────

func TestIsValidBasicSemver(t *testing.T) {
	f := semverstrategy.NewSemverFormat("", nil)
	cases := []struct {
		version string
		want    bool
	}{
		{"1.0.0", true},
		{"0.1.0", true},
		{"1.2.3", true},
		{"1.0.0-alpha.1", true},
		{"not-a-version", false},
		{"v1.0.0", false},
		{"", false},
	}
	for _, tc := range cases {
		got := f.IsValid(tc.version)
		if got != tc.want {
			t.Errorf("IsValid(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}

func TestIsValidWithPrefix(t *testing.T) {
	f := semverstrategy.NewSemverFormat("v", nil)
	if !f.IsValid("v1.0.0") {
		t.Error("expected v1.0.0 to be valid with prefix v")
	}
	if f.IsValid("1.0.0") {
		t.Error("expected 1.0.0 to be invalid without prefix v")
	}
}

func TestIsValidWithMajorConstraint(t *testing.T) {
	f := semverstrategy.NewSemverFormat("", map[string]string{"major": "1"})
	if !f.IsValid("1.0.0") {
		t.Error("expected 1.0.0 valid (major=1)")
	}
	if f.IsValid("2.0.0") {
		t.Error("expected 2.0.0 invalid (major constraint is 1)")
	}
}

func TestIsValidWithMajorMinorConstraint(t *testing.T) {
	f := semverstrategy.NewSemverFormat("", map[string]string{"major": "1", "minor": "2"})
	if !f.IsValid("1.2.0") {
		t.Error("expected 1.2.0 valid")
	}
	if f.IsValid("1.3.0") {
		t.Error("expected 1.3.0 invalid (minor constraint is 2)")
	}
}

func TestCompare(t *testing.T) {
	f := semverstrategy.NewSemverFormat("", nil)
	cmp, err := f.Compare("1.0.0", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if cmp >= 0 {
		t.Errorf("expected 1.0.0 < 2.0.0, got %d", cmp)
	}
	cmp, err = f.Compare("1.0.0", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if cmp != 0 {
		t.Errorf("expected equal, got %d", cmp)
	}
}

// ── In-memory repo helpers ────────────────────────────────────────────────────

func newRepo(t *testing.T) *gogit.Repository {
	t.Helper()
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	createCommit(t, repo)
	return repo
}

func createCommit(t *testing.T, r *gogit.Repository) plumbing.Hash {
	t.Helper()
	wt, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	f, err := wt.Filesystem.Create("foo.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("content"))
	_, _ = wt.Add("foo.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	h, err := wt.Commit("test commit", &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func createTag(t *testing.T, r *gogit.Repository, tag string) {
	t.Helper()
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateTag(tag, head.Hash(), nil); err != nil {
		t.Fatal(err)
	}
}

func headHash(t *testing.T, r *gogit.Repository) plumbing.Hash {
	t.Helper()
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	return head.Hash()
}

// ── fakeProject implements semverstrategy.GitProject ─────────────────────────

type fakeProject struct {
	repo   *gogit.Repository
	hash   plumbing.Hash
	branch string
}

func newFakeProject(t *testing.T, repo *gogit.Repository, branch string) *fakeProject {
	t.Helper()
	return &fakeProject{repo: repo, hash: headHash(t, repo), branch: branch}
}

func (fp *fakeProject) BranchName() (string, error) { return fp.branch, nil }

func (fp *fakeProject) CommitHash() (string, error) { return fp.hash.String(), nil }

func (fp *fakeProject) LastTag(f gitformat.VersionFormat) (string, error) {
	p, err := gitpkg.NewProjectFromRepo(fp.repo, fp.hash)
	if err != nil {
		return "", err
	}
	return p.LastTag(f)
}

func (fp *fakeProject) IsHeadTagged(tag string) (bool, error) {
	p, err := gitpkg.NewProjectFromRepo(fp.repo, fp.hash)
	if err != nil {
		return false, err
	}
	return p.IsHeadTagged(tag)
}

func (fp *fakeProject) CommitSinceTag(tag string) ([]*object.Commit, error) {
	p, err := gitpkg.NewProjectFromRepo(fp.repo, fp.hash)
	if err != nil {
		return nil, err
	}
	return p.CommitSinceTag(tag)
}

// ── Strategy helpers ──────────────────────────────────────────────────────────

func mainConfig() config.SemverConfig {
	return config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: "^refs/heads/main$", Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .LastTag }}-{{ .Branch }}.{{ .CommitCount }}"},
		},
	}
}

// ── Strategy tests ────────────────────────────────────────────────────────────

func TestLastReturnsInitialWhenNoTag(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Last(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", got)
	}
}

func TestLastReturnsLastTag(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Last(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %s", got)
	}
}

func TestLastRespectsMajorConstraint(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createTag(t, repo, "2.0.0")
	createCommit(t, repo)

	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: `^refs/heads/release/(?P<major>\d+)\.x$`, Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .LastTag }}-dev.{{ .CommitCount }}"},
		},
	}
	p := newFakeProject(t, repo, "refs/heads/release/1.x")
	s := semverstrategy.NewStrategy(cfg)

	got, err := s.Last(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.0.0" {
		t.Fatalf("expected 1.0.0 (major=1 constraint ignores 2.0.0), got %s", got)
	}
}

func TestCurrentReturnsTagWhenHeadIsTagged(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.5.0")
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.5.0" {
		t.Fatalf("expected 1.5.0, got %s", got)
	}
}

func TestCurrentReturnsLastTagOnReleaseBranchUntagged(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.0.0" {
		t.Fatalf("expected 1.0.0 (release, untagged), got %s", got)
	}
}

func TestCurrentReturnsFormattedVersionOnPreReleaseBranch(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/feature/my-feat")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p)
	if err != nil {
		t.Fatal(err)
	}
	// 2 commits after tag, branch short name = "feature/my-feat"
	expected := "1.0.0-feature/my-feat.2"
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestCurrentReturnsInitialWhenNoTag(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", got)
	}
}
```

- [ ] **Step 4: Add GitProject interface and Strategy to strategy/semver/semver.go**

Append to `strategy/semver/semver.go` (after the existing SemverFormat code):

```go
import (
	// add to existing imports:
	"bytes"
	"regexp"
	"strings"
	"text/template"

	"gover/config"
	"gover/format"
	gitobject "github.com/go-git/go-git/v5/plumbing/object"
)

// GitProject is the interface Strategy uses to interact with the git repository.
// *git.Project satisfies this interface.
type GitProject interface {
	LastTag(f format.VersionFormat) (string, error)
	IsHeadTagged(tag string) (bool, error)
	CommitSinceTag(tag string) ([]*gitobject.Commit, error)
	BranchName() (string, error)
	CommitHash() (string, error)
}

// Strategy computes semver versions from the git history.
type Strategy struct {
	cfg config.SemverConfig
}

// NewStrategy returns a Strategy configured by cfg.
func NewStrategy(cfg config.SemverConfig) Strategy {
	return Strategy{cfg: cfg}
}

// Last returns the last valid semver tag reachable from HEAD, respecting any
// version constraints extracted from the branch name pattern. Returns cfg.Initial
// when no tag is found.
func (s Strategy) Last(p GitProject) (string, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return "", fmt.Errorf("getting branch name: %w", err)
	}

	_, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
	f := NewSemverFormat(s.cfg.TagPrefix, constraints)

	tag, err := p.LastTag(f)
	if err != nil {
		return "", err
	}
	if tag == "0.0.0" {
		return s.cfg.Initial, nil
	}
	return tag, nil
}

// Current returns the version at HEAD:
//   - Exact tag if HEAD is a tagged commit
//   - Last tag if on a release branch (untagged HEAD)
//   - Rendered format template if on a pre-release branch
//   - cfg.Initial if no tag exists at all
func (s Strategy) Current(p GitProject) (string, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return "", fmt.Errorf("getting branch name: %w", err)
	}

	branchCfg, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
	f := NewSemverFormat(s.cfg.TagPrefix, constraints)

	lastTag, err := p.LastTag(f)
	if err != nil {
		return "", err
	}

	// No tag found at all
	if lastTag == "0.0.0" {
		return s.cfg.Initial, nil
	}

	// Check if HEAD is the tagged commit
	tagged, err := p.IsHeadTagged(lastTag)
	if err != nil {
		return "", err
	}
	if tagged {
		return lastTag, nil
	}

	// Release branch: return last tag as-is
	if branchCfg.Release {
		return lastTag, nil
	}

	// Pre-release branch: render the format template
	commits, err := p.CommitSinceTag(lastTag)
	if err != nil {
		return "", err
	}
	commitCount := len(commits) - 1 // exclude the tagged commit itself

	shortBranch := strings.TrimPrefix(branchName, "refs/heads/")
	commitHash, err := p.CommitHash()
	if err != nil {
		return "", err
	}
	shortHash := commitHash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	vars := map[string]interface{}{
		"LastTag":     lastTag,
		"Branch":      shortBranch,
		"CommitCount": commitCount,
		"ShortHash":   shortHash,
	}
	// Merge named captures from the branch pattern
	for k, v := range captures {
		vars[k] = v
	}

	return renderTemplate(branchCfg.Format, vars)
}

// matchBranch finds the first BranchConfig whose Pattern matches branchName.
// Returns the config and any named capture groups extracted from the match.
// If no pattern matches, returns a default pre-release config.
func (s Strategy) matchBranch(branchName string) (config.BranchConfig, map[string]string) {
	for _, b := range s.cfg.Branches {
		re, err := regexp.Compile(b.Pattern)
		if err != nil {
			continue
		}
		match := re.FindStringSubmatch(branchName)
		if match == nil {
			continue
		}
		captures := map[string]string{}
		for i, name := range re.SubexpNames() {
			if name != "" && i < len(match) {
				captures[name] = match[i]
			}
		}
		return b, captures
	}
	return config.BranchConfig{
		Release: false,
		Format:  "{{ .LastTag }}-{{ .Branch }}.{{ .CommitCount }}",
	}, map[string]string{}
}

// versionConstraints extracts only major/minor/patch keys from named captures.
func versionConstraints(captures map[string]string) map[string]string {
	c := map[string]string{}
	for _, key := range []string{"major", "minor", "patch"} {
		if v, ok := captures[key]; ok {
			c[key] = v
		}
	}
	return c
}

// renderTemplate executes a Go text/template with the given variables.
func renderTemplate(tmpl string, vars map[string]interface{}) (string, error) {
	t, err := template.New("version").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing version template %q: %w", tmpl, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("executing version template: %w", err)
	}
	return buf.String(), nil
}
```

**Important:** the template uses `{{ .LastTag }}` etc. but `vars` is a `map[string]interface{}`. Go templates access map keys with `.Key` syntax when the data is a `map[string]interface{}` — this works correctly.

- [ ] **Step 5: Add NewProjectFromRepo to git/git.go**

Append after `NewProject`:

```go
// NewProjectFromRepo builds a Project directly from an already-opened repository
// and a commit hash. Used in tests with in-memory repositories.
func NewProjectFromRepo(repo *git.Repository, hash plumbing.Hash) (*Project, error) {
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("reading commit %s: %w", hash, err)
	}
	return &Project{repo, commit}, nil
}
```

- [ ] **Step 6: Run all tests**

```bash
go test ./git/ ./config/ ./strategy/semver/ -v
```
Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add git/git.go strategy/semver/semver.go strategy/semver/semver_test.go
git commit -m "feat(semver): implement Strategy with Last() and Current()"
```

---

### Task 5: Wire CLI — Current and Last commands

**Goal:** Implement the `Current` and `Last` command handlers in `command/commands.go`, add a `--repo` flag, remove dead commands (`Next`, `Previous`, `Release`), and delete `format/semver.go`.

**Files:**
- Modify: `command/commands.go`
- Delete: `format/semver.go`

**Acceptance Criteria:**
- [ ] `gg-version current` prints the current version and exits 0
- [ ] `gg-version last` prints the last tag and exits 0
- [ ] `--repo <path>` flag works (defaults to `.`)
- [ ] `--config <path>` flag works (defaults to `.gg-version.yml`)
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` still passes

**Verify:** `go build ./...` → no errors; `go test ./...` → all PASS

**Steps:**

- [ ] **Step 1: Replace command/commands.go**

```go
package command

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"gover/config"
	gitpkg "gover/git"
	semverstrategy "gover/strategy/semver"
)

var (
	configPath string
	repoPath   string
)

func Run() error {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Value:       ".gg-version.yml",
				Destination: &configPath,
				Usage:       "path to the configuration file",
			},
			&cli.StringFlag{
				Name:        "repo",
				Value:       ".",
				Destination: &repoPath,
				Usage:       "path to the git repository",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "current",
				Usage:  "print the current version at HEAD",
				Action: currentCmd,
			},
			{
				Name:   "last",
				Usage:  "print the last valid semver tag reachable from HEAD",
				Action: lastCmd,
			},
		},
	}

	return cmd.Run(context.Background(), os.Args)
}

func currentCmd(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	project, err := gitpkg.NewProject(repoPath, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}

	strategy := semverstrategy.NewStrategy(cfg.Semver)
	version, err := strategy.Current(project)
	if err != nil {
		return fmt.Errorf("computing current version: %w", err)
	}

	fmt.Println(version)
	return nil
}

func lastCmd(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	project, err := gitpkg.NewProject(repoPath, "")
	if err != nil {
		return fmt.Errorf("opening repo: %w", err)
	}

	strategy := semverstrategy.NewStrategy(cfg.Semver)
	version, err := strategy.Last(project)
	if err != nil {
		return fmt.Errorf("computing last version: %w", err)
	}

	fmt.Println(version)
	return nil
}
```

Note: `*gitpkg.Project` satisfies `semverstrategy.GitProject` because it implements all required methods (`LastTag`, `IsHeadTagged`, `CommitSinceTag`, `BranchName`, `CommitHash`).

- [ ] **Step 2: Delete format/semver.go**

```bash
rm format/semver.go
```

Then verify nothing still imports it:

```bash
grep -r "format.NewSemverFormat\|format.SemverFormat" --include="*.go" .
```

Expected: the `git/git_test.go` still references `format.NewSemverFormat`. Update the import in `git/git_test.go` to use the new location:

Since `git/git_test.go` calls `format.NewSemverFormat("")` (the helper `semverFormat` in the test), and `NewSemverFormat` now lives in `strategy/semver`, update `git/git_test.go`:

Replace the `semverFormat` helper in `git/git_test.go`:

```go
// Remove the import: "gover/format"
// Add the import: semverstrategy "gover/strategy/semver"

func semverFormat(prefix string) format.VersionFormat {
	return semverstrategy.NewSemverFormat(prefix, nil)
}
```

Update imports in `git/git_test.go`:
- Remove: `"gover/format"`
- Add: `semverstrategy "gover/strategy/semver"`
- Add: `"gover/format"` — still needed for the `format.VersionFormat` type in the helper return type

Actually, the helper returns `format.VersionFormat` (the interface). Keep `"gover/format"` import. Just change the body to call `semverstrategy.NewSemverFormat(prefix, nil)`.

Updated `semverFormat` helper in `git/git_test.go`:

```go
import (
	"gover/format"
	semverstrategy "gover/strategy/semver"
	// ... other imports unchanged
)

func semverFormat(prefix string) format.VersionFormat {
	return semverstrategy.NewSemverFormat(prefix, nil)
}
```

- [ ] **Step 3: Build and test**

```bash
go build ./...
go test ./...
```
Expected: builds cleanly, all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add command/commands.go git/git_test.go
git rm format/semver.go
git commit -m "feat: wire Current and Last CLI commands, remove format/semver.go"
```

---

## Self-Review Notes

- **Spec coverage:** All spec requirements covered: `current` command, `last` command, branch classification, configurable pre-release format template, version constraints from branch name patterns, config loading with defaults. ✓
- **Placeholder scan:** No TBDs. All code steps are complete. ✓
- **Type consistency:** `GitProject` interface defined in `strategy/semver/semver.go` Task 4 Step 4. `*git.Project` satisfies it (all methods match). `NewProjectFromRepo` added in Task 4 Step 5. `fakeProject` in tests implements `GitProject`. ✓
- **Template vars:** `map[string]interface{}` used with Go `text/template` — `.LastTag`, `.Branch` etc. work because template accesses map keys with dot notation. ✓
- **Import cycle:** `strategy/semver` imports `gover/format`, `gover/config`, `gover/git` — no cycles. `git` imports only `gover/format`. `config` imports only `gopkg.in/yaml.v3`. ✓

package semver_test

import (
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"gover/config"
	semverstrategy "gover/strategy/semver"
)

// defaultCC returns a ConventionalCommitsConfig with standard defaults for tests.
func defaultCC() config.ConventionalCommitsConfig {
	return config.ConventionalCommitsConfig{
		Format: `^\w+(?:\(.+\))?!?:`,
		Major:  []string{`^\w+(?:\(.+\))?!:`, `BREAKING[- ]CHANGE:`},
		Minor:  []string{`^feat(?:\(.+\))?:`},
		Patch:  []string{`^fix(?:\(.+\))?:`},
	}
}

// makeCommit creates an in-memory commit with the given message.
func makeCommit(t *testing.T, repo *gogit.Repository, msg string) *object.Commit {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	f, err := wt.Filesystem.Create("dummy.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte(msg))
	_, _ = wt.Add("dummy.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	h, err := wt.Commit(msg, &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAnalyzeBump_feat(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "feat: add new login page")
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
	if level != 2 {
		t.Errorf("expected level 2 (minor), got %d", level)
	}
	if hasNonCC {
		t.Error("expected hasNonCC=false for CC-formatted commit")
	}
}

func TestAnalyzeBump_fix(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "fix: correct null pointer")
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
	if level != 1 {
		t.Errorf("expected level 1 (patch), got %d", level)
	}
	if hasNonCC {
		t.Error("expected hasNonCC=false")
	}
}

func TestAnalyzeBump_breakingExclamation(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "feat!: remove deprecated endpoint")
	c2 := makeCommit(t, repo, "fix(api)!: change response format")
	for _, c := range []*object.Commit{c1, c2} {
		level, _ := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
		if level != 3 {
			t.Errorf("expected level 3 (major) for %q, got %d", c.Message, level)
		}
	}
}

func TestAnalyzeBump_breakingFooter(t *testing.T) {
	repo := newRepo(t)
	msgSpace := "feat: new api\n\nBREAKING CHANGE: old api removed"
	msgHyphen := "feat: new api\n\nBREAKING-CHANGE: old api removed"
	c1 := makeCommit(t, repo, msgSpace)
	c2 := makeCommit(t, repo, msgHyphen)
	for _, c := range []*object.Commit{c1, c2} {
		level, _ := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
		if level != 3 {
			t.Errorf("expected level 3 (major) for footer %q, got %d", c.Message, level)
		}
	}
}

func TestAnalyzeBump_noneType(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "chore: update dependencies")
	c2 := makeCommit(t, repo, "docs: fix typo in README")
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{c1, c2}, defaultCC())
	if level != 0 {
		t.Errorf("expected level 0 (no bump) for chore/docs commits, got %d", level)
	}
	if hasNonCC {
		t.Error("expected hasNonCC=false — chore/docs are CC-formatted")
	}
}

func TestAnalyzeBump_nonCC(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "update readme with new examples")
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
	if level != 1 {
		t.Errorf("expected level 1 (patch default) for non-CC commit, got %d", level)
	}
	if !hasNonCC {
		t.Error("expected hasNonCC=true for non-CC commit")
	}
}

func TestAnalyzeBump_allNone(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "chore: bump go version")
	c2 := makeCommit(t, repo, "style: reformat files")
	level, _ := semverstrategy.AnalyzeBump([]*object.Commit{c1, c2}, defaultCC())
	if level != 0 {
		t.Errorf("expected level 0 (all no-bump types), got %d", level)
	}
}

func TestAnalyzeBump_mixed(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "fix: correct crash")
	c2 := makeCommit(t, repo, "feat: add search")
	level, _ := semverstrategy.AnalyzeBump([]*object.Commit{c1, c2}, defaultCC())
	if level != 2 {
		t.Errorf("expected level 2 (minor wins over patch), got %d", level)
	}
}

func TestAnalyzeBump_noCommits(t *testing.T) {
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{}, defaultCC())
	if level != 0 {
		t.Errorf("expected level 0 for no commits, got %d", level)
	}
	if hasNonCC {
		t.Error("expected hasNonCC=false for no commits")
	}
}

func TestBumpVersion_patch(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3", "", 1)
	if got != "1.2.4" {
		t.Errorf("expected 1.2.4, got %s", got)
	}
}

func TestBumpVersion_minor(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3", "", 2)
	if got != "1.3.0" {
		t.Errorf("expected 1.3.0, got %s", got)
	}
}

func TestBumpVersion_major(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3", "", 3)
	if got != "2.0.0" {
		t.Errorf("expected 2.0.0, got %s", got)
	}
}

func TestBumpVersion_withPrefix(t *testing.T) {
	got := semverstrategy.BumpVersion("v1.2.3", "v", 1)
	if got != "1.2.4" {
		t.Errorf("expected 1.2.4 (no prefix in result), got %s", got)
	}
}

func TestBumpVersion_none(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3", "", 0)
	if got != "1.2.3" {
		t.Errorf("expected 1.2.3 unchanged for level 0, got %s", got)
	}
}

func TestBumpVersion_clearsPreRelease(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3-rc.1", "", 1)
	if got != "1.2.4" {
		t.Errorf("expected 1.2.4 (pre-release cleared), got %s", got)
	}
}

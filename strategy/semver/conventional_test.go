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

// fakeFiles retourne une fonction qui mappe SHA de commit → liste de fichiers.
func fakeFiles(m map[string][]string) func(*object.Commit) ([]string, error) {
	return func(c *object.Commit) ([]string, error) {
		return m[c.Hash.String()], nil
	}
}

func TestFilterCommits_includePath(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "feat: api change")
	c2 := makeCommit(t, repo, "feat: web change")
	fm := fakeFiles(map[string][]string{
		c1.Hash.String(): {"api/handler.go"},
		c2.Hash.String(): {"web/index.html"},
	})
	cfg := semverstrategy.FilterConfig{IncludePaths: []string{"api/**"}}
	result := semverstrategy.FilterCommits([]*object.Commit{c1, c2}, fm, cfg)
	if len(result) != 1 || result[0].Hash != c1.Hash {
		t.Errorf("expected only api commit, got %d commits", len(result))
	}
}

func TestFilterCommits_excludeAll(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "docs: update readme")
	fm := fakeFiles(map[string][]string{
		c.Hash.String(): {"README.md", "CHANGELOG.md"},
	})
	cfg := semverstrategy.FilterConfig{ExcludePaths: []string{"*.md"}}
	result := semverstrategy.FilterCommits([]*object.Commit{c}, fm, cfg)
	if len(result) != 0 {
		t.Errorf("expected commit excluded (all files are *.md), got %d", len(result))
	}
}

func TestFilterCommits_excludePartial(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "feat: api + readme")
	fm := fakeFiles(map[string][]string{
		c.Hash.String(): {"api/handler.go", "README.md"},
	})
	cfg := semverstrategy.FilterConfig{ExcludePaths: []string{"*.md"}}
	result := semverstrategy.FilterCommits([]*object.Commit{c}, fm, cfg)
	// api/handler.go ne matche pas *.md → commit inclus
	if len(result) != 1 {
		t.Errorf("expected commit included (not all files are *.md), got %d", len(result))
	}
}

func TestFilterCommits_ignoreCommitSHA(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "fix: something")
	fm := fakeFiles(map[string][]string{c.Hash.String(): {"api/x.go"}})
	cfg := semverstrategy.FilterConfig{IgnoreCommits: []string{c.Hash.String()}}
	result := semverstrategy.FilterCommits([]*object.Commit{c}, fm, cfg)
	if len(result) != 0 {
		t.Errorf("expected commit excluded by full SHA, got %d", len(result))
	}
}

func TestFilterCommits_ignoreCommitShortSHA(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "fix: something")
	shortSHA := c.Hash.String()[:7]
	fm := fakeFiles(map[string][]string{c.Hash.String(): {"api/x.go"}})
	cfg := semverstrategy.FilterConfig{IgnoreCommits: []string{shortSHA}}
	result := semverstrategy.FilterCommits([]*object.Commit{c}, fm, cfg)
	if len(result) != 0 {
		t.Errorf("expected commit excluded by short SHA (%s), got %d", shortSHA, len(result))
	}
}

func TestFilterCommits_rootExcludesComponents(t *testing.T) {
	repo := newRepo(t)
	// Commit qui touche UNIQUEMENT api/ → doit être exclu de @root
	cOnlyApi := makeCommit(t, repo, "feat: api only")
	// Commit qui touche api/ ET go.mod → doit être inclus dans @root
	cApiAndRoot := makeCommit(t, repo, "feat: api + root")
	fm := fakeFiles(map[string][]string{
		cOnlyApi.Hash.String():    {"api/handler.go"},
		cApiAndRoot.Hash.String(): {"api/handler.go", "go.mod"},
	})
	// @root exclut api/**
	cfg := semverstrategy.FilterConfig{ExcludePaths: []string{"api/**"}}
	result := semverstrategy.FilterCommits([]*object.Commit{cOnlyApi, cApiAndRoot}, fm, cfg)
	if len(result) != 1 || result[0].Hash != cApiAndRoot.Hash {
		t.Errorf("expected only cApiAndRoot in @root, got %d commits", len(result))
	}
}

func TestFilterCommits_noFilter(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "fix: a")
	c2 := makeCommit(t, repo, "fix: b")
	fm := fakeFiles(map[string][]string{})
	cfg := semverstrategy.FilterConfig{} // aucun filtre
	result := semverstrategy.FilterCommits([]*object.Commit{c1, c2}, fm, cfg)
	if len(result) != 2 {
		t.Errorf("expected 2 commits with no filter, got %d", len(result))
	}
}

func TestFilterCommits_globDoublestar(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "feat: nested")
	fm := fakeFiles(map[string][]string{
		c.Hash.String(): {"packages/api/handler.go"},
	})
	cfg := semverstrategy.FilterConfig{IncludePaths: []string{"packages/api/**"}}
	result := semverstrategy.FilterCommits([]*object.Commit{c}, fm, cfg)
	if len(result) != 1 {
		t.Errorf("expected ** to match nested path, got %d", len(result))
	}
}

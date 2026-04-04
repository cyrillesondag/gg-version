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
	if !f.IsValid("1.5.3") {
		t.Error("expected 1.5.3 valid (major=1)")
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
	if !f.IsValid("1.2.9") {
		t.Error("expected 1.2.9 valid")
	}
	if f.IsValid("1.3.0") {
		t.Error("expected 1.3.0 invalid (minor constraint is 2)")
	}
	if f.IsValid("2.2.0") {
		t.Error("expected 2.2.0 invalid (major constraint is 1)")
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

// ── Strategy config helpers ───────────────────────────────────────────────────

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
		t.Fatalf("expected 1.0.0 (release branch, untagged HEAD), got %s", got)
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

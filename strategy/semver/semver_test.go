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
			{Pattern: ".*", Release: false, Format: "{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}"},
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
			{Pattern: ".*", Release: false, Format: "{{ .semver.LastTag }}-dev.{{ .semver.CommitCount }}"},
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

	got, err := s.Current(p, nil)
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

	got, err := s.Current(p, nil)
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

	got, err := s.Current(p, nil)
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

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", got)
	}
}

// ── Vars tests ────────────────────────────────────────────────────────────────

func TestVars_semverNamespace(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	createCommit(t, repo)
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	semverVars, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	if semverVars["LastTag"] != "1.2.3" {
		t.Errorf("expected LastTag 1.2.3, got %v", semverVars["LastTag"])
	}
	if semverVars["CommitCount"] != 2 {
		t.Errorf("expected CommitCount 2, got %v", semverVars["CommitCount"])
	}
	hash, ok := semverVars["ShortHash"].(string)
	if !ok || len(hash) != 7 {
		t.Errorf("expected ShortHash to be a 7-char string, got %v", semverVars["ShortHash"])
	}
}

func TestVars_gitNamespace(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/feature/foo")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	gitVars, ok := vars["git"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"git\"] to be a map")
	}
	if gitVars["Branch"] != "feature/foo" {
		t.Errorf("expected Branch feature/foo, got %v", gitVars["Branch"])
	}
}

func TestVars_regexNamespace(t *testing.T) {
	repo := newRepo(t)
	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: `^refs/heads/release/(?P<major>\d+)\.x$`, Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .semver.LastTag }}-dev.{{ .semver.CommitCount }}"},
		},
	}
	p := newFakeProject(t, repo, "refs/heads/release/1.x")
	s := semverstrategy.NewStrategy(cfg)

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	regexVars, ok := vars["regex"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"regex\"] to be a map")
	}
	if regexVars["major"] != "1" {
		t.Errorf("expected regex.major=1, got %v", regexVars["major"])
	}
}

func TestVars_varNamespace(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, map[string]string{"env": "prod", "team": "platform"})
	if err != nil {
		t.Fatal(err)
	}
	varVars, ok := vars["var"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"var\"] to be a map")
	}
	if varVars["env"] != "prod" {
		t.Errorf("expected var.env=prod, got %v", varVars["env"])
	}
	if varVars["team"] != "platform" {
		t.Errorf("expected var.team=platform, got %v", varVars["team"])
	}
}

func TestVars_semverMajorMinorPatch(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	if sv["Major"] != "1" {
		t.Errorf("expected Major=1, got %v", sv["Major"])
	}
	if sv["Minor"] != "2" {
		t.Errorf("expected Minor=2, got %v", sv["Minor"])
	}
	if sv["Patch"] != "3" {
		t.Errorf("expected Patch=3, got %v", sv["Patch"])
	}
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty, got %v", sv["PreRelease"])
	}
}

func TestVars_semverPreRelease(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3-rc.1")
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	if sv["PreRelease"] != "rc.1" {
		t.Errorf("expected PreRelease=rc.1, got %v", sv["PreRelease"])
	}
}

func TestVars_semverNoTag(t *testing.T) {
	repo := newRepo(t)
	// Pas de tag → fallback sur cfg.Initial = "0.1.0"
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	if sv["Major"] != "0" {
		t.Errorf("expected Major=0 (from cfg.Initial 0.1.0), got %v", sv["Major"])
	}
	if sv["Minor"] != "1" {
		t.Errorf("expected Minor=1 (from cfg.Initial 0.1.0), got %v", sv["Minor"])
	}
	if sv["Patch"] != "0" {
		t.Errorf("expected Patch=0 (from cfg.Initial 0.1.0), got %v", sv["Patch"])
	}
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty (from cfg.Initial 0.1.0), got %v", sv["PreRelease"])
	}
}

func TestVars_semverParseFailure(t *testing.T) {
	repo := newRepo(t)
	// cfg.Initial is not a valid semver string — parse will fail
	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "not-a-version",
		Branches: []config.BranchConfig{
			{Pattern: ".*", Release: false, Format: "{{ .semver.LastTag }}-dev.{{ .semver.CommitCount }}"},
		},
	}
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(cfg)

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	if sv["Major"] != "" {
		t.Errorf("expected Major empty on parse failure, got %v", sv["Major"])
	}
	if sv["Minor"] != "" {
		t.Errorf("expected Minor empty on parse failure, got %v", sv["Minor"])
	}
	if sv["Patch"] != "" {
		t.Errorf("expected Patch empty on parse failure, got %v", sv["Patch"])
	}
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty on parse failure, got %v", sv["PreRelease"])
	}
}

func TestVars_gitDate(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	gitVars, ok := vars["git"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"git\"] to be a map")
	}
	date, ok := gitVars["Date"].(string)
	if !ok {
		t.Fatalf("expected git.Date to be a string, got %T", gitVars["Date"])
	}
	// Vérifie le format YYYY-MM-DD
	if len(date) != 10 || date[4] != '-' || date[7] != '-' {
		t.Errorf("expected git.Date in YYYY-MM-DD format, got %q", date)
	}
}

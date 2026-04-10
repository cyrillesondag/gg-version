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
	"gover/gitmodel"
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

func (fp *fakeProject) CommitSinceTag(tag string) ([]*object.Commit, bool, error) {
	p, err := gitpkg.NewProjectFromRepo(fp.repo, fp.hash)
	if err != nil {
		return nil, false, err
	}
	return p.CommitSinceTag(tag)
}

func (fp *fakeProject) IsShallow() bool { return false }

func (fp *fakeProject) CommitFiles(c *object.Commit) ([]string, error) {
	p, err := gitpkg.NewProjectFromRepo(fp.repo, fp.hash)
	if err != nil {
		return nil, err
	}
	return p.CommitFiles(c)
}

func (fp *fakeProject) CommitDate() (time.Time, time.Time, error) {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), nil
}

func (fp *fakeProject) CreateTag(name, message string) error { return nil }
func (fp *fakeProject) PushTags() error                      { return nil }

func (fp *fakeProject) CommitHistory() ([]gitmodel.CommitWithTags, error) {
	p, err := gitpkg.NewProjectFromRepo(fp.repo, fp.hash)
	if err != nil {
		return nil, err
	}
	return p.CommitHistory()
}

// ── Strategy config helpers ───────────────────────────────────────────────────

func mainConfig() config.SemverConfig {
	return config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: "^refs/heads/main$", Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"},
		},
		ConventionalCommits: config.ConventionalCommitsConfig{
			Format: `^\w+(?:\(.+\))?!?:`,
			Major: []string{
				`^\w+(?:\(.+\))?!:`,
				`BREAKING[- ]CHANGE:`,
			},
			Minor: []string{`^feat(?:\(.+\))?:`},
			Patch: []string{`^fix(?:\(.+\))?:`},
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
			{Pattern: ".*", Release: false, Format: "{{ .semver.Semver }}-dev.{{ .git.CommitCount }}"},
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
	createCommit(t, repo) // non-CC → patch default → 1.0.1
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.0.1" {
		t.Fatalf("expected 1.0.1 (CC patch bump), got %s", got)
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
	// 2 non-CC commits → patch bump → Semver=1.0.1
	// template: {{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}
	expected := "1.0.1-feature/my-feat.2"
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
	// With 2 non-CC commits after tag 1.2.3 → patch bump → Semver = "1.2.4"
	if semverVars["Semver"] != "1.2.4" {
		t.Errorf("expected Semver 1.2.4, got %v", semverVars["Semver"])
	}
	if semverVars["LastVersion"] != "1.2.3" {
		t.Errorf("expected LastVersion 1.2.3, got %v", semverVars["LastVersion"])
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
	// Hash is a 40-char string
	hash, ok := gitVars["Hash"].(string)
	if !ok || len(hash) != 40 {
		t.Errorf("expected Hash to be a 40-char string, got %v", gitVars["Hash"])
	}
	// ShortHash is 7 chars
	shortHash, ok := gitVars["ShortHash"].(string)
	if !ok || len(shortHash) != 7 {
		t.Errorf("expected ShortHash to be a 7-char string, got %v", gitVars["ShortHash"])
	}
	// CommitCount is 0 (no tag, no commits since tag)
	if gitVars["CommitCount"] != 0 {
		t.Errorf("expected CommitCount 0, got %v", gitVars["CommitCount"])
	}
	// LastTag is empty string (no tag)
	if gitVars["LastTag"] != "" {
		t.Errorf("expected LastTag empty (no tag), got %v", gitVars["LastTag"])
	}
	if gitVars["IsShallow"] != false {
		t.Errorf("expected IsShallow=false, got %v", gitVars["IsShallow"])
	}
	if gitVars["Truncated"] != false {
		t.Errorf("expected Truncated=false, got %v", gitVars["Truncated"])
	}
}

func TestVars_regexNamespace(t *testing.T) {
	repo := newRepo(t)
	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: `^refs/heads/release/(?P<major>\d+)\.x$`, Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .semver.Semver }}-dev.{{ .git.CommitCount }}"},
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
	createCommit(t, repo) // non-CC → patch default
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
	// CC-calculated version: 1.2.3 + patch default → 1.2.4
	if sv["Major"] != "1" {
		t.Errorf("expected Major=1, got %v", sv["Major"])
	}
	if sv["Minor"] != "2" {
		t.Errorf("expected Minor=2, got %v", sv["Minor"])
	}
	if sv["Patch"] != "4" {
		t.Errorf("expected Patch=4 (bumped from 3), got %v", sv["Patch"])
	}
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty, got %v", sv["PreRelease"])
	}
	// Last tag components
	if sv["LastVersion"] != "1.2.3" {
		t.Errorf("expected LastVersion=1.2.3, got %v", sv["LastVersion"])
	}
	if sv["LastMajor"] != "1" {
		t.Errorf("expected LastMajor=1, got %v", sv["LastMajor"])
	}
	if sv["LastMinor"] != "2" {
		t.Errorf("expected LastMinor=2, got %v", sv["LastMinor"])
	}
	if sv["LastPatch"] != "3" {
		t.Errorf("expected LastPatch=3, got %v", sv["LastPatch"])
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
	// CC version clears pre-release
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty for CC version, got %v", sv["PreRelease"])
	}
	// Last tag preserves pre-release
	if sv["LastPreRelease"] != "rc.1" {
		t.Errorf("expected LastPreRelease=rc.1, got %v", sv["LastPreRelease"])
	}
}

func TestVars_semverNoTag(t *testing.T) {
	repo := newRepo(t)
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
	// No tag → no CC analysis → Semver = cfg.Initial = "0.1.0"
	if sv["Semver"] != "0.1.0" {
		t.Errorf("expected Semver=0.1.0 (cfg.Initial), got %v", sv["Semver"])
	}
	if sv["Major"] != "0" {
		t.Errorf("expected Major=0, got %v", sv["Major"])
	}
	if sv["Minor"] != "1" {
		t.Errorf("expected Minor=1, got %v", sv["Minor"])
	}
	if sv["Patch"] != "0" {
		t.Errorf("expected Patch=0, got %v", sv["Patch"])
	}
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty, got %v", sv["PreRelease"])
	}
}

func TestVars_semverParseFailure(t *testing.T) {
	repo := newRepo(t)
	// cfg.Initial is not a valid semver string — parse will fail
	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "not-a-version",
		Branches: []config.BranchConfig{
			{Pattern: ".*", Release: false, Format: "{{ .semver.Semver }}-dev.{{ .git.CommitCount }}"},
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
	authorDate, ok := gitVars["AuthorDate"].(string)
	if !ok {
		t.Fatalf("expected git.AuthorDate to be a string, got %T", gitVars["AuthorDate"])
	}
	// Vérifie le format YYYY-MM-DD
	if len(authorDate) != 10 || authorDate[4] != '-' || authorDate[7] != '-' {
		t.Errorf("expected git.AuthorDate in YYYY-MM-DD format, got %q", authorDate)
	}
	committerDate, ok := gitVars["CommitterDate"].(string)
	if !ok {
		t.Fatalf("expected git.CommitterDate to be a string, got %T", gitVars["CommitterDate"])
	}
	// Vérifie le format YYYY-MM-DD
	if len(committerDate) != 10 || committerDate[4] != '-' || committerDate[7] != '-' {
		t.Errorf("expected git.CommitterDate in YYYY-MM-DD format, got %q", committerDate)
	}
	// Les dates du stub sont fixes (2026-01-01 et 2026-01-02) → reproductibles
	if authorDate != "2026-01-01" {
		t.Errorf("expected git.AuthorDate=2026-01-01, got %q", authorDate)
	}
	if committerDate != "2026-01-02" {
		t.Errorf("expected git.CommitterDate=2026-01-02, got %q", committerDate)
	}
}

// ── Conventional Commits integration tests ────────────────────────────────────

func TestCurrent_releaseWithFeat(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	// Create a feat commit
	wt, _ := repo.Worktree()
	f, _ := wt.Filesystem.Create("feat.txt")
	_, _ = f.Write([]byte("feature"))
	_, _ = wt.Add("feat.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	_, _ = wt.Commit("feat: add new feature", &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.3.0" {
		t.Fatalf("expected 1.3.0 (minor bump from feat:), got %s", got)
	}
}

func TestCurrent_releaseWithBreaking(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	wt, _ := repo.Worktree()
	f, _ := wt.Filesystem.Create("break.txt")
	_, _ = f.Write([]byte("breaking"))
	_, _ = wt.Add("break.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	_, _ = wt.Commit("feat!: remove old API", &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2.0.0" {
		t.Fatalf("expected 2.0.0 (major bump from feat!:), got %s", got)
	}
}

func TestCurrent_releaseAllNone(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	wt, _ := repo.Worktree()
	f, _ := wt.Filesystem.Create("chore.txt")
	_, _ = f.Write([]byte("chore"))
	_, _ = wt.Add("chore.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	_, _ = wt.Commit("chore: update deps", &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2.3" {
		t.Fatalf("expected 1.2.3 (no bump — all chore commits), got %s", got)
	}
}

// ── AllCurrent / AllLast / AllVars tests ──────────────────────────────────────

func twoComponentConfig() config.Config {
	return config.Config{
		Semver: config.SemverConfig{
			TagPrefix: "v",
			Initial:   "0.1.0",
			Branches: []config.BranchConfig{
				{Pattern: "^refs/heads/main$", Release: true},
				{Pattern: ".*", Release: false, Format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"},
			},
			ConventionalCommits: config.DefaultConfig().Semver.ConventionalCommits,
		},
		Components: map[string]config.ComponentConfig{
			"api": {Path: "api/**"},
			"web": {Path: "web/**"},
		},
	}
}

func TestAllCurrent_noComponents(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "v1.2.3")
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	results, err := s.AllCurrent(p, nil, config.Config{Semver: mainConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "" {
		t.Errorf("expected single unnamed result, got %v", results)
	}
}

func TestAllCurrent_withComponents_order(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "v1.0.0")

	p := newFakeProject(t, repo, "refs/heads/main")
	cfg := twoComponentConfig()
	s := semverstrategy.NewStrategy(cfg.Semver)

	results, err := s.AllCurrent(p, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "@root" {
		t.Errorf("expected @root first, got %q", results[0].Name)
	}
	if results[1].Name != "api" {
		t.Errorf("expected api second, got %q", results[1].Name)
	}
	if results[2].Name != "web" {
		t.Errorf("expected web third, got %q", results[2].Name)
	}
}

func TestAllLast_withComponents(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "v1.0.0")
	createTag(t, repo, "api/v1.2.3")

	p := newFakeProject(t, repo, "refs/heads/main")
	cfg := twoComponentConfig()
	s := semverstrategy.NewStrategy(cfg.Semver)

	results, err := s.AllLast(p, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Version != "v1.0.0" {
		t.Errorf("expected @root=v1.0.0, got %q", results[0].Version)
	}
	if results[1].Version != "api/v1.2.3" {
		t.Errorf("expected api=api/v1.2.3, got %q", results[1].Version)
	}
	if results[2].Version != "0.1.0" {
		t.Errorf("expected web=0.1.0 (initial), got %q", results[2].Version)
	}
}

func TestAllVars_withComponents(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/main")
	cfg := twoComponentConfig()
	s := semverstrategy.NewStrategy(cfg.Semver)

	results, err := s.AllVars(p, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "@root" {
		t.Errorf("expected @root first, got %q", results[0].Name)
	}
	for _, r := range results {
		if _, ok := r.Vars["semver"]; !ok {
			t.Errorf("expected semver namespace in vars for %q", r.Name)
		}
	}
}

func TestAllCurrent_tagged(t *testing.T) {
	// DefaultConfig uses TagPrefix="" so tags must be bare semver (no "v" prefix).
	cfg := config.Config{Semver: config.DefaultConfig().Semver}

	t.Run("HEAD on tag", func(t *testing.T) {
		repo := newRepo(t)
		createTag(t, repo, "1.0.0")
		fp := newFakeProject(t, repo, "main")

		strategy := semverstrategy.NewStrategy(cfg.Semver)
		results, err := strategy.AllCurrent(fp, nil, cfg)
		if err != nil {
			t.Fatalf("AllCurrent: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if !results[0].Tagged {
			t.Error("expected Tagged=true when HEAD is on tag")
		}
		if results[0].Version != "1.0.0" {
			t.Errorf("expected Version=1.0.0, got %q", results[0].Version)
		}
	})

	t.Run("HEAD not on tag", func(t *testing.T) {
		repo := newRepo(t)
		createTag(t, repo, "1.0.0")
		createCommit(t, repo) // advance HEAD past tag
		fp := newFakeProject(t, repo, "main")

		strategy := semverstrategy.NewStrategy(cfg.Semver)
		results, err := strategy.AllCurrent(fp, nil, cfg)
		if err != nil {
			t.Fatalf("AllCurrent: %v", err)
		}
		if results[0].Tagged {
			t.Error("expected Tagged=false when HEAD is not on tag")
		}
		if results[0].Version == "1.0.0" {
			t.Error("expected computed version (not the tag itself)")
		}
	})
}

func TestResolveTagPrefix(t *testing.T) {
	cases := []struct {
		name     string
		comp     config.ComponentConfig
		global   string
		expected string
	}{
		{"api", config.ComponentConfig{Path: "api/**"}, "v", "api/v"},
		{"web", config.ComponentConfig{Path: "web/**", TagScope: "my-web"}, "v", "my-web/v"},
		{"svc", config.ComponentConfig{Path: "svc/**"}, "", "svc/"},
	}
	for _, tc := range cases {
		got := semverstrategy.ResolveTagPrefix(tc.name, tc.comp, tc.global)
		if got != tc.expected {
			t.Errorf("ResolveTagPrefix(%q, ..., %q) = %q, want %q", tc.name, tc.global, got, tc.expected)
		}
	}
}

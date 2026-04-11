package semver

import (
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/go-git/go-billy/v5/memfs"

	"github.com/cyrillesondag/gg-version/config"
	gitpkg "github.com/cyrillesondag/gg-version/git"
)

// mkCommit creates a minimal fake commit for testing sharedHistory operations.
// The id byte makes each commit's hash distinct.
func mkCommit(id byte) *object.Commit {
	var h plumbing.Hash
	h[0] = id
	return &object.Commit{Hash: h}
}

// ── TestFindLastTag ───────────────────────────────────────────────────────────

func TestFindLastTag(t *testing.T) {
	f := NewSemverFormat("", nil)

	t.Run("EmptyHistory", func(t *testing.T) {
		hist := &sharedHistory{}
		tag, idx := hist.findLastTag(f)
		if tag != "0.0.0" || idx != -1 {
			t.Errorf("got (%q, %d), want (\"0.0.0\", -1)", tag, idx)
		}
	})

	t.Run("SingleValidTag", func(t *testing.T) {
		// Items in topological order: HEAD first, oldest last.
		// Tag is on item[1] (not HEAD).
		hist := &sharedHistory{
			items: []gitpkg.CommitWithTags{
				{Commit: mkCommit(1), Tags: nil},
				{Commit: mkCommit(2), Tags: []string{"1.0.0"}},
				{Commit: mkCommit(3), Tags: nil},
			},
		}
		tag, idx := hist.findLastTag(f)
		if tag != "1.0.0" {
			t.Errorf("got tag %q, want \"1.0.0\"", tag)
		}
		if idx != 1 {
			t.Errorf("got idx %d, want 1", idx)
		}
	})

	t.Run("InvalidTagSkipped", func(t *testing.T) {
		hist := &sharedHistory{
			items: []gitpkg.CommitWithTags{
				{Commit: mkCommit(1), Tags: []string{"not-semver", "release-1.0"}},
			},
		}
		tag, idx := hist.findLastTag(f)
		if tag != "0.0.0" || idx != -1 {
			t.Errorf("got (%q, %d), want (\"0.0.0\", -1)", tag, idx)
		}
	})

	t.Run("TiebreakerOnSameCommit", func(t *testing.T) {
		// Both "1.0.0" and "2.0.0" on the same commit: "2.0.0" wins via Compare.
		hist := &sharedHistory{
			items: []gitpkg.CommitWithTags{
				{Commit: mkCommit(1), Tags: []string{"1.0.0", "2.0.0"}},
			},
		}
		tag, idx := hist.findLastTag(f)
		if tag != "2.0.0" {
			t.Errorf("got tag %q, want \"2.0.0\" (higher version wins)", tag)
		}
		if idx != 0 {
			t.Errorf("got idx %d, want 0", idx)
		}
	})

	t.Run("HeadTaggedReturnsIndex0", func(t *testing.T) {
		hist := &sharedHistory{
			items: []gitpkg.CommitWithTags{
				{Commit: mkCommit(1), Tags: []string{"3.0.0"}},
				{Commit: mkCommit(2), Tags: []string{"2.0.0"}},
			},
		}
		// HEAD (index 0) has the tag — should be returned, not the older one.
		tag, idx := hist.findLastTag(f)
		if tag != "3.0.0" || idx != 0 {
			t.Errorf("got (%q, %d), want (\"3.0.0\", 0)", tag, idx)
		}
	})
}

// ── TestCommitsSince ─────────────────────────────────────────────────────────

func TestCommitsSince(t *testing.T) {
	c0 := mkCommit(1) // HEAD
	c1 := mkCommit(2)
	c2 := mkCommit(3) // oldest visible
	hist := &sharedHistory{
		items: []gitpkg.CommitWithTags{
			{Commit: c0},
			{Commit: c1},
			{Commit: c2},
		},
	}

	t.Run("TagAtIndex0_ZeroCommits", func(t *testing.T) {
		// Tag is at HEAD itself: no commits since the tag.
		commits, truncated := hist.commitsSince(0)
		if len(commits) != 0 {
			t.Errorf("got %d commits, want 0", len(commits))
		}
		if truncated {
			t.Error("truncated should be false when tag is found in history")
		}
	})

	t.Run("TagAtIndex2_TwoCommits", func(t *testing.T) {
		commits, truncated := hist.commitsSince(2)
		if len(commits) != 2 {
			t.Errorf("got %d commits, want 2", len(commits))
		}
		if truncated {
			t.Error("truncated should be false when tag is found in history")
		}
		if commits[0] != c0 {
			t.Error("commits[0] should be c0 (HEAD)")
		}
		if commits[1] != c1 {
			t.Error("commits[1] should be c1")
		}
	})

	t.Run("TagAtIndexMinus1_AllTruncated", func(t *testing.T) {
		// tagIdx == -1: shallow clone, tag not in accessible history.
		commits, truncated := hist.commitsSince(-1)
		if len(commits) != 3 {
			t.Errorf("got %d commits, want 3 (all items)", len(commits))
		}
		if !truncated {
			t.Error("truncated should be true for shallow clone case")
		}
	})
}

// ── TestVarsCoreFromHistory ───────────────────────────────────────────────────

func TestVarsCoreFromHistory(t *testing.T) {
	// Create an in-memory repo: one tagged commit, then one untagged commit.
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	hash1 := internalCreateCommit(t, repo, "initial.txt", "initial commit")
	if _, err := repo.CreateTag("1.0.0", hash1, nil); err != nil {
		t.Fatal(err)
	}
	internalCreateCommit(t, repo, "feature.txt", "feat: add feature")

	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}

	p, err := gitpkg.NewProjectFromRepo(repo, head.Hash())
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches:  []config.BranchConfig{{Pattern: ".*"}},
		ConventionalCommits: config.ConventionalCommitsConfig{
			Format: `^\w+(?:\(.+\))?!?:`,
			Minor:  []string{`^feat(?:\(.+\))?:`},
			Patch:  []string{`^fix(?:\(.+\))?:`},
		},
	}
	strat := semverStrategy{cfg: cfg}

	filterCfg := FilterConfig{
		ExcludePaths:  cfg.IgnorePaths,
		IgnoreCommits: cfg.IgnoreCommits,
	}

	// Ground truth from varsCore.
	wantVars, err := strat.varsCore(p, nil, cfg.TagPrefix, filterCfg)
	if err != nil {
		t.Fatalf("varsCore: %v", err)
	}

	// Optimised path via varsCoreFromHistory.
	hist, err := buildSharedHistory(p)
	if err != nil {
		t.Fatalf("buildSharedHistory: %v", err)
	}
	gotVars, err := strat.varsCoreFromHistory(p, nil, hist, cfg.TagPrefix, filterCfg)
	if err != nil {
		t.Fatalf("varsCoreFromHistory: %v", err)
	}

	// Both functions should produce identical values for the key fields.
	for _, ns := range []string{"semver", "git"} {
		wantNS := wantVars[ns].(map[string]interface{})
		gotNS := gotVars[ns].(map[string]interface{})
		for key, wv := range wantNS {
			gv, ok := gotNS[key]
			if !ok {
				t.Errorf("%s.%s: missing in varsCoreFromHistory output", ns, key)
				continue
			}
			if wv != gv {
				t.Errorf("%s.%s: varsCore=%v, varsCoreFromHistory=%v", ns, key, wv, gv)
			}
		}
		for key := range gotNS {
			if _, ok := wantNS[key]; !ok {
				t.Errorf("%s.%s: extra key in varsCoreFromHistory output", ns, key)
			}
		}
	}
}

// ── TestAllLint ───────────────────────────────────────────────────────────────

// lintConfig returns a SemverConfig with CC format configured for lint tests.
func lintConfig() config.SemverConfig {
	return config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches:  []config.BranchConfig{{Pattern: ".*"}},
		ConventionalCommits: config.ConventionalCommitsConfig{
			Format: `^\w+(?:\(.+\))?!?:`,
		},
	}
}

// internalCreateTag creates a lightweight tag on the HEAD of an in-memory repo.
func internalCreateTag(t *testing.T, r *gogit.Repository, tag string) {
	t.Helper()
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateTag(tag, head.Hash(), nil); err != nil {
		t.Fatal(err)
	}
}

// countingProject wraps a GitProject to count CommitHistory calls.
type countingProject struct {
	GitProject
	commitHistoryCallCount int
}

func (cp *countingProject) CommitHistory() ([]gitpkg.CommitWithTags, error) {
	cp.commitHistoryCallCount++
	return cp.GitProject.CommitHistory()
}

// newMonorepoProject builds a GitProject from a repo with HEAD at its current state.
func newMonorepoProject(t *testing.T, r *gogit.Repository) GitProject {
	t.Helper()
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	p, err := gitpkg.NewProjectFromRepo(r, head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAllLint_NonMonorepo_NoViolations(t *testing.T) {
	// Repo with 1 tag, 2 CC commits since, no violations expected.
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	internalCreateCommit(t, repo, "init.txt", "chore: init")
	internalCreateTag(t, repo, "1.0.0")
	internalCreateCommit(t, repo, "a.txt", "feat: add feature")
	internalCreateCommit(t, repo, "b.txt", "fix: correct bug")

	p := newMonorepoProject(t, repo)
	s := semverStrategy{cfg: lintConfig()}
	cfg := config.Config{Semver: s.cfg}

	results, err := s.AllLint(p, cfg)
	if err != nil {
		t.Fatalf("AllLint: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Name != "" {
		t.Errorf("Name=%q, want \"\"", results[0].Name)
	}
	if len(results[0].Violations) != 0 {
		t.Errorf("got %d violations, want 0: %v", len(results[0].Violations), results[0].Violations)
	}
	if results[0].Truncated {
		t.Error("Truncated=true, want false")
	}
}

func TestAllLint_NonMonorepo_WithViolations(t *testing.T) {
	// 1 CC commit + 1 non-CC commit since tag → 1 violation.
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	internalCreateCommit(t, repo, "init.txt", "chore: init")
	internalCreateTag(t, repo, "1.0.0")
	internalCreateCommit(t, repo, "a.txt", "feat: add feature")
	internalCreateCommit(t, repo, "b.txt", "WIP broken stuff") // no colon → not CC format

	p := newMonorepoProject(t, repo)
	s := semverStrategy{cfg: lintConfig()}
	cfg := config.Config{Semver: s.cfg}

	results, err := s.AllLint(p, cfg)
	if err != nil {
		t.Fatalf("AllLint: %v", err)
	}
	if len(results[0].Violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(results[0].Violations))
	}
	if results[0].Violations[0].Subject != "WIP broken stuff" {
		t.Errorf("Subject=%q, want \"WIP broken stuff\"", results[0].Violations[0].Subject)
	}
}

func TestAllLint_NonMonorepo_NoTag(t *testing.T) {
	// No tag → no violations (nothing to compare against).
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	internalCreateCommit(t, repo, "init.txt", "wip: no tag yet")

	p := newMonorepoProject(t, repo)
	s := semverStrategy{cfg: lintConfig()}
	cfg := config.Config{Semver: s.cfg}

	results, err := s.AllLint(p, cfg)
	if err != nil {
		t.Fatalf("AllLint: %v", err)
	}
	if len(results[0].Violations) != 0 {
		t.Errorf("got %d violations, want 0 (no tag)", len(results[0].Violations))
	}
}

func TestAllLint_NonMonorepo_HeadIsTagged(t *testing.T) {
	// When HEAD is exactly on a tag, no commits to lint → 0 violations.
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	internalCreateCommit(t, repo, "init.txt", "WIP bad commit") // non-CC commit
	internalCreateTag(t, repo, "1.0.0")                        // HEAD is at the tag

	p := newMonorepoProject(t, repo)
	s := semverStrategy{cfg: lintConfig()}

	results, err := s.AllLint(p, config.Config{Semver: s.cfg})
	if err != nil {
		t.Fatal(err)
	}
	if len(results[0].Violations) != 0 {
		t.Errorf("got %d violations on tagged HEAD, want 0", len(results[0].Violations))
	}
}

func TestAllLint_Monorepo_SingleTraversal(t *testing.T) {
	// CommitHistory must be called exactly once regardless of number of components.
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	internalCreateCommit(t, repo, "api/main.go", "feat(api): initial")
	internalCreateTag(t, repo, "api/1.0.0")
	internalCreateTag(t, repo, "web/1.0.0")
	internalCreateCommit(t, repo, "api/fix.go", "fix(api): patch")

	cp := &countingProject{GitProject: newMonorepoProject(t, repo)}
	s := semverStrategy{cfg: lintConfig()}
	cfg := config.Config{
		Semver: s.cfg,
		Components: map[string]config.ComponentConfig{
			"api": {Path: "api/**"},
			"web": {Path: "web/**"},
		},
	}

	_, err = s.AllLint(cp, cfg)
	if err != nil {
		t.Fatalf("AllLint: %v", err)
	}
	if cp.commitHistoryCallCount != 1 {
		t.Errorf("CommitHistory called %d times, want 1", cp.commitHistoryCallCount)
	}
}

func TestAllLint_Monorepo_AlphabeticalOrder(t *testing.T) {
	// Results must be: [@root, api, web] regardless of map iteration order.
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	internalCreateCommit(t, repo, "root.txt", "chore: init")

	p := newMonorepoProject(t, repo)
	s := semverStrategy{cfg: lintConfig()}
	cfg := config.Config{
		Semver: s.cfg,
		Components: map[string]config.ComponentConfig{
			"web": {Path: "web/**"},
			"api": {Path: "api/**"},
		},
	}

	results, err := s.AllLint(p, cfg)
	if err != nil {
		t.Fatalf("AllLint: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	names := []string{results[0].Name, results[1].Name, results[2].Name}
	want := []string{"@root", "api", "web"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("results[%d].Name=%q, want %q", i, n, want[i])
		}
	}
}

func TestAllLint_Monorepo_RootViolation(t *testing.T) {
	// Non-CC commit in root (touches no component path) → root has violation, api clean.
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	internalCreateCommit(t, repo, "api/main.go", "feat(api): init")
	internalCreateTag(t, repo, "1.0.0")     // root tag
	internalCreateTag(t, repo, "api/1.0.0") // api tag
	internalCreateCommit(t, repo, "root.txt", "bad root commit") // root violation

	p := newMonorepoProject(t, repo)
	s := semverStrategy{cfg: lintConfig()}
	cfg := config.Config{
		Semver: s.cfg,
		Components: map[string]config.ComponentConfig{
			"api": {Path: "api/**"},
		},
	}

	results, err := s.AllLint(p, cfg)
	if err != nil {
		t.Fatalf("AllLint: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	root := results[0]
	api := results[1]
	if root.Name != "@root" {
		t.Errorf("results[0].Name=%q, want \"@root\"", root.Name)
	}
	if len(root.Violations) != 1 {
		t.Errorf("root violations=%d, want 1", len(root.Violations))
	}
	if api.Name != "api" {
		t.Errorf("results[1].Name=%q, want \"api\"", api.Name)
	}
	if len(api.Violations) != 0 {
		t.Errorf("api violations=%d, want 0", len(api.Violations))
	}
}

func TestAllLint_Monorepo_ComponentViolation(t *testing.T) {
	// Non-CC commit touching api/** → api has violation, root clean.
	repo, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}
	internalCreateCommit(t, repo, "api/main.go", "feat(api): init")
	internalCreateTag(t, repo, "1.0.0")
	internalCreateTag(t, repo, "api/1.0.0")
	internalCreateCommit(t, repo, "api/bad.go", "bad api commit") // api violation

	p := newMonorepoProject(t, repo)
	s := semverStrategy{cfg: lintConfig()}
	cfg := config.Config{
		Semver: s.cfg,
		Components: map[string]config.ComponentConfig{
			"api": {Path: "api/**"},
		},
	}

	results, err := s.AllLint(p, cfg)
	if err != nil {
		t.Fatalf("AllLint: %v", err)
	}
	root := results[0]
	api := results[1]
	if len(root.Violations) != 0 {
		t.Errorf("root violations=%d, want 0", len(root.Violations))
	}
	if len(api.Violations) != 1 {
		t.Errorf("api violations=%d, want 1", len(api.Violations))
	}
	if api.Violations[0].Subject != "bad api commit" {
		t.Errorf("api violation subject=%q, want \"bad api commit\"", api.Violations[0].Subject)
	}
}

// internalCreateCommit creates a commit in an in-memory repo with unique content.
func internalCreateCommit(t *testing.T, r *gogit.Repository, filename, msg string) plumbing.Hash {
	t.Helper()
	wt, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	f, err := wt.Filesystem.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("content"))
	_ = f.Close()
	_, _ = wt.Add(filename)
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	h, err := wt.Commit(msg, &gogit.CommitOptions{
		Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

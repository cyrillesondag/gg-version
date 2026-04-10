package semver

import (
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/go-git/go-billy/v5/memfs"

	"gover/config"
	gitpkg "gover/git"
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
		Branches:  []config.BranchConfig{{Pattern: ".*", Release: true}},
		ConventionalCommits: config.ConventionalCommitsConfig{
			Format: `^\w+(?:\(.+\))?!?:`,
			Minor:  []string{`^feat(?:\(.+\))?:`},
			Patch:  []string{`^fix(?:\(.+\))?:`},
		},
	}
	s := NewStrategy(cfg)

	filterCfg := FilterConfig{
		ExcludePaths:  cfg.IgnorePaths,
		IgnoreCommits: cfg.IgnoreCommits,
	}

	// Ground truth from varsCore.
	wantVars, err := s.varsCore(p, nil, cfg.TagPrefix, filterCfg)
	if err != nil {
		t.Fatalf("varsCore: %v", err)
	}

	// Optimised path via varsCoreFromHistory.
	hist, err := buildSharedHistory(p)
	if err != nil {
		t.Fatalf("buildSharedHistory: %v", err)
	}
	gotVars, err := s.varsCoreFromHistory(p, nil, hist, cfg.TagPrefix, filterCfg)
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

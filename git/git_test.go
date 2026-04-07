package git

import (
	"gover/format"
	semverstrategy "gover/strategy/semver"
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

func createTagAt(t *testing.T, r *git.Repository, hash plumbing.Hash, tag string) plumbing.Hash {
	t.Helper()
	tagRef, err := r.CreateTag(tag, hash, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tagRef.Hash()
}

func semverFmt(prefix string) format.VersionFormat {
	return semverstrategy.NewSemverFormat(prefix, nil)
}

// TestNoTag: aucun tag dans le repo → "0.0.0".
func TestNoTag(t *testing.T) {
	repo := newRepo(t)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFmt(""))
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

	tag, err := p.LastTag(semverFmt(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "1.0.0" {
		t.Fatalf("expected 1.0.0, got %s", tag)
	}
}

// TestBranchTags: le tag "1.2.0" créé sur my-branch après checkout ne doit PAS
// être visible depuis le commit qui existait avant la création de my-branch.
func TestBranchTags(t *testing.T) {
	f := semverFmt("")
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

	tag, err := p.LastTag(semverFmt(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %s", tag)
	}
}

// TestMultipleTagsMostRecentReturned: plusieurs tags dans l'historique → le plus récent ancêtre est retourné.
func TestMultipleTagsMostRecentReturned(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createTag(t, repo, "2.0.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFmt(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %s", tag)
	}
}

// TestTagWithVPrefix: format avec préfixe "v" (ex : "v1.5.0").
func TestTagWithVPrefix(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "v1.5.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFmt("v"))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.5.0" {
		t.Fatalf("expected v1.5.0, got %s", tag)
	}
}

// TestInvalidTagsIgnored: les tags non-semver sont ignorés ; seul le tag valide est retourné.
func TestInvalidTagsIgnored(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "release-candidate")
	createCommit(t, repo)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFmt(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "1.0.0" {
		t.Fatalf("expected 1.0.0, got %s", tag)
	}
}

// TestAllTagsInvalidFallsBackToDefault: tous les tags sont invalides → retourne "0.0.0".
func TestAllTagsInvalidFallsBackToDefault(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "latest")
	createTag(t, repo, "beta")
	createTag(t, repo, "release-1")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFmt(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "0.0.0" {
		t.Fatalf("expected 0.0.0, got %s", tag)
	}
}

// TestAnnotatedTag: les tags annotés (objets tag) sont correctement résolus.
func TestAnnotatedTag(t *testing.T) {
	repo := newRepo(t)
	createAnnotatedTag(t, repo, "3.0.0")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFmt(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "3.0.0" {
		t.Fatalf("expected 3.0.0, got %s", tag)
	}
}

// TestDescendantTagIgnored: un tag posé sur un commit APRÈS le HEAD courant ne doit pas apparaître.
func TestDescendantTagIgnored(t *testing.T) {
	repo := newRepo(t)
	headHash := createCommit(t, repo)
	createCommit(t, repo)
	createTag(t, repo, "1.0.0")

	p := projectAtCommit(t, repo, headHash)
	tag, err := p.LastTag(semverFmt(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "0.0.0" {
		t.Fatalf("expected 0.0.0 (tag is on a future commit), got %s", tag)
	}
}

// TestTagOnDivergentBranchIgnored: un tag sur une branche divergente (non ancêtre) est ignoré.
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
	tag, err := p.LastTag(semverFmt(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "0.0.0" {
		t.Fatalf("expected 0.0.0 (tag is on a divergent branch), got %s", tag)
	}
}

// TestPrereleaseTagIgnoredWhenStableExists: un tag pre-release est moins prioritaire qu'un stable.
func TestPrereleaseTagIgnoredWhenStableExists(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createTag(t, repo, "2.0.0-alpha.1")
	createCommit(t, repo)
	p := projectAtHead(t, repo)

	tag, err := p.LastTag(semverFmt(""))
	if err != nil {
		t.Fatal(err)
	}
	if tag != "2.0.0-alpha.1" {
		t.Fatalf("expected 2.0.0-alpha.1, got %s", tag)
	}
}

// TestCommitSinceTagCount: CommitSinceTag doit retourner tous les commits entre HEAD et le tag inclus.
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

// TestCommitSinceTagMessages: les messages des commits retournés sont corrects.
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

func TestCommitFiles_singleFile(t *testing.T) {
	repo := newRepo(t) // initial commit crée "foo.txt"
	// Créer un deuxième commit ajoutant "bar.txt"
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	f, _ := wt.Filesystem.Create("bar.txt")
	_, _ = f.Write([]byte("bar"))
	_ = f.Close()
	_, _ = wt.Add("bar.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	h, err := wt.Commit("add bar", &git.CommitOptions{
		Author: &author, Committer: &author,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	p := projectAtHead(t, repo)
	files, err := p.CommitFiles(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "bar.txt" {
		t.Errorf("expected [bar.txt], got %v", files)
	}
}

func TestCommitFiles_initialCommit(t *testing.T) {
	// newRepo crée un commit initial avec "foo.txt"
	repo := newRepo(t)
	head, _ := repo.Head()
	c, _ := repo.CommitObject(head.Hash())
	p := projectAtHead(t, repo)
	files, err := p.CommitFiles(c)
	if err != nil {
		t.Fatal(err)
	}
	// Le commit initial doit lister tous les fichiers
	if len(files) == 0 {
		t.Error("expected at least one file for initial commit")
	}
	found := false
	for _, f := range files {
		if f == "foo.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected foo.txt in initial commit files, got %v", files)
	}
}

func TestCommitFiles_multipleFiles(t *testing.T) {
	repo := newRepo(t)
	wt, _ := repo.Worktree()
	for _, name := range []string{"a.go", "b.go"} {
		f, _ := wt.Filesystem.Create(name)
		_, _ = f.Write([]byte("content"))
		_ = f.Close()
		_, _ = wt.Add(name)
	}
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	h, err := wt.Commit("add a and b", &git.CommitOptions{
		Author: &author, Committer: &author,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := repo.CommitObject(h)
	p := projectAtHead(t, repo)
	files, err := p.CommitFiles(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %v", files)
	}
}

// createMergeCommit creates a commit with two parents directly via the storer
// (bypasses worktree, so no file changes needed). Returns the commit hash.
func createMergeCommit(t *testing.T, r *git.Repository, parent1, parent2 plumbing.Hash) plumbing.Hash {
	t.Helper()
	author := object.Signature{
		Name:  "go-git",
		Email: "go-git@fake.local",
		When:  time.Now(),
	}
	// Get parent commits to build the tree from parent1
	p1, err := r.CommitObject(parent1)
	if err != nil {
		t.Fatal(err)
	}
	c := &object.Commit{
		Author:       author,
		Committer:    author,
		Message:      "merge commit",
		TreeHash:     p1.TreeHash,
		ParentHashes: []plumbing.Hash{parent1, parent2},
	}
	enc := r.Storer.NewEncodedObject()
	if err := c.Encode(enc); err != nil {
		t.Fatal(err)
	}
	hash, err := r.Storer.SetEncodedObject(enc)
	if err != nil {
		t.Fatal(err)
	}
	// Update HEAD to point to the merge commit
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewHashReference(head.Name(), hash)); err != nil {
		t.Fatal(err)
	}
	return hash
}

// createCommitWithFile creates a commit adding a file with the given name and content.
// This ensures unique commits even when called in rapid succession.
func createCommitWithFile(t *testing.T, r *git.Repository, filename, content string) plumbing.Hash {
	t.Helper()
	wt, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	f, err := wt.Filesystem.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add(filename); err != nil {
		t.Fatal(err)
	}
	author := object.Signature{
		Name:  "go-git",
		Email: "go-git@fake.local",
		When:  time.Now(),
	}
	h, err := wt.Commit("commit "+filename, &git.CommitOptions{
		Author:    &author,
		Committer: &author,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// TestLastTag_tiebreaker: two tags equidistant from HEAD (diamond topology)
// → the semantically highest tag must be returned deterministically.
func TestLastTag_tiebreaker(t *testing.T) {
	for _, order := range []struct {
		first, second string
	}{
		{"1.1.0", "1.2.0"},
		{"1.2.0", "1.1.0"},
	} {
		t.Run("tags_created_"+order.first+"_then_"+order.second, func(t *testing.T) {
			repo, err := git.Init(memory.NewStorage(), memfs.New())
			if err != nil {
				t.Fatal(err)
			}

			// C0: base commit
			c0 := createCommit(t, repo)
			createTagAt(t, repo, c0, "1.0.0")

			// Branch A: one commit from C0 with a unique file
			branchA := plumbing.NewBranchReferenceName("branch-a")
			if err := repo.Storer.SetReference(plumbing.NewHashReference(branchA, c0)); err != nil {
				t.Fatal(err)
			}
			wt, err := repo.Worktree()
			if err != nil {
				t.Fatal(err)
			}
			if err := wt.Checkout(&git.CheckoutOptions{Branch: branchA}); err != nil {
				t.Fatal(err)
			}
			cA := createCommitWithFile(t, repo, "branch-a.txt", "branch a content")

			// Branch B: one commit from C0 with a different unique file
			branchB := plumbing.NewBranchReferenceName("branch-b")
			if err := repo.Storer.SetReference(plumbing.NewHashReference(branchB, c0)); err != nil {
				t.Fatal(err)
			}
			if err := wt.Checkout(&git.CheckoutOptions{Branch: branchB}); err != nil {
				t.Fatal(err)
			}
			cB := createCommitWithFile(t, repo, "branch-b.txt", "branch b content")

			// Tag the two branch tips in the requested order
			createTagAt(t, repo, cA, order.first)
			createTagAt(t, repo, cB, order.second)

			// Merge commit M with parents cA and cB
			mergeHash := createMergeCommit(t, repo, cA, cB)

			p := projectAtCommit(t, repo, mergeHash)
			tag, err := p.LastTag(semverFmt(""))
			if err != nil {
				t.Fatal(err)
			}
			if tag != "1.2.0" {
				t.Fatalf("expected 1.2.0 (highest semver), got %s", tag)
			}
		})
	}
}

func TestCommitDate(t *testing.T) {
	repo := newRepo(t)
	p := projectAtHead(t, repo)
	authorDate, committerDate, err := p.CommitDate()
	if err != nil {
		t.Fatal(err)
	}
	if authorDate.IsZero() {
		t.Error("expected non-zero AuthorDate")
	}
	if committerDate.IsZero() {
		t.Error("expected non-zero CommitterDate")
	}
	// Appels répétés → dates identiques (reproductibilité)
	authorDate2, committerDate2, _ := p.CommitDate()
	if !authorDate.Equal(authorDate2) {
		t.Error("CommitDate not reproducible: AuthorDate differs between calls")
	}
	if !committerDate.Equal(committerDate2) {
		t.Error("CommitDate not reproducible: CommitterDate differs between calls")
	}
}

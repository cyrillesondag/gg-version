package git

import (
	"fmt"
	"gover/format"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type Project struct {
	repo *git.Repository
	head *object.Commit
}

// CommitWithTags groups a commit with the tag names that point directly to it.
// Tags is nil for most commits.
type CommitWithTags struct {
	Commit *object.Commit
	Tags   []string
}

// CommitHistory returns all commits reachable from HEAD in topological order
// (HEAD first, ancestors later), each annotated with the tag names pointing to it.
// Annotated tags are resolved to their target commit before matching.
// Tags whose commits cannot be resolved (e.g. shallow clone) are silently skipped.
func (p Project) CommitHistory() ([]CommitWithTags, error) {
	// Pass 1: build map from commit hash → tag names
	tagsByCommit := map[plumbing.Hash][]string{}
	tagsRef, err := p.repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer tagsRef.Close()
	if err := tagsRef.ForEach(func(ref *plumbing.Reference) error {
		tagCommit, err := getCommitFromTag(p.repo, ref)
		if err != nil {
			return nil // skip unresolvable tags (e.g. beyond shallow boundary)
		}
		tagsByCommit[tagCommit.Hash] = append(tagsByCommit[tagCommit.Hash], ref.Name().Short())
		return nil
	}); err != nil {
		return nil, err
	}

	// Sort tags for each commit to ensure deterministic output.
	for hash, tags := range tagsByCommit {
		sort.Strings(tags)
		tagsByCommit[hash] = tags
	}

	// Pass 2: walk commits from HEAD, annotate with tags
	iter := object.NewCommitPreorderIter(p.head, nil, nil)
	defer iter.Close()
	var result []CommitWithTags
	if err := iter.ForEach(func(c *object.Commit) error {
		result = append(result, CommitWithTags{
			Commit: c,
			Tags:   tagsByCommit[c.Hash],
		})
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
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

// NewProjectFromRepo builds a Project directly from an already-opened repository
// and a commit hash. Used in tests with in-memory repositories.
func NewProjectFromRepo(repo *git.Repository, hash plumbing.Hash) (*Project, error) {
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("reading commit %s: %w", hash, err)
	}
	return &Project{repo, commit}, nil
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
			return nil
		}

		// Neither is ancestor of the other → equidistant tags.
		// Use Compare as a deterministic tiebreaker: keep the semantically highest.
		lastIsNewer, _ := isAncestor(lastTagCommit, tagCommit)
		if !lastIsNewer {
			cmp, err := f.Compare(tagRef.Name().Short(), lastTag.Name().Short())
			if err == nil && cmp > 0 {
				lastTag = tagRef
				lastTagCommit = tagCommit
			}
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

// CommitSinceTag returns all commits reachable from HEAD back to (and including)
// the tagged commit. The bool truncated is true when the tag commit was not found
// during traversal — this happens in shallow clones where the tag is beyond the
// clone depth. In that case the returned commits are those that were traversable.
func (p Project) CommitSinceTag(tag string) ([]*object.Commit, bool, error) {
	ref, err := p.repo.Tag(tag)
	if err != nil {
		return nil, false, fmt.Errorf("tag %q not found: %w", tag, err)
	}

	ancestor, err := getCommitFromTag(p.repo, ref)
	if err != nil {
		return nil, false, err
	}

	iter := object.NewCommitPreorderIter(p.head, nil, nil)
	defer iter.Close()

	found := false
	var history []*object.Commit
	err = iter.ForEach(func(c *object.Commit) error {
		history = append(history, c)
		if c.Hash == ancestor.Hash {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	return history, !found, nil
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

// CommitDate returns the author and committer timestamps of the HEAD commit.
func (p Project) CommitDate() (time.Time, time.Time, error) {
	return p.head.Author.When, p.head.Committer.When, nil
}

// CreateTag creates an annotated tag on HEAD with the given message.
// Returns an error if the tag already exists.
func (p Project) CreateTag(name, message string) error {
	sig := &object.Signature{
		Name:  "gg-version",
		Email: "gg-version@local",
		When:  time.Now(),
	}
	_, err := p.repo.CreateTag(name, p.head.Hash, &git.CreateTagOptions{
		Message: message,
		Tagger:  sig,
	})
	return err
}

// PushTags pushes all local tags to the "origin" remote.
// Uses SSH agent for authentication; falls back to nil auth for HTTPS remotes
// whose credentials are managed by the OS credential store.
func (p Project) PushTags() error {
	auth, _ := gitssh.NewSSHAgentAuth("git")
	return p.repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{"refs/tags/*:refs/tags/*"},
		Auth:       auth,
	})
}

// IsShallow reports whether this repository is a shallow clone.
// A shallow clone has one or more grafted commits (commits whose parents
// were artificially cut by git clone --depth=N).
func (p Project) IsShallow() bool {
	hashes, err := p.repo.Storer.Shallow()
	return err == nil && len(hashes) > 0
}

// CommitFiles returns the list of files changed in c relative to its first parent.
// For the initial commit (no parent), returns all files in the tree.
func (p Project) CommitFiles(c *object.Commit) ([]string, error) {
	if c.NumParents() == 0 {
		var files []string
		iter, err := c.Files()
		if err != nil {
			return nil, fmt.Errorf("listing files for initial commit: %w", err)
		}
		err = iter.ForEach(func(f *object.File) error {
			files = append(files, f.Name)
			return nil
		})
		return files, err
	}
	stats, err := c.Stats()
	if err != nil {
		return nil, fmt.Errorf("getting commit stats: %w", err)
	}
	files := make([]string, 0, len(stats))
	for _, s := range stats {
		files = append(files, s.Name)
	}
	return files, nil
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

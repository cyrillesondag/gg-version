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

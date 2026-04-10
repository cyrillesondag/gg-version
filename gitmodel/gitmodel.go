// Package gitmodel holds shared types used by both the git and strategy/semver packages.
// It is kept minimal to avoid import cycles.
package gitmodel

import "github.com/go-git/go-git/v5/plumbing/object"

// CommitWithTags groups a commit with the tag names that point directly to it.
// Tags is nil for most commits.
type CommitWithTags struct {
	Commit *object.Commit
	Tags   []string
}

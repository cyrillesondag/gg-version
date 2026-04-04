package main

type Version string

type Commit struct {
	LastVersion Version

	Message string

	Hash string

	BranchName string

	IsMainBranch bool

	IsReleaseBranch bool

	lastCommits []Commit
}

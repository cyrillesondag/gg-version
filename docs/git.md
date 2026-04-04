# Git Environment

`gg-version` extract various information from git history and tags and use it to generate version numbers.

## Basic information extraction

`gg-version` can extract information such as commit hashes, branch names, and tag names from git history and tags. This information can be used to generate version numbers that reflect the state of the codebase at a specific point in time


### Environment variables

| Name                  | Description             |
|-----------------------|-------------------------|
| `.Git.CommitHash`     | The current commit hash |
| `.Git.CommitMessage`  | Commit message          |
| `.Git.Branch`         | Commit branch           |
| `.Git.AuthorName`     | Commit author's name    |
| `.Git.AuthorEmail`    | Commit auhtor's email   |
| `.Git.AuthorDate`     | Commit author's date    |
| `.Git.CommitterName`  | Commit committer name   |
| `.Git.CommitterEmail` | Commit committer email  |
| `.Git.CommitterDate`  | Commit committer date   |


## Branching

You can also define branching rules for your versioning based on branch, and extract information from the branch name.

### Exemple

```yaml 
branches:
  releases:
    pattern: "release-*"
  
  
  
  schema: incremental 
  tagPrefix: "v" 
  format: "{{ .Incremental.NextInt }}"
```
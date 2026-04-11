# How-to guides

These guides answer specific goals. Pick the one that matches your situation.

---

## Use a `v` prefix on tags

By default no prefix is used. To make `gg-version` recognise tags like `v1.2.3`:

```yaml
# .gg-version.yml
semver:
  tag_prefix: "v"
```

```bash
git tag v1.0.0
gg-version next
# → v1.0.0

# After a feat: commit:
gg-version next
# → v1.1.0
```

---

## Customise the pre-release version format

On branches that are not releases, the version is rendered via a Go template. Default: `{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}`.

To include the short hash:

```yaml
semver:
  tag_prefix: "v"
  branches:
    - pattern: "main"
    - pattern: ".*"
      version_format: "{{ .semver.Semver }}-{{ .git.ShortHash }}"
```

```bash
# On branch feat/login, after a feat: commit:
gg-version next
# → 1.3.0-a1b2c3d
```

Another example — include the date:

```yaml
version_format: "{{ .semver.Semver }}-{{ .git.AuthorDate }}.{{ .git.CommitCount }}"
```

```bash
gg-version next
# → 1.3.0-2026-04-06.7
```

---

## Configure additional release branches

By default only `main` is a release branch. To add `master` and `release/*` branches:

```yaml
semver:
  tag_prefix: "v"
  branches:
    - pattern: "main"
    - pattern: "master"
    - pattern: "release/.*"
    - pattern: ".*"
      version_format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
```

On `release/1.x`, the version will be a full semver (e.g. `v1.4.2`), not a pre-release.

---

## Extract information from the branch name

Branch patterns are Go regular expressions with named captures. Captures are available as `{{ .regex.<name> }}`.

Example: extract the ticket number from `feat/PROJ-123-my-feature`:

```yaml
semver:
  branches:
    - pattern: "main"
    - pattern: "feat/(?P<ticket>[A-Z]+-[0-9]+)-.*"
      version_format: "{{ .semver.Semver }}-{{ .regex.ticket }}.{{ .git.CommitCount }}"
    - pattern: ".*"
      version_format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
```

```bash
# On branch feat/PROJ-123-login
gg-version next
# → 1.3.0-PROJ-123.4
```

---

## Pass custom variables to the template

`--var name=value` injects variables accessible as `{{ .var.name }}`:

```bash
gg-version --var env=staging --var region=eu-west next
```

With format:

```yaml
version_format: "{{ .semver.Semver }}-{{ .var.env }}-{{ .git.ShortHash }}"
```

```
1.3.0-staging-a1b2c3d
```

Useful for including build metadata without modifying the configuration.

---

## Ignore certain file paths

To exclude commits that only touch documentation from version analysis:

```yaml
semver:
  tag_prefix: "v"
  ignore_paths:
    - "docs/**"
    - "*.md"
    - ".github/**"
```

A commit that modifies only `docs/tutorial.md` does not contribute to the version. A commit that modifies both `docs/tutorial.md` and `src/api.go` is kept (all files must match for a commit to be excluded).

---

## Ignore specific commits by SHA

To exclude a specific commit (CI hotfix, automatic merge commit…):

```yaml
semver:
  ignore_commits:
    - "a1b2c3d"      # short prefix is enough
    - "deadbeef12"
```

```bash
# Check the effect
gg-version env | grep CommitCount
# git.CommitCount=4   ← the ignored commit is not counted
```

---

## Use gg-version in a monorepo

For projects where multiple components are versioned independently in the same repository:

```yaml
# .gg-version.yml
semver:
  tag_prefix: "v"

components:
  api:
    path: "api/**"
  frontend:
    path: "frontend/**"
  shared:
    path: "shared/**"
    tag_scope: "libs"   # tags will be libs/v1.0.0 instead of shared/v1.0.0
```

```bash
gg-version next
# @root        v2.1.0
# api          v0.5.0
# frontend     v3.2.1
# shared       v1.0.0
```

Each component is versioned from commits that touch its directory. `@root` represents everything that does not touch any declared component.

Filter to a single component:

```bash
gg-version next --component api
# → v0.5.0

gg-version next --root
# → v2.1.0
```

List components and their tag patterns:

```bash
gg-version components
# api          path=api/**                      tag=api/v*
# frontend     path=frontend/**                 tag=frontend/v*
# shared       path=shared/**                   tag=libs/v*
```

---

## Integrate gg-version in a CI pipeline

### GitHub Actions

```yaml
- name: Compute version
  id: version
  run: echo "value=$(gg-version next)" >> $GITHUB_OUTPUT

- name: Build
  run: docker build -t myapp:${{ steps.version.outputs.value }} .
```

### GitLab CI

```yaml
compute-version:
  script:
    - export APP_VERSION=$(gg-version next)
    - echo "APP_VERSION=$APP_VERSION" >> build.env
  artifacts:
    reports:
      dotenv: build.env
```

### Makefile

```makefile
VERSION := $(shell gg-version next)

.PHONY: build
build:
	go build -ldflags="-X main.version=$(VERSION)" ./...
```

---

## Inject the version into a Go binary

```bash
gg-version next
# v1.4.2
```

```makefile
VERSION := $(shell gg-version next)

build:
	go build -ldflags="-X main.Version=$(VERSION)" -o myapp .
```

```go
// main.go
var Version = "dev"

func main() {
    fmt.Println("Version:", Version)
}
```

```bash
make build && ./myapp
# Version: v1.4.2
```

---

## Debug version computation

When the displayed version surprises you, inspect all variables:

```bash
gg-version env
```

```
git.AuthorDate=2026-04-06
git.Branch=main
git.CommitCount=3
git.CommitterDate=2026-04-06
git.Hash=abc1234def5678...
git.LastTag=v1.2.0
git.ShortHash=abc1234
semver.HasNonConventionalCommits=false
semver.IsBreakingChange=false
semver.IsPreRelease=false
semver.LastMajor=1
semver.LastMinor=2
semver.LastPatch=0
semver.LastVersion=1.2.0
semver.Major=1
semver.Minor=3
semver.Patch=0
semver.Semver=1.3.0
```

In JSON format for CI parsing:

```bash
gg-version env --format json
```

```json
{
  "git": {
    "AuthorDate": "2026-04-06",
    "Branch": "main",
    "CommitCount": 3,
    "CommitterDate": "2026-04-06",
    "Hash": "abc1234def5678...",
    "LastTag": "v1.2.0",
    "ShortHash": "abc1234"
  },
  "semver": {
    "Major": "1",
    "Minor": "3",
    "Patch": "0",
    "Semver": "1.3.0"
  }
}
```

Check the effective configuration (useful to diagnose a misread `.gg-version.yml`):

```bash
gg-version config
# config from: .gg-version.yml
# semver:
#   tag_prefix: v
#   ...
```

---

## Work with a remote repository or a subdirectory

```bash
# Repository in another directory
gg-version --repo /path/to/other-project next

# Config file in a non-standard location
gg-version --config config/versioning.yml next

# Both combined
gg-version --repo ../backend --config ../backend/.gg-version.yml next
```

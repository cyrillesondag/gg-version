# Branch Version Constraint Design

## Goal

Replace the implicit version constraint mechanism (named captures `major`/`minor`/`patch` in branch patterns) with an explicit `constraint` field on `BranchConfig`. The `constraint` value is a Go template rendered at runtime, producing a wildcard semver string that restricts which tags are considered valid for that branch.

## Problem with the Current Design

Version constraints are currently extracted implicitly: if a branch pattern contains named captures named exactly `major`, `minor`, or `patch`, they are silently used as version constraints via `versionConstraints()`. This is:

- **Invisible** — nothing in the config signals that `(?P<major>[0-9]+)` has special behaviour beyond being a regex capture
- **Conflicting** — the names `major`/`minor`/`patch` also appear in `conventional_commits` with a different meaning
- **Inflexible** — constraints cannot be injected from `--var` flags; they can only come from the branch name

## Design

### `BranchConfig` — before and after

```go
// Before
type BranchConfig struct {
    Pattern       string `yaml:"pattern"`
    VersionFormat string `yaml:"version_format"`
}

// After
type BranchConfig struct {
    Pattern       string `yaml:"pattern"`
    VersionFormat string `yaml:"version_format"`
    Constraint    string `yaml:"constraint"` // wildcard template, e.g. "{{ .regex.major }}.x.x"
}
```

### `constraint` field semantics

`constraint` is a Go template string. When a branch matches, the template is rendered with two namespaces:

- `.regex.*` — named captures from the matching branch pattern
- `.var.*` — variables injected via `--var name=value` on the CLI

The namespaces `.semver.*` and `.git.*` are **not** available: the constraint filters which tags to walk, so it is evaluated before git history is read.

The rendered value must follow the wildcard semver format:

```
<component>.<component>.<component>
```

where each `<component>` is either a non-negative integer or `x` (wildcard = no constraint on that component).

### Wildcard parsing — `parseWildcardConstraint`

```
"1.x.x"  → {"major": "1"}               (only tags with major=1)
"1.2.x"  → {"major": "1", "minor": "2"} (only tags with major=1, minor=2)
"1.2.3"  → {"major": "1", "minor": "2", "patch": "3"}
"x.x.x"  → {}                           (no filter, equivalent to no constraint)
""        → {}                           (absent constraint = no filter)
```

Any value that does not match `(x|\d+)\.(x|\d+)\.(x|\d+)` after rendering is a runtime error. The strategy logs the error and falls back to no constraint (rather than failing the command), to avoid breaking CI on misconfigured non-critical branches.

### Removal of implicit mechanism

`versionConstraints()` is deleted. Named captures `major`, `minor`, and `patch` in branch patterns become ordinary regex captures — they are available as `{{ .regex.major }}` in both `version_format` and `constraint` templates, but carry no implicit meaning.

### Default config

The default config has no `constraint` on any branch — behaviour is unchanged for users who have not configured constraints:

```go
Branches: []BranchConfig{
    {Pattern: "main"},
    {Pattern: "master"},
    {
        Pattern:       ".*",
        VersionFormat: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}",
    },
},
```

### YAML examples

```yaml
semver:
  tag_prefix: "v"
  branches:
    # Release branch locked to a major version from the branch name
    - pattern: "release/(?P<major>[0-9]+)\\.x"
      constraint: "{{ .regex.major }}.x.x"

    # Release branch locked to a major+minor version from the branch name
    - pattern: "release/(?P<major>[0-9]+)\\.(?P<minor>[0-9]+)"
      constraint: "{{ .regex.major }}.{{ .regex.minor }}.x"

    # Constraint injected via --var (e.g. gg-version --var stream=2 next)
    - pattern: "main"
      constraint: "{{ .var.stream }}.x.x"

    # No constraint — considers all valid tags
    - pattern: ".*"
      version_format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
```

## Validation

`validate.go` validates `Constraint` as a syntactically valid Go template (template parse, not render — runtime values are not known at validation time). An invalid template produces a validation error:

```
semver.branches[0].constraint "{{ .Unclosed": ...
```

## Files Changed

| File | Change |
|---|---|
| `config/config.go` | Add `Constraint string \`yaml:"constraint"\`` to `BranchConfig` |
| `config/validate.go` | Validate `Constraint` as a Go template; error message: `semver.branches[N].constraint` |
| `config/config_test.go` | Add round-trip test for `constraint` field |
| `config/validate_test.go` | Add test for invalid `constraint` template |
| `strategy/semver/semver.go` | Delete `versionConstraints()`; add `parseWildcardConstraint()`; update `Current()`, `Last()`, `Vars()` to render + parse `Constraint` |
| `strategy/semver/semver_test.go` | Rewrite `TestLastRespectsMajorConstraint` using `constraint:`; add `parseWildcardConstraint` unit tests |
| `docs/reference.md` | Document `constraint:` in schema; remove mention of implicit named-capture mechanism |
| `docs/how-to.md` | Update "Extract information from the branch name" section |
| `docs/explanation.md` | Update version constraint explanation |

## Acceptance Criteria

- [ ] `BranchConfig` has a `Constraint string \`yaml:"constraint"\`` field
- [ ] `versionConstraints()` function is deleted; named captures `major`/`minor`/`patch` are no longer special
- [ ] `parseWildcardConstraint("1.x.x")` returns `{"major": "1"}`, etc.
- [ ] `constraint: "{{ .regex.major }}.x.x"` on `release/1.x` restricts tags to `1.*.*`
- [ ] `constraint: "{{ .var.stream }}.x.x"` with `--var stream=2` restricts tags to `2.*.*`
- [ ] Invalid constraint template produces validation error in `validate.go`
- [ ] Malformed rendered constraint (not matching wildcard format) is logged and ignored (no crash)
- [ ] `go test ./...` passes
- [ ] Documentation updated

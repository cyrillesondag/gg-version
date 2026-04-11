# Branch Version Format Redesign

## Goal

Simplify the branch configuration by removing the `release` boolean flag and renaming `format` to `version_format`. The presence or absence of `version_format` is the sole signal for whether a branch produces a release or a pre-release version.

## Problem with the Current Design

`BranchConfig` couples two orthogonal concerns into one boolean:

1. **Format control** — does the output use `tagPrefix + semver` or render a template?
2. **Stability semantics** — is this a stable release or a pre-release (`semver.IsPreRelease`)?

Additionally, the field name `format` is ambiguous: it sounds like it could control the tag format or the version format, and its current value must include the full semver prefix (e.g., `{{ .semver.Semver }}-...`) which is redundant — every branch produces a semver-based version.

## Design

### `BranchConfig` — before and after

```go
// Before
type BranchConfig struct {
    Pattern string `yaml:"pattern"`
    Release bool   `yaml:"release"`
    Format  string `yaml:"format"` // full version template
}

// After
type BranchConfig struct {
    Pattern       string `yaml:"pattern"`
    VersionFormat string `yaml:"version_format"` // suffix template; empty = release
}
```

No backward compatibility. Fields `release` and `format` are silently ignored by the YAML parser if present in existing configs.

### Version output rule

```
output = tag_prefix + rendered(version_format || "{{ .semver.Semver }}")
```

- `version_format` absent or empty → rendered as `{{ .semver.Semver }}` = just the computed semver → output = `tagPrefix + semver` (e.g., `v1.3.0`)
- `version_format` set → rendered with all template variables → output = `tagPrefix + rendered` (e.g., `v1.3.0-feat/login.4`)

`tag_prefix` is always prepended to the rendered output. It is never included in the `version_format` template.

### `semver.IsPreRelease`

`IsPreRelease = version_format != ""`

A branch with no `version_format` (or empty `version_format`) is a release branch. A branch with a non-empty `version_format` is a pre-release branch.

### Default fallback (no pattern matches)

When no branch pattern matches, the strategy uses this default:

```go
config.BranchConfig{
    VersionFormat: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}",
}
```

### Default config

```go
Branches: []BranchConfig{
    {Pattern: "main"},   // no version_format = release
    {Pattern: "master"}, // no version_format = release
    {
        Pattern:       ".*",
        VersionFormat: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}",
    },
},
```

Equivalent YAML:
```yaml
semver:
  tag_prefix: "v"
  initial: "0.1.0"
  branches:
    - pattern: "main"
    - pattern: "master"
    - pattern: ".*"
      version_format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
```

## Version constraints from branch name

The existing mechanism is unchanged: named captures `major`, `minor`, `patch` in the branch `pattern` are automatically extracted and used as tag constraints in `SemverFormat`. Example:

```yaml
branches:
  - pattern: "release/(?P<major>[0-9]+)\\.x"
    # version_format absent = release, constrained to that major
```

On branch `release/1.x`, only tags `1.*.*` are considered valid. This prevents a `feat!:` commit from bumping to `2.0.0` on a maintenance branch.

## Template variables available in `version_format`

All four namespaces remain available: `semver`, `git`, `regex`, `var`.

Common patterns:

```yaml
version_format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
# → v1.3.0-feat/login.4

version_format: "{{ .semver.Semver }}-{{ .git.ShortHash }}"
# → v1.3.0-a1b2c3d

version_format: "{{ .semver.Semver }}-{{ .regex.ticket }}.{{ .git.CommitCount }}"
# → v1.3.0-PROJ-123.4  (with pattern "feat/(?P<ticket>[A-Z]+-[0-9]+)-.*")

version_format: "{{ .semver.Semver }}-{{ .git.AuthorDate }}.{{ .git.CommitCount }}"
# → v1.3.0-2026-04-11.7
```

## Files Changed

| File | Change |
|---|---|
| `config/config.go` | Remove `Release bool`, rename `Format` → `VersionFormat string`; update `DefaultConfig()` to add `master` pattern and use new field |
| `config/validate.go` | Remove any validation referencing `Release` or `Format` |
| `config/config_test.go` | Update test fixtures and assertions |
| `config/validate_test.go` | Update test fixtures |
| `strategy/semver/semver.go` | Update `Current()`: replace `branchCfg.Release` check with `branchCfg.VersionFormat == ""`; update `varsCore()`: `IsPreRelease = branchCfg.VersionFormat != ""`; update `matchBranch()` fallback; update `renderTemplate` call sites |
| `strategy/semver/semver_test.go` | Update `fakeProject` fixtures and test cases |
| `docs/reference.md` | Update config schema section: remove `release`, rename `format` → `version_format`; update examples |
| `docs/how-to.md` | Update all branch config examples |
| `docs/tutorial.md` | Update step 4 (config file) and step 5 (feature branch) |
| `docs/explanation.md` | Update "releases vs pre-releases" section |

## Acceptance Criteria

- [ ] `config.BranchConfig` has no `Release` field and uses `VersionFormat string`
- [ ] `DefaultConfig()` includes `main`, `master` (no `version_format`), and `.*` with default `version_format`
- [ ] `gg-version next` on `main` with `tag_prefix: "v"` outputs `v1.3.0` (not `1.3.0`)
- [ ] `gg-version next` on a feature branch outputs `v1.3.0-feat/my-feature.4`
- [ ] `semver.IsPreRelease = true` on branches with a non-empty `version_format`
- [ ] `semver.IsPreRelease = false` on branches with no `version_format`
- [ ] Version constraints from named captures (`major`, `minor`, `patch`) still work
- [ ] `go test ./...` passes
- [ ] Reference documentation updated to show new config schema

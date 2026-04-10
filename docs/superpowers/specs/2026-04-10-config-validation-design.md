# Config Validation — Design Spec

**Date:** 2026-04-10

## Problem

Fields in `.gg-version.yml` are accepted without validation at `Load()` time and fail later with cryptic runtime errors:
- An invalid Go regex in a branch pattern panics or silently never matches
- An invalid semver in `initial` causes an obscure error during version computation
- An empty component `path` silently disables path filtering
- An invalid template in a branch format crashes at render time

A validation pass at startup surfaces all configuration problems at once with clear messages.

---

## Architecture

A new file `config/validate.go` (package `config`) contains a single exported function:

```go
func Validate(cfg Config) error
```

`Load()` calls `Validate()` after YAML unmarshalling. If validation fails, `Load()` returns an error and a zero `Config`. `DefaultConfig()` is unchanged — its values always pass validation.

**Error format:** collect all violations into `[]string`, join with `\n`, return as:

```
config validation failed:
  semver.initial "not-a-semver": invalid semver
  semver.branches[0].pattern "(?invalid": error parsing regexp...
  components.api.path: must not be empty
```

**Dependencies** (all already in `go.mod`):
- `regexp.Compile` — Go regex patterns
- `text/template` — branch format templates
- `github.com/coreos/go-semver/semver` — semver initial
- `github.com/bmatcuk/doublestar/v4` — glob patterns

---

## Fields Validated

| Field | Rule |
|---|---|
| `semver.initial` | Valid semver string if non-empty (via `go-semver`) |
| `semver.branches[i].pattern` | Valid Go regex via `regexp.Compile` |
| `semver.branches[i].format` | Valid Go template via `template.Parse` if `release: false` |
| `semver.conventional_commits.format` | Valid Go regex if non-empty |
| `semver.conventional_commits.major/minor/patch[j]` | Each entry is a valid Go regex |
| `semver.ignore_paths[j]` | Valid doublestar glob via `doublestar.ValidatePattern` |
| `components.<name>.path` | Non-empty + valid doublestar glob |
| `components.<name>.tag_scope` | Valid git ref name component if non-empty |

### Git ref name validation for `tag_scope`

Pure Go implementation (no `git` binary required). Rejects:
- ASCII control characters or whitespace
- Characters forbidden in git refs: `~`, `^`, `:`, `?`, `*`, `[`, `\`
- Sequences: `..`, `@{`
- Suffix: `.lock`
- Leading `.` or `-`

### Edge cases

- `semver.branches` empty → no violation (valid config)
- Template validation for `format`: syntactic only (`text/template.Parse`); undefined variables are not checked at load time
- `tag_scope` validation runs only when the field is non-empty (empty = default to component name, which is a plain identifier already constrained by YAML map keys)

---

## Tests

File: `config/validate_test.go`, package `config_test`.

| Test | Scenario |
|---|---|
| `TestValidate_default` | `DefaultConfig()` passes with no error |
| `TestValidate_invalidInitial` | `initial: "not-a-semver"` → violation reported |
| `TestValidate_invalidBranchPattern` | branch `pattern: "(?invalid"` → violation |
| `TestValidate_invalidBranchFormat` | branch `format: "{{ .Unclosed"` → violation |
| `TestValidate_invalidCCFormat` | CC `format: "(?bad"` → violation |
| `TestValidate_invalidCCPatterns` | one Major/Minor/Patch pattern invalid → violation |
| `TestValidate_invalidIgnorePath` | `ignore_paths` entry invalid → violation |
| `TestValidate_componentEmptyPath` | component `path: ""` → violation |
| `TestValidate_componentInvalidPath` | component `path` invalid glob → violation |
| `TestValidate_componentInvalidTagScope` | `tag_scope: "bad name"` (space) → violation |
| `TestValidate_multipleViolations` | config with 3 invalid fields → 3 violations in error |

`Load()` integration: a YAML file with an invalid config must return an error from `Load()` (one test using a temp file).

---

## Files

- **Create:** `config/validate.go` — `Validate(cfg Config) error` + git ref name helper
- **Create:** `config/validate_test.go` — all test cases above
- **Modify:** `config/config.go` — `Load()` calls `Validate()` before returning

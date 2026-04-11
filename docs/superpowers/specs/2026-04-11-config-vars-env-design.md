# Config Vars and Environment Variable Resolution Design

## Goal

Allow users to define template variables in the config file and resolve environment variables in templates, reducing reliance on `--var` flags at the CLI call site.

## Problem with the Current Design

Variables injected via `{{ .var.* }}` can only be supplied at the CLI with `--var key=value`. There is no way to:

- Define default variable values in the config file
- Reference environment variables in templates (e.g. `$CI_BUILD_NUMBER`)

This forces every CI invocation to repeat `--var` flags that rarely change, and makes it impossible to compose env-var values with a fallback.

## Design

### Priority Order

When multiple sources define the same key in `.var.*`, the last one wins:

```
config vars: (rendered with .env.*)  →  CLI --var  →  result in .var.*
```

CLI always wins. Config vars are defaults.

### `SemverConfig.Vars` — new field

```go
type SemverConfig struct {
    // ...existing fields...
    Vars map[string]string `yaml:"vars"` // values are Go templates rendered with .env.*
}
```

Each value is a Go template string. Only `.env.*` is available when rendering (`.var.*`, `.semver.*`, `.git.*` are not yet known). Rendered results become the base `.var.*` values before CLI overrides are applied.

### `.env.*` namespace

All OS environment variables are available as `{{ .env.<NAME> }}` in **every** template in the system: `version_format`, `constraint`, `vars:` values, and component equivalents.

Implementation: a single `envMap() map[string]interface{}` call reads `os.Environ()` once per command invocation and is threaded through all template rendering calls.

If an env var is absent, `{{ .env.MISSING }}` produces an empty string (Go template default with `missingkey=zero`).

### `default` template function

Added to the `FuncMap` of `renderTemplate`:

```go
"default": func(def, val interface{}) interface{} {
    s, _ := val.(string)
    if s == "" {
        return def
    }
    return val
},
```

This enables: `{{ .env.STREAM | default "1" }}` — returns `"1"` when `STREAM` is absent or empty.

### YAML examples

```yaml
semver:
  tag_prefix: "v"
  vars:
    stream: "{{ .env.STREAM | default \"1\" }}"   # env var with fallback
    build:  "{{ .env.CI_BUILD_NUMBER }}"           # raw env var
    env:    "prod"                                 # static value
  branches:
    - pattern: "main"
      constraint: "{{ .var.stream }}.x.x"
      version_format: "{{ .semver.Semver }}+{{ .var.build }}"
    - pattern: ".*"
      version_format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
```

```bash
# Uses stream=1 from config (STREAM not set), build from CI_BUILD_NUMBER env var
gg-version next

# Overrides stream to 2 regardless of config or STREAM env var
gg-version --var stream=2 next

# .env.* also usable directly in templates
# version_format: "{{ .semver.Semver }}+{{ .env.CI_BUILD_NUMBER }}"
```

### Resolution sequence in `commands.go`

A new helper `resolveConfigVars(vars map[string]string, env map[string]interface{}) map[string]string`:

1. For each entry in `config.Semver.Vars`, render the value as a Go template with `{"env": envMap}` as data
2. On render error: log warning to stderr, use empty string (non-fatal, same pattern as `resolveConstraint`)
3. Merge with `flags.Vars` — CLI keys overwrite config keys
4. Return merged map as the `extra` passed to all strategy calls

### Template data in the strategy layer

`renderTemplate` receives `envMap` as an additional argument and injects it under `"env"`. All callers of `renderTemplate` — `varsCore`, `resolveConstraint`, component variants — receive `envMap` threaded through.

The data map for template rendering:

```go
map[string]interface{}{
    "semver": {...},   // existing
    "git":    {...},   // existing
    "regex":  {...},   // existing
    "var":    {...},   // existing (merged config vars + CLI)
    "env":    envMap,  // new: all OS environment variables
}
```

## Validation

`validate.go` validates each `vars` value as a syntactically valid Go template (template parse only — runtime values not known at validation time):

```
semver.vars[stream] "{{ .Unclosed": template: ...
```

## Files Changed

| File | Change |
|---|---|
| `config/config.go` | Add `Vars map[string]string \`yaml:"vars"\`` to `SemverConfig` |
| `config/validate.go` | Validate each `Vars` value as Go template; error key `semver.vars[<key>]` |
| `config/config_test.go` | YAML round-trip test for `vars:` |
| `config/validate_test.go` | Test invalid template in `vars:` value |
| `strategy/semver/semver.go` | Add `envMap()`; add `default` to FuncMap; thread `envMap` into all `renderTemplate` calls; update `varsCore`, `Current`, `Last` |
| `strategy/semver/semver_test.go` | Tests: `.env.*` in `version_format`, `vars:` rendered with `.env.*`, CLI priority over config vars |
| `strategy/semver/component.go` | Thread `envMap` through `varsCoreFromHistory`, `AllLast`, `AllLint` |
| `cmd/gg-version/commands.go` | Add `resolveConfigVars()`; merge with `flags.Vars`; pass merged extra to all strategy calls |
| `docs/reference.md` | Document `vars:`, `.env.*` namespace, `default` function |
| `docs/how-to.md` | Add CI/CD examples with env vars |

## Acceptance Criteria

- [ ] `semver.vars:` in YAML is loaded as `SemverConfig.Vars map[string]string`
- [ ] Each `vars:` value is rendered as a Go template with `.env.*` available
- [ ] Rendered config vars are overridden by `--var` (CLI wins)
- [ ] `{{ .env.NAME }}` is available in `version_format`, `constraint`, and `vars:` values
- [ ] `{{ .env.MISSING | default "fallback" }}` returns `"fallback"` when env var is absent
- [ ] Invalid template in `vars:` produces validation error `semver.vars[<key>]`
- [ ] Render error in a `vars:` value at runtime logs warning and uses empty string (no crash)
- [ ] `go test ./...` passes

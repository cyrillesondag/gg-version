# Software Versioning Formats

`gg-version` supports multiple versioning schemas for release tagging and version rendering.

## Overview

Choose the schema that matches your release process:

- **SemVer** for software and APIs that follow semantic compatibility rules
- **CalVer** for date-based releases
- **Incremental** for simple numeric versioning

## How version selection works

`gg-version` looks for the most relevant valid tag reachable from the current commit history.

- Tags that do not match the selected schema are ignored
- Tags on unrelated or non-reachable branches are ignored
- If no valid tag is found, the default version is returned

## SemVer

Semantic Versioning uses the format: MAJOR.MINOR.PATCH

Example: 2.5.1

Meaning:

- **MAJOR**: incompatible API changes
- **MINOR**: backward-compatible features
- **PATCH**: bug fixes

Pre-release examples: 1.0.0-alpha 1.0.0-beta 1.0.0-rc.1

📖 Specification:
https://semver.org/

### Configuration

```yaml 
version: 
  schema: semver 
  tagPrefix: "v" 
  format: "{{ .Semver.Major }}.{{ .Semver.Minor }}.{{ .Semver.Patch }}"
```

### Environment variables

When using the `semver` schema, the following variables are available:

| Name                       | Description                                      |
|----------------------------|--------------------------------------------------|
| `.Semver.Version`          | The current SemVer value being processed         |
| `.Semver.Major`            | Current major version                            |
| `.Semver.Minor`            | Current minor version                            |
| `.Semver.Patch`            | Current patch version                            |
| `.Semver.LastVersion`      | Previous valid version from history              |
| `.Semver.LastMajor`        | Major version from the previous valid version    |
| `.Semver.LastMinor`        | Minor version from the previous valid version    |
| `.Semver.LastPatch`        | Patch version from the previous valid version    |
| `.Semver.IsPreRelease`     | True if the current version is a Pre-release     |
| `.Semver.IsBreakingChange` | True if the current version is a breaking change |


### Notes

- Use `tagPrefix` if your Git tags are prefixed, for example `v1.2.3`
- The prefix is used for tag matching and rendering consistency
- Pre-release tags are supported if they are valid SemVer values

### Common pitfalls

- `1.0` is not a valid SemVer version
- `v1.0.0` is only valid when the prefix is expected by configuration
- Tags on future commits or divergent branches are ignored

## CalVer

Calendar Versioning uses a date-based format: `YYYY.MM.DD YY.MM`

Examples: 
 - `2026.04`
 - `2026.04.03`

Use CalVer when releases are tied to time rather than API compatibility.

Typical examples include:

- Ubuntu
- Arch Linux

📖 Specification:
https://calver.org/

### Configuration

```yaml
version:
  schema: calver 
  tagPrefix: "v" 
  format: "{{ .Calver.Year }}.{{ .Calver.Month }}.{{ .Calver.Day }}"
```

### Environment variables

When using the `calver` schema, the following variables are available:

| Name                | Description                           |
|---------------------|---------------------------------------|
| `.Calver.Year`      | Release year                          |
| `.Calver.Month`     | Release month                         |
| `.Calver.Day`       | Release day                           |
| `.Calver.Hour`      | Release hour                          |
| `.Calver.LastYear`  | Year from the previous valid version  |
| `.Calver.LastMonth` | Month from the previous valid version |
| `.Calver.LastDay`   | Day from the previous valid version   |
| `.Calver.LastHour`  | Hour from the previous valid version  |

### Notes

- CalVer is useful for regularly scheduled releases
- Make sure your pipeline uses a consistent source of time/date information
- If you use a prefix such as `v`, ensure your Git tags follow the same convention

### Common pitfalls

- Mixing commit date, build date, and release date can produce inconsistent versions
- `YY.MM` and `YYYY.MM.DD` should not be mixed in the same release line unless intentionally planned

## Incremental

Incremental versioning is a simple counter: 1, 2, 3

With a prefix: v1, v2, v3

Use this schema when you only need a monotonically increasing number and do not require compatibility semantics.

📖 Reference:
No formal specification

### Configuration

```yaml 
version: 
  schema: incremental 
  tagPrefix: "v" 
  format: "{{ .Incremental.NextInt }}"
```

### Environment variables

When using the `incremental` schema, the following variables are available:

| Name                   | Description            |
|------------------------|------------------------|
| `.Incremental.NextInt` | Next version increment |
| `.Incremental.LastInt` | Last version increment |

### Notes

- Best suited for internal builds or lightweight release tracking
- Not ideal for public APIs where compatibility meaning matters
- Ensure your CI/CD pipeline preserves the counter source consistently

## Schema comparison

| Schema      | Best for                      | Pros                                   | Cons                          |
|-------------|-------------------------------|----------------------------------------|-------------------------------|
| SemVer      | APIs and application releases | Widely understood, compatibility-aware | Requires discipline           |
| CalVer      | Time-based releases           | Easy to relate to release date         | Less useful for compatibility |
| Incremental | Internal build numbers        | Simple and predictable                 | Not descriptive               |

## CI/CD usage tips

- Keep tag naming consistent across branches and environments
- Use one schema per release stream
- Validate tags before publishing artifacts
- Document the prefix convention in your pipeline
- Avoid creating tags on branches that should not influence release version selection

## Validation examples

Valid examples:

- `1.2.3`
- `v1.2.3`
- `2026.04.03`
- `v2026.04`

Invalid examples:

- `1.0`
- `release-1`
- `latest`
- `beta`

## Recommended usage

- **Use SemVer** for library, API, and application release management
- **Use CalVer** for scheduled distribution releases
- **Use Incremental** for internal tools, previews, or simple counters
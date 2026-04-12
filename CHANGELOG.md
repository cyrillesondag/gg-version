# Changelog

All notable changes to `gg-version` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- `vars:` field in config — define default template variables rendered with `.env.*`
- `.env.*` namespace in all templates — access OS environment variables via `{{ .env.VAR }}`
- `default` template function — `{{ .env.VAR | default "fallback" }}`
- `constraint:` field on branch config — explicit semver constraint replacing implicit named-capture mechanism
- Shallow clone detection — warning on stderr when history may be truncated
- OSSF Scorecard workflow — publishes score to GitHub Security tab
- MIT License, SECURITY.md, CONTRIBUTING.md

### Changed
- All GitHub Actions workflows hardened: `permissions: read-all` + SHA-pinned action references
- Upgraded to golangci-lint v2

---

<!-- Releases will be added here automatically by GoReleaser -->

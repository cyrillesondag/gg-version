# OSSF Scorecard & Repo Best Practices Design

## Goal

Configure the `cyrillesondag/gg-version` GitHub repository to follow open-source best practices and achieve a high OSSF Scorecard score.

## Scope

This covers four independent areas, each mapping to one or more OSSF Scorecard checks:

| Area | OSSF checks addressed |
|---|---|
| Compliance files (LICENSE, SECURITY.md, CODEOWNERS) | License, Security-Policy, Code-Review |
| Workflow hardening (permissions, pinned SHAs) | Token-Permissions, Pinned-Dependencies |
| OSSF Scorecard workflow + badge | Scorecard published to GitHub Security tab |
| Branch protection on `main` | Branch-Protection, Code-Review |

---

## Section 1 — Compliance Files

### `LICENSE`

MIT licence, copyright `Cyrille Sondag`, year `2024`.

Standard MIT text at the repo root.

### `SECURITY.md`

Responsible disclosure policy:

- Reporting channel: GitHub Security Advisories (`https://github.com/cyrillesondag/gg-version/security/advisories/new`)
- Response target: 7 days acknowledgement, 90 days resolution
- No bug bounty
- No public issue reports for vulnerabilities — use advisories only

### `.github/CODEOWNERS`

```
* @cyrillesondag
```

All files owned by `@cyrillesondag`. Ensures any future PR review requirement is routed correctly.

---

## Section 2 — Workflow Hardening

### Principle: least-privilege `GITHUB_TOKEN`

Every workflow gets `permissions: read-all` at the top level. Individual jobs that need broader access declare their own `permissions:` block:

| Workflow | Job-level override |
|---|---|
| `ci.yml` | none needed (read-all sufficient) |
| `security-codeql.yml` | `security-events: write`, `actions: read`, `contents: read` |
| `security-vulncheck.yml` | none needed |
| `release.yml` | `contents: write` on the release job |
| `scorecard.yml` | `security-events: write`, `id-token: write`, `contents: read`, `actions: read` |

### Principle: pin all action references to commit SHA

Replace all `uses: owner/action@vX.Y.Z` with `uses: owner/action@<sha> # vX.Y.Z`.

Dependabot (already configured for `github-actions` ecosystem) will open PRs to bump the SHAs when new versions are released.

**SHA table (at time of writing):**

| Action | Tag | Commit SHA |
|---|---|---|
| `actions/checkout` | v6.0.2 | `de0fac2e4500dabe0009e67214ff5f5447ce83dd` |
| `actions/setup-go` | v6.4.0 | `4a3601121dd01d1626a1e23e37211e3254c1c06c` |
| `golangci/golangci-lint-action` | v9.2.0 | `1e7e51e771db61008b38414a730f564565cf7c20` |
| `goreleaser/goreleaser-action` | v7.0.0 | `ec59f474b9834571250b370d4735c50f8e2d1e29` |
| `github/codeql-action` | v4.35.1 | `c10b8064de6f491fea524254123dbe5e09572f13` |
| `golang/govulncheck-action` | v1.0.4 | `b625fbe08f3bccbe446d94fbf87fcc875a4f50ee` |
| `ossf/scorecard-action` | v2.4.3 | `4eaacf0543bb3f2c246792bd56e8cdeffafb205a` |

### `security-vulncheck.yml` — use official action

Replace the `go install golang.org/x/vuln/cmd/govulncheck@latest` + manual run with:

```yaml
- uses: golang/govulncheck-action@b625fbe08f3bccbe446d94fbf87fcc875a4f50ee # v1.0.4
```

---

## Section 3 — OSSF Scorecard Workflow + Badge

### `.github/workflows/scorecard.yml`

```yaml
name: Scorecard

on:
  push:
    branches: [main]
  schedule:
    - cron: "0 3 * * 1"   # weekly, Monday 03:00 UTC
  pull_request:
    branches: [main]

permissions: read-all

jobs:
  scorecard:
    name: OSSF Scorecard
    runs-on: ubuntu-latest
    permissions:
      security-events: write
      id-token: write
      contents: read
      actions: read
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
        with:
          persist-credentials: false
      - uses: ossf/scorecard-action@4eaacf0543bb3f2c246792bd56e8cdeffafb205a # v2.4.3
        with:
          results_file: results.sarif
          results_format: sarif
          publish_results: true
      - uses: github/codeql-action/upload-sarif@c10b8064de6f491fea524254123dbe5e09572f13 # v4.35.1
        with:
          sarif_file: results.sarif
          category: ossf-scorecard
```

### Badge in `README.md`

Add after the existing badges (or at the top if none exist):

```markdown
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/cyrillesondag/gg-version/badge)](https://securityscorecards.dev/viewer/?uri=github.com/cyrillesondag/gg-version)
```

---

## Section 4 — Branch Protection on `main`

Applied via `gh api` (GitHub REST API v3).

**Rules:**

| Setting | Value |
|---|---|
| Require pull request before merging | yes |
| Required approving reviews | 0 (no review required) |
| Dismiss stale reviews | no |
| Require status checks to pass | yes |
| Required status checks | `Test (ubuntu-latest)`, `Test (macos-14)`, `Test (windows-latest)`, `Build (ubuntu-latest)`, `Build (macos-14)`, `Build (windows-latest)`, `Lint` |
| Require branches to be up to date | yes |
| Allow force pushes | no |
| Allow deletions | no |
| Restrict who can push to matching branches | no (any contributor with write access) |

Applied with:

```bash
gh api repos/cyrillesondag/gg-version/branches/main/protection \
  --method PUT \
  --input protection.json
```

Where `protection.json` contains the full protection payload.

---

## Files Changed

| File | Action |
|---|---|
| `LICENSE` | Create — MIT licence |
| `SECURITY.md` | Create — responsible disclosure policy |
| `.github/CODEOWNERS` | Create |
| `.github/workflows/ci.yml` | Modify — `permissions: read-all`, pin SHAs |
| `.github/workflows/security-codeql.yml` | Modify — `permissions: read-all`, pin SHAs, add `pull_request` trigger |
| `.github/workflows/security-vulncheck.yml` | Modify — `permissions: read-all`, pin SHAs, use `golang/govulncheck-action` |
| `.github/workflows/release.yml` | Modify — `permissions: read-all`, pin SHAs, job-level `contents: write` |
| `.github/workflows/scorecard.yml` | Create — OSSF Scorecard workflow |
| `README.md` | Modify — add OSSF Scorecard badge |
| Branch protection | Configure via `gh api` (not a file) |

---

## Acceptance Criteria

- [ ] `LICENSE` exists at repo root with MIT text
- [ ] `SECURITY.md` exists with GitHub Advisory reporting channel
- [ ] `.github/CODEOWNERS` routes all files to `@cyrillesondag`
- [ ] All workflow files have `permissions: read-all` at top level
- [ ] All `uses:` lines reference SHA not a tag alias
- [ ] `scorecard.yml` workflow exists and runs without error on push to `main`
- [ ] OSSF Scorecard badge appears in `README.md`
- [ ] `main` branch requires PR and passing CI before merge
- [ ] `go test ./...` still passes (no functional regression)

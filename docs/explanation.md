# Concepts: how gg-version computes versions

This document explains the reasoning behind how `gg-version` works. Read it if you want to understand *why* the tool behaves the way it does, not just *how* to use it.

---

## The fundamental principle: read without writing

`gg-version` never creates a tag, never makes a commit, never modifies any file. It reads the Git history and computes what the version *should* be — the decision to create a tag remains entirely yours.

This separation is deliberate. The tool can be run at any time with no side effects, making it ideal in CI: the same call produces the same result whether you are checking locally or inside a pipeline.

---

## How the version is computed

The computation happens in three phases.

### Phase 1: find the last tag

`gg-version` walks all ancestors of HEAD and identifies valid tags (those that start with the configured `tag_prefix` and contain a valid semver). It keeps the **topologically closest** tag — not the most recent in time, but the closest in the commit graph.

If multiple tags are at equal distance, the semantically highest one is kept.

If no tag is found, the `initial` value is used as the base (default `0.1.0`).

### Phase 2: analyse the commits in between

`gg-version` retrieves all commits between the last tag and HEAD (the tag itself excluded). It analyses each commit message against the Conventional Commits rules:

- The **subject** (first line) is tested against the `major`, `minor`, `patch` patterns in that order.
- **Footers** (lines after the first blank line) are also tested — this is where `BREAKING CHANGE:` is recognised.
- A commit that does not follow the CC format is treated as a patch (and sets `HasNonConventionalCommits=true`).
- A CC commit of an unknown type (e.g. `docs:`, `chore:`) contributes `BumpNone` — it is recognised but does not increment the version.

The final bump level is the maximum across all analysed commits.

### Phase 3: produce the version

The bump is applied to the last tag's version to get the computed semver (e.g. `1.3.0`). Then:

**If HEAD is exactly on a tag** → the version is that tag, without computation.

**If the branch is a release branch** (no `version_format` or empty) → the version is `tag_prefix + semver` (e.g. `v1.3.0`). This is the version you should tag.

**If the branch is a pre-release branch** (non-empty `version_format`) → the template is rendered with all available variables and appended to the tag prefix. The resulting version identifies the build without claiming to be a release.

---

## Releases vs pre-releases

The `version_format` field in the branch configuration controls whether a branch is a release or a pre-release.

A branch with **no `version_format`** (or an empty one) is a **release branch** (`main`, `master`, `release/x.y`…). It produces clean versions ready to be tagged: `v1.3.0`. These versions are stable and mean "this code is ready to ship".

A branch with a **non-empty `version_format`** is a **pre-release branch** (feature, hotfix, develop…). The template is rendered and appended to the tag prefix: `v1.3.0-feat/login.5`. These versions allow tracing a precise build without polluting the stable version namespace.

`semver.IsPreRelease` is `true` when the current branch has a non-empty `version_format`.

---

## Why `last` and `next` are different

`gg-version last` answers: *"What is the last tag that was created?"*

`gg-version next` answers: *"What version does this code represent?"*

On an untagged commit on `main` with `feat:` commits since `v1.2.0`, the answers are:

```bash
gg-version last     # v1.2.0  — the last existing tag
gg-version next     # v1.3.0  — the version this code should have
```

`last` is useful for checking what was shipped. `next` is useful for naming what is going to be shipped.

---

## How path filtering works (monorepo)

When `ignore_paths` or components are configured, commits are filtered before the CC analysis.

The exclusion rule is intentionally strict: a commit is ignored only if **all** its modified files match an exclusion pattern. A commit that touches both `docs/README.md` and `src/api.go` is not ignored — it contributes to the version even though documentation is excluded.

This rule avoids false negatives: it is better to over-count a commit than to miss it and produce a version that under-estimates the real change.

For components, the logic is symmetric: a commit belongs to a component if at least one of its files matches the component's `path` (partial inclusion).

---

## How version constraints work

When a `constraint` field is set on a branch configuration, `gg-version` restricts which tags it considers when searching for the last tag.

The `constraint` value is a Go template rendered with:
- `.regex.*` — named captures from the matching branch pattern
- `.var.*` — variables injected via `--var` flags

The rendered result must be a wildcard semver in the form `N.N.N`, where each component is either an integer or `x` (wildcard). Examples: `1.x.x`, `2.3.x`.

Only tags whose version falls within the constraint range are considered. Tags outside the range are invisible to the search, as if they did not exist.

**Why this matters:** without a constraint, a `feat!:` commit on a `release/1.x` maintenance branch would cause `gg-version` to compute `2.0.0`, because the last reachable tag might be `v1.9.0` and a major bump produces `2.0.0`. With `constraint: "{{ .regex.major }}.x.x"`, the search is limited to `v1.*.*` tags, so the bump stays within the `1.x.x` range.

This mechanism is explicit and intentional: you set `constraint` when you want to confine a branch to a version range. Branches without `constraint` always consider all reachable tags.

---

## Component isolation in a monorepo

Each component lives in its own tag namespace (`{scope}/{prefix}{version}`) and its own commit namespace (filtered by `path`).

`@root` is the implicit component that represents "everything that does not touch a declared component". Its commits are those that belong to no component. This is useful for versioning global configuration, deployment scripts, or any shared code that does not deserve its own component.

Important consequence: a commit that touches two components (`api/` and `frontend/`) counts for both. This is not a bug — a shared change must be reflected in the version of both components.

---

## Go templates

Version formats use the standard Go template syntax (`text/template`). A few useful reminders:

```
{{ .semver.Major }}                       → raw value
{{ printf "%02d" .git.CommitCount }}      → numeric formatting
```

Templates have access to all variables from the four namespaces: `semver`, `git`, `regex`, `var`. A missing variable produces an empty string without error.

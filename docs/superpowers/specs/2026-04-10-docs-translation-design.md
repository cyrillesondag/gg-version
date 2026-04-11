# Documentation Translation Design

## Goal

Translate all user-facing Markdown documentation from French to English. Apply the one-language strategy: existing files are overwritten in-place; no parallel French copies are kept. Fix stale references to the removed `current` command and the placeholder `yourorg` repository URL at the same time.

## Scope

Five files:

| File | Lines | Changes beyond translation |
|---|---|---|
| `README.md` | 56 | Replace `gg-version current` → `gg-version next`; replace `github.com/yourorg/gg-version` → `github.com/cyrillesondag/gg-version` |
| `docs/tutorial.md` | 176 | Replace all `gg-version current` occurrences → `gg-version next`; update surrounding explanations accordingly |
| `docs/how-to.md` | 370 | Replace all `gg-version current` occurrences → `gg-version next`; update surrounding explanations accordingly |
| `docs/reference.md` | 582 | Translation only (stale-ref note about `current` already present, remove it since the command no longer exists) |
| `docs/explanation.md` | 111 | Replace `current` command references in conceptual examples → `next` |

## Translation Rules

1. **Prose** — translate to English.
2. **Command names, flags, YAML keys, code blocks** — leave unchanged.
3. **Technical terms** (Conventional Commits, semver, monorepo, Go template) — leave unchanged.
4. **Inline code comments** — translate to English.
5. **Tone** — match the existing tone: direct, imperative, no filler.

## `current` → `next` Replacement Rules

The `current` command was removed. Its semantics (version at HEAD, whether tagged or not) are now covered by `next`:

- `gg-version current` → `gg-version next`
- `VERSION=$(gg-version current)` → `VERSION=$(gg-version next)`
- Prose describing what `current` does → rewrite to describe `next`
- In `explanation.md`, the section "Pourquoi `last` et `current` sont différents" → retitle as "Why `last` and `next` are different"; update the examples to use `next`
- In `reference.md`, remove the stale note "La commande `current` a été supprimée. Utilisez `next`..." since the command is gone and no longer needs a migration note

## README fixes

- `github.com/yourorg/gg-version` → `github.com/cyrillesondag/gg-version` (two occurrences: `go install` and `git clone`)
- Quick-start example: replace `gg-version current` → `gg-version next` with appropriate comment

## Implementation Approach

One task per file, one commit per file. Tasks are independent (no cross-file dependencies in content). Order: README first (shortest, good warm-up), then tutorial, how-to, reference, explanation.

Commit message format per file:
```
docs: translate <filename> to English
```

## Acceptance Criteria

- [ ] All five files are in English
- [ ] No `gg-version current` occurrences remain anywhere in `docs/` or `README.md`
- [ ] `github.com/cyrillesondag/gg-version` used everywhere in README (no `yourorg`)
- [ ] Cross-document links (e.g. `[how-to.md](how-to.md)`) still work
- [ ] `git grep -i "current" docs/ README.md` returns only legitimate occurrences (e.g. "current branch", "current version" in prose — not the removed CLI command)
- [ ] `go test ./...` still passes (docs changes have no impact on code)

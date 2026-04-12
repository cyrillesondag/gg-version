# Contributing to gg-version

Thank you for your interest in contributing!

## Reporting Bugs

Open a [GitHub issue](https://github.com/cyrillesondag/gg-version/issues/new?template=bug_report.md) with:
- `gg-version` version (`gg-version --version`)
- Operating system and Go version
- Minimal reproduction steps
- Actual vs. expected output

Do **not** report security vulnerabilities as public issues — use [GitHub Security Advisories](https://github.com/cyrillesondag/gg-version/security/advisories/new) instead.

## Submitting Changes

1. **Fork** the repository and create a branch from `main`
2. **Write tests** — all changes must include tests; run `go test ./...`
3. **Follow Conventional Commits** for commit messages:
   - `feat:` new feature
   - `fix:` bug fix
   - `chore:` maintenance (no behavior change)
   - `docs:` documentation only
   - `test:` test-only changes
4. **Run the linter** before opening a PR: `golangci-lint run`
5. **Open a pull request** against `main` with a clear description of the change and a link to the related issue

## Development Setup

```bash
git clone https://github.com/cyrillesondag/gg-version.git
cd gg-version
go test ./...                              # run all tests
go build ./cmd/gg-version                  # build the binary
golangci-lint run                          # lint
```

All tests use in-memory git repositories — no real git installation is required.

## Code Style

- Follow standard Go formatting (`gofmt`)
- Keep functions small and focused
- Avoid adding dependencies unless strictly necessary — this is an intentional zero-dependency-at-runtime tool
- Do not write to the repository from the tool (core invariant)

## Licence

By contributing you agree that your contributions will be licensed under the [MIT License](LICENSE).

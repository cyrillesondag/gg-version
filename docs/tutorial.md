# Tutorial: your first automatic version

In this tutorial you will configure `gg-version` on a real Git repository and get your first automatically computed version number. No prior knowledge is required.

**What you will have at the end:** a command usable in your CI pipeline that prints `v1.2.3` (or the appropriate version) on every build.

---

## Prerequisites

- `gg-version` installed (`go install` or downloaded binary)
- A Git repository with at least one commit

---

## Step 1 — Check that gg-version sees your repository

From the root of your repository:

```bash
gg-version next
```

Expected result if you have no tags:

```
0.1.0
```

This is the default initial version. The tool works out of the box with no configuration.

---

## Step 2 — Create your first version tag

`gg-version` reads existing Git tags. Create your starting point:

```bash
git tag v1.0.0
```

Verify that the tool recognises it:

```bash
gg-version next
```

```
v1.0.0
```

`gg-version` confirms that HEAD points to a tagged commit — the computed version is exactly that tag.

---

## Step 3 — Add commits and observe the evolution

Make a few commits after the tag. Use the Conventional Commits format:

```bash
echo "change" >> README.md
git add README.md
git commit -m "fix: correct typo in README"

echo "feature" >> feature.txt
git add feature.txt
git commit -m "feat: add feature.txt"
```

Run again:

```bash
gg-version next
```

```
v1.1.0
```

`gg-version` analysed the two commits:
- `fix:` → patch bump
- `feat:` → minor bump (overrides the patch)

The result is `v1.1.0`, the next tag that should be created.

---

## Step 4 — Create a configuration file

Without configuration, the tag prefix is empty. Most projects use `v`. Create `.gg-version.yml` at the root:

```yaml
semver:
  tag_prefix: "v"
  initial: "0.1.0"
```

Verify the loaded configuration:

```bash
gg-version config
```

```yaml
# config from: .gg-version.yml
semver:
  tag_prefix: "v"
  initial: "0.1.0"
  branches:
    - pattern: main
    - pattern: master
    - pattern: .*
      version_format: '{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}'
  ...
```

---

## Step 5 — Observe the behaviour on a feature branch

Create a branch:

```bash
git checkout -b feat/my-feature
echo "work" >> work.txt
git add work.txt
git commit -m "feat: add work"
```

```bash
gg-version next
```

```
1.2.0-feat/my-feature.1
```

On a branch that is not `main`, `gg-version` generates a pre-release identifier with the branch name and the commit count.

---

## Step 6 — Use the version in a script

```bash
VERSION=$(gg-version next)
echo "Building version $VERSION"
docker build -t myapp:$VERSION .
```

On `main`:
```
Building version v1.2.0
```

On a feature branch:
```
Building version 1.2.0-feat/my-feature.1
```

---

## Summary

You have learned to:

1. Get a version with no configuration
2. Anchor a version with a Git tag
3. Observe how Conventional Commits evolve the version
4. Create a minimal configuration file
5. See the difference between a release branch and a feature branch
6. Capture the version in a CI script

**Next step:** see the [how-to guides](how-to.md) for specific scenarios, or the [reference](reference.md) for the full list of options.

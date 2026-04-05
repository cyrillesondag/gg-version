# Conventional Commits Version Calculation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Calculer `current` en analysant les commits depuis le dernier tag via Conventional Commits, avec réorganisation complète des namespaces de variables de template.

**Architecture:** Trois modifications indépendantes et séquentielles : (1) config — ajout de `ConventionalCommitsConfig` avec patterns regex ; (2) nouveau fichier `conventional.go` dans le package `semver` pour le parsing CC et le calcul de bump ; (3) refactoring de `Vars()` et `Current()` pour intégrer CC et réorganiser les namespaces `semver` et `git`. Le package `go-git` est déjà utilisé pour accéder aux messages de commit.

**Tech Stack:** Go, `github.com/coreos/go-semver/semver` (déjà présent), `regexp` (stdlib), `github.com/go-git/go-git/v5/plumbing/object` (déjà présent)

---

### Task 1: ConventionalCommitsConfig dans config.go

**Goal:** Ajouter `ConventionalCommitsConfig` dans `SemverConfig` avec les patterns regex Conventional Commits par défaut.

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`

**Acceptance Criteria:**
- [ ] `SemverConfig` contient `ConventionalCommits ConventionalCommitsConfig`
- [ ] `ConventionalCommitsConfig` a les champs `Format`, `Major`, `Minor`, `Patch` (tous `[]string` sauf `Format` qui est `string`)
- [ ] `DefaultConfig()` initialise les patterns par défaut
- [ ] `TestDefaultConfig_conventionalCommits` passe
- [ ] `/usr/local/go/bin/go test ./config/ -v` → PASS

**Verify:** `/usr/local/go/bin/go test ./config/ -v` → PASS

**Steps:**

- [ ] **Step 1 : Écrire le test qui échoue dans `config/config_test.go`**

Ajouter à la fin du fichier :

```go
func TestDefaultConfig_conventionalCommits(t *testing.T) {
	cfg := config.DefaultConfig()
	cc := cfg.Semver.ConventionalCommits
	if cc.Format == "" {
		t.Error("expected ConventionalCommits.Format to be set")
	}
	if len(cc.Major) == 0 {
		t.Error("expected ConventionalCommits.Major to have at least one pattern")
	}
	if len(cc.Minor) == 0 {
		t.Error("expected ConventionalCommits.Minor to have at least one pattern")
	}
	if len(cc.Patch) == 0 {
		t.Error("expected ConventionalCommits.Patch to have at least one pattern")
	}
	// Verify breaking change patterns are present
	foundExclamation := false
	foundFooter := false
	for _, p := range cc.Major {
		if strings.Contains(p, "!") {
			foundExclamation = true
		}
		if strings.Contains(p, "BREAKING") {
			foundFooter = true
		}
	}
	if !foundExclamation {
		t.Error("expected Major patterns to include breaking change exclamation pattern")
	}
	if !foundFooter {
		t.Error("expected Major patterns to include BREAKING CHANGE footer pattern")
	}
}
```

Ajouter `"strings"` dans le bloc import du fichier de test :
```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gover/config"
)
```

- [ ] **Step 2 : Vérifier que le test échoue**

```bash
/usr/local/go/bin/go test ./config/ -v -run TestDefaultConfig_conventionalCommits
```

Résultat attendu : FAIL — `ConventionalCommits` n'existe pas encore.

- [ ] **Step 3 : Implémenter dans `config/config.go`**

Ajouter le struct `ConventionalCommitsConfig` et mettre à jour `SemverConfig` :

```go
type ConventionalCommitsConfig struct {
	Format string   `yaml:"format"`
	Major  []string `yaml:"major"`
	Minor  []string `yaml:"minor"`
	Patch  []string `yaml:"patch"`
}

type SemverConfig struct {
	TagPrefix           string                   `yaml:"tag_prefix"`
	Initial             string                   `yaml:"initial"`
	Branches            []BranchConfig           `yaml:"branches"`
	ConventionalCommits ConventionalCommitsConfig `yaml:"conventional_commits"`
}
```

Mettre à jour `DefaultConfig()` pour initialiser `ConventionalCommits` :

```go
func DefaultConfig() Config {
	return Config{
		Semver: SemverConfig{
			TagPrefix: "",
			Initial:   "0.1.0",
			Branches: []BranchConfig{
				{
					Pattern: "main",
					Release: true,
					Format:  "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}",
				},
				{
					Pattern: ".*",
					Release: false,
					Format:  "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}",
				},
			},
			ConventionalCommits: ConventionalCommitsConfig{
				Format: `^\w+(?:\(.+\))?!?:`,
				Major: []string{
					`^\w+(?:\(.+\))?!:`,
					`BREAKING[- ]CHANGE:`,
				},
				Minor: []string{`^feat(?:\(.+\))?:`},
				Patch: []string{`^fix(?:\(.+\))?:`},
			},
		},
	}
}
```

**Note :** Les formats de branches passent de `{{ .semver.LastTag }}` / `{{ .semver.CommitCount }}` à `{{ .semver.Semver }}` / `{{ .git.CommitCount }}` — c'est le nouveau namespace validé en design.

- [ ] **Step 4 : Vérifier que le test passe**

```bash
/usr/local/go/bin/go test ./config/ -v
```

Résultat attendu : tous les tests PASS (existants + `TestDefaultConfig_conventionalCommits`).

- [ ] **Step 5 : Vérifier le build global**

```bash
/usr/local/go/bin/go build ./...
```

Résultat attendu : succès (des warnings de compilation dans `strategy/semver/` sont possibles si les templates dans les tests référencent encore les anciens champs, mais le build doit passer).

```json:metadata
{"files": ["config/config.go", "config/config_test.go"], "verifyCommand": "/usr/local/go/bin/go test ./config/ -v", "acceptanceCriteria": ["ConventionalCommitsConfig struct avec Format/Major/Minor/Patch", "SemverConfig contient ConventionalCommits", "DefaultConfig() initialise les patterns", "TestDefaultConfig_conventionalCommits passe"]}
```

---

### Task 2: analyzeBump et bumpVersion dans conventional.go

**Goal:** Créer `strategy/semver/conventional.go` avec `analyzeBump` (scanning CC des commits) et `bumpVersion` (application du bump), plus 9 tests unitaires.

**Files:**
- Create: `strategy/semver/conventional.go`
- Create: `strategy/semver/conventional_test.go`

**Acceptance Criteria:**
- [ ] `analyzeBump(commits, cfg)` retourne `(level int, hasNonCC bool)`
- [ ] `bumpVersion(effectiveLastTag, prefix, level)` retourne la version bumped SANS préfixe
- [ ] `feat:` → minor (level 2)
- [ ] `fix:` → patch (level 1)
- [ ] `feat!:` et `feat(scope)!:` → major (level 3)
- [ ] `BREAKING CHANGE:` et `BREAKING-CHANGE:` dans le footer → major
- [ ] Type CC non mappé (`chore:`, `docs:`) → no bump (level 0)
- [ ] Message non-CC → patch par défaut + `hasNonCC = true`
- [ ] Tous les commits no-bump → level 0
- [ ] Mix → le niveau le plus élevé gagne
- [ ] Aucun commit → level 0, `hasNonCC = false`
- [ ] `/usr/local/go/bin/go test ./strategy/semver/ -run "TestAnalyzeBump|TestBumpVersion" -v` → PASS

**Verify:** `/usr/local/go/bin/go test ./strategy/semver/ -run "TestAnalyzeBump|TestBumpVersion" -v` → PASS

**Steps:**

- [ ] **Step 1 : Créer `strategy/semver/conventional_test.go` avec les tests qui échouent**

```go
package semver_test

import (
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"gover/config"
	semverstrategy "gover/strategy/semver"
)

// defaultCC returns a ConventionalCommitsConfig with standard defaults for tests.
func defaultCC() config.ConventionalCommitsConfig {
	return config.ConventionalCommitsConfig{
		Format: `^\w+(?:\(.+\))?!?:`,
		Major:  []string{`^\w+(?:\(.+\))?!:`, `BREAKING[- ]CHANGE:`},
		Minor:  []string{`^feat(?:\(.+\))?:`},
		Patch:  []string{`^fix(?:\(.+\))?:`},
	}
}

// makeCommit creates an in-memory commit with the given message (does not require a repo).
func makeCommit(t *testing.T, repo *gogit.Repository, msg string) *object.Commit {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	f, err := wt.Filesystem.Create("dummy.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte(msg))
	_, _ = wt.Add("dummy.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	h, err := wt.Commit(msg, &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAnalyzeBump_feat(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "feat: add new login page")
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
	if level != 2 {
		t.Errorf("expected level 2 (minor), got %d", level)
	}
	if hasNonCC {
		t.Error("expected hasNonCC=false for CC-formatted commit")
	}
}

func TestAnalyzeBump_fix(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "fix: correct null pointer")
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
	if level != 1 {
		t.Errorf("expected level 1 (patch), got %d", level)
	}
	if hasNonCC {
		t.Error("expected hasNonCC=false")
	}
}

func TestAnalyzeBump_breakingExclamation(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "feat!: remove deprecated endpoint")
	c2 := makeCommit(t, repo, "fix(api)!: change response format")
	for _, c := range []*object.Commit{c1, c2} {
		level, _ := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
		if level != 3 {
			t.Errorf("expected level 3 (major) for %q, got %d", c.Message, level)
		}
	}
}

func TestAnalyzeBump_breakingFooter(t *testing.T) {
	repo := newRepo(t)
	msgSpace := "feat: new api\n\nBREAKING CHANGE: old api removed"
	msgHyphen := "feat: new api\n\nBREAKING-CHANGE: old api removed"
	c1 := makeCommit(t, repo, msgSpace)
	c2 := makeCommit(t, repo, msgHyphen)
	for _, c := range []*object.Commit{c1, c2} {
		level, _ := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
		if level != 3 {
			t.Errorf("expected level 3 (major) for footer %q, got %d", c.Message, level)
		}
	}
}

func TestAnalyzeBump_noneType(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "chore: update dependencies")
	c2 := makeCommit(t, repo, "docs: fix typo in README")
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{c1, c2}, defaultCC())
	if level != 0 {
		t.Errorf("expected level 0 (no bump) for chore/docs commits, got %d", level)
	}
	if hasNonCC {
		t.Error("expected hasNonCC=false — chore/docs are CC-formatted")
	}
}

func TestAnalyzeBump_nonCC(t *testing.T) {
	repo := newRepo(t)
	c := makeCommit(t, repo, "update readme with new examples")
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{c}, defaultCC())
	if level != 1 {
		t.Errorf("expected level 1 (patch default) for non-CC commit, got %d", level)
	}
	if !hasNonCC {
		t.Error("expected hasNonCC=true for non-CC commit")
	}
}

func TestAnalyzeBump_allNone(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "chore: bump go version")
	c2 := makeCommit(t, repo, "style: reformat files")
	level, _ := semverstrategy.AnalyzeBump([]*object.Commit{c1, c2}, defaultCC())
	if level != 0 {
		t.Errorf("expected level 0 (all no-bump types), got %d", level)
	}
}

func TestAnalyzeBump_mixed(t *testing.T) {
	repo := newRepo(t)
	c1 := makeCommit(t, repo, "fix: correct crash")
	c2 := makeCommit(t, repo, "feat: add search")
	level, _ := semverstrategy.AnalyzeBump([]*object.Commit{c1, c2}, defaultCC())
	if level != 2 {
		t.Errorf("expected level 2 (minor wins over patch), got %d", level)
	}
}

func TestAnalyzeBump_noCommits(t *testing.T) {
	level, hasNonCC := semverstrategy.AnalyzeBump([]*object.Commit{}, defaultCC())
	if level != 0 {
		t.Errorf("expected level 0 for no commits, got %d", level)
	}
	if hasNonCC {
		t.Error("expected hasNonCC=false for no commits")
	}
}

func TestBumpVersion_patch(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3", "", 1)
	if got != "1.2.4" {
		t.Errorf("expected 1.2.4, got %s", got)
	}
}

func TestBumpVersion_minor(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3", "", 2)
	if got != "1.3.0" {
		t.Errorf("expected 1.3.0, got %s", got)
	}
}

func TestBumpVersion_major(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3", "", 3)
	if got != "2.0.0" {
		t.Errorf("expected 2.0.0, got %s", got)
	}
}

func TestBumpVersion_withPrefix(t *testing.T) {
	got := semverstrategy.BumpVersion("v1.2.3", "v", 1)
	if got != "1.2.4" {
		t.Errorf("expected 1.2.4 (no prefix in result), got %s", got)
	}
}

func TestBumpVersion_none(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3", "", 0)
	if got != "1.2.3" {
		t.Errorf("expected 1.2.3 unchanged for level 0, got %s", got)
	}
}

func TestBumpVersion_clearsPreRelease(t *testing.T) {
	got := semverstrategy.BumpVersion("1.2.3-rc.1", "", 1)
	if got != "1.2.4" {
		t.Errorf("expected 1.2.4 (pre-release cleared), got %s", got)
	}
}
```

- [ ] **Step 2 : Vérifier que les tests échouent**

```bash
/usr/local/go/bin/go test ./strategy/semver/ -run "TestAnalyzeBump|TestBumpVersion" -v 2>&1 | head -20
```

Résultat attendu : erreur de compilation — `AnalyzeBump` et `BumpVersion` n'existent pas.

- [ ] **Step 3 : Créer `strategy/semver/conventional.go`**

```go
package semver

import (
	"regexp"
	"strings"

	gosemver "github.com/coreos/go-semver/semver"
	"github.com/go-git/go-git/v5/plumbing/object"

	"gover/config"
)

// Bump levels — exported so tests can reference them symbolically.
const (
	BumpNone  = 0
	BumpPatch = 1
	BumpMinor = 2
	BumpMajor = 3
)

// AnalyzeBump scans commits and returns:
//   - level: 0=no bump, 1=patch, 2=minor, 3=major
//   - hasNonCC: true if at least one commit subject did not match the CC format
func AnalyzeBump(commits []*object.Commit, cfg config.ConventionalCommitsConfig) (level int, hasNonCC bool) {
	formatRe := compilePattern(cfg.Format)
	majorRes := compilePatterns(cfg.Major)
	minorRes := compilePatterns(cfg.Minor)
	patchRes := compilePatterns(cfg.Patch)

	for _, c := range commits {
		lines := strings.Split(strings.TrimRight(c.Message, "\n"), "\n")
		if len(lines) == 0 {
			continue
		}

		subject := lines[0]
		footerLines := extractFooterLines(lines)

		// Classify subject
		subjectLevel := levelFromPatterns(subject, majorRes, minorRes, patchRes)
		if subjectLevel >= 0 {
			// matched a bump pattern — use it
		} else if formatRe != nil && formatRe.MatchString(subject) {
			subjectLevel = BumpNone // CC-formatted but unrecognized type → no contribution
		} else {
			hasNonCC = true
			subjectLevel = BumpPatch // non-CC → patch by default
		}

		// Classify footer lines (no format check — just bump patterns)
		footerLevel := BumpNone
		for _, fl := range footerLines {
			l := levelFromPatterns(fl, majorRes, minorRes, patchRes)
			if l > footerLevel {
				footerLevel = l
			}
		}

		commitLevel := subjectLevel
		if footerLevel > commitLevel {
			commitLevel = footerLevel
		}
		if commitLevel > level {
			level = commitLevel
		}
	}
	return level, hasNonCC
}

// BumpVersion applies the bump level to effectiveLastTag (stripping prefix) and returns
// the bumped version WITHOUT prefix. Returns the stripped version unchanged for BumpNone.
func BumpVersion(effectiveLastTag, prefix string, level int) string {
	versionStr := strings.TrimPrefix(effectiveLastTag, prefix)
	if level == BumpNone {
		return versionStr
	}
	sv, err := gosemver.NewVersion(versionStr)
	if err != nil {
		return versionStr // unparseable — return as-is
	}
	switch level {
	case BumpMajor:
		sv.Major++
		sv.Minor = 0
		sv.Patch = 0
		sv.PreRelease = gosemver.PreRelease("")
	case BumpMinor:
		sv.Minor++
		sv.Patch = 0
		sv.PreRelease = gosemver.PreRelease("")
	case BumpPatch:
		sv.Patch++
		sv.PreRelease = gosemver.PreRelease("")
	}
	return sv.String()
}

// levelFromPatterns returns the bump level matching the line, or -1 if no pattern matched.
func levelFromPatterns(line string, majorRes, minorRes, patchRes []*regexp.Regexp) int {
	for _, re := range majorRes {
		if re.MatchString(line) {
			return BumpMajor
		}
	}
	for _, re := range minorRes {
		if re.MatchString(line) {
			return BumpMinor
		}
	}
	for _, re := range patchRes {
		if re.MatchString(line) {
			return BumpPatch
		}
	}
	return -1
}

// extractFooterLines returns lines after the first blank line in the commit message.
func extractFooterLines(lines []string) []string {
	for i := 1; i < len(lines); i++ {
		if lines[i] == "" && i+1 < len(lines) {
			return lines[i+1:]
		}
	}
	return nil
}

func compilePattern(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re
}

func compilePatterns(patterns []string) []*regexp.Regexp {
	res := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			res = append(res, re)
		}
	}
	return res
}
```

- [ ] **Step 4 : Vérifier que tous les tests CC passent**

```bash
/usr/local/go/bin/go test ./strategy/semver/ -run "TestAnalyzeBump|TestBumpVersion" -v
```

Résultat attendu : tous PASS.

```json:metadata
{"files": ["strategy/semver/conventional.go", "strategy/semver/conventional_test.go"], "verifyCommand": "/usr/local/go/bin/go test ./strategy/semver/ -run \"TestAnalyzeBump|TestBumpVersion\" -v", "acceptanceCriteria": ["AnalyzeBump retourne level int + hasNonCC bool", "BumpVersion retourne version sans préfixe", "9 tests AnalyzeBump passent", "6 tests BumpVersion passent"]}
```

---

### Task 3: Refactoring Vars() et Current() + mise à jour tests

**Goal:** Mettre à jour `Vars()` pour intégrer CC et réorganiser les namespaces `semver`/`git`, mettre à jour `Current()` pour retourner la version CC-calculée sur les branches release, et corriger tous les tests impactés.

**Files:**
- Modify: `strategy/semver/semver.go`
- Modify: `strategy/semver/semver_test.go`
- Modify: `config/config.go` (format fallback dans `matchBranch`)

**Context — breaking changes sur les tests existants :**

Les tests suivants font référence à des clés qui changent de namespace ou de valeur :
- `TestVars_semverNamespace` : `semver.LastTag`, `semver.CommitCount`, `semver.ShortHash` → déplacés dans `git`
- `TestVars_gitNamespace` : ajouter assertions pour `Hash`, `ShortHash`, `CommitCount`, `LastTag`
- `TestVars_semverMajorMinorPatch` : `semver.Major` etc. reflètent maintenant la version CC (bumped)
- `TestVars_semverPreRelease` : `semver.PreRelease` est vide pour versions CC ; `semver.LastPreRelease` = `"rc.1"`
- `TestVars_semverNoTag` : `semver.Major` = composants de `semver.Semver` (= `semver.LastVersion` quand pas de bump possible)
- `TestVars_regexNamespace` : format dans le config de test référence encore `semver.LastTag`
- `TestCurrentReturnsLastTagOnReleaseBranchUntagged` : retourne maintenant la version bumped
- `TestCurrentReturnsFormattedVersionOnPreReleaseBranch` : template utilise nouveaux keys + base bumped

**Acceptance Criteria:**
- [ ] `git` namespace : `Branch`, `Date`, `LastTag` (raw tag ou `""`), `Hash`, `ShortHash`, `CommitCount`
- [ ] `semver` namespace : `Semver`, `Major`, `Minor`, `Patch`, `PreRelease` (CC-calculé) + `Last*` + flags booléens
- [ ] `Current()` branche release : retourne `prefix + semver.Semver`
- [ ] `Current()` branche non-release : template avec nouveaux keys
- [ ] Tous les tests passent : `/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./...` → PASS

**Verify:** `/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./...` → PASS

**Steps:**

- [ ] **Step 1 : Mettre à jour `strategy/semver/semver.go` — réécrire `Vars()` et `Current()`**

**Imports** — ajouter rien (tout est déjà importé). Vérifier que `strconv`, `strings`, `time`, `gosemver`, `object` sont présents.

**Réécrire `Vars()`** (remplacer la fonction entière) :

```go
// Vars returns all template variables as a nested map, grouped by namespace:
//   - "semver": Semver, Major, Minor, Patch, PreRelease (CC-calculated),
//               LastVersion, LastMajor, LastMinor, LastPatch, LastPreRelease,
//               IsBreakingChange, IsPreRelease, HasNonConventionalCommits
//   - "git":    Branch, Date, LastTag, Hash, ShortHash, CommitCount
//   - "regex":  named captures from the matching branch pattern
//   - "var":    key=value pairs from extra
func (s Strategy) Vars(p GitProject, extra map[string]string) (map[string]interface{}, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return nil, fmt.Errorf("getting branch name: %w", err)
	}

	branchCfg, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
	f := NewSemverFormat(s.cfg.TagPrefix, constraints)

	lastTag, err := p.LastTag(f)
	if err != nil {
		return nil, err
	}

	effectiveLastTag := lastTag
	if lastTag == "0.0.0" {
		effectiveLastTag = s.cfg.Initial
	}

	// Fetch commits since last tag (for CommitCount + CC analysis)
	var commitsSinceTag []*object.Commit
	commitCount := 0
	if lastTag != "0.0.0" {
		tagged, err := p.IsHeadTagged(lastTag)
		if err != nil {
			return nil, err
		}
		if !tagged {
			all, err := p.CommitSinceTag(lastTag)
			if err != nil {
				return nil, err
			}
			// all includes the tagged commit at index len-1; exclude it
			if len(all) > 1 {
				commitsSinceTag = all[:len(all)-1]
			}
			commitCount = len(commitsSinceTag)
		}
	}

	// Conventional Commits bump analysis
	bumpLevel := BumpNone
	hasNonCC := false
	if len(commitsSinceTag) > 0 {
		bumpLevel, hasNonCC = AnalyzeBump(commitsSinceTag, s.cfg.ConventionalCommits)
	}

	// CC-calculated version (without prefix)
	semverStr := BumpVersion(effectiveLastTag, s.cfg.TagPrefix, bumpLevel)

	// Parse semver components for Semver (CC-calculated)
	var nextMajor, nextMinor, nextPatch, nextPreRelease string
	if sv, err := gosemver.NewVersion(semverStr); err == nil {
		nextMajor = strconv.FormatInt(sv.Major, 10)
		nextMinor = strconv.FormatInt(sv.Minor, 10)
		nextPatch = strconv.FormatInt(sv.Patch, 10)
		nextPreRelease = string(sv.PreRelease)
	}

	// Parse semver components for LastVersion (last tag stripped of prefix)
	lastVersionStr := strings.TrimPrefix(effectiveLastTag, s.cfg.TagPrefix)
	var lastMajor, lastMinor, lastPatch, lastPreRelease string
	if sv, err := gosemver.NewVersion(lastVersionStr); err == nil {
		lastMajor = strconv.FormatInt(sv.Major, 10)
		lastMinor = strconv.FormatInt(sv.Minor, 10)
		lastPatch = strconv.FormatInt(sv.Patch, 10)
		lastPreRelease = string(sv.PreRelease)
	}

	// Git metadata
	commitHashFull, err := p.CommitHash()
	if err != nil {
		return nil, err
	}
	shortHash := commitHashFull
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	shortBranch := strings.TrimPrefix(branchName, "refs/heads/")

	// Raw git tag (empty string when no tag found)
	rawLastTag := lastTag
	if lastTag == "0.0.0" {
		rawLastTag = ""
	}

	regexVars := map[string]interface{}{}
	for k, v := range captures {
		regexVars[k] = v
	}
	varVars := map[string]interface{}{}
	for k, v := range extra {
		varVars[k] = v
	}

	return map[string]interface{}{
		"semver": map[string]interface{}{
			"Semver":                   semverStr,
			"Major":                    nextMajor,
			"Minor":                    nextMinor,
			"Patch":                    nextPatch,
			"PreRelease":               nextPreRelease,
			"LastVersion":              lastVersionStr,
			"LastMajor":                lastMajor,
			"LastMinor":                lastMinor,
			"LastPatch":                lastPatch,
			"LastPreRelease":           lastPreRelease,
			"IsBreakingChange":         bumpLevel == BumpMajor,
			"IsPreRelease":             !branchCfg.Release,
			"HasNonConventionalCommits": hasNonCC,
		},
		"git": map[string]interface{}{
			"Branch":      shortBranch,
			"Date":        time.Now().UTC().Format("2006-01-02"),
			"LastTag":     rawLastTag,
			"Hash":        commitHashFull,
			"ShortHash":   shortHash,
			"CommitCount": commitCount,
		},
		"regex": regexVars,
		"var":   varVars,
	}, nil
}
```

**Réécrire `Current()`** (remplacer la fonction entière) :

```go
// Current returns the version at HEAD:
//   - Exact tag if HEAD is a tagged commit
//   - cfg.Initial if no tag exists at all
//   - CC-calculated version (with prefix) on a release branch with untagged HEAD
//   - Rendered format template on a pre-release branch
func (s Strategy) Current(p GitProject, extra map[string]string) (string, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return "", fmt.Errorf("getting branch name: %w", err)
	}

	branchCfg, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
	f := NewSemverFormat(s.cfg.TagPrefix, constraints)

	lastTag, err := p.LastTag(f)
	if err != nil {
		return "", err
	}

	if lastTag == "0.0.0" {
		return s.cfg.Initial, nil
	}

	tagged, err := p.IsHeadTagged(lastTag)
	if err != nil {
		return "", err
	}
	if tagged {
		return lastTag, nil
	}

	_ = captures // used above via matchBranch

	vars, err := s.Vars(p, extra)
	if err != nil {
		return "", err
	}

	semverMap := vars["semver"].(map[string]interface{})

	if branchCfg.Release {
		// Return CC-calculated version with prefix (consistent with tag format)
		return s.cfg.TagPrefix + semverMap["Semver"].(string), nil
	}

	return renderTemplate(branchCfg.Format, vars)
}
```

**Mettre à jour le format fallback dans `matchBranch()`** — remplacer :

```go
return config.BranchConfig{
    Release: false,
    Format:  "{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}",
}, map[string]string{}
```

par :

```go
return config.BranchConfig{
    Release: false,
    Format:  "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}",
}, map[string]string{}
```

- [ ] **Step 2 : Vérifier que le build passe (avant les tests)**

```bash
/usr/local/go/bin/go build ./...
```

Résultat attendu : succès. Si des erreurs de compilation apparaissent, les corriger avant de continuer.

- [ ] **Step 3 : Mettre à jour `strategy/semver/semver_test.go` — corriger les tests impactés**

**3a — `TestVars_semverNamespace`** : les clés `LastTag`, `CommitCount`, `ShortHash` sont maintenant dans `git`. Remplacer le corps du test :

```go
func TestVars_semverNamespace(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	createCommit(t, repo)
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	semverVars, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	// With 2 non-CC commits after tag 1.2.3 → patch bump → Semver = "1.2.4"
	if semverVars["Semver"] != "1.2.4" {
		t.Errorf("expected Semver 1.2.4, got %v", semverVars["Semver"])
	}
	if semverVars["LastVersion"] != "1.2.3" {
		t.Errorf("expected LastVersion 1.2.3, got %v", semverVars["LastVersion"])
	}
}
```

**3b — `TestVars_gitNamespace`** : ajouter assertions pour les nouveaux champs :

```go
func TestVars_gitNamespace(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/feature/foo")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	gitVars, ok := vars["git"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"git\"] to be a map")
	}
	if gitVars["Branch"] != "feature/foo" {
		t.Errorf("expected Branch feature/foo, got %v", gitVars["Branch"])
	}
	// Hash is a 40-char string
	hash, ok := gitVars["Hash"].(string)
	if !ok || len(hash) != 40 {
		t.Errorf("expected Hash to be a 40-char string, got %v", gitVars["Hash"])
	}
	// ShortHash is 7 chars
	shortHash, ok := gitVars["ShortHash"].(string)
	if !ok || len(shortHash) != 7 {
		t.Errorf("expected ShortHash to be a 7-char string, got %v", gitVars["ShortHash"])
	}
	// CommitCount is 0 (no tag, no commits since tag)
	if gitVars["CommitCount"] != 0 {
		t.Errorf("expected CommitCount 0, got %v", gitVars["CommitCount"])
	}
	// LastTag is empty string (no tag)
	if gitVars["LastTag"] != "" {
		t.Errorf("expected LastTag empty (no tag), got %v", gitVars["LastTag"])
	}
}
```

**3c — `TestVars_semverMajorMinorPatch`** : `Major/Minor/Patch` reflètent maintenant la version CC. Tag `"1.2.3"` + 1 commit non-CC → patch bump → `Semver = "1.2.4"` :

```go
func TestVars_semverMajorMinorPatch(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	createCommit(t, repo) // non-CC → patch default
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	// CC-calculated version: 1.2.3 + patch default → 1.2.4
	if sv["Major"] != "1" {
		t.Errorf("expected Major=1, got %v", sv["Major"])
	}
	if sv["Minor"] != "2" {
		t.Errorf("expected Minor=2, got %v", sv["Minor"])
	}
	if sv["Patch"] != "4" {
		t.Errorf("expected Patch=4 (bumped from 3), got %v", sv["Patch"])
	}
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty, got %v", sv["PreRelease"])
	}
	// Last tag components
	if sv["LastVersion"] != "1.2.3" {
		t.Errorf("expected LastVersion=1.2.3, got %v", sv["LastVersion"])
	}
	if sv["LastMajor"] != "1" {
		t.Errorf("expected LastMajor=1, got %v", sv["LastMajor"])
	}
	if sv["LastMinor"] != "2" {
		t.Errorf("expected LastMinor=2, got %v", sv["LastMinor"])
	}
	if sv["LastPatch"] != "3" {
		t.Errorf("expected LastPatch=3, got %v", sv["LastPatch"])
	}
}
```

**3d — `TestVars_semverPreRelease`** : tag `"1.2.3-rc.1"` + 1 non-CC commit → `Semver = "1.2.4"`, `LastPreRelease = "rc.1"` :

```go
func TestVars_semverPreRelease(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3-rc.1")
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	// CC version clears pre-release
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty for CC version, got %v", sv["PreRelease"])
	}
	// Last tag preserves pre-release
	if sv["LastPreRelease"] != "rc.1" {
		t.Errorf("expected LastPreRelease=rc.1, got %v", sv["LastPreRelease"])
	}
}
```

**3e — `TestVars_semverNoTag`** : pas de tag → pas d'analyse CC possible → `Semver == LastVersion == cfg.Initial` :

```go
func TestVars_semverNoTag(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	// No tag → no CC analysis → Semver = cfg.Initial = "0.1.0"
	if sv["Semver"] != "0.1.0" {
		t.Errorf("expected Semver=0.1.0 (cfg.Initial), got %v", sv["Semver"])
	}
	if sv["Major"] != "0" {
		t.Errorf("expected Major=0, got %v", sv["Major"])
	}
	if sv["Minor"] != "1" {
		t.Errorf("expected Minor=1, got %v", sv["Minor"])
	}
	if sv["Patch"] != "0" {
		t.Errorf("expected Patch=0, got %v", sv["Patch"])
	}
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty, got %v", sv["PreRelease"])
	}
}
```

**3f — `TestVars_semverParseFailure`** : vérifier que les composants CC sont `""` quand le parse échoue :

```go
func TestVars_semverParseFailure(t *testing.T) {
	repo := newRepo(t)
	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "not-a-version",
		Branches: []config.BranchConfig{
			{Pattern: ".*", Release: false, Format: "{{ .semver.Semver }}-dev.{{ .git.CommitCount }}"},
		},
	}
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(cfg)

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	sv, ok := vars["semver"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"semver\"] to be a map")
	}
	if sv["Major"] != "" {
		t.Errorf("expected Major empty on parse failure, got %v", sv["Major"])
	}
	if sv["Minor"] != "" {
		t.Errorf("expected Minor empty on parse failure, got %v", sv["Minor"])
	}
	if sv["Patch"] != "" {
		t.Errorf("expected Patch empty on parse failure, got %v", sv["Patch"])
	}
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty on parse failure, got %v", sv["PreRelease"])
	}
}
```

**3g — `TestVars_regexNamespace`** : mettre à jour le format dans la config du test :

```go
func TestVars_regexNamespace(t *testing.T) {
	repo := newRepo(t)
	cfg := config.SemverConfig{
		TagPrefix: "",
		Initial:   "0.1.0",
		Branches: []config.BranchConfig{
			{Pattern: `^refs/heads/release/(?P<major>\d+)\.x$`, Release: true},
			{Pattern: ".*", Release: false, Format: "{{ .semver.Semver }}-dev.{{ .git.CommitCount }}"},
		},
	}
	// ... reste identique
```

**3h — `TestCurrentReturnsLastTagOnReleaseBranchUntagged`** : retourne maintenant la version bumped (1 non-CC commit → patch → `"1.0.1"`) :

```go
func TestCurrentReturnsLastTagOnReleaseBranchUntagged(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo) // non-CC → patch default → 1.0.1
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.0.1" {
		t.Fatalf("expected 1.0.1 (CC patch bump), got %s", got)
	}
}
```

**3i — `TestCurrentReturnsFormattedVersionOnPreReleaseBranch`** : 2 non-CC commits après `1.0.0` → `Semver = "1.0.1"`, template → `"1.0.1-feature/my-feat.2"` :

```go
func TestCurrentReturnsFormattedVersionOnPreReleaseBranch(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.0.0")
	createCommit(t, repo)
	createCommit(t, repo)
	p := newFakeProject(t, repo, "refs/heads/feature/my-feat")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 2 non-CC commits → patch bump → Semver=1.0.1
	// template: {{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}
	expected := "1.0.1-feature/my-feat.2"
	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}
```

- [ ] **Step 4 : Ajouter les nouveaux tests d'intégration CC dans `semver_test.go`**

Ajouter à la fin du fichier :

```go
// ── Conventional Commits integration tests ────────────────────────────────────

func TestCurrent_releaseWithFeat(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	// Create a feat commit
	wt, _ := repo.Worktree()
	f, _ := wt.Filesystem.Create("feat.txt")
	_, _ = f.Write([]byte("feature"))
	_, _ = wt.Add("feat.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	_, _ = wt.Commit("feat: add new feature", &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.3.0" {
		t.Fatalf("expected 1.3.0 (minor bump from feat:), got %s", got)
	}
}

func TestCurrent_releaseWithBreaking(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	wt, _ := repo.Worktree()
	f, _ := wt.Filesystem.Create("break.txt")
	_, _ = f.Write([]byte("breaking"))
	_, _ = wt.Add("break.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	_, _ = wt.Commit("feat!: remove old API", &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2.0.0" {
		t.Fatalf("expected 2.0.0 (major bump from feat!:), got %s", got)
	}
}

func TestCurrent_releaseAllNone(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
	wt, _ := repo.Worktree()
	f, _ := wt.Filesystem.Create("chore.txt")
	_, _ = f.Write([]byte("chore"))
	_, _ = wt.Add("chore.txt")
	author := object.Signature{Name: "test", Email: "t@t.local", When: time.Now()}
	_, _ = wt.Commit("chore: update deps", &gogit.CommitOptions{
		All: true, Author: &author, Committer: &author, AllowEmptyCommits: true,
	})
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	got, err := s.Current(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2.3" {
		t.Fatalf("expected 1.2.3 (no bump — all chore commits), got %s", got)
	}
}
```

- [ ] **Step 5 : Vérifier que tous les tests passent**

```bash
/usr/local/go/bin/go test ./strategy/semver/ -v
```

Résultat attendu : tous PASS. Si des tests échouent, corriger les assertions avant de continuer.

- [ ] **Step 6 : Vérifier le build et tous les tests du projet**

```bash
/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./...
```

Résultat attendu : build succès, tous les tests PASS.

```json:metadata
{"files": ["strategy/semver/semver.go", "strategy/semver/semver_test.go", "config/config.go"], "verifyCommand": "/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./...", "acceptanceCriteria": ["git namespace réorganisé avec LastTag/Hash/ShortHash/CommitCount", "semver namespace réorganisé avec CC Semver + Last* + flags", "Current() release retourne version bumped avec préfixe", "tous tests existants mis à jour et passants", "3 nouveaux tests Current_release* passent"]}
```

# Semver Vars Enrichment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enrichir `Strategy.Vars()` avec les composants semver (`Major`, `Minor`, `Patch`, `PreRelease`) parsés depuis le dernier tag, et la date du jour (`git.Date`).

**Architecture:** Modification unique dans `strategy/semver/semver.go` — ajout du parsing semver et de `time.Now()` dans `Vars()`. `gosemver` et `strconv` sont déjà importés ; seul `time` s'ajoute. 4 nouveaux tests dans `semver_test.go`.

**Tech Stack:** Go, `github.com/coreos/go-semver/semver` (déjà présent), `time` (stdlib)

---

### Task 1: Enrichir Vars() avec composants semver et git.Date

**Goal:** Ajouter `semver.Major/Minor/Patch/PreRelease` (strings) et `git.Date` (YYYY-MM-DD) dans la map retournée par `Vars()`.

**Files:**
- Modify: `strategy/semver/semver.go`
- Modify: `strategy/semver/semver_test.go`

**Acceptance Criteria:**
- [ ] `vars["semver"]["Major"]` = `"1"` pour le tag `"1.2.3"`
- [ ] `vars["semver"]["Minor"]` = `"2"` pour le tag `"1.2.3"`
- [ ] `vars["semver"]["Patch"]` = `"3"` pour le tag `"1.2.3"`
- [ ] `vars["semver"]["PreRelease"]` = `""` pour `"1.2.3"`, `"rc.1"` pour `"1.2.3-rc.1"`
- [ ] Pas de tag → composants parsés depuis `cfg.Initial` (`"0.1.0"` → `Major="0"`, `Minor="1"`, `Patch="0"`)
- [ ] Parse échoue → composants restent `""`
- [ ] `vars["git"]["Date"]` est une string au format `YYYY-MM-DD`
- [ ] `/usr/local/go/bin/go test ./strategy/semver/ -v` → tous PASS

**Verify:** `/usr/local/go/bin/go test ./strategy/semver/ -v` → tous PASS

**Steps:**

- [ ] **Step 1 : Écrire les tests qui échouent dans `strategy/semver/semver_test.go`**

Ajouter à la fin du fichier (après `TestVars_varNamespace`) :

```go
func TestVars_semverMajorMinorPatch(t *testing.T) {
	repo := newRepo(t)
	createTag(t, repo, "1.2.3")
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
	if sv["Major"] != "1" {
		t.Errorf("expected Major=1, got %v", sv["Major"])
	}
	if sv["Minor"] != "2" {
		t.Errorf("expected Minor=2, got %v", sv["Minor"])
	}
	if sv["Patch"] != "3" {
		t.Errorf("expected Patch=3, got %v", sv["Patch"])
	}
	if sv["PreRelease"] != "" {
		t.Errorf("expected PreRelease empty, got %v", sv["PreRelease"])
	}
}

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
	if sv["PreRelease"] != "rc.1" {
		t.Errorf("expected PreRelease=rc.1, got %v", sv["PreRelease"])
	}
}

func TestVars_semverNoTag(t *testing.T) {
	repo := newRepo(t)
	// Pas de tag → fallback sur cfg.Initial = "0.1.0"
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
	if sv["Major"] != "0" {
		t.Errorf("expected Major=0 (from cfg.Initial 0.1.0), got %v", sv["Major"])
	}
	if sv["Minor"] != "1" {
		t.Errorf("expected Minor=1 (from cfg.Initial 0.1.0), got %v", sv["Minor"])
	}
	if sv["Patch"] != "0" {
		t.Errorf("expected Patch=0 (from cfg.Initial 0.1.0), got %v", sv["Patch"])
	}
}

func TestVars_gitDate(t *testing.T) {
	repo := newRepo(t)
	p := newFakeProject(t, repo, "refs/heads/main")
	s := semverstrategy.NewStrategy(mainConfig())

	vars, err := s.Vars(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	gitVars, ok := vars["git"].(map[string]interface{})
	if !ok {
		t.Fatal("expected vars[\"git\"] to be a map")
	}
	date, ok := gitVars["Date"].(string)
	if !ok {
		t.Fatalf("expected git.Date to be a string, got %T", gitVars["Date"])
	}
	// Vérifie le format YYYY-MM-DD
	if len(date) != 10 || date[4] != '-' || date[7] != '-' {
		t.Errorf("expected git.Date in YYYY-MM-DD format, got %q", date)
	}
}
```

- [ ] **Step 2 : Vérifier que les tests échouent**

```bash
/usr/local/go/bin/go test ./strategy/semver/ -v -run "TestVars_semverMajorMinorPatch|TestVars_semverPreRelease|TestVars_semverNoTag|TestVars_gitDate"
```

Résultat attendu : FAIL — les clés `Major`, `Minor`, `Patch`, `PreRelease`, `Date` n'existent pas encore.

- [ ] **Step 3 : Implémenter dans `strategy/semver/semver.go`**

**3a — Ajouter `"time"` aux imports** (les autres imports sont déjà présents) :

```go
import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	gosemver "github.com/coreos/go-semver/semver"
	"github.com/go-git/go-git/v5/plumbing/object"

	"gover/config"
	"gover/format"
)
```

**3b — Remplacer le bloc `return` dans `Vars()`** (actuellement lignes 186–198) par :

```go
	// Parse semver components from lastTag (or cfg.Initial if no tag found)
	versionToParse := strings.TrimPrefix(lastTag, s.cfg.TagPrefix)
	if lastTag == "0.0.0" {
		versionToParse = s.cfg.Initial
	}
	major, minor, patch, preRelease := "", "", "", ""
	if sv, err := gosemver.NewVersion(versionToParse); err == nil {
		major = strconv.FormatInt(sv.Major, 10)
		minor = strconv.FormatInt(sv.Minor, 10)
		patch = strconv.FormatInt(sv.Patch, 10)
		preRelease = string(sv.PreRelease)
	}

	return map[string]interface{}{
		"semver": map[string]interface{}{
			"LastTag":     effectiveLastTag,
			"CommitCount": commitCount,
			"ShortHash":   shortHash,
			"Major":       major,
			"Minor":       minor,
			"Patch":       patch,
			"PreRelease":  preRelease,
		},
		"git": map[string]interface{}{
			"Branch": shortBranch,
			"Date":   time.Now().UTC().Format("2006-01-02"),
		},
		"regex": regexVars,
		"var":   varVars,
	}, nil
```

- [ ] **Step 4 : Vérifier que tous les tests passent**

```bash
/usr/local/go/bin/go test ./strategy/semver/ -v
```

Résultat attendu : tous les tests PASS (existants + 4 nouveaux).

- [ ] **Step 5 : Vérifier le build complet et tous les tests**

```bash
/usr/local/go/bin/go build ./... && /usr/local/go/bin/go test ./...
```

Résultat attendu : build succès, tous les tests PASS.

```json:metadata
{"files": ["strategy/semver/semver.go", "strategy/semver/semver_test.go"], "verifyCommand": "/usr/local/go/bin/go test ./strategy/semver/ -v", "acceptanceCriteria": ["semver.Major/Minor/Patch/PreRelease strings dans Vars()", "git.Date YYYY-MM-DD dans Vars()", "fallback sur cfg.Initial si lastTag==0.0.0", "parse fail → champs vides", "4 nouveaux tests passent"]}
```

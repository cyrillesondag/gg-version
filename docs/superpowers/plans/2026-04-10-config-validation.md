# Config Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ajouter une passe de validation exhaustive au chargement de la config, qui collecte toutes les violations et les retourne en une seule erreur multi-lignes.

**Architecture:** `config/validate.go` expose `Validate(cfg Config) error` (collect-all). `Load()` dans `config/config.go` appelle `Validate()` après unmarshal et retourne l'erreur si elle est non-nil. Aucune nouvelle dépendance externe — `go-semver`, `doublestar`, `text/template` et `regexp` sont déjà dans le module.

**Tech Stack:** Go 1.24, `github.com/coreos/go-semver/semver`, `github.com/bmatcuk/doublestar/v4 v4.10.0`, `text/template`, `regexp`.

---

### Task 1: `Validate` — logique et tests

**Goal:** Créer `config/validate.go` avec `Validate(cfg Config) error` et `config/validate_test.go` couvrant tous les champs validés.

**Files:**
- Create: `config/validate.go`
- Create: `config/validate_test.go`

**Acceptance Criteria:**
- [ ] `TestValidate_default` PASS
- [ ] `TestValidate_invalidInitial` PASS
- [ ] `TestValidate_invalidBranchPattern` PASS
- [ ] `TestValidate_invalidBranchFormat` PASS
- [ ] `TestValidate_branchFormatIgnoredWhenRelease` PASS
- [ ] `TestValidate_invalidCCFormat` PASS
- [ ] `TestValidate_invalidCCPatterns` PASS
- [ ] `TestValidate_invalidIgnorePath` PASS
- [ ] `TestValidate_componentEmptyPath` PASS
- [ ] `TestValidate_componentInvalidPath` PASS
- [ ] `TestValidate_componentInvalidTagScope` PASS
- [ ] `TestValidate_multipleViolations` PASS
- [ ] `go test ./config/` PASS

**Verify:** `go test ./config/ -v -run TestValidate` → tous PASS

**Steps:**

- [ ] **Step 1 : Écrire les tests dans `config/validate_test.go`**

```go
package config_test

import (
	"strings"
	"testing"

	"gover/config"
)

func TestValidate_default(t *testing.T) {
	if err := config.Validate(config.DefaultConfig()); err != nil {
		t.Errorf("DefaultConfig() should pass validation, got: %v", err)
	}
}

func TestValidate_invalidInitial(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.Initial = "not-a-semver"
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid initial, got nil")
	}
	if !strings.Contains(err.Error(), "semver.initial") {
		t.Errorf("expected mention of semver.initial, got: %v", err)
	}
}

func TestValidate_invalidBranchPattern(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.Branches = []config.BranchConfig{
		{Pattern: "(?invalid", Release: false, Format: "{{ .semver.Semver }}"},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid branch pattern, got nil")
	}
	if !strings.Contains(err.Error(), "semver.branches[0].pattern") {
		t.Errorf("expected mention of semver.branches[0].pattern, got: %v", err)
	}
}

func TestValidate_invalidBranchFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.Branches = []config.BranchConfig{
		{Pattern: "main", Release: false, Format: "{{ .Unclosed"},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid branch format, got nil")
	}
	if !strings.Contains(err.Error(), "semver.branches[0].format") {
		t.Errorf("expected mention of semver.branches[0].format, got: %v", err)
	}
}

func TestValidate_branchFormatIgnoredWhenRelease(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.Branches = []config.BranchConfig{
		{Pattern: "main", Release: true, Format: "{{ .Unclosed"},
	}
	if err := config.Validate(cfg); err != nil {
		t.Errorf("invalid format on release branch should not be validated, got: %v", err)
	}
}

func TestValidate_invalidCCFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.ConventionalCommits.Format = "(?bad"
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid CC format, got nil")
	}
	if !strings.Contains(err.Error(), "semver.conventional_commits.format") {
		t.Errorf("expected mention of semver.conventional_commits.format, got: %v", err)
	}
}

func TestValidate_invalidCCPatterns(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.ConventionalCommits.Major = []string{"^valid", "(?invalid"}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid CC major pattern, got nil")
	}
	if !strings.Contains(err.Error(), "semver.conventional_commits.major") {
		t.Errorf("expected mention of semver.conventional_commits.major, got: %v", err)
	}
}

func TestValidate_invalidIgnorePath(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.IgnorePaths = []string{"[invalid"}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid ignore_path glob, got nil")
	}
	if !strings.Contains(err.Error(), "semver.ignore_paths[0]") {
		t.Errorf("expected mention of semver.ignore_paths[0], got: %v", err)
	}
}

func TestValidate_componentEmptyPath(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Components = map[string]config.ComponentConfig{
		"api": {Path: ""},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty component path, got nil")
	}
	if !strings.Contains(err.Error(), "components.api.path") {
		t.Errorf("expected mention of components.api.path, got: %v", err)
	}
}

func TestValidate_componentInvalidPath(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Components = map[string]config.ComponentConfig{
		"api": {Path: "[invalid"},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid component path glob, got nil")
	}
	if !strings.Contains(err.Error(), "components.api.path") {
		t.Errorf("expected mention of components.api.path, got: %v", err)
	}
}

func TestValidate_componentInvalidTagScope(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Components = map[string]config.ComponentConfig{
		"api": {Path: "api/**", TagScope: "bad name"},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid tag_scope, got nil")
	}
	if !strings.Contains(err.Error(), "components.api.tag_scope") {
		t.Errorf("expected mention of components.api.tag_scope, got: %v", err)
	}
}

func TestValidate_multipleViolations(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.Initial = "bad"
	cfg.Semver.ConventionalCommits.Format = "(?invalid"
	cfg.Components = map[string]config.ComponentConfig{
		"api": {Path: ""},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for multiple violations, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "semver.initial") {
		t.Errorf("expected mention of semver.initial, got: %v", msg)
	}
	if !strings.Contains(msg, "semver.conventional_commits.format") {
		t.Errorf("expected mention of semver.conventional_commits.format, got: %v", msg)
	}
	if !strings.Contains(msg, "components.api.path") {
		t.Errorf("expected mention of components.api.path, got: %v", msg)
	}
}
```

- [ ] **Step 2 : Vérifier que les tests échouent (Validate non définie)**

```bash
go test ./config/ -v -run TestValidate
```

Résultat attendu : erreur de compilation `undefined: config.Validate`.

- [ ] **Step 3 : Créer `config/validate.go`**

```go
package config

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/bmatcuk/doublestar/v4"
	gosemver "github.com/coreos/go-semver/semver"
)

// Validate checks all fields of cfg for correctness. It collects every
// violation and returns them as a single multi-line error, or nil if valid.
func Validate(cfg Config) error {
	var violations []string
	add := func(msg string) { violations = append(violations, "  "+msg) }

	// semver.initial
	if cfg.Semver.Initial != "" {
		if _, err := gosemver.NewVersion(cfg.Semver.Initial); err != nil {
			add(fmt.Sprintf("semver.initial %q: invalid semver: %v", cfg.Semver.Initial, err))
		}
	}

	// semver.branches
	for i, b := range cfg.Semver.Branches {
		if _, err := regexp.Compile(b.Pattern); err != nil {
			add(fmt.Sprintf("semver.branches[%d].pattern %q: %v", i, b.Pattern, err))
		}
		if !b.Release && b.Format != "" {
			if _, err := template.New("").Parse(b.Format); err != nil {
				add(fmt.Sprintf("semver.branches[%d].format %q: %v", i, b.Format, err))
			}
		}
	}

	// semver.conventional_commits.format
	if cfg.Semver.ConventionalCommits.Format != "" {
		if _, err := regexp.Compile(cfg.Semver.ConventionalCommits.Format); err != nil {
			add(fmt.Sprintf("semver.conventional_commits.format %q: %v", cfg.Semver.ConventionalCommits.Format, err))
		}
	}

	// semver.conventional_commits.major / minor / patch
	for level, patterns := range map[string][]string{
		"major": cfg.Semver.ConventionalCommits.Major,
		"minor": cfg.Semver.ConventionalCommits.Minor,
		"patch": cfg.Semver.ConventionalCommits.Patch,
	} {
		for j, p := range patterns {
			if _, err := regexp.Compile(p); err != nil {
				add(fmt.Sprintf("semver.conventional_commits.%s[%d] %q: %v", level, j, p, err))
			}
		}
	}

	// semver.ignore_paths
	for j, p := range cfg.Semver.IgnorePaths {
		if !doublestar.ValidatePattern(p) {
			add(fmt.Sprintf("semver.ignore_paths[%d] %q: invalid glob pattern", j, p))
		}
	}

	// components
	for name, comp := range cfg.Components {
		if comp.Path == "" {
			add(fmt.Sprintf("components.%s.path: must not be empty", name))
		} else if !doublestar.ValidatePattern(comp.Path) {
			add(fmt.Sprintf("components.%s.path %q: invalid glob pattern", name, comp.Path))
		}
		if comp.TagScope != "" && !isValidGitRefComponent(comp.TagScope) {
			add(fmt.Sprintf("components.%s.tag_scope %q: invalid git ref name component", name, comp.TagScope))
		}
	}

	if len(violations) == 0 {
		return nil
	}
	return fmt.Errorf("config validation failed:\n%s", strings.Join(violations, "\n"))
}

// isValidGitRefComponent reports whether s is a valid git ref name component
// suitable for use as tag_scope (e.g. "api", "my-service").
// Implements the rules from git-check-ref-format(1) for a single path segment.
func isValidGitRefComponent(s string) bool {
	if s == "" {
		return false
	}
	// Must not start with '.' or '-'
	if s[0] == '.' || s[0] == '-' {
		return false
	}
	// Must not end with '.'
	if s[len(s)-1] == '.' {
		return false
	}
	// Must not end with '.lock'
	if strings.HasSuffix(s, ".lock") {
		return false
	}
	// Must not contain forbidden sequences
	for _, seq := range []string{"..", "@{"} {
		if strings.Contains(s, seq) {
			return false
		}
	}
	// Must not contain forbidden characters
	for _, c := range s {
		if c <= 0x1f || c == 0x7f {
			return false
		}
		switch c {
		case ' ', '~', '^', ':', '?', '*', '[', '\\':
			return false
		}
	}
	return true
}
```

- [ ] **Step 4 : Vérifier que les tests passent**

```bash
go test ./config/ -v -run TestValidate
```

Résultat attendu : tous les `TestValidate_*` PASS.

- [ ] **Step 5 : Vérifier que tous les tests passent**

```bash
go test ./...
```

Résultat attendu :
```
ok  	gover/config
ok  	gover/git
ok  	gover/strategy/semver
```

- [ ] **Step 6 : Commit**

```bash
git add config/validate.go config/validate_test.go
git commit -m "feat: add Validate to check config fields at load time"
```

---

### Task 2: Intégrer `Validate` dans `Load()` et test d'intégration

**Goal:** Modifier `Load()` pour appeler `Validate()` après unmarshal, et ajouter un test vérifiant qu'un fichier YAML invalide produit une erreur depuis `Load()`.

**Files:**
- Modify: `config/config.go`
- Modify: `config/validate_test.go` (ajouter `TestLoad_invalidConfig`)

**Acceptance Criteria:**
- [ ] `TestLoad_invalidConfig` PASS
- [ ] `go build ./cmd/gg-version` réussit
- [ ] `go test ./...` PASS

**Verify:** `go build ./cmd/gg-version && go test ./...` → PASS

**Steps:**

- [ ] **Step 1 : Ajouter le test d'intégration dans `config/validate_test.go`**

Ajouter à la fin du fichier :

```go
func TestLoad_invalidConfig(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("semver:\n  initial: not-a-semver\n")
	f.Close()

	_, err = config.Load(f.Name())
	if err == nil {
		t.Fatal("expected Load to return error for invalid config, got nil")
	}
	if !strings.Contains(err.Error(), "semver.initial") {
		t.Errorf("expected error to mention semver.initial, got: %v", err)
	}
}
```

Ajouter `"os"` aux imports du fichier de test :

```go
import (
	"os"
	"strings"
	"testing"

	"gover/config"
)
```

- [ ] **Step 2 : Vérifier que le test échoue**

```bash
go test ./config/ -v -run TestLoad_invalidConfig
```

Résultat attendu : FAIL — `Load` retourne nil pour l'instant.

- [ ] **Step 3 : Modifier `Load()` dans `config/config.go`**

Remplacer la dernière partie de `Load()` (après `yaml.Unmarshal`) :

```go
// Load reads the YAML config file at path and returns a Config.
// If the file does not exist, DefaultConfig is returned without error.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("reading config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("config file %s: %w", path, err)
	}
	return cfg, nil
}
```

- [ ] **Step 4 : Vérifier que les tests passent**

```bash
go test ./config/ -v -run TestLoad_invalidConfig
```

Résultat attendu : PASS.

- [ ] **Step 5 : Vérifier build et tous les tests**

```bash
go build ./cmd/gg-version && go test ./...
```

Résultat attendu : build OK, tous les tests PASS.

- [ ] **Step 6 : Commit**

```bash
git add config/config.go config/validate_test.go
git commit -m "feat: wire Validate into Load for early config error reporting"
```

package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/cyrillesondag/gg-version/config"
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
		{Pattern: "(?invalid", VersionFormat: "{{ .semver.Semver }}"},
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
		{Pattern: "main", VersionFormat: "{{ .Unclosed"},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid branch version_format, got nil")
	}
	if !strings.Contains(err.Error(), "semver.branches[0].version_format") {
		t.Errorf("expected mention of semver.branches[0].version_format, got: %v", err)
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
	if !strings.Contains(err.Error(), f.Name()) {
		t.Errorf("expected error to contain file path %s, got: %v", f.Name(), err)
	}
}

func TestValidate_invalidBranchConstraint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.Branches = []config.BranchConfig{
		{Pattern: "main", Constraint: "{{ .Unclosed"},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid branch constraint, got nil")
	}
	if !strings.Contains(err.Error(), "semver.branches[0].constraint") {
		t.Errorf("expected mention of semver.branches[0].constraint, got: %v", err)
	}
}

func TestValidate_invalidVar(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Semver.Vars = map[string]string{
		"stream": "{{ .Unclosed",
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid var template, got nil")
	}
	if !strings.Contains(err.Error(), "semver.vars[stream]") {
		t.Errorf("expected mention of semver.vars[stream], got: %v", err)
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

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cyrillesondag/gg-version/config"
)

func TestDefaultConfigWhenFileMissing(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/.gg-version.yaml")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Semver.Initial != "0.1.0" {
		t.Fatalf("expected initial 0.1.0, got %s", cfg.Semver.Initial)
	}
	if cfg.Semver.TagPrefix != "" {
		t.Fatalf("expected empty tag_prefix, got %s", cfg.Semver.TagPrefix)
	}
	if len(cfg.Semver.Branches) != 3 {
		t.Fatalf("expected 3 default branch patterns (main, master, .*), got %d", len(cfg.Semver.Branches))
	}
	if cfg.Semver.Branches[0].Pattern != "main" {
		t.Errorf("expected branches[0].Pattern=main, got %q", cfg.Semver.Branches[0].Pattern)
	}
	if cfg.Semver.Branches[0].VersionFormat != "" {
		t.Errorf("expected branches[0].VersionFormat empty (release), got %q", cfg.Semver.Branches[0].VersionFormat)
	}
	if cfg.Semver.Branches[1].Pattern != "master" {
		t.Errorf("expected branches[1].Pattern=master, got %q", cfg.Semver.Branches[1].Pattern)
	}
	if cfg.Semver.Branches[1].VersionFormat != "" {
		t.Errorf("expected branches[1].VersionFormat empty (release), got %q", cfg.Semver.Branches[1].VersionFormat)
	}
	if cfg.Semver.Branches[2].Pattern != ".*" {
		t.Errorf("expected branches[2].Pattern=.*, got %q", cfg.Semver.Branches[2].Pattern)
	}
	if cfg.Semver.Branches[2].VersionFormat == "" {
		t.Error("expected branches[2].VersionFormat to be set (pre-release template)")
	}
}

func TestLoadValidYAML(t *testing.T) {
	content := `
semver:
  tag_prefix: "v"
  initial: "1.0.0"
  branches:
    - pattern: "^refs/heads/main$"
    - pattern: ".*"
      version_format: "{{ .semver.Semver }}-dev.{{ .git.CommitCount }}"
`
	dir := t.TempDir()
	path := filepath.Join(dir, ".gg-version.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Semver.TagPrefix != "v" {
		t.Fatalf("expected tag_prefix v, got %s", cfg.Semver.TagPrefix)
	}
	if cfg.Semver.Initial != "1.0.0" {
		t.Fatalf("expected initial 1.0.0, got %s", cfg.Semver.Initial)
	}
	if len(cfg.Semver.Branches) != 2 {
		t.Fatalf("expected 2 branch configs, got %d", len(cfg.Semver.Branches))
	}
	if cfg.Semver.Branches[0].VersionFormat != "" {
		t.Errorf("expected first branch VersionFormat empty (release), got %q", cfg.Semver.Branches[0].VersionFormat)
	}
	if cfg.Semver.Branches[1].VersionFormat != "{{ .semver.Semver }}-dev.{{ .git.CommitCount }}" {
		t.Fatalf("unexpected VersionFormat: %s", cfg.Semver.Branches[1].VersionFormat)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gg-version.yaml")
	if err := os.WriteFile(path, []byte("semver: {unclosed: [bracket"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

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

func TestLoadComponents(t *testing.T) {
	content := `
semver:
  tag_prefix: "v"
  initial: "0.1.0"
components:
  api:
    path: "api/**"
  web:
    path: "web/**"
    tag_scope: "my-web"
`
	dir := t.TempDir()
	path := filepath.Join(dir, ".gg-version.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(cfg.Components))
	}
	api, ok := cfg.Components["api"]
	if !ok {
		t.Fatal("expected component 'api'")
	}
	if api.Path != "api/**" {
		t.Errorf("expected api.Path=api/**, got %q", api.Path)
	}
	if api.TagScope != "" {
		t.Errorf("expected api.TagScope empty (default), got %q", api.TagScope)
	}
	web := cfg.Components["web"]
	if web.TagScope != "my-web" {
		t.Errorf("expected web.TagScope=my-web, got %q", web.TagScope)
	}
}

func TestLoadIgnorePaths(t *testing.T) {
	content := `
semver:
  tag_prefix: "v"
  ignore_paths:
    - "*.md"
    - "docs/**"
  ignore_commits:
    - "abc1234"
    - "deadbeef"
`
	dir := t.TempDir()
	path := filepath.Join(dir, ".gg-version.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Semver.IgnorePaths) != 2 {
		t.Fatalf("expected 2 ignore_paths, got %d", len(cfg.Semver.IgnorePaths))
	}
	if cfg.Semver.IgnorePaths[0] != "*.md" {
		t.Errorf("expected *.md, got %q", cfg.Semver.IgnorePaths[0])
	}
	if len(cfg.Semver.IgnoreCommits) != 2 {
		t.Fatalf("expected 2 ignore_commits, got %d", len(cfg.Semver.IgnoreCommits))
	}
	if cfg.Semver.IgnoreCommits[0] != "abc1234" {
		t.Errorf("expected abc1234, got %q", cfg.Semver.IgnoreCommits[0])
	}
}

func TestLoadBranchConstraint(t *testing.T) {
	content := `
semver:
  tag_prefix: "v"
  branches:
    - pattern: "release/(?P<major>[0-9]+)\\.x"
      constraint: "{{ .regex.major }}.x.x"
    - pattern: ".*"
      version_format: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}"
`
	dir := t.TempDir()
	path := filepath.Join(dir, ".gg-version.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Semver.Branches[0].Constraint != "{{ .regex.major }}.x.x" {
		t.Errorf("expected constraint %q, got %q", "{{ .regex.major }}.x.x", cfg.Semver.Branches[0].Constraint)
	}
	if cfg.Semver.Branches[0].VersionFormat != "" {
		t.Errorf(
			"expected VersionFormat empty on constrained release branch, got %q",
			cfg.Semver.Branches[0].VersionFormat,
		)
	}
}

func TestLoadSemverVars(t *testing.T) {
	content := `
semver:
  tag_prefix: "v"
  vars:
    stream: "{{ .env.STREAM | default \"1\" }}"
    build: "{{ .env.CI_BUILD_NUMBER }}"
    env: "prod"
`
	dir := t.TempDir()
	path := filepath.Join(dir, ".gg-version.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Semver.Vars) != 3 {
		t.Fatalf("expected 3 vars, got %d: %v", len(cfg.Semver.Vars), cfg.Semver.Vars)
	}
	if cfg.Semver.Vars["stream"] != `{{ .env.STREAM | default "1" }}` {
		t.Errorf("stream: got %q", cfg.Semver.Vars["stream"])
	}
	if cfg.Semver.Vars["build"] != "{{ .env.CI_BUILD_NUMBER }}" {
		t.Errorf("build: got %q", cfg.Semver.Vars["build"])
	}
	if cfg.Semver.Vars["env"] != "prod" {
		t.Errorf("env: got %q", cfg.Semver.Vars["env"])
	}
}

func TestDefaultConfig_noComponents(t *testing.T) {
	cfg := config.DefaultConfig()
	if len(cfg.Components) != 0 {
		t.Errorf("expected no components in DefaultConfig, got %d", len(cfg.Components))
	}
	if len(cfg.Semver.IgnorePaths) != 0 {
		t.Errorf("expected empty IgnorePaths, got %v", cfg.Semver.IgnorePaths)
	}
	if len(cfg.Semver.IgnoreCommits) != 0 {
		t.Errorf("expected empty IgnoreCommits, got %v", cfg.Semver.IgnoreCommits)
	}
}

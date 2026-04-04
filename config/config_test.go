package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"gover/config"
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
	if len(cfg.Semver.Branches) == 0 {
		t.Fatal("expected at least one default branch pattern")
	}
}

func TestLoadValidYAML(t *testing.T) {
	content := `
semver:
  tag_prefix: "v"
  initial: "1.0.0"
  branches:
    - pattern: "^refs/heads/main$"
      release: true
    - pattern: ".*"
      release: false
      format: "{{ .semver.LastTag }}-dev.{{ .semver.CommitCount }}"
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
	if !cfg.Semver.Branches[0].Release {
		t.Fatal("expected first branch to be release")
	}
	if cfg.Semver.Branches[1].Release {
		t.Fatal("expected second branch to be pre-release")
	}
	if cfg.Semver.Branches[1].Format != "{{ .semver.LastTag }}-dev.{{ .semver.CommitCount }}" {
		t.Fatalf("unexpected format: %s", cfg.Semver.Branches[1].Format)
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

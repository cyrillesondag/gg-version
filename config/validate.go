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
		if b.VersionFormat != "" {
			if err := validateTemplate(b.VersionFormat); err != nil {
				add(fmt.Sprintf("semver.branches[%d].version_format %q: %v", i, b.VersionFormat, err))
			}
		}
		if b.Constraint != "" {
			if err := validateTemplate(b.Constraint); err != nil {
				add(fmt.Sprintf("semver.branches[%d].constraint %q: %v", i, b.Constraint, err))
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

	// semver.vars
	for key, val := range cfg.Semver.Vars {
		if err := validateTemplate(val); err != nil {
			add(fmt.Sprintf("semver.vars[%s] %q: %v", key, val, err))
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

// validateTemplate parses a Go template string and returns an error for
// structural syntax problems. Errors about undefined functions are ignored,
// because additional functions may be registered at render time.
func validateTemplate(tmpl string) error {
	_, err := template.New("").Parse(tmpl)
	if err != nil && !strings.Contains(err.Error(), "not defined") {
		return err
	}
	return nil
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

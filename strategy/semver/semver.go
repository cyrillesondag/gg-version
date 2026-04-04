package semver

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	gosemver "github.com/coreos/go-semver/semver"
	"github.com/go-git/go-git/v5/plumbing/object"

	"gover/config"
	"gover/format"
)

// SemverFormat validates semver tags, optionally requiring a prefix and
// enforcing version component constraints (major, minor, patch).
type SemverFormat struct {
	Prefix      string
	Constraints map[string]string // e.g. {"major": "1", "minor": "2"}
}

// NewSemverFormat creates a SemverFormat. Pass nil constraints for no restriction.
func NewSemverFormat(prefix string, constraints map[string]string) SemverFormat {
	if constraints == nil {
		constraints = map[string]string{}
	}
	return SemverFormat{Prefix: prefix, Constraints: constraints}
}

// IsValid returns true if version is a valid semver (with the configured prefix)
// and satisfies all component constraints.
func (s SemverFormat) IsValid(version string) bool {
	v, err := s.parse(version)
	if err != nil {
		return false
	}
	if maj, ok := s.Constraints["major"]; ok {
		if strconv.FormatInt(v.Major, 10) != maj {
			return false
		}
	}
	if min, ok := s.Constraints["minor"]; ok {
		if strconv.FormatInt(v.Minor, 10) != min {
			return false
		}
	}
	if patch, ok := s.Constraints["patch"]; ok {
		if strconv.FormatInt(v.Patch, 10) != patch {
			return false
		}
	}
	return true
}

// Compare returns 0 if equal, negative if version1 < version2, positive if version1 > version2.
func (s SemverFormat) Compare(version1, version2 string) (int, error) {
	v1, err := s.parse(version1)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: %w", version1, err)
	}
	v2, err := s.parse(version2)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: %w", version2, err)
	}
	return v1.Compare(*v2), nil
}

func (s SemverFormat) parse(version string) (*gosemver.Version, error) {
	trimmed := strings.TrimPrefix(version, s.Prefix)
	if s.Prefix != "" && trimmed == version {
		return nil, fmt.Errorf("version %q does not start with prefix %q", version, s.Prefix)
	}
	return gosemver.NewVersion(trimmed)
}

// compile-time check that SemverFormat implements format.VersionFormat
var _ format.VersionFormat = SemverFormat{}

// GitProject is the interface Strategy uses to interact with the git repository.
// *git.Project satisfies this interface.
type GitProject interface {
	LastTag(f format.VersionFormat) (string, error)
	IsHeadTagged(tag string) (bool, error)
	CommitSinceTag(tag string) ([]*object.Commit, error)
	BranchName() (string, error)
	CommitHash() (string, error)
}

// Strategy computes semver versions from the git history.
type Strategy struct {
	cfg config.SemverConfig
}

// NewStrategy returns a Strategy configured by cfg.
func NewStrategy(cfg config.SemverConfig) Strategy {
	return Strategy{cfg: cfg}
}

// Last returns the last valid semver tag reachable from HEAD, respecting any
// version constraints extracted from the branch name pattern. Returns cfg.Initial
// when no tag is found.
func (s Strategy) Last(p GitProject) (string, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return "", fmt.Errorf("getting branch name: %w", err)
	}

	_, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
	f := NewSemverFormat(s.cfg.TagPrefix, constraints)

	tag, err := p.LastTag(f)
	if err != nil {
		return "", err
	}
	if tag == "0.0.0" {
		return s.cfg.Initial, nil
	}
	return tag, nil
}

// Vars returns all template variables as a nested map, grouped by namespace:
//   - "semver": LastTag, CommitCount, ShortHash
//   - "git":    Branch (short name, without refs/heads/)
//   - "regex":  named captures from the matching branch pattern
//   - "var":    key=value pairs from extra
func (s Strategy) Vars(p GitProject, extra map[string]string) (map[string]interface{}, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return nil, fmt.Errorf("getting branch name: %w", err)
	}

	_, captures := s.matchBranch(branchName)
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

	commitCount := 0
	if lastTag != "0.0.0" {
		tagged, err := p.IsHeadTagged(lastTag)
		if err != nil {
			return nil, err
		}
		if !tagged {
			commits, err := p.CommitSinceTag(lastTag)
			if err != nil {
				return nil, err
			}
			commitCount = len(commits) - 1
		}
	}

	commitHashFull, err := p.CommitHash()
	if err != nil {
		return nil, err
	}
	shortHash := commitHashFull
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	shortBranch := strings.TrimPrefix(branchName, "refs/heads/")

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
			"LastTag":     effectiveLastTag,
			"CommitCount": commitCount,
			"ShortHash":   shortHash,
		},
		"git": map[string]interface{}{
			"Branch": shortBranch,
		},
		"regex": regexVars,
		"var":   varVars,
	}, nil
}

// Current returns the version at HEAD:
//   - Exact tag if HEAD is a tagged commit
//   - Last tag if on a release branch (untagged HEAD)
//   - Rendered format template if on a pre-release branch
//   - cfg.Initial if no tag exists at all
//
// extra is a map of key=value pairs injected into the "var" template namespace.
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

	if branchCfg.Release {
		return lastTag, nil
	}

	// Pre-release branch: build vars and render template
	vars, err := s.Vars(p, extra)
	if err != nil {
		return "", err
	}
	return renderTemplate(branchCfg.Format, vars)
}

// matchBranch finds the first BranchConfig whose Pattern matches branchName.
// Returns the config and any named capture groups extracted from the match.
// If no pattern matches, returns a default pre-release config.
func (s Strategy) matchBranch(branchName string) (config.BranchConfig, map[string]string) {
	for _, b := range s.cfg.Branches {
		re, err := regexp.Compile(b.Pattern)
		if err != nil {
			continue
		}
		match := re.FindStringSubmatch(branchName)
		if match == nil {
			continue
		}
		captures := map[string]string{}
		for i, name := range re.SubexpNames() {
			if name != "" && i < len(match) {
				captures[name] = match[i]
			}
		}
		return b, captures
	}
	return config.BranchConfig{
		Release: false,
		Format:  "{{ .semver.LastTag }}-{{ .git.Branch }}.{{ .semver.CommitCount }}",
	}, map[string]string{}
}

// versionConstraints extracts only major/minor/patch keys from named captures.
func versionConstraints(captures map[string]string) map[string]string {
	c := map[string]string{}
	for _, key := range []string{"major", "minor", "patch"} {
		if v, ok := captures[key]; ok {
			c[key] = v
		}
	}
	return c
}

// renderTemplate executes a Go text/template with the given variables.
// vars is a map[string]interface{} so named captures can be added dynamically.
func renderTemplate(tmpl string, vars map[string]interface{}) (string, error) {
	t, err := template.New("version").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing version template %q: %w", tmpl, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("executing version template: %w", err)
	}
	return buf.String(), nil
}

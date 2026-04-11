package semver

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	gosemver "github.com/coreos/go-semver/semver"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/cyrillesondag/gg-version/config"
	"github.com/cyrillesondag/gg-version/format"
	gitpkg "github.com/cyrillesondag/gg-version/git"
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
	CommitSinceTag(tag string) ([]*object.Commit, bool, error)
	BranchName() (string, error)
	CommitHash() (string, error)
	CommitFiles(c *object.Commit) ([]string, error)
	CommitDate() (time.Time, time.Time, error)
	IsShallow() bool
	CreateTag(name, message string) error
	PushTags() error
	CommitHistory() ([]gitpkg.CommitWithTags, error)
}

// ComponentLintResult holds the lint result for one entity (component or root).
type ComponentLintResult struct {
	Name       string       // "" = non-monorepo, "@root" = root, otherwise component name
	Violations []LintResult // commits whose subject does not match the CC format
	Truncated  bool         // true if the history is truncated (shallow clone)
}

// Strategy computes semver versions from the git history.
// Use NewStrategy to obtain an instance.
type Strategy interface {
	Current(p GitProject, extra map[string]string) (string, error)
	Last(p GitProject) (string, error)
	Vars(p GitProject, extra map[string]string) (map[string]interface{}, error)
	AllCurrent(p GitProject, extra map[string]string, cfg config.Config) ([]ComponentResult, error)
	AllLast(p GitProject, cfg config.Config) ([]ComponentResult, error)
	AllVars(p GitProject, extra map[string]string, cfg config.Config) ([]ComponentVarsResult, error)
	AllLint(p GitProject, cfg config.Config) ([]ComponentLintResult, error)
}

type semverStrategy struct {
	cfg config.SemverConfig
}

// compile-time check that semverStrategy implements Strategy
var _ Strategy = semverStrategy{}

// NewStrategy returns a Strategy configured by cfg.
func NewStrategy(cfg config.SemverConfig) Strategy {
	return semverStrategy{cfg: cfg}
}

// Last returns the last valid semver tag reachable from HEAD, respecting any
// version constraints extracted from the branch name pattern. Returns cfg.Initial
// when no tag is found.
func (s semverStrategy) Last(p GitProject) (string, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return "", fmt.Errorf("getting branch name: %w", err)
	}

	branchCfg, captures := s.matchBranch(branchName)
	constraints := resolveConstraint(branchCfg, captures, nil)
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
//   - "semver": Semver, Major, Minor, Patch, PreRelease (CC-calculated),
//               LastVersion, LastMajor, LastMinor, LastPatch, LastPreRelease,
//               IsBreakingChange, IsPreRelease, HasNonConventionalCommits
//   - "git":    Branch, AuthorDate, CommitterDate, LastTag, Hash, ShortHash, CommitCount, IsShallow, Truncated
//   - "regex":  named captures from the matching branch pattern
//   - "var":    key=value pairs from extra
func (s semverStrategy) Vars(p GitProject, extra map[string]string) (map[string]interface{}, error) {
	return s.varsCore(p, extra, s.cfg.TagPrefix, FilterConfig{
		ExcludePaths:  s.cfg.IgnorePaths,
		IgnoreCommits: s.cfg.IgnoreCommits,
	})
}

// varsCore is the parameterised implementation of Vars, allowing callers to
// override the tag prefix and filter configuration (used by component methods).
func (s semverStrategy) varsCore(p GitProject, extra map[string]string, tagPrefix string, filterCfg FilterConfig) (map[string]interface{}, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return nil, fmt.Errorf("getting branch name: %w", err)
	}

	branchCfg, captures := s.matchBranch(branchName)
	constraints := resolveConstraint(branchCfg, captures, extra)
	f := NewSemverFormat(tagPrefix, constraints)

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
	var isTruncated bool
	if lastTag != "0.0.0" {
		tagged, err := p.IsHeadTagged(lastTag)
		if err != nil {
			return nil, err
		}
		if !tagged {
			all, truncated, err := p.CommitSinceTag(lastTag)
			if err != nil {
				return nil, err
			}
			isTruncated = truncated
			// all includes the tagged commit at index len-1; exclude it
			if len(all) > 1 {
				commitsSinceTag = all[:len(all)-1]
			}
			// Apply filtering
			commitsSinceTag = FilterCommits(commitsSinceTag, p.CommitFiles, filterCfg)
			commitCount = len(commitsSinceTag)
		}
	} else if p.IsShallow() {
		// Shallow clone with no tag found: the tag is likely beyond the clone depth.
		isTruncated = true
	}

	// Conventional Commits bump analysis
	bumpLevel := BumpNone
	hasNonCC := false
	if len(commitsSinceTag) > 0 {
		bumpLevel, hasNonCC = AnalyzeBump(commitsSinceTag, s.cfg.ConventionalCommits)
	}

	// CC-calculated version (without prefix)
	semverStr := BumpVersion(effectiveLastTag, tagPrefix, bumpLevel)

	// Parse semver components for Semver (CC-calculated)
	var nextMajor, nextMinor, nextPatch, nextPreRelease string
	if sv, err := gosemver.NewVersion(semverStr); err == nil {
		nextMajor = strconv.FormatInt(sv.Major, 10)
		nextMinor = strconv.FormatInt(sv.Minor, 10)
		nextPatch = strconv.FormatInt(sv.Patch, 10)
		nextPreRelease = string(sv.PreRelease)
	}

	// Parse semver components for LastVersion (last tag stripped of prefix)
	lastVersionStr := strings.TrimPrefix(effectiveLastTag, tagPrefix)
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

	authorDate, committerDate, err := p.CommitDate()
	if err != nil {
		return nil, fmt.Errorf("getting commit date: %w", err)
	}

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
			"Semver":                    semverStr,
			"Major":                     nextMajor,
			"Minor":                     nextMinor,
			"Patch":                     nextPatch,
			"PreRelease":                nextPreRelease,
			"LastVersion":               lastVersionStr,
			"LastMajor":                 lastMajor,
			"LastMinor":                 lastMinor,
			"LastPatch":                 lastPatch,
			"LastPreRelease":            lastPreRelease,
			"IsBreakingChange":          bumpLevel == BumpMajor,
			"IsPreRelease":              branchCfg.VersionFormat != "",
			"HasNonConventionalCommits": hasNonCC,
		},
		"git": map[string]interface{}{
			"Branch":        shortBranch,
			"AuthorDate":    authorDate.UTC().Format("2006-01-02"),
			"CommitterDate": committerDate.UTC().Format("2006-01-02"),
			"LastTag":       rawLastTag,
			"Hash":          commitHashFull,
			"ShortHash":     shortHash,
			"CommitCount":   commitCount,
			"IsShallow":     p.IsShallow(),
			"Truncated":     isTruncated,
		},
		"regex": regexVars,
		"var":   varVars,
	}, nil
}

// Current returns the computed version at HEAD — always, whether or not HEAD is tagged.
// This is equivalent to the `next` CLI command semantics.
//
// NOTE: the CLI `current` command uses AllCurrent (which surfaces Tagged) to return
// the existing tag or an empty string. This method is kept for direct strategy use
// and tests but is not invoked by any CLI command path.
func (s semverStrategy) Current(p GitProject, extra map[string]string) (string, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return "", fmt.Errorf("getting branch name: %w", err)
	}

	branchCfg, captures := s.matchBranch(branchName)
	constraints := resolveConstraint(branchCfg, captures, extra)
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

	vars, err := s.Vars(p, extra)
	if err != nil {
		return "", err
	}

	versionFormat := branchCfg.VersionFormat
	if versionFormat == "" {
		versionFormat = "{{ .semver.Semver }}"
	}
	suffix, err := renderTemplate(versionFormat, vars)
	if err != nil {
		return "", err
	}
	return s.cfg.TagPrefix + suffix, nil
}

// matchBranch finds the first BranchConfig whose Pattern matches branchName.
// Returns the config and any named capture groups extracted from the match.
// If no pattern matches, returns a default pre-release config.
func (s semverStrategy) matchBranch(branchName string) (config.BranchConfig, map[string]string) {
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
		VersionFormat: "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}",
	}, map[string]string{}
}

// ParseWildcardConstraint parses a wildcard semver string like "1.x.x" into a
// constraint map. Each component must be a non-negative integer or "x".
// Returns empty map for "" or "x.x.x". Returns error for invalid input.
func ParseWildcardConstraint(s string) (map[string]string, error) {
	if s == "" {
		return map[string]string{}, nil
	}
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("constraint %q: expected 3 dot-separated components (e.g. 1.x.x), got %d", s, len(parts))
	}
	keys := []string{"major", "minor", "patch"}
	result := map[string]string{}
	for i, p := range parts {
		if p == "x" {
			continue
		}
		if _, err := strconv.Atoi(p); err != nil {
			return nil, fmt.Errorf("constraint %q: component %q must be a non-negative integer or 'x'", s, p)
		}
		result[keys[i]] = p
	}
	return result, nil
}

// resolveConstraint renders the branch Constraint template and parses the wildcard result.
// Only .regex.* and .var.* are available (no .semver.* or .git.*).
// Returns empty constraints (no filter) if Constraint is empty or on render/parse error.
func resolveConstraint(branchCfg config.BranchConfig, captures map[string]string, extra map[string]string) map[string]string {
	if branchCfg.Constraint == "" {
		return map[string]string{}
	}
	regexVars := map[string]interface{}{}
	for k, v := range captures {
		regexVars[k] = v
	}
	varVars := map[string]interface{}{}
	for k, v := range extra {
		varVars[k] = v
	}
	vars := map[string]interface{}{
		"regex": regexVars,
		"var":   varVars,
	}
	rendered, err := renderTemplate(branchCfg.Constraint, vars)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: rendering branch constraint %q: %v\n", branchCfg.Constraint, err)
		return map[string]string{}
	}
	constraints, err := ParseWildcardConstraint(rendered)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: parsing branch constraint %q (rendered: %q): %v\n", branchCfg.Constraint, rendered, err)
		return map[string]string{}
	}
	return constraints
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

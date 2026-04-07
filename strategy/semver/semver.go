package semver

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
	CommitFiles(c *object.Commit) ([]string, error)
	CommitDate() (time.Time, time.Time, error)
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
//   - "semver": Semver, Major, Minor, Patch, PreRelease (CC-calculated),
//               LastVersion, LastMajor, LastMinor, LastPatch, LastPreRelease,
//               IsBreakingChange, IsPreRelease, HasNonConventionalCommits
//   - "git":    Branch, AuthorDate, CommitterDate, LastTag, Hash, ShortHash, CommitCount
//   - "regex":  named captures from the matching branch pattern
//   - "var":    key=value pairs from extra
func (s Strategy) Vars(p GitProject, extra map[string]string) (map[string]interface{}, error) {
	return s.varsCore(p, extra, s.cfg.TagPrefix, FilterConfig{
		ExcludePaths:  s.cfg.IgnorePaths,
		IgnoreCommits: s.cfg.IgnoreCommits,
	})
}

// varsCore is the parameterised implementation of Vars, allowing callers to
// override the tag prefix and filter configuration (used by component methods).
func (s Strategy) varsCore(p GitProject, extra map[string]string, tagPrefix string, filterCfg FilterConfig) (map[string]interface{}, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return nil, fmt.Errorf("getting branch name: %w", err)
	}

	branchCfg, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
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
			// Apply filtering
			commitsSinceTag = FilterCommits(commitsSinceTag, p.CommitFiles, filterCfg)
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
			"IsPreRelease":              !branchCfg.Release,
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
		},
		"regex": regexVars,
		"var":   varVars,
	}, nil
}

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

	vars, err := s.Vars(p, extra)
	if err != nil {
		return "", err
	}

	semverMap, ok := vars["semver"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("internal error: semver namespace missing from Vars output")
	}

	if branchCfg.Release {
		// Return CC-calculated version with prefix (consistent with tag format)
		semverStr, ok := semverMap["Semver"].(string)
		if !ok {
			return "", fmt.Errorf("internal error: semver.Semver is not a string")
		}
		return s.cfg.TagPrefix + semverStr, nil
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
		Format:  "{{ .semver.Semver }}-{{ .git.Branch }}.{{ .git.CommitCount }}",
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

package semver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	gosemver "github.com/coreos/go-semver/semver"
	"github.com/go-git/go-git/v5/plumbing/object"

	"gover/config"
	"gover/format"
	gitpkg "gover/git"
)

// ComponentResult holds the version result for one entity.
type ComponentResult struct {
	Name    string
	Version string
	Tagged  bool // true if HEAD is exactly on this tag
}

// ComponentVarsResult holds vars for one entity.
type ComponentVarsResult struct {
	Name string
	Vars map[string]interface{}
}

// ResolveTagPrefix returns the full tag prefix for a component.
// scope = comp.TagScope if set, otherwise = name.
// Result = "{scope}/{globalPrefix}", e.g. "api/v" for component "api" with prefix "v".
func ResolveTagPrefix(name string, comp config.ComponentConfig, globalPrefix string) string {
	scope := comp.TagScope
	if scope == "" {
		scope = name
	}
	return scope + "/" + globalPrefix
}

// allComponentPaths returns all Path values from the components map.
func allComponentPaths(components map[string]config.ComponentConfig) []string {
	paths := make([]string, 0, len(components))
	for _, c := range components {
		paths = append(paths, c.Path)
	}
	return paths
}

// sortedComponentNames returns component names in alphabetical order.
func sortedComponentNames(components map[string]config.ComponentConfig) []string {
	names := make([]string, 0, len(components))
	for name := range components {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// sharedHistory is a pre-fetched snapshot of the commit graph, built once
// and shared across all component computations in AllCurrent/AllLast/AllVars.
type sharedHistory struct {
	items []gitpkg.CommitWithTags
}

// buildSharedHistory fetches the commit history once from the git layer.
func buildSharedHistory(p GitProject) (*sharedHistory, error) {
	items, err := p.CommitHistory()
	if err != nil {
		return nil, err
	}
	return &sharedHistory{items: items}, nil
}

// findLastTag returns the nearest ancestor tag valid for f, and its index in items.
// Items are in topological order (HEAD first), so the first valid tag encountered is
// the nearest ancestor. If multiple valid tags share the same commit, Compare is used
// as a deterministic tiebreaker (same logic as git.Project.LastTag).
// Returns ("0.0.0", -1) when no valid tag is found.
func (h *sharedHistory) findLastTag(f format.VersionFormat) (string, int) {
	for i, item := range h.items {
		var best string
		found := false
		for _, tag := range item.Tags {
			if !f.IsValid(tag) {
				continue
			}
			if !found {
				best, found = tag, true
			} else if cmp, err := f.Compare(tag, best); err == nil && cmp > 0 {
				best = tag
			}
		}
		if found {
			return best, i
		}
	}
	return "0.0.0", -1
}

// commitsSince returns the commits strictly between HEAD and the tagged commit
// (items[:tagIdx], the tagged commit itself is excluded).
// When tagIdx == -1 (shallow clone: tag not in accessible history),
// returns all fetched commits with truncated=true.
func (h *sharedHistory) commitsSince(tagIdx int) ([]*object.Commit, bool) {
	if tagIdx < 0 {
		commits := make([]*object.Commit, len(h.items))
		for i, item := range h.items {
			commits[i] = item.Commit
		}
		return commits, true
	}
	commits := make([]*object.Commit, tagIdx)
	for i, item := range h.items[:tagIdx] {
		commits[i] = item.Commit
	}
	return commits, false
}

// varsCoreFromHistory is the monorepo-optimised variant of varsCore.
// It uses a pre-fetched sharedHistory instead of calling p.LastTag and p.CommitSinceTag,
// so the commit graph is traversed only once across all components.
func (s Strategy) varsCoreFromHistory(
	p GitProject,
	extra map[string]string,
	hist *sharedHistory,
	tagPrefix string,
	filterCfg FilterConfig,
) (map[string]interface{}, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return nil, fmt.Errorf("getting branch name: %w", err)
	}

	branchCfg, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
	f := NewSemverFormat(tagPrefix, constraints)

	lastTag, tagIdx := hist.findLastTag(f)

	effectiveLastTag := lastTag
	if lastTag == "0.0.0" {
		effectiveLastTag = s.cfg.Initial
	}

	var commitsSinceTag []*object.Commit
	commitCount := 0
	var isTruncated bool
	if lastTag != "0.0.0" {
		tagged, err := p.IsHeadTagged(lastTag)
		if err != nil {
			return nil, err
		}
		if !tagged {
			rawCommits, truncated := hist.commitsSince(tagIdx)
			isTruncated = truncated
			commitsSinceTag = FilterCommits(rawCommits, p.CommitFiles, filterCfg)
			commitCount = len(commitsSinceTag)
		}
	} else if p.IsShallow() {
		// Shallow clone with no tag found: the tag is likely beyond the clone depth.
		isTruncated = true
	}

	bumpLevel := BumpNone
	hasNonCC := false
	if len(commitsSinceTag) > 0 {
		bumpLevel, hasNonCC = AnalyzeBump(commitsSinceTag, s.cfg.ConventionalCommits)
	}

	semverStr := BumpVersion(effectiveLastTag, tagPrefix, bumpLevel)

	var nextMajor, nextMinor, nextPatch, nextPreRelease string
	if sv, err := gosemver.NewVersion(semverStr); err == nil {
		nextMajor = strconv.FormatInt(sv.Major, 10)
		nextMinor = strconv.FormatInt(sv.Minor, 10)
		nextPatch = strconv.FormatInt(sv.Patch, 10)
		nextPreRelease = string(sv.PreRelease)
	}

	lastVersionStr := strings.TrimPrefix(effectiveLastTag, tagPrefix)
	var lastMajor, lastMinor, lastPatch, lastPreRelease string
	if sv, err := gosemver.NewVersion(lastVersionStr); err == nil {
		lastMajor = strconv.FormatInt(sv.Major, 10)
		lastMinor = strconv.FormatInt(sv.Minor, 10)
		lastPatch = strconv.FormatInt(sv.Patch, 10)
		lastPreRelease = string(sv.PreRelease)
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

	authorDate, committerDate, err := p.CommitDate()
	if err != nil {
		return nil, fmt.Errorf("getting commit date: %w", err)
	}

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
			"IsShallow":     p.IsShallow(),
			"Truncated":     isTruncated,
		},
		"regex": regexVars,
		"var":   varVars,
	}, nil
}

// AllCurrent returns current versions for @root and all components.
// When no components are defined, returns a single result with Name="" (backward compat).
func (s Strategy) AllCurrent(p GitProject, extra map[string]string, cfg config.Config) ([]ComponentResult, error) {
	if len(cfg.Components) == 0 {
		filter := FilterConfig{
			ExcludePaths:  s.cfg.IgnorePaths,
			IgnoreCommits: s.cfg.IgnoreCommits,
		}
		vars, err := s.varsCore(p, extra, s.cfg.TagPrefix, filter)
		if err != nil {
			return nil, err
		}
		v, tagged, err := s.currentFromVars(p, vars, s.cfg.TagPrefix)
		if err != nil {
			return nil, err
		}
		return []ComponentResult{{Name: "", Version: v, Tagged: tagged}}, nil
	}

	// @root: excludes all component paths
	rootFilter := FilterConfig{
		ExcludePaths:  append(append([]string{}, s.cfg.IgnorePaths...), allComponentPaths(cfg.Components)...),
		IgnoreCommits: s.cfg.IgnoreCommits,
	}
	rootVars, err := s.varsCore(p, extra, s.cfg.TagPrefix, rootFilter)
	if err != nil {
		return nil, err
	}
	rootVersion, rootTagged, err := s.currentFromVars(p, rootVars, s.cfg.TagPrefix)
	if err != nil {
		return nil, err
	}
	results := []ComponentResult{{Name: "@root", Version: rootVersion, Tagged: rootTagged}}

	for _, name := range sortedComponentNames(cfg.Components) {
		comp := cfg.Components[name]
		tagPrefix := ResolveTagPrefix(name, comp, s.cfg.TagPrefix)
		compFilter := FilterConfig{
			IncludePaths:  []string{comp.Path},
			ExcludePaths:  s.cfg.IgnorePaths,
			IgnoreCommits: s.cfg.IgnoreCommits,
		}
		vars, err := s.varsCore(p, extra, tagPrefix, compFilter)
		if err != nil {
			return nil, err
		}
		version, tagged, err := s.currentFromVars(p, vars, tagPrefix)
		if err != nil {
			return nil, err
		}
		results = append(results, ComponentResult{Name: name, Version: version, Tagged: tagged})
	}
	return results, nil
}

// AllLast returns last tags for @root and all components.
func (s Strategy) AllLast(p GitProject, cfg config.Config) ([]ComponentResult, error) {
	if len(cfg.Components) == 0 {
		v, err := s.Last(p)
		if err != nil {
			return nil, err
		}
		return []ComponentResult{{Name: "", Version: v}}, nil
	}

	rootLast, err := s.lastWithPrefix(p, s.cfg.TagPrefix)
	if err != nil {
		return nil, err
	}
	results := []ComponentResult{{Name: "@root", Version: rootLast}}

	for _, name := range sortedComponentNames(cfg.Components) {
		comp := cfg.Components[name]
		tagPrefix := ResolveTagPrefix(name, comp, s.cfg.TagPrefix)
		v, err := s.lastWithPrefix(p, tagPrefix)
		if err != nil {
			return nil, err
		}
		results = append(results, ComponentResult{Name: name, Version: v})
	}
	return results, nil
}

// AllVars returns vars for @root and all components.
func (s Strategy) AllVars(p GitProject, extra map[string]string, cfg config.Config) ([]ComponentVarsResult, error) {
	if len(cfg.Components) == 0 {
		vars, err := s.Vars(p, extra)
		if err != nil {
			return nil, err
		}
		return []ComponentVarsResult{{Name: "", Vars: vars}}, nil
	}

	rootFilter := FilterConfig{
		ExcludePaths:  append(append([]string{}, s.cfg.IgnorePaths...), allComponentPaths(cfg.Components)...),
		IgnoreCommits: s.cfg.IgnoreCommits,
	}
	rootVars, err := s.varsCore(p, extra, s.cfg.TagPrefix, rootFilter)
	if err != nil {
		return nil, err
	}
	results := []ComponentVarsResult{{Name: "@root", Vars: rootVars}}

	for _, name := range sortedComponentNames(cfg.Components) {
		comp := cfg.Components[name]
		tagPrefix := ResolveTagPrefix(name, comp, s.cfg.TagPrefix)
		compFilter := FilterConfig{
			IncludePaths:  []string{comp.Path},
			ExcludePaths:  s.cfg.IgnorePaths,
			IgnoreCommits: s.cfg.IgnoreCommits,
		}
		vars, err := s.varsCore(p, extra, tagPrefix, compFilter)
		if err != nil {
			return nil, err
		}
		results = append(results, ComponentVarsResult{Name: name, Vars: vars})
	}
	return results, nil
}

// lastWithPrefix returns the last tag for a given tag prefix, or cfg.Initial if none.
func (s Strategy) lastWithPrefix(p GitProject, tagPrefix string) (string, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return "", err
	}
	_, captures := s.matchBranch(branchName)
	constraints := versionConstraints(captures)
	f := NewSemverFormat(tagPrefix, constraints)
	tag, err := p.LastTag(f)
	if err != nil {
		return "", err
	}
	if tag == "0.0.0" {
		return s.cfg.Initial, nil
	}
	return tag, nil
}

// currentFromVars derives the current version string from pre-computed vars.
// Returns (version, tagged, error) where tagged=true means HEAD is exactly on that tag.
func (s Strategy) currentFromVars(p GitProject, vars map[string]interface{}, tagPrefix string) (string, bool, error) {
	branchName, err := p.BranchName()
	if err != nil {
		return "", false, err
	}
	branchCfg, _ := s.matchBranch(branchName)

	semverMap, ok := vars["semver"].(map[string]interface{})
	if !ok {
		return "", false, fmt.Errorf("internal error: semver namespace missing")
	}

	gitMap, ok := vars["git"].(map[string]interface{})
	if !ok {
		return "", false, fmt.Errorf("internal error: git namespace missing")
	}
	rawLastTag, _ := gitMap["LastTag"].(string)
	if rawLastTag != "" {
		tagged, err := p.IsHeadTagged(rawLastTag)
		if err != nil {
			return "", false, err
		}
		if tagged {
			return rawLastTag, true, nil
		}
	}

	if rawLastTag == "" {
		return s.cfg.Initial, false, nil
	}

	semverStr, ok := semverMap["Semver"].(string)
	if !ok {
		return "", false, fmt.Errorf("internal error: semver.Semver is not a string")
	}

	if branchCfg.Release {
		return tagPrefix + semverStr, false, nil
	}
	v, err := renderTemplate(branchCfg.Format, vars)
	return v, false, err
}

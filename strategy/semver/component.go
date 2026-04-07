package semver

import (
	"fmt"
	"sort"

	"gover/config"
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

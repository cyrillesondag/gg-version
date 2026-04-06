package semver

import (
	"regexp"
	"strings"

	gosemver "github.com/coreos/go-semver/semver"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-git/go-git/v5/plumbing/object"

	"gover/config"
)

// Bump levels — exported so tests can reference them symbolically.
const (
	BumpNone  = 0
	BumpPatch = 1
	BumpMinor = 2
	BumpMajor = 3
)

// FilterConfig defines rules for including or excluding commits from CC analysis.
type FilterConfig struct {
	// IncludePaths: commit is included only if ≥1 modified file matches a pattern.
	// Empty = no include filter (all commits pass).
	IncludePaths []string
	// ExcludePaths: commit is excluded if ALL modified files match ≥1 pattern.
	ExcludePaths []string
	// IgnoreCommits: SHA prefixes — commit excluded if its full SHA starts with any entry.
	IgnoreCommits []string
}

// FilterCommits returns the subset of commits that pass the filter rules.
// files is a function that returns the list of files modified by a commit.
// Order of evaluation per commit:
//  1. SHA matches an IgnoreCommits prefix → excluded
//  2. IncludePaths defined AND no file matches → excluded
//  3. ExcludePaths defined AND ALL files match → excluded
//  4. Otherwise → included
func FilterCommits(
	commits []*object.Commit,
	files func(*object.Commit) ([]string, error),
	cfg FilterConfig,
) []*object.Commit {
	// fast path: no filter configured
	if len(cfg.IncludePaths) == 0 && len(cfg.ExcludePaths) == 0 && len(cfg.IgnoreCommits) == 0 {
		return commits
	}

	result := make([]*object.Commit, 0, len(commits))
	for _, c := range commits {
		if isIgnoredBySHA(c.Hash.String(), cfg.IgnoreCommits) {
			continue
		}

		changedFiles, err := files(c)
		if err != nil {
			// on error, include conservatively
			result = append(result, c)
			continue
		}

		if len(cfg.IncludePaths) > 0 && !anyFileMatches(changedFiles, cfg.IncludePaths) {
			continue
		}

		if len(cfg.ExcludePaths) > 0 && allFilesMatch(changedFiles, cfg.ExcludePaths) {
			continue
		}

		result = append(result, c)
	}
	return result
}

func isIgnoredBySHA(sha string, ignoreList []string) bool {
	for _, prefix := range ignoreList {
		if strings.HasPrefix(sha, prefix) {
			return true
		}
	}
	return false
}

// anyFileMatches returns true if at least one file matches at least one pattern.
func anyFileMatches(files []string, patterns []string) bool {
	for _, f := range files {
		for _, p := range patterns {
			if matched, _ := doublestar.Match(p, f); matched {
				return true
			}
		}
	}
	return false
}

// allFilesMatch returns true if every file matches at least one pattern.
// Returns false for empty file list.
func allFilesMatch(files []string, patterns []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		fileMatched := false
		for _, p := range patterns {
			if matched, _ := doublestar.Match(p, f); matched {
				fileMatched = true
				break
			}
		}
		if !fileMatched {
			return false
		}
	}
	return true
}

// AnalyzeBump scans commits and returns:
//   - level: 0=no bump, 1=patch, 2=minor, 3=major
//   - hasNonCC: true if at least one commit subject did not match the CC format
func AnalyzeBump(commits []*object.Commit, cfg config.ConventionalCommitsConfig) (level int, hasNonCC bool) {
	formatRe := compilePattern(cfg.Format)
	majorRes := compilePatterns(cfg.Major)
	minorRes := compilePatterns(cfg.Minor)
	patchRes := compilePatterns(cfg.Patch)

	for _, c := range commits {
		lines := strings.Split(strings.TrimRight(c.Message, "\n"), "\n")
		if len(lines) == 0 {
			continue
		}

		subject := lines[0]
		footerLines := extractFooterLines(lines)

		// Classify subject
		subjectLevel := levelFromPatterns(subject, majorRes, minorRes, patchRes)
		if subjectLevel >= 0 {
			// matched a bump pattern — use it
		} else if formatRe != nil && formatRe.MatchString(subject) {
			subjectLevel = BumpNone // CC-formatted but unrecognized type → no contribution
		} else {
			hasNonCC = true
			subjectLevel = BumpPatch // non-CC → patch by default
		}

		// Classify footer lines (no format check — just bump patterns)
		footerLevel := BumpNone
		for _, fl := range footerLines {
			l := levelFromPatterns(fl, majorRes, minorRes, patchRes)
			if l > footerLevel {
				footerLevel = l
			}
		}

		commitLevel := subjectLevel
		if footerLevel > commitLevel {
			commitLevel = footerLevel
		}
		if commitLevel > level {
			level = commitLevel
		}
	}
	return level, hasNonCC
}

// BumpVersion applies the bump level to effectiveLastTag (stripping prefix) and returns
// the bumped version WITHOUT prefix. Returns the stripped version unchanged for BumpNone.
func BumpVersion(effectiveLastTag, prefix string, level int) string {
	versionStr := strings.TrimPrefix(effectiveLastTag, prefix)
	if level == BumpNone {
		return versionStr
	}
	sv, err := gosemver.NewVersion(versionStr)
	if err != nil {
		return versionStr // unparseable — return as-is
	}
	switch level {
	case BumpMajor:
		sv.Major++
		sv.Minor = 0
		sv.Patch = 0
		sv.PreRelease = gosemver.PreRelease("")
	case BumpMinor:
		sv.Minor++
		sv.Patch = 0
		sv.PreRelease = gosemver.PreRelease("")
	case BumpPatch:
		sv.Patch++
		sv.PreRelease = gosemver.PreRelease("")
	}
	return sv.String()
}

// levelFromPatterns returns the bump level matching the line, or -1 if no pattern matched.
func levelFromPatterns(line string, majorRes, minorRes, patchRes []*regexp.Regexp) int {
	for _, re := range majorRes {
		if re.MatchString(line) {
			return BumpMajor
		}
	}
	for _, re := range minorRes {
		if re.MatchString(line) {
			return BumpMinor
		}
	}
	for _, re := range patchRes {
		if re.MatchString(line) {
			return BumpPatch
		}
	}
	return -1
}

// extractFooterLines returns lines after the first blank line in the commit message.
func extractFooterLines(lines []string) []string {
	for i := 1; i < len(lines); i++ {
		if lines[i] == "" && i+1 < len(lines) {
			return lines[i+1:]
		}
	}
	return nil
}

func compilePattern(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re
}

func compilePatterns(patterns []string) []*regexp.Regexp {
	res := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			res = append(res, re)
		}
	}
	return res
}

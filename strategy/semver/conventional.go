package semver

import (
	"regexp"
	"strings"

	gosemver "github.com/coreos/go-semver/semver"
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

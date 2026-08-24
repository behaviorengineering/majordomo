package staging

import (
	"regexp"
	"strings"
)

var excludePatterns []*regexp.Regexp

func init() {
	raw := []string{
		`.*\.lock$`,
		`.*-lock\.json$`,
		`.*\.min\.js$`,
		`.*\.min\.css$`,
		`.*\.pyc$`,
		`__pycache__/`,
		`.*\.svg$`,
		`.*\.png$`,
		`.*\.jpg$`,
		`.*\.jpeg$`,
		`.*\.gif$`,
		`.*\.ico$`,
		`.*\.woff2?$`,
		`.*\.ttf$`,
		`.*\.eot$`,
		`.*\.otf$`,
		`.*\.pdf$`,
		`^dist/`,
		`^build/`,
	}
	excludePatterns = make([]*regexp.Regexp, 0, len(raw))
	for _, p := range raw {
		excludePatterns = append(excludePatterns, regexp.MustCompile(p))
	}
}

// IsExcluded reports whether file matches a static exclusion pattern.
func IsExcluded(file string) bool {
	for _, p := range excludePatterns {
		if p.MatchString(file) {
			return true
		}
	}
	return false
}

// ParseSubmoduleStatusLines builds ^path/ exclusion regexes from git submodule status --cached output.
func ParseSubmoduleStatusLines(stdout string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "+-U")
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			p := strings.TrimRight(parts[1], "/")
			patterns = append(patterns, regexp.MustCompile("^"+regexp.QuoteMeta(p)+"/"))
		}
	}
	return patterns
}

// MatchGlob mimics Python fnmatch on Unix (* matches across '/'; ** is two stars).
func MatchGlob(pattern, name string) bool {
	pattern = strings.ReplaceAll(pattern, "**", "*")
	return matchStarSlash(pattern, name)
}

func matchStarSlash(pattern, name string) bool {
	// Recursive backtracking matcher: * matches any string including /
	var match func(p, n int) bool
	match = func(p, n int) bool {
		for p < len(pattern) {
			if pattern[p] == '*' {
				for ; p < len(pattern) && pattern[p] == '*'; p++ {
				}
				if p == len(pattern) {
					return true
				}
				for i := n; i <= len(name); i++ {
					if match(p, i) {
						return true
					}
				}
				return false
			}
			if n >= len(name) || pattern[p] != name[n] {
				// handle ? 
				if pattern[p] == '?' && n < len(name) {
					p++
					n++
					continue
				}
				return false
			}
			p++
			n++
		}
		return n == len(name)
	}
	return match(0, 0)
}

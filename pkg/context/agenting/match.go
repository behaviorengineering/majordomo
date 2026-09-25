package agenting

import "strings"

// MatchGlob mimics Python fnmatch (* matches across '/'; ** collapses to *).
func MatchGlob(pattern, name string) bool {
	pattern = strings.ReplaceAll(pattern, "**", "*")
	return matchStarSlash(pattern, name)
}

func matchStarSlash(pattern, name string) bool {
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

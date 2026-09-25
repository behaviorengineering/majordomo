package agenting

import "strings"

// Select returns pack ids that match mode and at least one changed path (or have no globs).
func Select(idx Index, mode string, changedFiles []string) []string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	var out []string
	seen := map[string]struct{}{}
	order := idx.PackIDs()
	for _, id := range order {
		pack, ok := idx.Packs[id]
		if !ok {
			continue
		}
		if !modeAllowed(pack, mode) {
			continue
		}
		if len(pack.Globs) == 0 || anyGlobMatch(pack.Globs, changedFiles) {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func modeAllowed(pack Pack, mode string) bool {
	for _, m := range pack.Modes {
		if strings.ToLower(strings.TrimSpace(m)) == mode {
			return true
		}
	}
	return false
}

func anyGlobMatch(globs, files []string) bool {
	for _, file := range files {
		for _, g := range globs {
			if MatchGlob(g, filepathToSlash(file)) {
				return true
			}
		}
	}
	return false
}

func filepathToSlash(s string) string {
	return strings.ReplaceAll(s, "\\", "/")
}

// ModeForSkill maps a mechanical prep skill dir to an agenting mode.
func ModeForSkill(skill string) string {
	switch strings.TrimSpace(skill) {
	case "pr-review-summary", "pr-review-blast-radius":
		return ModeSummary
	case "pr-review-technical":
		return ModeTechnical
	default:
		return ModeFiles
	}
}

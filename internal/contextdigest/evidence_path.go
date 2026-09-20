package contextdigest

import (
	"os"
	"path/filepath"
	"strings"
)

// readEvidenceRel reads primaryRel under dir, then each legacy name. Empty if none exist.
func readEvidenceRel(dir, primaryRel string, legacyRels ...string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	candidates := append([]string{primaryRel}, legacyRels...)
	for _, rel := range candidates {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			continue
		}
		return string(b)
	}
	return ""
}

// resolveEvidenceRel picks an existing relative path under dir (primary first, then legacy).
// Returns primaryRel when nothing exists yet (for new writes / manifest defaults).
func resolveEvidenceRel(dir, primaryRel string, legacyRels ...string) string {
	dir = strings.TrimSpace(dir)
	candidates := append([]string{primaryRel}, legacyRels...)
	for _, rel := range candidates {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		if dir == "" {
			return primaryRel
		}
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			return rel
		}
	}
	return primaryRel
}

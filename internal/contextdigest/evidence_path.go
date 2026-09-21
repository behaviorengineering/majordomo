package contextdigest

import (
	"os"
	"path/filepath"
	"strings"
)

// readEvidenceRel reads primaryRel under dir. Empty if missing.
func readEvidenceRel(dir, primaryRel string, _ ...string) string {
	dir = strings.TrimSpace(dir)
	rel := strings.TrimSpace(primaryRel)
	if dir == "" || rel == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return ""
	}
	return string(b)
}

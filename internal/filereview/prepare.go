package filereview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadReviewables reads reviewable file/slug pairs from a batch manifest.json.
func LoadReviewables(manifestPath string) ([]Reviewable, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("filereview prepare: read manifest: %w", err)
	}
	var raw struct {
		Reviewable []map[string]any `json:"reviewable"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("filereview prepare: decode manifest: %w", err)
	}
	out := make([]Reviewable, 0, len(raw.Reviewable))
	seen := map[string]struct{}{}
	for _, t := range raw.Reviewable {
		file, _ := t["file"].(string)
		slug, _ := t["slug"].(string)
		if file == "" || slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, Reviewable{File: file, Slug: slug})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("filereview prepare: no reviewables in %s", manifestPath)
	}
	return out, nil
}

// PerFileDir returns the Judge output directory for a skill.
func PerFileDir(skillOut string) string {
	return filepath.Join(skillOut, "per-file")
}

package contextstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Bootstrap writes the canonical empty context tree for repoID with cursor at cursorSHA.
func Bootstrap(dir, repoID, cursorSHA string, digestAt time.Time) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("bootstrap: dir is required")
	}
	if strings.TrimSpace(repoID) == "" {
		return fmt.Errorf("bootstrap: repo_id is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("bootstrap mkdir: %w", err)
	}
	ts := digestAt.UTC().Format(time.RFC3339)
	meta := Meta{
		SchemaVersion: CurrentSchemaVersion,
		RepoID:        repoID,
		LastMergedSHA: strings.TrimSpace(cursorSHA),
		LastDigestAt:  ts,
	}
	metaBytes, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("bootstrap meta.yaml: %w", err)
	}
	files := map[string]string{
		"README.md": `# Context branch

This orphan branch holds project understanding for the served repo.
Never merge these files into the default branch.
Context updates are PRs whose base is this branch.

<!-- majordomo-reading-toc:start -->
## Reading order

1. [README.md](README.md) (this file)
2. [mission.md](mission.md)
3. [architecture.md](architecture.md)
4. [conventions.md](conventions.md)
5. [weaknesses.md](weaknesses.md)
6. [chronology.md](chronology.md)
7. ` + "`evidence/typology/`" + ` — appears after a Typology survey seed

<!-- majordomo-reading-toc:end -->

## Story vs Typology evidence

- Root markdown (` + "`mission.md`" + `, ` + "`architecture.md`" + `, …) is the **teaching story**.
- ` + "`evidence/typology/`" + ` holds Typology **seed evidence** for this digest proposal (for example ` + "`architecture_brief.md`" + `). That brief is not the teaching story and not the confirmed ` + "`.typology/`" + ` catalog.
`,
		"meta.yaml": string(metaBytes),
		"mission.md": `# Mission

Describe what this repo exists to do.
`,
		"architecture.md": `# Architecture

> **Teaching story.** Living project architecture for humans and review grounding. Not Typology seed evidence (see ` + "`evidence/typology/architecture_brief.md`" + `).

Describe the high-level shape of the system.
`,
		"conventions.md": `# Conventions

Describe how contributors work in this repo.
`,
		"weaknesses.md": `# Weaknesses

Known gaps and risks worth remembering.
`,
		"chronology.md": `# Chronology

Newest first.
`,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("bootstrap %s: %w", name, err)
		}
	}
	if err := bootstrapAgenting(dir); err != nil {
		return err
	}
	if err := ApplyReadingPath(dir); err != nil {
		return err
	}
	return ValidateTree(dir)
}

func bootstrapAgenting(dir string) error {
	indexPath := filepath.Join(dir, "agenting", "index.yaml")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return err
	}
	index := `packs:
  overview:
    modes: [files, summary, technical, digest]
`
	if err := os.WriteFile(indexPath, []byte(index), 0o644); err != nil {
		return fmt.Errorf("bootstrap agenting/index.yaml: %w", err)
	}
	overviewDir := filepath.Join(dir, "agenting", "overview")
	if err := os.MkdirAll(overviewDir, 0o755); err != nil {
		return err
	}
	grounding := `# Overview

High-level project grounding for review. Digest expands this from mission.md and architecture.md.
`
	path := filepath.Join(overviewDir, "GROUNDING.md")
	if err := os.WriteFile(path, []byte(grounding), 0o644); err != nil {
		return fmt.Errorf("bootstrap agenting/overview/GROUNDING.md: %w", err)
	}
	return nil
}

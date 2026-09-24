package filereview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CollectReports parses per-file markdown for each reviewable.
func CollectReports(perFileDir string, reviewables []Reviewable) (map[string]Report, error) {
	out := make(map[string]Report, len(reviewables))
	for _, r := range reviewables {
		path := filepath.Join(perFileDir, r.Slug+".md")
		rep, err := ParseMarkdownReport(path, r.Slug)
		if err != nil {
			return nil, fmt.Errorf("filereview collect %s: %w", r.Slug, err)
		}
		if rep.File == rep.Slug && r.File != "" {
			rep.File = r.File
		}
		rep.Slug = r.Slug
		out[r.Slug] = rep
	}
	return out, nil
}

// Assemble writes findings.json and rewrites markdown from structured reports.
func Assemble(skillOut string, reports map[string]Report, reviewables []Reviewable) error {
	perFile := PerFileDir(skillOut)
	if err := os.MkdirAll(perFile, 0o755); err != nil {
		return err
	}
	ordered := make([]Report, 0, len(reviewables))
	for _, r := range reviewables {
		rep := reports[r.Slug]
		ordered = append(ordered, rep)
		md := FormatMarkdown(rep)
		if err := os.WriteFile(filepath.Join(perFile, r.Slug+".md"), []byte(md), 0o644); err != nil {
			return fmt.Errorf("filereview assemble md %s: %w", r.Slug, err)
		}
	}
	data, err := json.MarshalIndent(map[string]any{"reports": ordered}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(skillOut, "findings.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("filereview assemble findings.json: %w", err)
	}
	return nil
}

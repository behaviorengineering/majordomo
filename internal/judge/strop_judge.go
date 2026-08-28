package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/filereview"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

// FileReviewOptions configures strop-backed file review Judge step.
type FileReviewOptions struct {
	StagingDir string
	SkillOut   string
}

// FileReviewBatch runs the strop filereview generator for each reviewable and writes per-file markdown.
func FileReviewBatch(opts FileReviewOptions) error {
	if err := EnsureStropReady(); err != nil {
		return err
	}
	reviewables, err := filereview.LoadReviewables(filepath.Join(opts.StagingDir, "manifest.json"))
	if err != nil {
		return err
	}
	perFile := filereview.PerFileDir(opts.SkillOut)
	if err := os.MkdirAll(perFile, 0o755); err != nil {
		return err
	}
	grounding := readGrounding(opts.StagingDir)
	ctx := context.Background()
	for _, r := range reviewables {
		diff, err := readReviewableInput(opts.StagingDir, r)
		if err != nil {
			return err
		}
		out, err := Generate(ctx, jmodules.TaskFileReview, map[string]interface{}{
			"file_path":    r.File,
			"slug":         r.Slug,
			"diff_content": diff,
			"grounding":    grounding,
		}, 1)
		if err != nil {
			return fmt.Errorf("filereview %s: %w", r.Slug, err)
		}
		md, _ := out["markdown"].(string)
		if strings.TrimSpace(md) == "" {
			md = filereview.FormatMarkdown(filereview.Report{File: r.File, Slug: r.Slug, NoIssues: true})
		}
		path := filepath.Join(perFile, r.Slug+".md")
		if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func readReviewableInput(stagingDir string, r filereview.Reviewable) (string, error) {
	data, err := os.ReadFile(filepath.Join(stagingDir, "manifest.json"))
	if err != nil {
		return "", err
	}
	var raw struct {
		Reviewable []map[string]any `json:"reviewable"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", err
	}
	inputRel := ""
	for _, row := range raw.Reviewable {
		slug, _ := row["slug"].(string)
		if slug == r.Slug {
			inputRel, _ = row["input_file"].(string)
			break
		}
	}
	if inputRel == "" {
		return fmt.Sprintf("(no staged diff for %s)", r.File), nil
	}
	b, err := os.ReadFile(filepath.Join(stagingDir, inputRel))
	if err != nil {
		return "", fmt.Errorf("read input %s: %w", inputRel, err)
	}
	return string(b), nil
}

func readGrounding(stagingDir string) string {
	dir := filepath.Join(stagingDir, ".grounding")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var parts []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n\n")
}

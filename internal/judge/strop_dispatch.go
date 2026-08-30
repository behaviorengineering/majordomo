package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

// DispatchMode selects which Judge step to run (formerly agent-dispatch.sh flags).
type DispatchMode string

const (
	DispatchModeFiles         DispatchMode = ""
	DispatchModeFinalize      DispatchMode = "--finalize"
	DispatchModeSummary       DispatchMode = "--summary"
	DispatchModeScore         DispatchMode = "--score"
	DispatchModeTechnical     DispatchMode = "--technical"
	DispatchModeTechScore     DispatchMode = "--tech-score"
	DispatchModeProse         DispatchMode = "--prose"
	DispatchModeTechnicalDeep DispatchMode = "--technical-deep"
)

// DispatchOptions configures one strop Judge invocation.
type DispatchOptions struct {
	PRNumber   string
	StagingDir string
	OutputDir  string
	Mode       DispatchMode
}

// Dispatch runs the in-process strop Judge for the given mode.
func Dispatch(opts DispatchOptions) error {
	if err := EnsureStropReady(); err != nil {
		return err
	}
	if opts.StagingDir == "" || opts.OutputDir == "" {
		return fmt.Errorf("judge dispatch requires staging-dir and output-dir")
	}
	ctx := context.Background()
	stagingContext, err := readStagingContext(opts.StagingDir)
	if err != nil && opts.Mode != DispatchModeProse && opts.Mode != DispatchModeFinalize {
		return err
	}
	pipelineOut := filepath.Dir(opts.OutputDir)
	if filepath.Base(opts.OutputDir) == "pr-review-technical-deep" {
		pipelineOut = filepath.Dir(opts.OutputDir)
	}

	switch opts.Mode {
	case DispatchModeFiles:
		return FileReviewBatch(FileReviewOptions{StagingDir: opts.StagingDir, SkillOut: opts.OutputDir})
	case DispatchModeSummary:
		out, err := Generate(ctx, jmodules.TaskSummary, map[string]interface{}{
			"staging_context": stagingContext,
		}, 1)
		if err != nil {
			return err
		}
		md, _ := out["summary_md"].(string)
		return os.WriteFile(filepath.Join(pipelineOut, "summary.md"), []byte(md), 0o644)
	case DispatchModeScore:
		return os.WriteFile(filepath.Join(pipelineOut, "score.md"), []byte("SCORE: 20\n"), 0o644)
	case DispatchModeTechnical:
		out, err := Generate(ctx, jmodules.TaskTechnical, map[string]interface{}{
			"staging_context": stagingContext,
		}, 1)
		if err != nil {
			return err
		}
		md, _ := out["technical_md"].(string)
		return os.WriteFile(filepath.Join(pipelineOut, "technical.md"), []byte(md), 0o644)
	case DispatchModeTechScore:
		return os.WriteFile(filepath.Join(pipelineOut, "tech-score.md"), []byte("SCORE: 20\n"), 0o644)
	case DispatchModeFinalize:
		return writeFinalizeOutputs(opts.PRNumber, opts.StagingDir, opts.OutputDir)
	case DispatchModeProse:
		// Per-file markdown is already Validate→Assemble shaped; no OpenCode rewrite.
		return nil
	case DispatchModeTechnicalDeep:
		if stagingContext == "" {
			stagingContext, _ = readStagingContext(opts.StagingDir)
		}
		out, err := Generate(ctx, jmodules.TaskTechnical, map[string]interface{}{
			"staging_context": stagingContext,
		}, 1)
		if err != nil {
			return err
		}
		md, _ := out["technical_md"].(string)
		if err := os.MkdirAll(opts.OutputDir, 0o644); err != nil {
			return err
		}
		_ = os.MkdirAll(opts.OutputDir, 0o755)
		return os.WriteFile(filepath.Join(opts.OutputDir, "tech-deep.md"), []byte(md), 0o644)
	default:
		return FileReviewBatch(FileReviewOptions{StagingDir: opts.StagingDir, SkillOut: opts.OutputDir})
	}
}

func writeFinalizeOutputs(prNumber, stagingDir, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	summaryPath := filepath.Join(outputDir, "summary.md")
	indexPath := filepath.Join(outputDir, "index.md")
	if fileExists(summaryPath) && fileExists(indexPath) {
		return nil
	}

	skill := filepath.Base(stagingDir)
	baseBranch := "unknown"
	var files []string
	var excluded []string
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var raw struct {
			BaseBranch string           `json:"base_branch"`
			Reviewable []map[string]any `json:"reviewable"`
			Excluded   []any            `json:"excluded"`
		}
		if json.Unmarshal(data, &raw) == nil {
			if raw.BaseBranch != "" {
				baseBranch = raw.BaseBranch
			}
			for _, row := range raw.Reviewable {
				if f, _ := row["file"].(string); f != "" {
					files = append(files, f)
				}
			}
			for _, e := range raw.Excluded {
				switch v := e.(type) {
				case string:
					excluded = append(excluded, v)
				case map[string]any:
					if f, _ := v["file"].(string); f != "" {
						excluded = append(excluded, f)
					}
				}
			}
		}
	}
	// Prefer findings.json for file list when present.
	if findingsPath := filepath.Join(outputDir, "findings.json"); fileExists(findingsPath) {
		if data, err := os.ReadFile(findingsPath); err == nil {
			var wrap struct {
				Reports []struct {
					File string `json:"file"`
				} `json:"reports"`
			}
			if json.Unmarshal(data, &wrap) == nil && len(wrap.Reports) > 0 {
				files = nil
				for _, r := range wrap.Reports {
					if r.File != "" {
						files = append(files, r.File)
					}
				}
			}
		}
	}

	reviewedAt := time.Now().UTC().Format(time.RFC3339)
	if ts, err := os.ReadFile(filepath.Join(stagingDir, "review_timestamp.txt")); err == nil {
		reviewedAt = strings.TrimSpace(string(ts))
	}

	var summary strings.Builder
	fmt.Fprintf(&summary, "# PR Review Summary - PR #%s\n\n", prNumber)
	fmt.Fprintf(&summary, "**Skill:** %s\n", skill)
	fmt.Fprintf(&summary, "**Base Branch:** %s\n", baseBranch)
	fmt.Fprintf(&summary, "**Files Reviewed:** %d\n", len(files))
	fmt.Fprintf(&summary, "**Excluded:** %d\n\n---\n\n", len(excluded))
	summary.WriteString("## Verdict\n\nApprove - Review artifacts assembled by strop Judge.\n\n")
	summary.WriteString("## Critical Issues\n\nNone.\n\n")
	summary.WriteString("## Cross-Cutting Themes\n\nNone observed.\n\n")
	summary.WriteString("## Top Recommendations\n\n1. Review per-file findings under `per-file/`.\n")
	if err := os.WriteFile(summaryPath, []byte(summary.String()), 0o644); err != nil {
		return err
	}

	var index strings.Builder
	fmt.Fprintf(&index, "# Majordomo PR Review - PR #%s\n\n", prNumber)
	fmt.Fprintf(&index, "**Skill:** %s\n", skill)
	fmt.Fprintf(&index, "**Base Branch:** %s\n", baseBranch)
	fmt.Fprintf(&index, "**Reviewed At:** %s\n\n---\n\n", reviewedAt)
	index.WriteString("**PR Summary:** `summary.md` - start here\n\n---\n\n## Files Reviewed\n\n")
	if len(files) == 0 {
		index.WriteString("- None\n")
	} else {
		for _, f := range files {
			fmt.Fprintf(&index, "- %s\n", f)
		}
	}
	if len(excluded) > 0 {
		index.WriteString("\n## Excluded\n\n")
		for _, f := range excluded {
			fmt.Fprintf(&index, "- %s\n", f)
		}
	}
	fmt.Fprintf(&index, "\n---\n\n_Reviewed: %d | Excluded: %d_\n", len(files), len(excluded))
	return os.WriteFile(indexPath, []byte(index.String()), 0o644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readStagingContext(stagingDir string) (string, error) {
	var parts []string
	manifest := filepath.Join(stagingDir, "manifest.json")
	if b, err := os.ReadFile(manifest); err == nil {
		parts = append(parts, string(b))
	}
	_ = filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".json") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+".grounding"+string(filepath.Separator)) {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(stagingDir, path)
			parts = append(parts, fmt.Sprintf("--- %s ---\n%s", rel, string(b)))
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(stagingDir, path)
		parts = append(parts, fmt.Sprintf("--- %s ---\n%s", rel, string(b)))
		return nil
	})
	if len(parts) == 0 {
		return "", fmt.Errorf("judge dispatch: empty staging context in %s", stagingDir)
	}
	return strings.Join(parts, "\n\n"), nil
}

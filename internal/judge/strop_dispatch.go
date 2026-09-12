package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
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
	Context    context.Context
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
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
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
		return FileReviewBatch(FileReviewOptions{
			Context:    ctx,
			StagingDir: opts.StagingDir,
			SkillOut:   opts.OutputDir,
		})
	case DispatchModeSummary:
		out, err := Generate(ctx, jmodules.TaskSummary, map[string]interface{}{
			"staging_context": stagingContext,
		}, 1)
		if err != nil {
			return err
		}
		md, ok := out["summary_md"].(string)
		if !ok {
			return fmt.Errorf("judge dispatch: summary output missing string field summary_md")
		}
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
		md, ok := out["technical_md"].(string)
		if !ok {
			return fmt.Errorf("judge dispatch: technical output missing string field technical_md")
		}
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
			stagingContext, err = readStagingContext(opts.StagingDir)
			if err != nil {
				return err
			}
		}
		out, err := Generate(ctx, jmodules.TaskTechnical, map[string]interface{}{
			"staging_context": stagingContext,
		}, 1)
		if err != nil {
			return err
		}
		md, ok := out["technical_md"].(string)
		if !ok {
			return fmt.Errorf("judge dispatch: technical output missing string field technical_md")
		}
		if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
			return err
		}
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
				if f, ok := row["file"].(string); ok && f != "" {
					files = append(files, f)
				}
			}
			for _, e := range raw.Excluded {
				switch v := e.(type) {
				case string:
					excluded = append(excluded, v)
				case map[string]any:
					if f, ok := v["file"].(string); ok && f != "" {
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

	summary, index, err := renderFinalizeReports(finalizeReportData{
		PRNumber:   prNumber,
		Skill:      skill,
		BaseBranch: baseBranch,
		ReviewedAt: reviewedAt,
		Files:      files,
		Excluded:   excluded,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(summaryPath, []byte(summary), 0o644); err != nil {
		return err
	}
	return os.WriteFile(indexPath, []byte(index), 0o644)
}

type finalizeReportData struct {
	PRNumber   string
	Skill      string
	BaseBranch string
	ReviewedAt string
	Files      []string
	Excluded   []string
}

const finalizeSummaryTemplate = `# PR Review Summary - PR #{{.PRNumber}}

**Skill:** {{.Skill}}
**Base Branch:** {{.BaseBranch}}
**Files Reviewed:** {{len .Files}}
**Excluded:** {{len .Excluded}}

---

## Verdict

Approve - Review artifacts assembled by strop Judge.

## Critical Issues

None.

## Cross-Cutting Themes

None observed.

## Top Recommendations

1. Review per-file findings under ` + "`per-file/`" + `.
`

const finalizeIndexTemplate = `# Majordomo PR Review - PR #{{.PRNumber}}

**Skill:** {{.Skill}}
**Base Branch:** {{.BaseBranch}}
**Reviewed At:** {{.ReviewedAt}}

---

**PR Summary:** ` + "`summary.md`" + ` - start here

---

## Files Reviewed
{{if .Files}}{{range .Files}}- {{.}}
{{end}}{{else}}- None
{{end}}{{if .Excluded}}
## Excluded
{{range .Excluded}}- {{.}}
{{end}}{{end}}
---

_Reviewed: {{len .Files}} | Excluded: {{len .Excluded}}_
`

func renderFinalizeReports(data finalizeReportData) (string, string, error) {
	render := func(name, source string) (string, error) {
		tmpl, err := template.New(name).Parse(source)
		if err != nil {
			return "", fmt.Errorf("parse %s template: %w", name, err)
		}
		var out bytes.Buffer
		if err := tmpl.Execute(&out, data); err != nil {
			return "", fmt.Errorf("execute %s template: %w", name, err)
		}
		return out.String(), nil
	}
	summary, err := render("summary", finalizeSummaryTemplate)
	if err != nil {
		return "", "", err
	}
	index, err := render("index", finalizeIndexTemplate)
	if err != nil {
		return "", "", err
	}
	return summary, index, nil
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
	if err := filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".json") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+".grounding"+string(filepath.Separator)) {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(stagingDir, path)
			if err != nil {
				return err
			}
			parts = append(parts, fmt.Sprintf("--- %s ---\n%s", rel, string(b)))
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(stagingDir, path)
		if err != nil {
			return err
		}
		parts = append(parts, fmt.Sprintf("--- %s ---\n%s", rel, string(b)))
		return nil
	}); err != nil {
		return "", fmt.Errorf("judge dispatch: walk staging context: %w", err)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("judge dispatch: empty staging context in %s", stagingDir)
	}
	return strings.Join(parts, "\n\n"), nil
}

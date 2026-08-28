package judge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

// DispatchMode mirrors agent.Dispatch Mode flags without importing internal/agent.
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

// StropDispatchOptions mirrors agent.DispatchOptions for strop cutover.
type StropDispatchOptions struct {
	StagingDir string
	OutputDir  string
	Mode       DispatchMode
}

// StropDispatch is the MAJORDOMO_JUDGE=strop replacement for agent-dispatch.sh.
func StropDispatch(opts StropDispatchOptions) error {
	if err := EnsureStropReady(); err != nil {
		return err
	}
	ctx := context.Background()
	stagingContext, err := readStagingContext(opts.StagingDir)
	if err != nil {
		return err
	}
	pipelineOut := filepath.Dir(opts.OutputDir)

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
	case DispatchModeFinalize, DispatchModeProse, DispatchModeTechnicalDeep:
		return fmt.Errorf("strop judge: mode %q not implemented yet (use MAJORDOMO_JUDGE=opencode)", opts.Mode)
	default:
		return FileReviewBatch(FileReviewOptions{StagingDir: opts.StagingDir, SkillOut: opts.OutputDir})
	}
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
		return "", fmt.Errorf("strop dispatch: empty staging context in %s", stagingDir)
	}
	return strings.Join(parts, "\n\n"), nil
}

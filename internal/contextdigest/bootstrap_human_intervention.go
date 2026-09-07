package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

const maxHumanInterventionAttempts = 3

// HumanInterventionInput is the post-refine evidence pack for flagging human decisions.
type HumanInterventionInput struct {
	RepoID             string
	ArchitectureMD     string
	RefinedCatalogYAML string
	JourneyMD          string
	ClusterProposalMD  string
	FindingsList       string
	ValidationFeedback string
}

// HumanInterventionOutput is operator-facing debt and priority callouts.
type HumanInterventionOutput struct {
	JourneyMD           string
	HumanInterventionMD string
	WeaknessesSeedMD    string
	PRPriorityMD        string
}

// HumanInterventionGenerator flags architecture findings for human leadership.
type HumanInterventionGenerator interface {
	Generate(ctx context.Context, input HumanInterventionInput) (HumanInterventionOutput, error)
}

// JudgeHumanInterventionGenerator uses typology_human_intervention with evaluate/retry.
type JudgeHumanInterventionGenerator struct {
	Gen judge.Generator
}

// Generate flags findings for humans without inventing catalog fixes.
func (g JudgeHumanInterventionGenerator) Generate(ctx context.Context, input HumanInterventionInput) (HumanInterventionOutput, error) {
	findings := extractArchitectureFindings(input.ArchitectureMD)
	if len(findings) == 0 {
		return HumanInterventionOutput{
			JourneyMD:           openJourneyNoFindings(input.JourneyMD),
			HumanInterventionMD: emptyHumanInterventionNote(),
			WeaknessesSeedMD:    "# Weaknesses\n\nNo open Typology architecture findings after refine.\n",
			PRPriorityMD:        "",
		}, nil
	}
	gen := g.Gen
	if gen == nil {
		if !judge.StoryLLMAvailable() {
			return HumanInterventionOutput{}, fmt.Errorf("LLM human intervention unavailable")
		}
		gen = packageJudgeGenerator{}
	}
	if strings.TrimSpace(input.FindingsList) == "" {
		input.FindingsList = formatFindingsList(findings)
	}
	feedback := strings.TrimSpace(input.ValidationFeedback)
	var lastErr error
	for attempt := 1; attempt <= maxHumanInterventionAttempts; attempt++ {
		fields := map[string]interface{}{
			"repo_id":              input.RepoID,
			"architecture_md":      input.ArchitectureMD,
			"refined_catalog_yaml": input.RefinedCatalogYAML,
			"journey_md":           input.JourneyMD,
			"cluster_proposal_md":  input.ClusterProposalMD,
			"findings_list":        input.FindingsList,
			"validation_feedback":  feedback,
		}
		out, err := gen.Generate(ctx, jmodules.TaskTypologyHumanIntervention, fields, attempt)
		if err != nil {
			return HumanInterventionOutput{}, err
		}
		res := HumanInterventionOutput{
			JourneyMD:           stringField(out, "journey_md"),
			HumanInterventionMD: stringField(out, "human_intervention_md"),
			WeaknessesSeedMD:    stringField(out, "weaknesses_seed_md"),
			PRPriorityMD:        stringField(out, "pr_priority_md"),
		}
		if err := validateHumanInterventionOutputs(findings, res.JourneyMD, res.HumanInterventionMD, res.PRPriorityMD); err != nil {
			lastErr = err
			if attempt == maxHumanInterventionAttempts {
				return HumanInterventionOutput{}, err
			}
			feedback = err.Error()
			continue
		}
		agg, err := gen.Evaluate(ctx, jmodules.TaskTypologyHumanIntervention, fields, map[string]interface{}{
			"journey_md":            res.JourneyMD,
			"human_intervention_md": res.HumanInterventionMD,
			"weaknesses_seed_md":    res.WeaknessesSeedMD,
			"pr_priority_md":        res.PRPriorityMD,
		}, attempt)
		if err != nil {
			lastErr = err
			if attempt == maxHumanInterventionAttempts {
				return HumanInterventionOutput{}, fmt.Errorf("human intervention LLM evaluation: %w", err)
			}
			feedback = err.Error()
			continue
		}
		if !judge.EvalPassed(agg) {
			lastErr = fmt.Errorf("%s", judge.EvalFeedback(agg))
			if attempt == maxHumanInterventionAttempts {
				return HumanInterventionOutput{}, fmt.Errorf("human intervention LLM evaluation failed after %d attempts:\n%s", maxHumanInterventionAttempts, judge.EvalFeedback(agg))
			}
			feedback = judge.EvalFeedback(agg)
			continue
		}
		return res, nil
	}
	if lastErr != nil {
		return HumanInterventionOutput{}, lastErr
	}
	return HumanInterventionOutput{}, fmt.Errorf("human intervention exhausted retries")
}

func openJourneyNoFindings(journey string) string {
	j := strings.TrimSpace(journey)
	if j == "" {
		return "# Journey\n\n## Status\n\nOpen. No post-refine architecture findings.\n\n## Technical debt and boundary violations\n\nNone.\n"
	}
	return j
}

func flagHumanIntervention(ctx context.Context, evidenceDir string, gen HumanInterventionGenerator, judgeGen judge.Generator) error {
	manifestPath := filepath.Join(evidenceDir, "manifest.yaml")
	manifest, err := contextstore.ParseTypologyManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.Mode == contextstore.TypologyModeFallback ||
		strings.EqualFold(manifest.RefineStatus, contextstore.TypologyRefineSkipped) {
		return nil
	}
	archPath := filepath.Join(evidenceDir, manifest.ArchitecturePath)
	archMD, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("human intervention read architecture: %w", err)
	}
	refinedPath := filepath.Join(evidenceDir, manifest.RefinedSnapshotPath)
	refinedYAML, err := os.ReadFile(refinedPath)
	if err != nil {
		return fmt.Errorf("human intervention read refined catalog: %w", err)
	}
	journeyPath := filepath.Join(evidenceDir, manifest.JourneyPath)
	journeyMD, err := os.ReadFile(journeyPath)
	if err != nil {
		return fmt.Errorf("human intervention read journey: %w", err)
	}
	clusterMD := ""
	if p := strings.TrimSpace(manifest.ClusterProposalPath); p != "" {
		if b, err := os.ReadFile(filepath.Join(evidenceDir, p)); err == nil {
			clusterMD = string(b)
		}
	}
	if gen == nil {
		gen = JudgeHumanInterventionGenerator{Gen: judgeGen}
	}
	out, err := gen.Generate(ctx, HumanInterventionInput{
		RepoID:             manifest.RepoID,
		ArchitectureMD:     string(archMD),
		RefinedCatalogYAML: string(refinedYAML),
		JourneyMD:          string(journeyMD),
		ClusterProposalMD:  clusterMD,
		FindingsList:       formatFindingsList(extractArchitectureFindings(string(archMD))),
	})
	if err != nil {
		return err
	}
	if err := writeText(journeyPath, out.JourneyMD); err != nil {
		return err
	}
	interventionRel := strings.TrimSpace(manifest.HumanInterventionPath)
	if interventionRel == "" {
		interventionRel = "human_intervention.md"
	}
	if err := writeText(filepath.Join(evidenceDir, interventionRel), out.HumanInterventionMD); err != nil {
		return err
	}
	priorityPath := filepath.Join(evidenceDir, "pr_priority.md")
	if p := strings.TrimSpace(out.PRPriorityMD); p != "" {
		if err := writeText(priorityPath, p); err != nil {
			return err
		}
	} else if err := os.Remove(priorityPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("human intervention clear pr_priority: %w", err)
	}
	// Seed weaknesses before bootstrap_story so the story starts from the same priorities.
	weak := strings.TrimSpace(out.WeaknessesSeedMD)
	if weak != "" {
		ctxDir := filepath.Dir(filepath.Dir(evidenceDir)) // evidence/typology -> context root
		if err := writeText(filepath.Join(ctxDir, "weaknesses.md"), weak); err != nil {
			return err
		}
	}
	manifest.HumanInterventionPath = interventionRel
	return writeTypologyManifest(evidenceDir, manifest)
}

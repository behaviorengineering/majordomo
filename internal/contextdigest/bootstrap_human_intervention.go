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

// FindingCommentBody is tutor counsel for one architecture finding on the context PR.
type FindingCommentBody struct {
	Finding     string `json:"finding"`
	Fingerprint string `json:"fingerprint"`
	Body        string `json:"body"`
}

// HumanInterventionOutput is operator-facing debt, priority callouts, and PR comment bodies.
type HumanInterventionOutput struct {
	JourneyMD           string
	HumanInterventionMD string
	WeaknessesSeedMD    string
	PRPriorityMD        string
	FindingComments     []FindingCommentBody
}

// HumanInterventionGenerator flags architecture findings for human leadership.
type HumanInterventionGenerator interface {
	Generate(ctx context.Context, input HumanInterventionInput) (HumanInterventionOutput, error)
}

// JudgeHumanInterventionGenerator runs focused intervention CoT tasks with evaluate/retry.
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

	baseFields := map[string]interface{}{
		"repo_id":              input.RepoID,
		"architecture_md":      input.ArchitectureMD,
		"refined_catalog_yaml": input.RefinedCatalogYAML,
		"journey_md":           input.JourneyMD,
		"cluster_proposal_md":  input.ClusterProposalMD,
		"findings_list":        input.FindingsList,
	}

	journey, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyInterventionJourney, baseFields, "journey_md",
		func(out map[string]interface{}) error {
			return validateJourneyFindings(findings, stringField(out, "journey_md"))
		})
	if err != nil {
		return HumanInterventionOutput{}, err
	}
	baseFields["journey_md"] = journey

	brief, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyInterventionBrief, baseFields, "human_intervention_md",
		func(out map[string]interface{}) error {
			return validateNamedFindingCoverage(findings, "human_intervention_md", stringField(out, "human_intervention_md"))
		})
	if err != nil {
		return HumanInterventionOutput{}, err
	}
	baseFields["human_intervention_md"] = brief

	weakFields := copyStringMap(baseFields)
	weaknesses, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyInterventionWeaknesses, weakFields, "weaknesses_seed_md",
		func(out map[string]interface{}) error {
			return validateNamedFindingCoverage(findings, "weaknesses_seed_md", stringField(out, "weaknesses_seed_md"))
		})
	if err != nil {
		return HumanInterventionOutput{}, err
	}

	prFields := copyStringMap(baseFields)
	prPriority, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyInterventionPRPriority, prFields, "pr_priority_md",
		func(out map[string]interface{}) error {
			return validateNamedFindingCoverage(findings, "pr_priority_md", stringField(out, "pr_priority_md"))
		})
	if err != nil {
		return HumanInterventionOutput{}, err
	}

	comments := make([]FindingCommentBody, 0, len(findings))
	for _, finding := range findings {
		commentFields := map[string]interface{}{
			"repo_id":               input.RepoID,
			"architecture_md":       input.ArchitectureMD,
			"refined_catalog_yaml":  input.RefinedCatalogYAML,
			"journey_md":            journey,
			"human_intervention_md": brief,
			"finding":               finding,
		}
		body, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyFindingComment, commentFields, "comment_md",
			func(out map[string]interface{}) error {
				return validateNamedFindingCoverage([]string{finding}, "comment_md", stringField(out, "comment_md"))
			})
		if err != nil {
			return HumanInterventionOutput{}, fmt.Errorf("finding comment %q: %w", findingMatchNeedle(finding), err)
		}
		comments = append(comments, FindingCommentBody{
			Finding:     finding,
			Fingerprint: findingFingerprint(finding),
			Body:        body,
		})
	}

	return HumanInterventionOutput{
		JourneyMD:           journey,
		HumanInterventionMD: brief,
		WeaknessesSeedMD:    weaknesses,
		PRPriorityMD:        prPriority,
		FindingComments:     comments,
	}, nil
}

func generateInterventionStep(
	ctx context.Context,
	gen judge.Generator,
	task string,
	fields map[string]interface{},
	outKey string,
	validate func(map[string]interface{}) error,
) (string, error) {
	feedback := ""
	if v, ok := fields["validation_feedback"].(string); ok {
		feedback = strings.TrimSpace(v)
	}
	var lastErr error
	for attempt := 1; attempt <= maxHumanInterventionAttempts; attempt++ {
		stepFields := copyStringMap(fields)
		stepFields["validation_feedback"] = feedback
		out, err := gen.Generate(ctx, task, stepFields, attempt)
		if err != nil {
			return "", fmt.Errorf("%s: %w", task, err)
		}
		if strings.TrimSpace(stringField(out, outKey)) == "" {
			lastErr = fmt.Errorf("%s: %s is required", task, outKey)
			feedback = lastErr.Error()
			continue
		}
		if validate != nil {
			if err := validate(out); err != nil {
				lastErr = err
				if attempt == maxHumanInterventionAttempts {
					return "", err
				}
				feedback = err.Error()
				continue
			}
		}
		agg, err := gen.Evaluate(ctx, task, stepFields, out, attempt)
		if err != nil {
			lastErr = err
			if attempt == maxHumanInterventionAttempts {
				return "", fmt.Errorf("%s LLM evaluation: %w", task, err)
			}
			feedback = err.Error()
			continue
		}
		if !judge.EvalPassed(agg) {
			lastErr = fmt.Errorf("%s", judge.EvalFeedback(agg))
			if attempt == maxHumanInterventionAttempts {
				return "", fmt.Errorf("%s LLM evaluation failed after %d attempts:\n%s", task, maxHumanInterventionAttempts, judge.EvalFeedback(agg))
			}
			feedback = judge.EvalFeedback(agg)
			continue
		}
		return strings.TrimSpace(stringField(out, outKey)), nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("%s exhausted retries", task)
}

func copyStringMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
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
	weak := strings.TrimSpace(out.WeaknessesSeedMD)
	if weak != "" {
		ctxDir := filepath.Dir(filepath.Dir(evidenceDir)) // evidence/typology -> context root
		if err := writeText(filepath.Join(ctxDir, "weaknesses.md"), weak); err != nil {
			return err
		}
	}
	if err := saveFindingCommentBodies(evidenceDir, out.FindingComments); err != nil {
		return err
	}
	manifest.HumanInterventionPath = interventionRel
	return writeTypologyManifest(evidenceDir, manifest)
}

package judge

import (
	"context"
	"fmt"
	"strings"

	stropdspy "github.com/behaviorengineering/strop/dspy"
	"github.com/behaviorengineering/strop/dspy/actor"
	"github.com/behaviorengineering/strop/dspy/factory"
	"github.com/behaviorengineering/strop/dspy/registry"
	dspyTracing "github.com/behaviorengineering/strop/dspy/tracing"
	"github.com/behaviorengineering/strop/dspy/workflow"
	"github.com/behaviorengineering/strop/evaluation"
	"github.com/behaviorengineering/strop/evaluation/criteria"

	bootstrappack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/bootstrap"
	digestpack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/digest"
	typologypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/typology"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

// MinEvalPassScore is the WeightedScore threshold for digest evaluation workflows.
const MinEvalPassScore = 8.0

type digestEvalSpec struct {
	evaluatorKey       evaluation.EvaluatorKey
	consolidatorKey    evaluation.ConsolidatorKey
	evaluatorLabel     string
	consolidatorLabel  string
	criterionIDs       []criteria.CriterionID
	focusAreas         string
	feedbackSuffix     string
	scoreSuffix        string
	consolidatorSuffix string
}

var digestEvalSpecs = map[string]digestEvalSpec{
	jmodules.TaskTypologyRefine: {
		evaluatorKey:      "typology_quality",
		consolidatorKey:   "typology_quality_consolidator",
		evaluatorLabel:    "Typology Quality Evaluator",
		consolidatorLabel: "Typology Quality Consolidator",
		criterionIDs:      typologypack.CriterionIDs,
		focusAreas:        "slice objectives, surface placement, adapter vs CLI, journey debt consistency",
		feedbackSuffix: `Score generator_output refined_catalog_yaml and journey_md against the typology rubrics.
Reject hollow objectives like "Provide X functionality".
Reject process-exec adapters (cliexec) marked kind: cli.
Reject journeys that claim merges are done while debt rows still say Merge into.`,
		scoreSuffix: `Prefer low scores when objectives are template language, surfaces misuse kind: cli for exec adapters,
or journey debt contradicts Status.`,
		consolidatorSuffix: "Merge typology refine feedback. Prefer concrete catalog and journey fixes over style notes.",
	},
	jmodules.TaskTypologyHumanIntervention: {
		evaluatorKey:      "typology_intervention_quality",
		consolidatorKey:   "typology_intervention_quality_consolidator",
		evaluatorLabel:    "Typology Intervention Evaluator",
		consolidatorLabel: "Typology Intervention Consolidator",
		criterionIDs:      typologypack.InterventionCriterionIDs,
		focusAreas:        "human-decision coverage of architecture findings without inventing bindings; tutor-voice cold-read PR summary",
		feedbackSuffix: `Score generator_output journey_md, human_intervention_md, weaknesses_seed_md, and pr_priority_md.
Reject missing findings from findings_list.
Reject Status complete while findings remain.
Reject invented sliceBindings, libraries membership, or ownership rewrites that pretend findings are resolved.
Reject pr_priority_md that is only imperative task titles or catalog jargon with no gloss (for example "Formalize Config Access: Resolve missing bindings").
Reject copy that assumes the reader already knows SliceBinding, package roles, or why the seed cannot invent approvals.`,
		scoreSuffix:        `Prefer low scores when priorities omit findings, invent architecture decisions, or fail a cold read.`,
		consolidatorSuffix: "Merge intervention feedback. Prefer complete human callouts and tutor-voice explanations over polish.",
	},
	jmodules.TaskBootstrapStory: {
		evaluatorKey:      "bootstrap_quality",
		consolidatorKey:   "bootstrap_quality_consolidator",
		evaluatorLabel:    "Bootstrap Story Evaluator",
		consolidatorLabel: "Bootstrap Story Consolidator",
		criterionIDs:      bootstrappack.CriterionIDs,
		focusAreas:        "evidence-backed seed story, markdown form, honest seed chronology",
		feedbackSuffix: `Score generator_output story markdown fields against bootstrap rubrics.
Reject invented history and claims not grounded in typology/README evidence.`,
		scoreSuffix:        `Lower scores for unsupported architecture claims or reconstructed history.`,
		consolidatorSuffix: "Merge bootstrap story feedback. Prefer evidence and honesty over polish.",
	},
	jmodules.TaskDigestStory: {
		evaluatorKey:      "digest_quality",
		consolidatorKey:   "digest_quality_consolidator",
		evaluatorLabel:    "Digest Story Evaluator",
		consolidatorLabel: "Digest Story Consolidator",
		criterionIDs:      digestpack.CriterionIDs,
		focusAreas:        "diff-evidenced section updates and preserved markdown shape",
		feedbackSuffix: `Score generator_output updated_text against digest rubrics.
Reject claims not supported by commit_diff or commit_subject.`,
		scoreSuffix:        `Lower scores for invented architecture or broken section structure.`,
		consolidatorSuffix: "Merge digest story feedback. Prefer evidence adherence over style.",
	},
}

type singleRoleInfo struct {
	evaluatorKey      evaluation.EvaluatorKey
	consolidatorKey   evaluation.ConsolidatorKey
	evaluatorLabel    string
	consolidatorLabel string
}

func (s singleRoleInfo) EvaluatorName(key evaluation.EvaluatorKey) string {
	if key == s.evaluatorKey {
		return s.evaluatorLabel
	}
	return key.String()
}

func (s singleRoleInfo) HasEvaluator(key evaluation.EvaluatorKey) bool {
	return key == s.evaluatorKey
}

func (s singleRoleInfo) EvaluatorWeight(key evaluation.EvaluatorKey) float64 {
	if key == s.evaluatorKey {
		return 1.0
	}
	return 0
}

func (s singleRoleInfo) ConsolidatorKey() evaluation.ConsolidatorKey { return s.consolidatorKey }

func (s singleRoleInfo) ConsolidatorName() string { return s.consolidatorLabel }

func registerDigestEvaluationWorkflow(
	ctx context.Context,
	reg *registry.ModuleRegistry,
	evalFactory *factory.EvaluatorFactory,
	task string,
	provider stropdspy.ProviderConfig,
) error {
	spec, ok := digestEvalSpecs[task]
	if !ok {
		return nil
	}
	roleInfo := singleRoleInfo{
		evaluatorKey:      spec.evaluatorKey,
		consolidatorKey:   spec.consolidatorKey,
		evaluatorLabel:    spec.evaluatorLabel,
		consolidatorLabel: spec.consolidatorLabel,
	}

	feedbackPrompt, err := criteria.NewEvaluatorFeedbackPromptBuilder(
		spec.evaluatorLabel,
		spec.focusAreas,
		spec.criterionIDs,
	).WithSuffix(spec.feedbackSuffix + "\n\nAlways output at least one line in the feedback field.").Build()
	if err != nil {
		return fmt.Errorf("judge runtime: %s feedback prompt: %w", task, err)
	}
	scorePrompt := criteria.NewEvaluatorScorePromptBuilder(
		spec.evaluatorLabel,
		spec.criterionIDs,
	).WithSuffix(spec.scoreSuffix).Build()

	chained := &stropdspy.ChainedEvaluatorConfig{
		Signature:    stropdspy.DefaultChainedEvaluatorSignature(),
		Persona:      stropdspy.DefaultEvaluatorPersona,
		CriterionIDs: spec.criterionIDs,
		RolePrompts: map[evaluation.EvaluatorKey]stropdspy.ChainedEvaluatorRolePrompts{
			spec.evaluatorKey: {
				FeedbackAnalysisPrompt: feedbackPrompt,
				ScoreGenerationPrompt:  scorePrompt,
			},
		},
	}

	modules, err := factory.CreateChainedEvaluatorsFromConfig(
		evalFactory,
		ctx,
		provider,
		map[evaluation.EvaluatorKey]stropdspy.ProviderConfig{spec.evaluatorKey: provider},
		chained,
		stropdspy.DefaultEvaluatorSignatureFormatter(),
		task+" evaluator",
	)
	if err != nil {
		return fmt.Errorf("judge runtime: %s evaluators: %w", task, err)
	}
	evaluators := registry.WrapEvaluators(modules, roleInfo)
	reg.RegisterEvaluators(task, evaluators)

	consolidatorPrompt := stropdspy.NewConsolidatorPromptBuilder().WithSuffix(spec.consolidatorSuffix).Build()
	consolidator, err := factory.CreateConsolidator(
		evalFactory,
		ctx,
		provider,
		spec.consolidatorKey,
		func(evaluation.ConsolidatorKey) string { return spec.consolidatorLabel },
		consolidatorPrompt,
		stropdspy.DefaultConsolidatorPersona,
		stropdspy.CreateDefaultConsolidatorModule,
		task+" consolidator",
	)
	if err != nil {
		return fmt.Errorf("judge runtime: %s consolidator: %w", task, err)
	}
	reg.RegisterConsolidator(task, spec.consolidatorKey, spec.consolidatorLabel, consolidator)

	fieldNames := workflow.FieldNames{
		Score:                "score",
		CriterionScores:      "criterion_scores",
		Feedback:             "feedback",
		Rationale:            stropdspy.FieldDirectivesAck,
		IterationVersion:     stropdspy.FieldIterationVersion,
		IndividualFeedbacks:  "individual_feedbacks",
		AgentScores:          "agent_scores",
		WeightedScore:        "weighted_score",
		ConsolidatedFeedback: "consolidated_feedback",
	}
	wf, err := workflow.NewParallelEvaluationWorkflow(workflow.WorkflowConfig{
		Evaluators: evaluators,
		RoleInfo:   roleInfo,
		Consolidator: actor.Consolidator{
			Key:    spec.consolidatorKey,
			Label:  spec.consolidatorLabel,
			Module: consolidator,
		},
		Logger:              nopLogger{},
		FieldNames:          fieldNames,
		SanitizeError:       stropdspy.SanitizeDSPyError,
		ChainSpanCtxKeyType: dspyTracing.ChainSpanCtxKeyType(),
		RoleToCriterionIDs: map[evaluation.EvaluatorKey][]criteria.CriterionID{
			spec.evaluatorKey: spec.criterionIDs,
		},
	})
	if err != nil {
		return fmt.Errorf("judge runtime: %s workflow: %w", task, err)
	}
	reg.RegisterWorkflow(task, wf)
	return nil
}

// EvalPassed reports whether an AggregatedEvaluation clears the digest pass threshold.
func EvalPassed(eval *evaluation.AggregatedEvaluation) bool {
	return eval != nil && eval.WeightedScore >= MinEvalPassScore
}

// EvalFeedback returns consolidated evaluator feedback for retry prompts.
func EvalFeedback(eval *evaluation.AggregatedEvaluation) string {
	if eval == nil {
		return "evaluation returned no result"
	}
	if fb := strings.TrimSpace(eval.ConsolidatedFeedback); fb != "" {
		return fb
	}
	return fmt.Sprintf("weighted_score=%.2f (below pass threshold %.1f)", eval.WeightedScore, MinEvalPassScore)
}

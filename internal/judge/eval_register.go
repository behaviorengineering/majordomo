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
	jmodules.TaskTypologyCluster: {
		evaluatorKey:      "typology_cluster_quality",
		consolidatorKey:   "typology_cluster_quality_consolidator",
		evaluatorLabel:    "Typology Cluster Evaluator",
		consolidatorLabel: "Typology Cluster Consolidator",
		criterionIDs:      typologypack.ClusterCriterionIDs,
		focusAreas:        "consultant counsel; observed package_roles before any merge overlay",
		feedbackSuffix: `Score generator_output cluster_proposal_md against the cluster counsel and delivery rubrics.
Reject proposals that contradict package_roles.
Reject ignoring the mechanical_grouping_md seed.
Reject using folder names (dashboard, board, cli, server) as classification evidence.
Reject merge inventories that do not argue why this grouping, what was rejected, and the lean.
Reject "merge dto into aggregator as the UI" or "forge depends on UI" invented from parking board under dashboard.
Reject folding server into CLI for sole importer.
Reject treating exec_runner as CLI domain furniture.
Reject corporate "we reorganized the repository" framing.`,
		scoreSuffix:        `Prefer low scores when the proposal invents ownership from names, contradicts package_roles, or sounds like consented product work.`,
		consolidatorSuffix: "Merge cluster feedback. Prefer counsel that helps a human decide over polish.",
	},
	jmodules.TaskTypologyRefine: {
		evaluatorKey:      "typology_quality",
		consolidatorKey:   "typology_quality_consolidator",
		evaluatorLabel:    "Typology Quality Evaluator",
		consolidatorLabel: "Typology Quality Consolidator",
		criterionIDs:      typologypack.CriterionIDs,
		focusAreas:        "slice objectives, surface placement, adapter vs CLI, journey debt consistency and counsel",
		feedbackSuffix: `Score generator_output refined_catalog_yaml and journey_md against the typology rubrics.
Reject hollow objectives like "Provide X functionality".
Reject process-exec adapters (cliexec) marked kind: cli.
Reject folding server / goEmbed / NewMux packages into the CLI surface solely because cmd is the sole importer.
Reject journeys that claim merges are done while debt rows still say Merge into.
Reject journey decisions that omit what was rejected and why.
Reject open debt rows that only say "Approve binding or refactor" without smell, alternatives, and a lean.
Reject journey prose that claims the product team already reorganized the repo ("we successfully consolidated") instead of Majordomo/Typology proposals on the context branch.`,
		scoreSuffix: `Prefer low scores when objectives are template language, surfaces misuse kind: cli for exec adapters,
HTTP/embed packages are CLI furniture, journey debt contradicts Status, journey counsel is hollow, or speaker attribution sounds like shipped product work.`,
		consolidatorSuffix: "Merge typology refine feedback. Prefer concrete catalog fixes and argued journey debt over style notes.",
	},
	jmodules.TaskTypologyHumanIntervention: {
		evaluatorKey:      "typology_intervention_quality",
		consolidatorKey:   "typology_intervention_quality_consolidator",
		evaluatorLabel:    "Typology Intervention Evaluator",
		consolidatorLabel: "Typology Intervention Consolidator",
		criterionIDs:      typologypack.InterventionCriterionIDs,
		focusAreas:        "human-decision coverage without inventing bindings; tutor-voice cold-read PR summary with consultant counsel",
		feedbackSuffix: `Score generator_output journey_md, human_intervention_md, weaknesses_seed_md, and pr_priority_md.
Reject missing findings from findings_list.
Reject Status complete while findings remain.
Reject invented sliceBindings, libraries membership, or ownership rewrites that pretend findings are resolved.
Reject asking humans to rubber-stamp mechanical slice-to-library bindings Majordomo should have written into the proposal catalog.
Reject pr_priority_md that is only imperative task titles or catalog jargon with no gloss (for example "Formalize Config Access: Resolve missing bindings").
Reject copy that assumes the reader already knows SliceBinding, package roles, or why the seed cannot invent approvals.
Reject import inventory without smell, alternatives, and a lean.
Reject "approve or refactor" as the whole advice.
Reject "see journey_md" or other deferral to another file for the real argument.
Reject a gloss or explanation that has no recommended lean.
Reject corporate "we" claims that sound like a consented product-repo reorganization (for example "we successfully reorganized", "we consolidated", "we are proceeding").
Reject copy that treats Typology catalog merges as already-landed product work instead of Majordomo context-branch proposals.`,
		scoreSuffix:        `Prefer low scores when priorities omit findings, invent architecture decisions, ask for mechanical library-binding stamps, fail a cold read, argue without a lean, or imply humans already shipped the reorganization.`,
		consolidatorSuffix: "Merge intervention feedback. Prefer complete human callouts with counsel over polish.",
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

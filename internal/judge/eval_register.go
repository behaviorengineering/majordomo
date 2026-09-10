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
		criterionIDs:      typologypack.InterventionBriefCriterionIDs,
		focusAreas:        "operator briefing coverage without inventing bindings; tutor-voice counsel",
		feedbackSuffix: `Score generator_output human_intervention_md.
Reject missing findings from findings_list.
Reject invented sliceBindings or ownership rewrites.
Reject asking humans to rubber-stamp mechanical slice-to-library bindings.
Reject import inventory without smell, alternatives, and a lean.
Reject "approve or refactor" as the whole advice.
Reject "see journey_md".
Reject corporate "we reorganized" framing.`,
		scoreSuffix:        `Prefer low scores when the briefing omits findings, invents architecture, or argues without a lean.`,
		consolidatorSuffix: "Merge brief feedback. Prefer complete human callouts with counsel over polish.",
	},
	jmodules.TaskTypologyInterventionJourney: {
		evaluatorKey:      "typology_intervention_journey_quality",
		consolidatorKey:   "typology_intervention_journey_consolidator",
		evaluatorLabel:    "Typology Intervention Journey Evaluator",
		consolidatorLabel: "Typology Intervention Journey Consolidator",
		criterionIDs:      typologypack.InterventionJourneyCriterionIDs,
		focusAreas:        "open Status and debt coverage for every finding",
		feedbackSuffix: `Score generator_output journey_md.
Reject Status complete while findings remain.
Reject missing findings from the debt table.
Reject hollow "Approve binding or refactor" debt rows.
Reject corporate "we reorganized" framing.`,
		scoreSuffix:        `Prefer low scores when journey hides findings or debt lacks counsel.`,
		consolidatorSuffix: "Merge journey feedback. Prefer open Status and argued debt.",
	},
	jmodules.TaskTypologyInterventionBrief: {
		evaluatorKey:      "typology_intervention_brief_quality",
		consolidatorKey:   "typology_intervention_brief_consolidator",
		evaluatorLabel:    "Typology Intervention Brief Evaluator",
		consolidatorLabel: "Typology Intervention Brief Consolidator",
		criterionIDs:      typologypack.InterventionBriefCriterionIDs,
		focusAreas:        "operator briefing coverage without inventing bindings; tutor-voice counsel",
		feedbackSuffix: `Score generator_output human_intervention_md.
Reject missing findings from findings_list.
Reject invented sliceBindings or ownership rewrites.
Reject asking humans to rubber-stamp mechanical slice-to-library bindings.
Reject import inventory without smell, alternatives, and a lean.
Reject "see journey_md".
Reject corporate "we reorganized" framing.`,
		scoreSuffix:        `Prefer low scores when the briefing omits findings, invents architecture, or argues without a lean.`,
		consolidatorSuffix: "Merge brief feedback. Prefer complete human callouts with counsel over polish.",
	},
	jmodules.TaskTypologyInterventionWeaknesses: {
		evaluatorKey:      "typology_intervention_weaknesses_quality",
		consolidatorKey:   "typology_intervention_weaknesses_consolidator",
		evaluatorLabel:    "Typology Intervention Weaknesses Evaluator",
		consolidatorLabel: "Typology Intervention Weaknesses Consolidator",
		criterionIDs:      []criteria.CriterionID{typologypack.CriterionIDInterventionCoverage, typologypack.CriterionIDInterventionCounsel},
		focusAreas:        "weaknesses seed covers every finding with a lean",
		feedbackSuffix: `Score generator_output weaknesses_seed_md.
Reject missing findings from findings_list.
Reject hollow approve-or-refactor bullets without a lean.`,
		scoreSuffix:        `Prefer low scores when weaknesses omit findings or leans.`,
		consolidatorSuffix: "Merge weaknesses feedback.",
	},
	jmodules.TaskTypologyInterventionPRPriority: {
		evaluatorKey:      "typology_intervention_pr_priority_quality",
		consolidatorKey:   "typology_intervention_pr_priority_consolidator",
		evaluatorLabel:    "Typology Intervention PR Priority Evaluator",
		consolidatorLabel: "Typology Intervention PR Priority Consolidator",
		criterionIDs:      typologypack.InterventionPRPriorityCriterionIDs,
		focusAreas:        "cold-read PR summary with tutor voice and counsel",
		feedbackSuffix: `Score generator_output pr_priority_md.
Reject missing findings from findings_list.
Reject imperative task titles or jargon with no gloss.
Reject "see journey_md".
Reject corporate "we reorganized" framing.
Reject counsel without a lean.`,
		scoreSuffix:        `Prefer low scores when the PR summary fails a cold read or omits findings.`,
		consolidatorSuffix: "Merge PR priority feedback.",
	},
	jmodules.TaskTypologyFindingComment: {
		evaluatorKey:      "typology_finding_comment_quality",
		consolidatorKey:   "typology_finding_comment_consolidator",
		evaluatorLabel:    "Typology Finding Comment Evaluator",
		consolidatorLabel: "Typology Finding Comment Consolidator",
		criterionIDs:      typologypack.FindingCommentCriterionIDs,
		focusAreas:        "single-finding PR comment tutor counsel",
		feedbackSuffix: `Score generator_output comment_md for the single finding input.
Reject inventing catalog YAML or mechanical binding rubber-stamps.
Reject "see journey_md".
Reject counsel without smell, alternatives, and a lean.
Reject corporate "we reorganized" framing.`,
		scoreSuffix:        `Prefer low scores when the comment fails tutor counsel for that finding.`,
		consolidatorSuffix: "Merge finding-comment feedback.",
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

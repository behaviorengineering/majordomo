package modules

import (
	"github.com/XiaoConstantine/dspy-go/pkg/core"
	dspymodules "github.com/behaviorengineering/strop/pkg/dspy/modules"
)

// FileReviewModule builds the filereview generator signature.
func FileReviewModule() core.Module { return fileReviewModule() }

// DigestStoryModule builds the digest story section generator.
func DigestStoryModule() core.Module { return digestStoryModule() }

// BootstrapStoryModule builds the bootstrap story generator.
func BootstrapStoryModule() core.Module { return bootstrapStoryModule() }

// TypologyClusterModule builds the unattended typology slice-grouping generator.
func TypologyClusterModule() core.Module { return typologyClusterModule() }

// TypologyRefineModule builds the unattended typology slice-catalog generator.
func TypologyRefineModule() core.Module { return typologyRefineModule() }

// TypologyInspectModule builds the low-confidence package role inspector.
func TypologyInspectModule() core.Module { return typologyInspectModule() }

// TypologyHumanInterventionModule is the legacy alias for the operator brief module.
func TypologyHumanInterventionModule() core.Module { return typologyHumanInterventionModule() }

// TypologyInterventionJourneyModule rewrites journey status and debt for open findings.
func TypologyInterventionJourneyModule() core.Module { return typologyInterventionJourneyModule() }

// TypologyInterventionBriefModule writes human_intervention.md.
func TypologyInterventionBriefModule() core.Module { return typologyInterventionBriefModule() }

// TypologyInterventionWeaknessesModule seeds weaknesses.md.
func TypologyInterventionWeaknessesModule() core.Module {
	return typologyInterventionWeaknessesModule()
}

// TypologyInterventionPRPriorityModule writes pr_priority.md for the PR body.
func TypologyInterventionPRPriorityModule() core.Module {
	return typologyInterventionPRPriorityModule()
}

// TypologyFindingCommentModule writes one PR comment body for a single finding.
func TypologyFindingCommentModule() core.Module { return typologyFindingCommentModule() }

// SummaryModule builds the summary generator.
func SummaryModule() core.Module { return summaryModule() }

// TechnicalModule builds the technical review generator.
func TechnicalModule() core.Module { return technicalModule() }

// DigestGenerators returns constructors for context-digest / typology tasks.
// Private factories (majordomo-context) should pass this map as
// judge.RuntimeOptions.Generators until they own the prompt packages themselves.
func DigestGenerators() map[string]func() core.Module {
	return map[string]func() core.Module{
		TaskTypologyInspect:                TypologyInspectModule,
		TaskTypologySliceGrouping:          TypologyClusterModule,
		TaskTypologySliceCatalog:           TypologyRefineModule,
		TaskTypologyHumanIntervention:      TypologyHumanInterventionModule,
		TaskTypologyInterventionJourney:    TypologyInterventionJourneyModule,
		TaskTypologyInterventionBrief:      TypologyInterventionBriefModule,
		TaskTypologyInterventionWeaknesses: TypologyInterventionWeaknessesModule,
		TaskTypologyInterventionPRPriority: TypologyInterventionPRPriorityModule,
		TaskTypologyFindingComment:         TypologyFindingCommentModule,
		TaskBootstrapStory:                 BootstrapStoryModule,
		TaskDigestStory:                    DigestStoryModule,
	}
}

var _ core.Module = (*dspymodules.DirectivesCoT)(nil)

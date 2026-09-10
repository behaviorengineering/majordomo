package modules

import (
	"github.com/XiaoConstantine/dspy-go/pkg/core"
	dspymodules "github.com/behaviorengineering/strop/dspy/modules"
)

// FileReviewModule builds the filereview generator signature.
func FileReviewModule() core.Module { return fileReviewModule() }

// DigestStoryModule builds the digest story section generator.
func DigestStoryModule() core.Module { return digestStoryModule() }

// BootstrapStoryModule builds the bootstrap story generator.
func BootstrapStoryModule() core.Module { return bootstrapStoryModule() }

// TypologyClusterModule builds the unattended typology cluster-pass generator.
func TypologyClusterModule() core.Module { return typologyClusterModule() }

// TypologyRefineModule builds the unattended typology refine generator.
func TypologyRefineModule() core.Module { return typologyRefineModule() }

// TypologyInspectModule builds the low-confidence package role inspector.
func TypologyInspectModule() core.Module { return typologyInspectModule() }

// TypologyHumanInterventionModule builds the post-refine human-decision flagger.
func TypologyHumanInterventionModule() core.Module { return typologyHumanInterventionModule() }

// SummaryModule builds the summary generator.
func SummaryModule() core.Module { return summaryModule() }

// TechnicalModule builds the technical review generator.
func TechnicalModule() core.Module { return technicalModule() }

var _ core.Module = (*dspymodules.DirectivesCoT)(nil)

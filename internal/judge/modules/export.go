package modules

import (
	"github.com/XiaoConstantine/dspy-go/pkg/core"
	dspymodules "github.com/behaviorengineering/strop/dspy/modules"
)

// FileReviewModule builds the filereview generator signature.
func FileReviewModule() core.Module { return fileReviewModule() }

// DigestStoryModule builds the digest story section generator.
func DigestStoryModule() core.Module { return digestStoryModule() }

// SummaryModule builds the summary generator.
func SummaryModule() core.Module { return summaryModule() }

// TechnicalModule builds the technical review generator.
func TechnicalModule() core.Module { return technicalModule() }

var _ core.Module = (*dspymodules.DirectivesCoT)(nil)

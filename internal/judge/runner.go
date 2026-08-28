package judge

import (
	"github.com/behaviorengineering/strop/dspy/registry"
	"github.com/behaviorengineering/strop/dspy/runner"
	stroplog "github.com/behaviorengineering/strop/log"
	"github.com/behaviorengineering/strop/evaluation/criteria"

	digestpack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/digest"
	summarypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/summary"
	techpack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/tech"
)

// RegisterPacks installs Majordomo summary, tech, and digest rubrics onto the shared strop registry.
func RegisterPacks(r *criteria.CriterionRegistry) {
	summarypack.Register(r)
	techpack.Register(r)
	digestpack.Register(r)
}

// NewJobRunner builds a strop JobRunner. Call RegisterPacks before EvaluateWorkflow
// uses product rubrics. learning and formatter may be nil.
func NewJobRunner(
	reg *registry.ModuleRegistry,
	learning runner.LearningServiceForGeneration,
	formatter runner.ExampleFormatter,
	logger stroplog.Logger,
) *runner.JobRunner {
	if logger == nil {
		logger = nopLogger{}
	}
	return runner.NewJobRunner(reg, learning, formatter, logger)
}

type nopLogger struct{}

func (nopLogger) WithField(string, interface{}) stroplog.Logger { return nopLogger{} }
func (nopLogger) WithFields(map[string]interface{}) stroplog.Logger {
	return nopLogger{}
}
func (nopLogger) WithError(error) stroplog.Logger { return nopLogger{} }
func (nopLogger) Debug(...interface{})            {}
func (nopLogger) Info(...interface{})             {}
func (nopLogger) Warn(...interface{})             {}
func (nopLogger) Error(...interface{})            {}

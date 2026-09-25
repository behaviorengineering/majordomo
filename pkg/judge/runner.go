package judge

import (
	"github.com/behaviorengineering/strop/pkg/dspy/registry"
	"github.com/behaviorengineering/strop/pkg/dspy/runner"
	"github.com/behaviorengineering/strop/pkg/evaluation/criteria"
	stroplog "github.com/behaviorengineering/strop/pkg/log"

	summarypack "github.com/behaviorengineering/majordomo/pkg/judge/evaluation/summary"
	techpack "github.com/behaviorengineering/majordomo/pkg/judge/evaluation/tech"
)

// RegisterPacks installs Majordomo review rubrics (summary + tech).
// Digest/bootstrap/typology rubrics live in majordomo-context.
func RegisterPacks(r *criteria.CriterionRegistry) {
	summarypack.Register(r)
	techpack.Register(r)
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

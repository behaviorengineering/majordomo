package judge

import (
	"context"
	"sync"

	"github.com/behaviorengineering/strop/pkg/dspy/runner"
	"github.com/behaviorengineering/strop/pkg/evaluation"

	"github.com/behaviorengineering/majordomo/pkg/platform/aigateway"
)

func ensureRegistry() (*runner.JobRunner, error) {
	rt, err := DefaultRuntime()
	if err != nil {
		return nil, err
	}
	return rt.runner, nil
}

// StropReady reports whether strop generator modules are registered and an LLM key is configured.
func StropReady() bool {
	_, err := ensureRegistry()
	return err == nil
}

// EnsureStropReady fails when generator modules cannot be registered (usually missing LLM keys).
func EnsureStropReady() error {
	_, err := ensureRegistry()
	if err != nil {
		return ErrNotReady
	}
	return nil
}

// SharedRunner returns the process-wide strop JobRunner after lazy init.
func SharedRunner() (*runner.JobRunner, error) {
	return ensureRegistry()
}

// Generate runs one registered generator task on the process-wide runtime.
func Generate(ctx context.Context, task string, fields map[string]interface{}, version int) (map[string]interface{}, error) {
	rt, err := DefaultRuntime()
	if err != nil {
		return nil, err
	}
	return rt.Generate(ctx, task, fields, version)
}

// Evaluate runs one registered evaluation workflow on the process-wide runtime.
func Evaluate(
	ctx context.Context,
	task string,
	inputFields, outputFields map[string]interface{},
	version int,
) (*evaluation.AggregatedEvaluation, error) {
	rt, err := DefaultRuntime()
	if err != nil {
		return nil, err
	}
	return rt.Evaluate(ctx, task, inputFields, outputFields, version)
}

// StoryLLMAvailable is true when a Judge runtime can call strop (review or factory).
func StoryLLMAvailable() bool {
	return StropReady()
}

// ResetRegistryForTests clears lazy-init state (tests only).
func ResetRegistryForTests() {
	defaultRuntimeOnce = sync.Once{}
	defaultRuntimeMu.Lock()
	defaultRuntime = nil
	defaultRuntimeErr = nil
	defaultRuntimeMu.Unlock()
	aigateway.ResetForTests()
}

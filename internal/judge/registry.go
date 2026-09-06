package judge

import (
	"context"
	"sync"

	"github.com/behaviorengineering/strop/dspy/registry"
	"github.com/behaviorengineering/strop/dspy/runner"
	"github.com/behaviorengineering/strop/evaluation"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
)

var (
	registryOnce sync.Once
	registryErr  error
	sharedReg    *registry.ModuleRegistry
	sharedRunner *runner.JobRunner
)

func ensureRegistry() (*registry.ModuleRegistry, *runner.JobRunner, error) {
	rt, err := DefaultRuntime()
	if err != nil {
		registryErr = err
		return nil, nil, err
	}
	return rt.reg, rt.runner, nil
}

// StropReady reports whether strop generator modules are registered and an LLM key is configured.
func StropReady() bool {
	_, _, err := ensureRegistry()
	return err == nil
}

// EnsureStropReady fails when generator modules cannot be registered (usually missing LLM keys).
func EnsureStropReady() error {
	_, _, err := ensureRegistry()
	if err != nil {
		return ErrNotReady
	}
	return nil
}

// SharedRunner returns the process-wide strop JobRunner after lazy init.
func SharedRunner() (*runner.JobRunner, error) {
	_, jr, err := ensureRegistry()
	return jr, err
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

// StoryLLMAvailable is true when digest story generation can call strop.
func StoryLLMAvailable() bool {
	return StropReady()
}

// ResetRegistryForTests clears lazy-init state (tests only).
func ResetRegistryForTests() {
	registryOnce = sync.Once{}
	registryErr = nil
	sharedReg = nil
	sharedRunner = nil
	defaultRuntimeOnce = sync.Once{}
	defaultRuntimeMu.Lock()
	defaultRuntime = nil
	defaultRuntimeErr = nil
	defaultRuntimeMu.Unlock()
	aigateway.ResetForTests()
}

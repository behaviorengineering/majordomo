package judge

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/behaviorengineering/strop/dspy/factory"
	"github.com/behaviorengineering/strop/dspy/registry"
	"github.com/behaviorengineering/strop/dspy/runner"
	dspyTracing "github.com/behaviorengineering/strop/dspy/tracing"
	"github.com/behaviorengineering/strop/runreport"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
	"github.com/behaviorengineering/majordomo/internal/observability"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

var (
	registryOnce sync.Once
	registryErr  error
	sharedReg    *registry.ModuleRegistry
	sharedRunner *runner.JobRunner
)

func ensureRegistry() (*registry.ModuleRegistry, *runner.JobRunner, error) {
	registryOnce.Do(func() {
		provider, err := ResolveProvider()
		if err != nil {
			registryErr = err
			return
		}
		reg := registry.NewModuleRegistry()
		llmFactory := factory.NewLLMFactory(func(modelID string, providerType string) {
			reg.RegisterModelProvider(modelID, providerType)
		}, defaultModuleTimeout)
		llmFactory.SetInstrumentHTTP(observability.InstrumentHTTPClient)

		otelOn := true
		if v := os.Getenv("MAJORDOMO_OTEL_ENABLED"); v == "0" {
			otelOn = false
		}
		svc := os.Getenv("MAJORDOMO_OTEL_SERVICE_NAME")
		if svc == "" {
			svc = observability.DefaultServiceName
		}
		interceptorSetup := factory.NewInterceptorSetup(
			otelOn, svc, nil, defaultModuleTimeout,
			dspyTracing.OpenInferenceModuleInterceptor,
			nil,
			reg.GetModelProvider,
			reg.GetModuleModel,
			func(moduleName, modelID string) { reg.RegisterModuleModel(moduleName, modelID) },
			nil,
			runreport.Config{},
		)
		configurator := factory.NewModuleConfigurator(llmFactory, interceptorSetup, nil)
		genFactory := factory.NewGeneratorFactory(configurator)
		ctx := context.Background()

		type entry struct {
			task string
			new  func() core.Module
		}
		for _, e := range []entry{
			{jmodules.TaskFileReview, jmodules.FileReviewModule},
			{jmodules.TaskDigestStory, jmodules.DigestStoryModule},
			{jmodules.TaskSummary, jmodules.SummaryModule},
			{jmodules.TaskTechnical, jmodules.TechnicalModule},
		} {
			mod, err := genFactory.CreateGenerator(ctx, provider, func() (core.Module, error) {
				return e.new(), nil
			}, e.task, nil)
			if err != nil {
				registryErr = fmt.Errorf("register %s: %w", e.task, err)
				return
			}
			reg.RegisterGenerator(e.task, mod)
		}

		sharedReg = reg
		sharedRunner = NewJobRunner(reg, nil, nil, nil)
	})
	if registryErr != nil {
		return nil, nil, registryErr
	}
	return sharedReg, sharedRunner, nil
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

// Generate runs one registered generator task.
func Generate(ctx context.Context, task string, fields map[string]interface{}, version int) (map[string]interface{}, error) {
	jr, err := SharedRunner()
	if err != nil {
		return nil, err
	}
	cfg := runner.GenerationConfig{
		ModuleName:   task,
		JobName:      task,
		StepName:     task,
		ErrorMessage: task,
	}
	return jr.Generate(ctx, cfg, newMapInput(fields, version), nil)
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
	aigateway.ResetForTests()
}

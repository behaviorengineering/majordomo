package judge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/behaviorengineering/strop/dspy/factory"
	"github.com/behaviorengineering/strop/dspy/registry"
	"github.com/behaviorengineering/strop/dspy/runner"
	dspyTracing "github.com/behaviorengineering/strop/dspy/tracing"
	"github.com/behaviorengineering/strop/evaluation"
	"github.com/behaviorengineering/strop/evaluation/criteria"
	"github.com/behaviorengineering/strop/runreport"
	"github.com/behaviorengineering/strop/streaming"

	stropdspy "github.com/behaviorengineering/strop/dspy"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/llmusage"
	"github.com/behaviorengineering/majordomo/internal/observability"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

// errGatewayUnavailable is returned when a task needs the embedded Bifrost
// gateway but no Anthropic/OpenAI/Gemini key is configured.
var errGatewayUnavailable = errors.New("embedded gateway unavailable")

// DigestTasks are the generator modules used by context digest / bootstrap.
func DigestTasks() []string {
	return []string{
		jmodules.TaskTypologyInspect,
		jmodules.TaskTypologyCluster,
		jmodules.TaskTypologyRefine,
		jmodules.TaskTypologyInterventionJourney,
		jmodules.TaskTypologyInterventionBrief,
		jmodules.TaskTypologyInterventionWeaknesses,
		jmodules.TaskTypologyInterventionPRPriority,
		jmodules.TaskTypologyFindingComment,
		jmodules.TaskBootstrapStory,
		jmodules.TaskDigestStory,
	}
}

// ReviewTasks are the generator modules used by PR review orchestration.
func ReviewTasks() []string {
	return []string{jmodules.TaskFileReview, jmodules.TaskSummary, jmodules.TaskTechnical}
}

// AllGeneratorTasks is the full set of known generator task names.
func AllGeneratorTasks() []string {
	return []string{
		jmodules.TaskFileReview,
		jmodules.TaskTypologyInspect,
		jmodules.TaskTypologyCluster,
		jmodules.TaskTypologyRefine,
		jmodules.TaskTypologyHumanIntervention,
		jmodules.TaskTypologyInterventionJourney,
		jmodules.TaskTypologyInterventionBrief,
		jmodules.TaskTypologyInterventionWeaknesses,
		jmodules.TaskTypologyInterventionPRPriority,
		jmodules.TaskTypologyFindingComment,
		jmodules.TaskBootstrapStory,
		jmodules.TaskDigestStory,
		jmodules.TaskSummary,
		jmodules.TaskTechnical,
	}
}

// Generator is the narrow generation and evaluation surface used by digest workers.
type Generator interface {
	Generate(ctx context.Context, task string, fields map[string]interface{}, version int) (map[string]interface{}, error)
	Evaluate(ctx context.Context, task string, inputFields, outputFields map[string]interface{}, version int) (*evaluation.AggregatedEvaluation, error)
	Ready() bool
	TaskModel(task string) string
}

// Runtime owns a module registry and JobRunner configured per task.
type Runtime struct {
	reg    *registry.ModuleRegistry
	runner *runner.JobRunner
	models map[string]string
}

var (
	defaultRuntimeOnce sync.Once
	defaultRuntime     *Runtime
	defaultRuntimeErr  error
	defaultRuntimeMu   sync.RWMutex
)

// RuntimeOptions configures NewRuntime.
type RuntimeOptions struct {
	// Tasks limits registration; empty means all known generator tasks.
	Tasks []string
	// FallbackModel overrides MAJORDOMO_MODEL for embedded-gateway fallback.
	FallbackModel string
}

// NewRuntime builds a Judge runtime from central-config AI providers and job_configs.
// Tasks without an explicit provider fall back to the embedded gateway.
//
// When opts.Tasks is empty, tasks that need the gateway but have no LLM keys are
// skipped so a Polypus-only digest config does not fail on unused review modules.
// When opts.Tasks is non-empty, every listed task must resolve (fail-closed).
func NewRuntime(ctx context.Context, cfg config.RepoConfig, opts RuntimeOptions) (*Runtime, error) {
	strict := len(opts.Tasks) > 0
	tasks := opts.Tasks
	if !strict {
		tasks = AllGeneratorTasks()
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
	retryConfig := DefaultModuleRetryConfig()
	interceptorSetup := factory.NewInterceptorSetup(
		otelOn, svc, &retryConfig, defaultModuleTimeout,
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
	evalFactory := factory.NewEvaluatorFactory(configurator)

	RegisterPacks(criteria.DefaultRegistry())

	rt := &Runtime{
		reg:    reg,
		models: make(map[string]string, len(tasks)),
	}

	ctors := map[string]func() core.Module{
		jmodules.TaskFileReview:                     jmodules.FileReviewModule,
		jmodules.TaskTypologyInspect:                jmodules.TypologyInspectModule,
		jmodules.TaskTypologyCluster:                jmodules.TypologyClusterModule,
		jmodules.TaskTypologyRefine:                 jmodules.TypologyRefineModule,
		jmodules.TaskTypologyHumanIntervention:      jmodules.TypologyHumanInterventionModule,
		jmodules.TaskTypologyInterventionJourney:    jmodules.TypologyInterventionJourneyModule,
		jmodules.TaskTypologyInterventionBrief:      jmodules.TypologyInterventionBriefModule,
		jmodules.TaskTypologyInterventionWeaknesses: jmodules.TypologyInterventionWeaknessesModule,
		jmodules.TaskTypologyInterventionPRPriority: jmodules.TypologyInterventionPRPriorityModule,
		jmodules.TaskTypologyFindingComment:         jmodules.TypologyFindingCommentModule,
		jmodules.TaskBootstrapStory:                 jmodules.BootstrapStoryModule,
		jmodules.TaskDigestStory:                    jmodules.DigestStoryModule,
		jmodules.TaskSummary:                        jmodules.SummaryModule,
		jmodules.TaskTechnical:                      jmodules.TechnicalModule,
	}

	providers := make(map[string]stropdspy.ProviderConfig, len(tasks))
	for _, task := range tasks {
		ctor, ok := ctors[task]
		if !ok {
			return nil, fmt.Errorf("judge runtime: unknown task %q", task)
		}
		provider, err := resolveTaskProviderConfig(cfg, task, opts.FallbackModel)
		if err != nil {
			if !strict && errors.Is(err, errGatewayUnavailable) {
				continue
			}
			return nil, fmt.Errorf("judge runtime: resolve %s: %w", task, err)
		}
		mod, err := genFactory.CreateGenerator(ctx, provider, func() (core.Module, error) {
			return ctor(), nil
		}, task, nil)
		if err != nil {
			return nil, fmt.Errorf("judge runtime: register %s: %w", task, err)
		}
		reg.RegisterGenerator(task, mod)
		rt.models[task] = provider.Model
		providers[task] = provider
	}

	if len(rt.models) == 0 {
		return nil, fmt.Errorf("judge runtime: no generator tasks registered (configure job_configs or set an LLM provider key)")
	}

	for task, provider := range providers {
		if err := registerDigestEvaluationWorkflow(ctx, reg, evalFactory, task, provider); err != nil {
			return nil, err
		}
	}

	rt.runner = NewJobRunner(reg, nil, nil, nil)
	return rt, nil
}

func resolveTaskProviderConfig(cfg config.RepoConfig, task, fallbackModel string) (stropdspy.ProviderConfig, error) {
	if explicit, ok, err := cfg.ResolveTaskProvider(task); err != nil {
		return stropdspy.ProviderConfig{}, err
	} else if ok {
		if explicit.UsesEmbeddedGateway() {
			return resolveGatewayOrUnavailable(explicit.Model)
		}
		return explicit.ToStrop(), nil
	}
	model := strings.TrimSpace(fallbackModel)
	if model == "" {
		if pipe, ok := cfg.PipelineNamed("pr-review"); ok {
			model = strings.TrimSpace(pipe.Model)
		}
	}
	return resolveGatewayOrUnavailable(model)
}

func resolveGatewayOrUnavailable(model string) (stropdspy.ProviderConfig, error) {
	if !LLMConfigured() {
		return stropdspy.ProviderConfig{}, fmt.Errorf("%w: no Anthropic/OpenAI/Gemini key", errGatewayUnavailable)
	}
	return ResolveGatewayProvider(model)
}

// Generate runs one registered generator task.
func (rt *Runtime) Generate(ctx context.Context, task string, fields map[string]interface{}, version int) (map[string]interface{}, error) {
	if rt == nil || rt.runner == nil {
		return nil, ErrNotReady
	}
	cfg := runner.GenerationConfig{
		ModuleName:   task,
		JobName:      task,
		StepName:     task,
		ErrorMessage: task,
	}
	out, err := rt.runner.Generate(ctx, cfg, newMapInput(fields, version), nil)
	llmusage.RecordExecutionState(ctx, task)
	return out, err
}

// Evaluate runs the registered evaluation workflow for a generator task.
func (rt *Runtime) Evaluate(
	ctx context.Context,
	task string,
	inputFields, outputFields map[string]interface{},
	version int,
) (*evaluation.AggregatedEvaluation, error) {
	if rt == nil || rt.runner == nil {
		return nil, ErrNotReady
	}
	// strop v0.2.4 EvaluateStream blocks forever on nil eventChan (send on nil channel).
	// Drain a buffered channel until strop is bumped with the nil-safe JobRunner path.
	eventChan, stop := discardEventChannel()
	defer stop()
	out, err := rt.runner.EvaluateWorkflow(ctx, task, newMapInput(inputFields, version), outputFields, eventChan)
	llmusage.RecordExecutionState(ctx, task)
	return out, err
}

// discardEventChannel returns a stream sink so EvaluateWorkflow can emit start/end
// events without a TUI consumer. stop closes the channel and waits for the drain.
func discardEventChannel() (streaming.EventChannel, func()) {
	ch := make(streaming.EventChannel, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	return ch, func() {
		close(ch)
		<-done
	}
}

// Ready reports whether the runtime has a usable runner.
func (rt *Runtime) Ready() bool {
	return rt != nil && rt.runner != nil
}

// TaskModel returns the model ID registered for a task (tests and tracing).
func (rt *Runtime) TaskModel(task string) string {
	if rt == nil {
		return ""
	}
	return rt.models[task]
}

// SetDefaultRuntime installs the process-wide runtime used by package-level helpers.
func SetDefaultRuntime(rt *Runtime) {
	defaultRuntimeMu.Lock()
	defer defaultRuntimeMu.Unlock()
	defaultRuntime = rt
	defaultRuntimeErr = nil
	if rt != nil {
		sharedReg = rt.reg
		sharedRunner = rt.runner
	}
}

// DefaultRuntime returns the process-wide runtime, initializing the gateway fallback if needed.
func DefaultRuntime() (*Runtime, error) {
	defaultRuntimeMu.RLock()
	rt := defaultRuntime
	err := defaultRuntimeErr
	defaultRuntimeMu.RUnlock()
	if rt != nil || err != nil {
		return rt, err
	}

	defaultRuntimeOnce.Do(func() {
		built, buildErr := NewRuntime(context.Background(), config.RepoConfig{}, RuntimeOptions{})
		defaultRuntimeMu.Lock()
		defer defaultRuntimeMu.Unlock()
		if buildErr != nil {
			defaultRuntimeErr = buildErr
			return
		}
		defaultRuntime = built
		sharedReg = built.reg
		sharedRunner = built.runner
	})

	defaultRuntimeMu.RLock()
	defer defaultRuntimeMu.RUnlock()
	return defaultRuntime, defaultRuntimeErr
}

// EnsureRuntimeFromConfig builds and installs a process-wide runtime from central config.
func EnsureRuntimeFromConfig(cfg config.RepoConfig, opts RuntimeOptions) (*Runtime, error) {
	rt, err := NewRuntime(context.Background(), cfg, opts)
	if err != nil {
		return nil, err
	}
	SetDefaultRuntime(rt)
	return rt, nil
}

// ResolveGatewayProvider returns an OpenAI-schema ProviderConfig aimed at the embedded Bifrost loopback.
func ResolveGatewayProvider(model string) (stropdspy.ProviderConfig, error) {
	gw, err := aigateway.Ensure()
	if err != nil {
		return stropdspy.ProviderConfig{}, err
	}
	account, err := aigateway.NewAccountFromEnv()
	if err != nil {
		return stropdspy.ProviderConfig{}, fmt.Errorf("resolve gateway account: %w", err)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(os.Getenv("MAJORDOMO_MODEL"))
	}
	if model == "" {
		model = aigateway.LogicalModel(account)
	}
	return stropdspy.ProviderConfig{
		APIKey:    aigateway.DummyAPIKey,
		Model:     model,
		BaseURL:   gw.BaseURL(),
		APISchema: "openai",
		Timeout:   "120s",
	}, nil
}

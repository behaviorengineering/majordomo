package judge_test

import (
	"context"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

func TestNewRuntimeRegistersPerTaskModels(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")

	cfg := config.RepoConfig{
		AIProviders: map[string]config.AIProviderConfig{
			"poly": {
				APISchema: "openai",
				APIKey:    "local",
				Model:     "poly-model-a",
				BaseURL:   "http://127.0.0.1:1320/v1",
			},
			"gateway_story": {
				APISchema: "openai",
				APIKey:    "gateway",
				Model:     "claude-sonnet-4-20250514",
				BaseURL:   "gateway",
			},
		},
		JobConfigs: map[string]config.JobConfig{
			config.JobContextDigest: {
				Modules: map[string]config.ModuleTaskConfig{
					jmodules.TaskBootstrapStory: {Provider: "poly"},
					jmodules.TaskDigestStory:    {Provider: "gateway_story"},
				},
			},
		},
	}

	rt, err := judge.NewRuntime(context.Background(), cfg, judge.RuntimeOptions{
		Tasks: judge.DigestTasks(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := rt.TaskModel(jmodules.TaskBootstrapStory); got != "poly-model-a" {
		t.Fatalf("bootstrap model=%q", got)
	}
	if got := rt.TaskModel(jmodules.TaskDigestStory); got != "claude-sonnet-4-20250514" {
		t.Fatalf("digest model=%q", got)
	}
	if !rt.Ready() {
		t.Fatal("expected ready")
	}
}

func TestNewRuntimeFallsBackToGatewayModel(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("MAJORDOMO_MODEL", "")
	rt, err := judge.NewRuntime(context.Background(), config.RepoConfig{
		Pipelines: map[string]config.Pipeline{
			"pr-review": {Model: "claude-fallback-model"},
		},
	}, judge.RuntimeOptions{
		Tasks: []string{jmodules.TaskSummary},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := rt.TaskModel(jmodules.TaskSummary); got != "claude-fallback-model" {
		t.Fatalf("summary model=%q", got)
	}
}

func TestNewRuntimePolypusOnlySkipsUnusedGatewayTasks(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")

	cfg := config.RepoConfig{
		AIProviders: map[string]config.AIProviderConfig{
			"poly": {
				APISchema: "openai",
				APIKey:    "local",
				Model:     "gemma-poly",
				BaseURL:   "http://127.0.0.1:1320/v1",
			},
		},
		JobConfigs: map[string]config.JobConfig{
			config.JobContextDigest: {
				Modules: map[string]config.ModuleTaskConfig{
					jmodules.TaskBootstrapStory: {Provider: "poly"},
					jmodules.TaskDigestStory:    {Provider: "poly"},
				},
			},
		},
	}

	// Empty Tasks list previously failed on filereview gateway fallback.
	rt, err := judge.NewRuntime(context.Background(), cfg, judge.RuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := rt.TaskModel(jmodules.TaskBootstrapStory); got != "gemma-poly" {
		t.Fatalf("bootstrap model=%q", got)
	}
	if got := rt.TaskModel(jmodules.TaskDigestStory); got != "gemma-poly" {
		t.Fatalf("digest model=%q", got)
	}
	if got := rt.TaskModel(jmodules.TaskFileReview); got != "" {
		t.Fatalf("expected filereview skipped, got model %q", got)
	}
}

func TestNewRuntimeRegistersDigestEvaluationWorkflows(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")

	cfg := config.RepoConfig{
		AIProviders: map[string]config.AIProviderConfig{
			"poly": {
				APISchema: "openai",
				APIKey:    "local",
				Model:     "poly-model",
				BaseURL:   "http://127.0.0.1:1320/v1",
			},
		},
		JobConfigs: map[string]config.JobConfig{
			config.JobContextDigest: {
				Modules: map[string]config.ModuleTaskConfig{
					jmodules.TaskTypologyInspect:           {Provider: "poly"},
					jmodules.TaskTypologyCluster:           {Provider: "poly"},
					jmodules.TaskTypologyRefine:            {Provider: "poly"},
					jmodules.TaskTypologyHumanIntervention: {Provider: "poly"},
					jmodules.TaskBootstrapStory:            {Provider: "poly"},
					jmodules.TaskDigestStory:               {Provider: "poly"},
				},
			},
		},
	}

	// NewRuntime fails closed if chained evaluators / workflows cannot register.
	rt, err := judge.NewRuntime(context.Background(), cfg, judge.RuntimeOptions{
		Tasks: judge.DigestTasks(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range []string{jmodules.TaskTypologyRefine, jmodules.TaskTypologyHumanIntervention, jmodules.TaskBootstrapStory, jmodules.TaskDigestStory} {
		if rt.TaskModel(task) == "" {
			t.Fatalf("%s generator not registered", task)
		}
	}
}

func TestNewRuntimeDigestTasksFailClosedWithoutProvider(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")

	_, err := judge.NewRuntime(context.Background(), config.RepoConfig{}, judge.RuntimeOptions{
		Tasks: judge.DigestTasks(),
	})
	if err == nil {
		t.Fatal("expected error when digest tasks lack provider and gateway keys")
	}
}

package judge_test

import (
	"context"
	"testing"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/behaviorengineering/majordomo/pkg/judge"
	jmodules "github.com/behaviorengineering/majordomo/pkg/judge/modules"
	"github.com/behaviorengineering/majordomo/pkg/platform/config"
	dspymodules "github.com/behaviorengineering/strop/pkg/dspy/modules"
)

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

func TestNewRuntimeEmptyTasksIsReviewOnly(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")

	rt, err := judge.NewRuntime(context.Background(), config.RepoConfig{
		Pipelines: map[string]config.Pipeline{
			"pr-review": {Model: "claude-review"},
		},
	}, judge.RuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range judge.ReviewTasks() {
		if rt.TaskModel(task) == "" {
			t.Fatalf("expected review task %s registered", task)
		}
	}
	if got := rt.TaskModel("digest_story"); got != "" {
		t.Fatalf("digest task must not register on empty Tasks, got %q", got)
	}
}

func TestNewRuntimeFactoryRegistersViaGenerators(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")

	const factoryTask = "factory_probe"
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
					factoryTask: {Provider: "poly"},
				},
			},
		},
	}
	config.RegisterJobForTask(factoryTask, config.JobContextDigest)
	t.Cleanup(config.ResetJobForTaskRegistryForTests)

	rt, err := judge.NewRuntime(context.Background(), cfg, judge.RuntimeOptions{
		Tasks: []string{factoryTask},
		Generators: map[string]judge.GeneratorCtor{
			factoryTask: func() core.Module {
				sig := core.NewSignature(
					[]core.InputField{{Field: core.NewField("in")}},
					[]core.OutputField{{Field: core.NewField("out")}},
				)
				return dspymodules.New(sig, dspymodules.Config{Name: factoryTask})
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := rt.TaskModel(factoryTask); got != "gemma-poly" {
		t.Fatalf("factory model=%q", got)
	}
	if got := rt.TaskModel(jmodules.TaskFileReview); got != "" {
		t.Fatalf("expected filereview skipped, got model %q", got)
	}
}

func TestNewRuntimeUnknownTaskFailsClosed(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	_, err := judge.NewRuntime(context.Background(), config.RepoConfig{}, judge.RuntimeOptions{
		Tasks: []string{"digest_story"},
	})
	if err == nil {
		t.Fatal("expected unknown digest task without Generators")
	}
}

func TestNewRuntimeDigestTaskWithoutGeneratorsFailsClosed(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")

	_, err := judge.NewRuntime(context.Background(), config.RepoConfig{}, judge.RuntimeOptions{
		Tasks: []string{"bootstrap_story", "digest_story"},
	})
	if err == nil {
		t.Fatal("expected error when digest tasks lack Generators")
	}
}

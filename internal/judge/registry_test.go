package judge_test

import (
	"os"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
	"github.com/behaviorengineering/majordomo/internal/judge"
)

func TestStropReadyWithoutKeys(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")
	if judge.StropReady() {
		t.Fatal("expected not ready without keys")
	}
	if err := judge.EnsureStropReady(); err == nil {
		t.Fatal("expected EnsureStropReady error")
	}
}

func TestResolveProviderRequiresGateway(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")
	_, err := judge.ResolveProvider()
	if err == nil {
		t.Fatal("expected error without keys")
	}
}

func TestResolveProviderOpenAISchema(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	p, err := judge.ResolveProvider()
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if p.APISchema != "openai" {
		t.Fatalf("expected openai schema, got %q", p.APISchema)
	}
	if p.APIKey != aigateway.DummyAPIKey {
		t.Fatalf("expected dummy key, got %q", p.APIKey)
	}
	if p.BaseURL == "" || p.BaseURL == "https://api.anthropic.com" {
		t.Fatalf("expected loopback base URL, got %q", p.BaseURL)
	}
	aigateway.ResetForTests()
}

func TestEnsureStropReadyRequiresKeys(t *testing.T) {
	judge.ResetRegistryForTests()
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GOOGLE_API_KEY")
	os.Unsetenv("GOOGLE_GENERATIVE_AI_API_KEY")
	if err := judge.EnsureStropReady(); err == nil {
		t.Fatal("expected error")
	}
}

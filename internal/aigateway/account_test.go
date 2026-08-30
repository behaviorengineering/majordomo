package aigateway_test

import (
	"testing"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
	"github.com/maximhq/bifrost/core/schemas"
)

func TestNewAccountFromEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")
	if _, err := aigateway.NewAccountFromEnv(); err == nil {
		t.Fatal("expected error without keys")
	}

	t.Setenv("ANTHROPIC_API_KEY", "sk-ant")
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	acc, err := aigateway.NewAccountFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	providers, err := acc.GetConfiguredProviders()
	if err != nil || len(providers) != 2 {
		t.Fatalf("providers=%v err=%v", providers, err)
	}
	keys, err := acc.GetKeysForProvider(t.Context(), schemas.Anthropic)
	if err != nil || len(keys) != 1 || keys[0].Value.Val != "sk-ant" {
		t.Fatalf("keys=%v err=%v", keys, err)
	}
}

func TestGatewayLoopback(t *testing.T) {
	aigateway.ResetForTests()
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	gw, err := aigateway.Ensure()
	if err != nil {
		t.Fatal(err)
	}
	defer aigateway.ResetForTests()
	if gw.BaseURL() == "" {
		t.Fatal("empty base URL")
	}
	env := gw.ChildEnv([]string{
		"ANTHROPIC_API_KEY=secret",
		"OPENAI_API_KEY=secret",
		"PATH=/bin",
	})
	for _, e := range env {
		if e == "ANTHROPIC_API_KEY=secret" || e == "OPENAI_API_KEY=secret" {
			t.Fatalf("real key leaked into child env: %s", e)
		}
	}
	foundDummy := false
	for _, e := range env {
		if e == "OPENAI_API_KEY="+aigateway.DummyAPIKey {
			foundDummy = true
		}
	}
	if !foundDummy {
		t.Fatal("expected dummy OPENAI_API_KEY in child env")
	}
}

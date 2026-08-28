package judge_test

import (
	"os"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/judge"
)

func TestStropReadyWithoutKeys(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	if judge.StropReady() {
		t.Fatal("expected not ready without keys")
	}
	if err := judge.EnsureStropReady(); err == nil {
		t.Fatal("expected EnsureStropReady error")
	}
}

func TestResolveProviderAnthropic(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")
	p, err := judge.ResolveProvider()
	if err != nil || p.APISchema != "anthropic" {
		t.Fatalf("provider=%+v err=%v", p, err)
	}
}

func TestResolveProviderOpenAI(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "test-key")
	p, err := judge.ResolveProvider()
	if err != nil || p.APISchema != "openai" {
		t.Fatalf("provider=%+v err=%v", p, err)
	}
}

func TestEnsureStropReadyRequiresKeys(t *testing.T) {
	judge.ResetRegistryForTests()
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	if err := judge.EnsureStropReady(); err == nil {
		t.Fatal("expected error")
	}
}

package judge_test

import (
	"errors"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/judge"
)

func TestEnsureStropReadyWithoutKeys(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	err := judge.EnsureStropReady()
	if !errors.Is(err, judge.ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}

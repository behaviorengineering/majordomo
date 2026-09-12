package judge

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/XiaoConstantine/dspy-go/pkg/interceptors"
)

type flakyLLM struct {
	core.BaseLLM
	failsLeft atomic.Int32
	calls     atomic.Int32
}

func (f *flakyLLM) Generate(context.Context, string, ...core.GenerateOption) (*core.LLMResponse, error) {
	f.calls.Add(1)
	if f.failsLeft.Add(-1) >= 0 {
		return nil, errors.New("502 bad gateway")
	}
	return &core.LLMResponse{Content: "ok"}, nil
}

func (f *flakyLLM) GenerateWithContent(context.Context, []core.ContentBlock, ...core.GenerateOption) (*core.LLMResponse, error) {
	return f.Generate(context.Background(), "")
}

func TestWrapLLMWithRetrySucceedsAfterTransientFailures(t *testing.T) {
	t.Parallel()
	inner := &flakyLLM{}
	inner.failsLeft.Store(2)
	llm := WrapLLMWithRetry(inner, interceptors.RetryConfig{
		MaxAttempts: 3,
		Delay:       time.Millisecond,
		MaxBackoff:  5 * time.Millisecond,
		Backoff:     2.0,
	})
	resp, err := llm.Generate(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || resp.Content != "ok" {
		t.Fatalf("resp=%v", resp)
	}
	if got := inner.calls.Load(); got != 3 {
		t.Fatalf("calls=%d want 3", got)
	}
}

func TestWrapLLMWithRetryExhausts(t *testing.T) {
	t.Parallel()
	inner := &flakyLLM{}
	inner.failsLeft.Store(10)
	llm := WrapLLMWithRetry(inner, interceptors.RetryConfig{
		MaxAttempts: 2,
		Delay:       time.Millisecond,
		MaxBackoff:  2 * time.Millisecond,
		Backoff:     2.0,
	})
	_, err := llm.Generate(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := inner.calls.Load(); got != 2 {
		t.Fatalf("calls=%d want 2", got)
	}
}

func TestWrapLLMWithRetryHonorsContextCancel(t *testing.T) {
	t.Parallel()
	inner := &flakyLLM{}
	inner.failsLeft.Store(10)
	llm := WrapLLMWithRetry(inner, interceptors.RetryConfig{
		MaxAttempts: 5,
		Delay:       50 * time.Millisecond,
		MaxBackoff:  50 * time.Millisecond,
		Backoff:     2.0,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := llm.Generate(ctx, "hi")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
}

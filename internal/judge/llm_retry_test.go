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

func (f *flakyLLM) GenerateWithContent(ctx context.Context, _ []core.ContentBlock, opts ...core.GenerateOption) (*core.LLMResponse, error) {
	return f.Generate(ctx, "", opts...)
}

func (f *flakyLLM) GenerateWithJSON(context.Context, string, ...core.GenerateOption) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func (f *flakyLLM) GenerateWithFunctions(context.Context, string, []map[string]any, ...core.GenerateOption) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func (f *flakyLLM) CreateEmbedding(context.Context, string, ...core.EmbeddingOption) (*core.EmbeddingResult, error) {
	return nil, errors.New("not implemented")
}

func (f *flakyLLM) CreateEmbeddings(context.Context, []string, ...core.EmbeddingOption) (*core.BatchEmbeddingResult, error) {
	return nil, errors.New("not implemented")
}

func (f *flakyLLM) StreamGenerate(context.Context, string, ...core.GenerateOption) (*core.StreamResponse, error) {
	return nil, errors.New("not implemented")
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

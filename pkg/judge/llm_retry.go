package judge

import (
	"context"
	"fmt"
	"time"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/XiaoConstantine/dspy-go/pkg/interceptors"
)

// DefaultModuleRetryConfig matches the RetryModuleInterceptor settings used by
// NewRuntime (DSPy generate/evaluate). Apply the same budget to bare LLM hops
// (for example RLM CreateLLM) so provider 502s and transport blips back off.
func DefaultModuleRetryConfig() interceptors.RetryConfig {
	return interceptors.RetryConfig{
		MaxAttempts: defaultModuleRetryAttempts,
		Delay:       2 * time.Second,
		MaxBackoff:  30 * time.Second,
		Backoff:     2.0,
	}
}

// WrapLLMWithRetry returns an LLM that retries Generate and GenerateWithContent
// with exponential backoff. Other LLM methods pass through unchanged.
// Nil llm is returned unchanged.
func WrapLLMWithRetry(llm core.LLM, cfg interceptors.RetryConfig) core.LLM {
	if llm == nil {
		return nil
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaultModuleRetryAttempts
	}
	if cfg.Delay <= 0 {
		cfg.Delay = 2 * time.Second
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = 2.0
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 30 * time.Second
	}
	return &retryLLM{LLM: llm, cfg: cfg}
}

type retryLLM struct {
	core.LLM
	cfg interceptors.RetryConfig
}

func (r *retryLLM) Generate(ctx context.Context, prompt string, opts ...core.GenerateOption) (*core.LLMResponse, error) {
	return withLLMRetry(ctx, r.cfg, "Generate", func() (*core.LLMResponse, error) {
		return r.LLM.Generate(ctx, prompt, opts...)
	})
}

func (r *retryLLM) GenerateWithContent(ctx context.Context, content []core.ContentBlock, opts ...core.GenerateOption) (*core.LLMResponse, error) {
	return withLLMRetry(ctx, r.cfg, "GenerateWithContent", func() (*core.LLMResponse, error) {
		return r.LLM.GenerateWithContent(ctx, content, opts...)
	})
}

func withLLMRetry(ctx context.Context, cfg interceptors.RetryConfig, op string, call func() (*core.LLMResponse, error)) (*core.LLMResponse, error) {
	var lastErr error
	delay := cfg.Delay
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resp, err := call()
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if attempt == cfg.MaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		delay = time.Duration(float64(delay) * cfg.Backoff)
		if cfg.MaxBackoff > 0 && delay > cfg.MaxBackoff {
			delay = cfg.MaxBackoff
		}
	}
	return nil, fmt.Errorf("llm %s failed after %d attempts: %w", op, cfg.MaxAttempts, lastErr)
}

package process

import (
	"context"
	"testing"
	"time"
)

func TestRunRequiresContextDeadline(t *testing.T) {
	runner := NewRunner()

	if err := runner.Run(nil, "test", "true", nil, nil, "", nil, nil); err == nil {
		t.Fatal("expected nil context error")
	}
	if err := runner.Run(context.Background(), "test", "true", nil, nil, "", nil, nil); err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRetryable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if retryable(ctx.Err()) {
		t.Fatal("context errors must not be retried")
	}
}

package command

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunRequiresContextDeadline(t *testing.T) {
	if _, _, err := Run(nilContext(), "test", "true", nil, nil, ""); err == nil {
		t.Fatal("expected nil-context error")
	}
	if _, _, err := Run(context.Background(), "test", "true", nil, nil, ""); err == nil {
		t.Fatal("expected missing-deadline error")
	}
}

func nilContext() context.Context {
	return nil
}

func TestRunExecutesCommandWithDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, stderr, err := Run(ctx, "test", "printf", []string{"ok"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ok" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestIsRetryableClassifiesTransientFailures(t *testing.T) {
	if !isRetryable(context.Background(), "connection reset by peer", errors.New("exit status 1")) {
		t.Fatal("expected connection reset to be retryable")
	}
	if isRetryable(context.Background(), "authentication failed", errors.New("exit status 1")) {
		t.Fatal("authentication failure must not be retried")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if isRetryable(ctx, "connection reset by peer", errors.New("exit status 1")) {
		t.Fatal("canceled context must stop retries")
	}
}

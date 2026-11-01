package process

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	t.Parallel()

	_, _, err := Run(context.Background(), "test", "true", nil, "", nil)
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunRejectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ctx, deadlineCancel := context.WithTimeout(ctx, time.Minute)
	defer deadlineCancel()

	_, _, err := Run(ctx, "test", "true", nil, "", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}
}

func TestIsRetryableClassifiesTransientCommandOutput(t *testing.T) {
	t.Parallel()

	cmdErr := exec.Command("sh", "-c", "exit 1").Run()
	if !isRetryable(result{stderr: "connection reset by peer"}, cmdErr) {
		t.Fatal("expected connection reset to be retryable")
	}
	if isRetryable(result{stderr: "authentication failed"}, cmdErr) {
		t.Fatal("did not expect authentication failure to be retryable")
	}
}

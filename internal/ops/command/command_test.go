package command

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunRequiresContext(t *testing.T) {
	_, err := Run(Options{Name: "git"})
	if err == nil || err.Error() != "command: context is required" {
		t.Fatalf("expected missing-context error, got %v", err)
	}
}

func TestRunRequiresDeadline(t *testing.T) {
	_, err := Run(Options{Context: context.Background(), Name: "git"})
	if err == nil || err.Error() != "command: context deadline is required" {
		t.Fatalf("expected missing-deadline error, got %v", err)
	}
}

func TestRetryableNetworkOutput(t *testing.T) {
	if !isRetryable(Result{Stderr: "fatal: could not resolve host"}, errors.New("git failed")) {
		t.Fatal("expected network resolution failures to be retryable")
	}
	if isRetryable(Result{Stderr: "fatal: authentication failed"}, errors.New("git failed")) {
		t.Fatal("authentication failures must not be retried")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if isRetryable(Result{}, ctx.Err()) {
		t.Fatal("canceled contexts must not be retried")
	}
}

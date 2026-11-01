package command

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunRejectsMissingDeadline(t *testing.T) {
	called := false
	_, err := Run(context.Background(), "test-missing-deadline", "tool", nil, nil, "", func(context.Context, string, []string, []string, string) (Result, error) {
		called = true
		return Result{}, nil
	})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
	if called {
		t.Fatal("runner called without a deadline")
	}
}

func TestRunRetriesTransientCommandFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	attempts := 0
	result, err := Run(ctx, "test-retry", "tool", nil, nil, "", func(context.Context, string, []string, []string, string) (Result, error) {
		attempts++
		if attempts == 1 {
			return Result{Stderr: "connection reset by peer"}, errors.New("command failed")
		}
		return Result{Stdout: "ok"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
	if result.Stdout != "ok" {
		t.Fatalf("stdout=%q, want ok", result.Stdout)
	}
}

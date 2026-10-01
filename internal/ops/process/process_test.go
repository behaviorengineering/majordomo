package process

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunnerRequiresDeadline(t *testing.T) {
	_, _, err := (Runner{}).Run(context.Background(), Spec{Name: "git"})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunnerUsesInjectedExecutor(t *testing.T) {
	called := false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	runner := Runner{
		Execute: func(_ context.Context, spec Spec) (string, string, error) {
			called = true
			if spec.Name != "test-command" {
				return "", "", errors.New("unexpected command")
			}
			return "stdout", "stderr", nil
		},
	}

	stdout, stderr, err := runner.Run(ctx, Spec{Name: "test-command"})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected injected executor to run")
	}
	if stdout != "stdout" || stderr != "stderr" {
		t.Fatalf("output = %q, %q", stdout, stderr)
	}
}

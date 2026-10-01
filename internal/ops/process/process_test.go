package process

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	executor := NewExecutor(func(context.Context, string, []string, []string, string) (string, string, error) {
		t.Fatal("runner should not be called")
		return "", "", nil
	})
	if _, _, err := executor.Run(context.Background(), "git", nil, nil, ""); err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunUsesRunner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	called := false
	executor := NewExecutor(func(_ context.Context, name string, args []string, env []string, dir string) (string, string, error) {
		called = true
		if name != "git" || len(args) != 1 || args[0] != "status" {
			t.Fatalf("unexpected command: %s %v", name, args)
		}
		return "clean\n", "", nil
	})
	stdout, stderr, err := executor.Run(ctx, "git", []string{"status"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "clean\n" || stderr != "" || !called {
		t.Fatalf("got stdout=%q stderr=%q called=%v", stdout, stderr, called)
	}
}

func TestRunRetriesTransientFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	attempts := 0
	executor := NewExecutor(func(context.Context, string, []string, []string, string) (string, string, error) {
		attempts++
		if attempts == 1 {
			return "", "connection reset by peer", errors.New("exit status 1")
		}
		return "ok", "", nil
	})
	stdout, _, err := executor.Run(ctx, "git", nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ok" || attempts != 2 {
		t.Fatalf("got stdout=%q attempts=%d", stdout, attempts)
	}
}

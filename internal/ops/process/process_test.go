package process

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	executor := NewExecutorWithRunner(func(context.Context, Spec) error {
		t.Fatal("runner should not be called")
		return nil
	})

	err := executor.Run(context.Background(), Spec{Name: "test"})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunRetriesClassifiedFailures(t *testing.T) {
	attempts := 0
	transient := errors.New("transient")
	executor := NewExecutorWithRunner(func(context.Context, Spec) error {
		attempts++
		if attempts < 2 {
			return transient
		}
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := executor.Run(ctx, Spec{
		Name:       "test",
		Dependency: "test-dependency",
		Retryable:  func(err error) bool { return errors.Is(err, transient) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
}

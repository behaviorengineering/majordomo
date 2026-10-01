package process

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunRequiresBoundedContext(t *testing.T) {
	if err := Run(nil, "test", nil, func() error { return nil }); err == nil {
		t.Fatal("expected nil context error")
	}
	if err := Run(context.Background(), "test", nil, func() error { return nil }); err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunRetriesRetryableFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var attempts int
	err := Run(ctx, "test-retry", func(error) bool { return true }, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d, want 3", attempts)
	}
}

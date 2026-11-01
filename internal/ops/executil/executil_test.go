package executil

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunnerRequiresDeadline(t *testing.T) {
	err := NewRunner().Run(context.Background(), Request{Name: "true"})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{name: "timeout", err: errors.New("connection timed out"), retryable: true},
		{name: "server error", err: errors.New("remote returned 503"), retryable: true},
		{name: "auth error", err: errors.New("authentication failed"), retryable: false},
		{name: "canceled", err: context.Canceled, retryable: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isRetryable(test.err); got != test.retryable {
				t.Fatalf("isRetryable(%v) = %v, want %v", test.err, got, test.retryable)
			}
		})
	}
}

func TestRunnerRejectsExpiredContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	err := NewRunner().Run(ctx, Request{Name: "true"})
	if err == nil {
		t.Fatal("expected expired context error")
	}
}

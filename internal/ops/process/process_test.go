package process

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunRequiresContext(t *testing.T) {
	err := Run(Options{Name: "true"})
	if err == nil {
		t.Fatal("expected missing context error")
	}
}

func TestRunRetryRequiresDeadline(t *testing.T) {
	err := Run(Options{
		Context: context.Background(),
		Name:    "true",
		Retry:   true,
	})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunExecutesCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := Run(Options{Context: ctx, Name: "true"}); err != nil {
		t.Fatal(err)
	}
}

func TestRetryable(t *testing.T) {
	if Retryable(context.Canceled) {
		t.Fatal("canceled commands must not be retried")
	}
	if Retryable(errors.New("invalid arguments")) {
		t.Fatal("permanent errors must not be retried")
	}
}

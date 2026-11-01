package commandexec

import (
	"context"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, err := Run(context.Background(), Request{Name: "git"})
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunRequiresRetryClassifier(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	_, err := Run(ctx, Request{Name: "git", Retry: true})
	if err == nil {
		t.Fatal("expected missing retry classifier error")
	}
}

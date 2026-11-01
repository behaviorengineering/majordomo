package process

import (
	"context"
	"testing"
	"time"
)

func TestRunRequiresContextDeadline(t *testing.T) {
	t.Parallel()

	if _, err := Run(nil, "echo", nil, nil, ""); err == nil {
		t.Fatal("expected nil context error")
	}
	if _, err := Run(context.Background(), "echo", nil, nil, ""); err == nil {
		t.Fatal("expected missing deadline error")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Run(ctx, "echo", nil, nil, ""); err != nil {
		t.Fatalf("expected deadline-bearing context to pass validation: %v", err)
	}
}

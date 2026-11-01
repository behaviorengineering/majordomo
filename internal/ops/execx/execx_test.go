package execx

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresContext(t *testing.T) {
	_, _, err := Run(nil, "test", "true", nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunRequiresDeadline(t *testing.T) {
	_, _, err := Run(context.Background(), "test", "true", nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "deadline is required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, _, err := Run(ctx, "test", "true", nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("err=%v", err)
	}
}

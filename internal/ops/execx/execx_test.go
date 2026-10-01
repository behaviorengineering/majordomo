package execx

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresBoundedContext(t *testing.T) {
	_, err := Run(context.Background(), "printf", []string{"ok"}, nil, "", false)
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

func TestRunCapturesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := Run(ctx, "printf", []string{"ok"}, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "ok" {
		t.Fatalf("stdout = %q, want %q", result.Stdout, "ok")
	}
}

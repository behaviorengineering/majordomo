package executil

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, _, err := Run(context.Background(), "printf", []string{"ok"}, nil, "")
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

func TestRunExecutesWithDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, stderr, err := Run(ctx, "printf", []string{"ok"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ok" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

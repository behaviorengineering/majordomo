package process

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, _, err := Run(context.Background(), "printf", []string{"ok"}, nil, "")
	if !errors.Is(err, ErrDeadlineRequired) {
		t.Fatalf("expected deadline error, got %v", err)
	}
}

func TestRunExecutesWithDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, stderr, err := Run(ctx, "printf", []string{"ok"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ok" || strings.TrimSpace(stderr) != "" {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

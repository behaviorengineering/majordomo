package execx

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, _, err := New().Run(context.Background(), "printf", []string{"ok"}, nil, "")
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

func TestRunCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stdout, stderr, err := New().Run(ctx, "printf", []string{"ok"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ok" {
		t.Fatalf("stdout=%q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr=%q", stderr)
	}
}

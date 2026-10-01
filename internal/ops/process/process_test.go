package process

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunCapturesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := Run(ctx, Options{
		Name: "sh",
		Args: []string{"-c", "printf stdout; printf stderr >&2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "stdout" || result.Stderr != "stderr" {
		t.Fatalf("result=%+v", result)
	}
}

func TestRunRequiresDeadline(t *testing.T) {
	_, err := Run(context.Background(), Options{Name: "sh", Args: []string{"-c", "exit 0"}})
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

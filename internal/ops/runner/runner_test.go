package runner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	_, err := Run(context.Background(), Options{Name: "echo"})
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("expected missing deadline error, got %v", err)
	}
}

func TestRunCapturesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := Run(ctx, Options{
		Name: "sh",
		Args: []string{"-c", "printf output"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "output" {
		t.Fatalf("stdout=%q", result.Stdout)
	}
}

package process

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunRequiresDeadline(t *testing.T) {
	t.Parallel()

	_, err := NewRunner().Run(context.Background(), "test", "sh", []string{"-c", "exit 0"}, nil, "")
	if err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunCapturesOutput(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	output, err := NewRunner().Run(
		ctx,
		"test",
		"sh",
		[]string{"-c", "printf stdout; printf stderr >&2"},
		nil,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.Stdout, "stdout") {
		t.Fatalf("stdout=%q", output.Stdout)
	}
	if !strings.Contains(output.Stderr, "stderr") {
		t.Fatalf("stderr=%q", output.Stderr)
	}
}

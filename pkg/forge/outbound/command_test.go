package outbound_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/pkg/forge/outbound"
)

func TestRunCommandRequiresDeadline(t *testing.T) {
	_, err := outbound.RunCommand(context.Background(), "not-run", nil, nil, "")
	if !errors.Is(err, outbound.ErrMissingDeadline) {
		t.Fatalf("error=%v", err)
	}
}

func TestRunCommandRetriesTransientFailure(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "retry.sh")
	counter := filepath.Join(dir, "counter")
	scriptBody := `#!/bin/sh
count=$(cat "$1" 2>/dev/null || echo 0)
count=$((count + 1))
printf '%s' "$count" > "$1"
if [ "$count" -lt 3 ]; then
  echo "connection reset" >&2
  exit 1
fi
echo ready
`
	if err := os.WriteFile(script, []byte(scriptBody), 0o700); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := outbound.RunCommand(ctx, script, []string{counter}, nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "ready\n" {
		t.Fatalf("stdout=%q", result.Stdout)
	}
}

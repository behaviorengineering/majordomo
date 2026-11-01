package process

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

func TestRunRequiresContextDeadline(t *testing.T) {
	if err := Run(context.Background(), os.Args[0], nil, nil, "", nil, nil); err == nil {
		t.Fatal("expected missing deadline error")
	}
}

func TestRunExecutesCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	env := append(os.Environ(), "MAJORDOMO_PROCESS_TEST_HELPER=1")
	if err := Run(ctx, os.Args[0], []string{"-test.run=TestProcessHelper"}, env, "", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
}

func TestProcessHelper(t *testing.T) {
	if os.Getenv("MAJORDOMO_PROCESS_TEST_HELPER") != "1" {
		return
	}
}

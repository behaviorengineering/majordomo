package status

import "testing"

func TestStateValidation(t *testing.T) {
	err := Run(Options{SCM: "github", CommitSHA: "abc", State: "NOPE"})
	if err == nil {
		t.Fatal("expected error")
	}
}

package orchestrate

import "testing"

func TestNormalizeUntil(t *testing.T) {
	got, err := NormalizeUntil("  Waves ")
	if err != nil || got != StageWaves {
		t.Fatalf("got %q %v", got, err)
	}
	got, err = NormalizeUntil("publish")
	if err != nil || got != "" {
		t.Fatalf("publish should mean full run, got %q %v", got, err)
	}
	if _, err := NormalizeUntil("clone"); err == nil {
		t.Fatal("expected error for clone")
	}
	if _, err := NormalizeUntil("nope"); err == nil {
		t.Fatal("expected error for unknown")
	}
}

func TestShouldRun(t *testing.T) {
	if !shouldRun("", StageWaves) {
		t.Fatal("empty until must run all stages")
	}
	if !shouldRun(StagePrep, StagePrep) {
		t.Fatal("until prep must run prep")
	}
	if shouldRun(StagePrep, StageWaves) {
		t.Fatal("until prep must not run waves")
	}
	if !shouldRun(StageWaves, StagePrep) {
		t.Fatal("until waves must still run prep")
	}
	if shouldRun(StageWaves, StageFinalize) {
		t.Fatal("until waves must not run finalize")
	}
}

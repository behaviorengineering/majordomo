package contextdigest

import (
	"strings"
	"testing"
)

func TestEnsureStoryArchitectureBanner(t *testing.T) {
	got := ensureStoryArchitectureBanner("# Architecture\n\nBody.\n")
	if !strings.Contains(got, storyArchitectureRoleMarker) {
		t.Fatalf("missing banner: %s", got)
	}
	if !strings.Contains(got, "evidence/typology/") {
		t.Fatalf("missing evidence pointer: %s", got)
	}
	again := ensureStoryArchitectureBanner(got)
	if strings.Count(again, storyArchitectureRoleMarker) != 1 {
		t.Fatalf("banner duplicated: %s", again)
	}
}

func TestEnsureTypologyArchitectureBanner(t *testing.T) {
	got := ensureTypologyArchitectureBanner("# Typology Architecture\n\nDrift.\n")
	if !strings.Contains(got, typologyArchitectureRoleMarker) {
		t.Fatalf("missing banner: %s", got)
	}
	if !strings.Contains(got, "teaching-story") && !strings.Contains(got, "architecture.md") {
		t.Fatalf("missing story pointer: %s", got)
	}
}

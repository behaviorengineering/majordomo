package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGroundingPathsResolvesBatchAndSkillDirs(t *testing.T) {
	batchDir := t.TempDir()
	skillDir := filepath.Join(batchDir, "pr-review-code")
	groundDir := filepath.Join(batchDir, ".grounding")
	if err := os.MkdirAll(groundDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groundDir, "overview.md"), []byte("# overview\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "review_agents": {"pr-review-code": ["internal/a.go"]},
  "grounding_packs": [{"id": "overview", "file": ".grounding/overview.md"}]
}`
	if err := os.WriteFile(filepath.Join(batchDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := GroundingPaths(batchDir, skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != "overview.md" {
		t.Fatalf("paths = %v", paths)
	}
}

func TestGroundingSkillDirByMode(t *testing.T) {
	batchDir := t.TempDir()
	manifest := `{"review_agents": {"pr-review-summary": ["a.go"]}}`
	if err := os.WriteFile(filepath.Join(batchDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := GroundingSkillDir(batchDir, ModeSummary)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(batchDir, "pr-review-summary")
	if got != want {
		t.Fatalf("GroundingSkillDir = %q, want %q", got, want)
	}
	if _, err := GroundingSkillDir(batchDir, ModeFinalize); err != nil {
		t.Fatal(err)
	}
}

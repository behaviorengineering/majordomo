package staging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAttachGroundingStagesSelectedPacks(t *testing.T) {
	contextDir := t.TempDir()
	writeAgentingTree(t, contextDir)
	batchDir := t.TempDir()
	manifest := map[string]any{
		"review_agents": map[string][]string{
			"pr-review-code": {"internal/auth/login.go"},
		},
	}
	writeManifest(t, filepath.Join(batchDir, "manifest.json"), manifest)
	batches := []BatchEntry{{
		Skill:      "pr-review-code",
		BatchNum:   "001",
		StagingDir: batchDir,
	}}
	if err := AttachGrounding(contextDir, batches); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(batchDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		GroundingPacks []struct {
			ID   string `json:"id"`
			File string `json:"file"`
		} `json:"grounding_packs"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.GroundingPacks) != 2 {
		t.Fatalf("grounding_packs = %+v", out.GroundingPacks)
	}
	if _, err := os.Stat(filepath.Join(batchDir, out.GroundingPacks[0].File)); err != nil {
		t.Fatalf("staged file missing: %v", err)
	}
}

func writeAgentingTree(t *testing.T, dir string) {
	t.Helper()
	index := `packs:
  overview:
    modes: [files, summary]
  auth:
    globs: ["**/auth/**"]
    modes: [files]
`
	indexPath := filepath.Join(dir, "agenting", "index.yaml")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"overview", "auth"} {
		packDir := filepath.Join(dir, "agenting", id)
		if err := os.MkdirAll(packDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(packDir, "GROUNDING.md"), []byte("# "+id+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeManifest(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClusterCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.py", "b.py"}
	clusterSHA := sha256Hex("cluster-seed")
	analysis := filepath.Join(dir, "analysis-in.json")
	payload := `{"reviewable":[{"slug":"a-py","file":"a.py"}]}`
	if err := os.WriteFile(analysis, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	reports := filepath.Join(dir, "reports")
	_ = os.MkdirAll(reports, 0o755)
	if err := os.WriteFile(filepath.Join(reports, "a-py.md"), []byte("# ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(dir, "cache")
	storeOut, err := Store(StoreOptions{
		CacheDir:              cacheDir,
		SkillName:             "pr-review-code",
		ClusterSHA:            clusterSHA,
		FingerprintVersion:    "v1",
		ClusterFiles:          files,
		ModelID:               "model",
		InstructionBundleHash: "i1",
		PromptTemplateHash:    "p1",
		ScoringRubricHash:     "s1",
		OutputSchemaVersion:   "o1",
		AnalysisFile:          analysis,
		ReportsDir:            reports,
	})
	if err != nil {
		t.Fatal(err)
	}
	if storeOut["written"] != true {
		t.Fatalf("store: %+v", storeOut)
	}

	pre, err := Precheck(PrecheckOptions{
		ProjectID:           "demo",
		CacheDir:            cacheDir,
		GlobalRetentionDays: 180,
		MinRetentionDays:    30,
	})
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(dir, "index.json")
	raw, _ := json.MarshalIndent(pre, "", "  ")
	if err := os.WriteFile(indexPath, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	hit, err := Lookup(LookupOptions{
		IndexFile:             indexPath,
		ClusterSHA:            clusterSHA,
		SkillName:             "pr-review-code",
		FingerprintVersion:    "v1",
		ClusterFiles:          files,
		ModelID:               "model",
		InstructionBundleHash: "i1",
		PromptTemplateHash:    "p1",
		ScoringRubricHash:     "s1",
		OutputSchemaVersion:   "o1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if hit["hit"] != true {
		t.Fatalf("lookup miss: %+v", hit)
	}

	entry, _ := hit["file"].(string)
	outDir := filepath.Join(dir, "restored")
	restored, err := Restore(RestoreOptions{
		CacheDir:  cacheDir,
		EntryFile: entry,
		OutputDir: outDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if restored["restored"] != true {
		t.Fatalf("restore: %+v", restored)
	}
	if _, err := os.Stat(filepath.Join(outDir, "a-py.md")); err != nil {
		t.Fatal(err)
	}
}
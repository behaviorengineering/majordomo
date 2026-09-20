package contextdigest

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

// generatorReplayFixture is one captured Predict:<task> Process span.
type generatorReplayFixture struct {
	Source          string                 `json:"source"`
	Operation       string                 `json:"operation"`
	RepoID          string                 `json:"repo_id"`
	Task            string                 `json:"task"`
	Fields          map[string]interface{} `json:"fields"`
	RecordedOutputs map[string]interface{} `json:"recorded_outputs"`
}

func generatorReplayFixturePath(task string) string {
	return filepath.Join("testdata", "generator_replay", "gitboard_"+task+"_span.json")
}

func generatorReplayFixtureCandidates(task string) []string {
	out := []string{generatorReplayFixturePath(task)}
	for _, legacy := range jmodules.LegacyTaskAliases(task) {
		out = append(out, generatorReplayFixturePath(legacy))
	}
	// Reverse: when task is already a legacy id used in older fixture filenames.
	switch task {
	case jmodules.TaskTypologySliceGrouping:
		out = append(out, generatorReplayFixturePath(jmodules.LegacyTaskTypologyCluster))
	case jmodules.TaskTypologySliceCatalog:
		out = append(out, generatorReplayFixturePath(jmodules.LegacyTaskTypologyRefine))
	}
	seen := map[string]struct{}{}
	uniq := make([]string, 0, len(out))
	for _, p := range out {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		uniq = append(uniq, p)
	}
	return uniq
}

func loadGeneratorReplayFixture(t *testing.T, task string) (generatorReplayFixture, bool) {
	t.Helper()
	var lastErr error
	for _, path := range generatorReplayFixtureCandidates(task) {
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				lastErr = err
				continue
			}
			t.Fatalf("read fixture %s: %v", path, err)
		}
		var doc generatorReplayFixture
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("decode fixture %s: %v", path, err)
		}
		if strings.TrimSpace(doc.Task) == "" {
			doc.Task = task
		}
		if len(doc.Fields) == 0 {
			t.Fatalf("fixture %s fields empty", path)
		}
		return doc, true
	}
	_ = lastErr
	return generatorReplayFixture{}, false
}

func polypusReachable(t *testing.T) bool {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:1320/v1/models")
	if err != nil {
		t.Logf("polypus probe: %v", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func generatorReplayConfigDir(t *testing.T) string {
	t.Helper()
	if d := strings.TrimSpace(os.Getenv("MAJORDOMO_CENTRAL_CONFIG")); d != "" {
		return d
	}
	// contextdigest → majordomo root → tower tmp → tower root
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "majordomo-central-config"),
		filepath.Join("..", "..", "majordomo-central-config"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "_defaults.yaml")); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(c)
			if err != nil {
				return c
			}
			return abs
		}
	}
	t.Fatal("majordomo-central-config not found; set MAJORDOMO_CENTRAL_CONFIG")
	return ""
}

func truncateReplayLog(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func liveGeneratorReplayEnabled() bool {
	return os.Getenv("MAJORDOMO_LIVE_GENERATOR_REPLAY") == "1" ||
		os.Getenv("MAJORDOMO_LIVE_CLUSTER_REPLAY") == "1"
}

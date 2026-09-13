package contextdigest

import (
	"os"
	"path/filepath"
	"strings"
)

// rlmTraceDir returns a scratch TraceDir for strop RLM JSONL dumps (not teaching branch).
func rlmTraceDir(analysisDir, task string) string {
	task = strings.TrimSpace(task)
	if task == "" {
		task = "rlm"
	}
	base := strings.TrimSpace(analysisDir)
	if base == "" {
		base = "tmp"
	}
	dir := filepath.Join(base, "rlm-traces", task)
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// runreportDir returns a scratch directory for strop runreport JSON (not teaching branch).
func runreportDir(analysisDir string) string {
	base := strings.TrimSpace(analysisDir)
	if base == "" {
		base = "tmp"
	}
	dir := filepath.Join(base, "logs", "runs")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

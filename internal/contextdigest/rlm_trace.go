package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	stropdspy "github.com/behaviorengineering/strop/pkg/dspy"
)

// rlmTraceDir returns TraceDir for strop RLM JSONL dumps (not teaching branch).
// Prefer opts.WorkStoryDir so local digests keep traces after the analysis clone is removed.
func rlmTraceDir(workRoot, task string) string {
	task = strings.TrimSpace(task)
	if task == "" {
		task = "rlm"
	}
	base := strings.TrimSpace(workRoot)
	if base == "" {
		base = "tmp"
	}
	dir := filepath.Join(base, "rlm-traces", task)
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// moduleTraceDir returns the durable dump root for strop CoT/Predict TraceSession JSONL.
// Sibling of rlm-traces; InterceptorSetup TracingInterceptor writes here when a session is on ctx.
func moduleTraceDir(workRoot string) string {
	base := strings.TrimSpace(workRoot)
	if base == "" {
		base = "tmp"
	}
	dir := filepath.Join(base, "module-traces")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// runreportDir returns a directory for strop runreport JSON (not teaching branch).
func runreportDir(workRoot string) string {
	base := strings.TrimSpace(workRoot)
	if base == "" {
		base = "tmp"
	}
	dir := filepath.Join(base, "logs", "runs")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// inferenceWorkRoot prefers the durable AI work-story directory when set.
// Tests that omit WorkStoryDir keep traces under analysisDir (often t.TempDir()).
func inferenceWorkRoot(opts Options, analysisDir string) string {
	if s := strings.TrimSpace(opts.WorkStoryDir); s != "" {
		return s
	}
	return strings.TrimSpace(analysisDir)
}

// resolveWorkStoryDir picks a durable dump root for RLM traces, module traces, and runreports.
// Explicit opts.WorkStoryDir wins; else MAJORDOMO_DIGEST_WORK_STORY_DIR/<repo>-<ts>;
// else tmp/digest-runs/<repo>-<ts> under the process cwd.
func resolveWorkStoryDir(opts Options, now time.Time) (string, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	repo := strings.TrimSpace(opts.RepoID)
	if repo == "" {
		repo = "repo"
	}
	stamp := now.UTC().Format("20060102-150405Z")
	leaf := fmt.Sprintf("%s-%s", repo, stamp)

	if s := strings.TrimSpace(opts.WorkStoryDir); s != "" {
		return filepath.Clean(s), nil
	}
	if parent := strings.TrimSpace(os.Getenv("MAJORDOMO_DIGEST_WORK_STORY_DIR")); parent != "" {
		return filepath.Join(filepath.Clean(parent), leaf), nil
	}
	return filepath.Join("tmp", "digest-runs", leaf), nil
}

func ensureWorkStoryDir(opts Options, now time.Time) (string, error) {
	dir, err := resolveWorkStoryDir(opts, now)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("work story mkdir %s: %w", dir, err)
	}
	readme := filepath.Join(dir, "README.md")
	if _, err := os.Stat(readme); err != nil {
		body := "# Digest AI work story\n\n" +
			"Durable local dumps for testing AI (not the teaching context branch).\n\n" +
			"- `rlm-traces/<task>/` — strop RLM REPL JSONL (rlm-viewer)\n" +
			"- `module-traces/` — strop CoT/Predict TraceSession JSONL (module inputs/outputs)\n" +
			"- `logs/runs/` — strop runreport JSON for Judge/refine sessions\n\n" +
			"The analysis clone under OS temp is deleted when digest exits; this directory is kept.\n"
		if writeErr := os.WriteFile(readme, []byte(body), 0o644); writeErr != nil {
			return "", fmt.Errorf("work story readme: %w", writeErr)
		}
	}
	return dir, nil
}

// attachModuleTrace starts a durable CoT TraceSession on opts.Context under module-traces/.
// Returns a closer that MUST be deferred after prepareWorkStory in Run.
func attachModuleTrace(opts *Options) (func() error, error) {
	if opts == nil {
		return func() error { return nil }, fmt.Errorf("attach module trace: options is nil")
	}
	workRoot := strings.TrimSpace(opts.WorkStoryDir)
	if workRoot == "" {
		return func() error { return nil }, fmt.Errorf("attach module trace: WorkStoryDir is empty")
	}
	dir := moduleTraceDir(workRoot)
	repo := strings.TrimSpace(opts.RepoID)
	if repo == "" {
		repo = "repo"
	}
	newCtx, closeFn, err := stropdspy.AttachModuleTrace(opts.Context, dir, map[string]any{
		"pipeline": "context-digest",
		"repo_id":  repo,
	})
	if err != nil {
		return nil, err
	}
	opts.Context = newCtx
	logf("INFO", "digest module traces dir=%s (CoT/Predict TraceSession; survives analysis cleanup)", dir)
	return closeFn, nil
}

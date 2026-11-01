// Package sa runs staticAnalysis tools from central config against changed files.
package sa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/ops/process"
	"github.com/behaviorengineering/majordomo/pkg/platform/config"
	"github.com/behaviorengineering/majordomo/pkg/review/staging"
)

// ToolRunner executes run-sa-tool.sh (tests inject fakes).
type ToolRunner func(ctx context.Context, scriptPath, slug, image, command, repoRoot string, files []string) error

// Options configures a majordomo sa run.
type Options struct {
	Context     context.Context
	ConfigDir   string
	RepoID      string
	RepoRoot    string
	BaseBranch  string
	ScriptsDir  string
	ImagePrefix string
	// Runner overrides script execution (tests).
	Runner ToolRunner
	// ChangedFiles is injectable for tests. An empty value uses git diff via staging.SetupGit.
	ChangedFiles []string
	// Logger receives structured operator events.
	Logger Logger
}

// Logger receives a structured event.
type Logger interface {
	Log(level, message string, fields map[string]any) error
}

type stderrLogger struct{}

func (stderrLogger) Log(level, message string, fields map[string]any) error {
	record := map[string]any{"level": level, "message": message}
	for key, value := range fields {
		record[key] = value
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal log event: %w", err)
	}
	_, err = fmt.Fprintln(os.Stderr, string(data))
	return err
}

func logf(logger Logger, level, format string, args ...any) error {
	return logger.Log(level, fmt.Sprintf(format, args...), nil)
}

// Run executes configured staticAnalysis tools.
func Run(opts Options) error {
	if opts.Context == nil {
		return fmt.Errorf("sa: context is required")
	}
	if opts.ConfigDir == "" || opts.RepoID == "" {
		return fmt.Errorf("sa requires --config-dir and --repo-id")
	}
	if opts.BaseBranch == "" {
		return fmt.Errorf("sa requires --base-branch")
	}
	cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
	if err != nil {
		return err
	}
	if len(cfg.StaticAnalysis) == 0 {
		logger := opts.Logger
		if logger == nil {
			logger = stderrLogger{}
		}
		if err := logf(logger, "INFO", "no staticAnalysis tools configured for %s - skipping", opts.RepoID); err != nil {
			return fmt.Errorf("log static analysis configuration: %w", err)
		}
		return nil
	}
	logger := opts.Logger
	if logger == nil {
		logger = stderrLogger{}
	}

	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return err
	}

	files := opts.ChangedFiles
	if files == nil {
		setup, err := staging.SetupGit(opts.BaseBranch, "", repoRoot)
		if err != nil {
			return fmt.Errorf("list changed files: %w", err)
		}
		files = setup.AllFiles
	}
	if err := logf(logger, "INFO", "========== majordomo sa =========="); err != nil {
		return fmt.Errorf("log static analysis header: %w", err)
	}
	if err := logf(logger, "INFO", "repo %s: %d changed file(s), %d tool(s)", opts.RepoID, len(files), len(cfg.StaticAnalysis)); err != nil {
		return fmt.Errorf("log static analysis summary: %w", err)
	}

	scriptsDir := opts.ScriptsDir
	if scriptsDir == "" {
		scriptsDir, err = resolveScriptsDir(repoRoot)
		if err != nil {
			return err
		}
	}
	scriptPath := filepath.Join(scriptsDir, "run-sa-tool.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("run-sa-tool.sh not found at %s: %w", scriptPath, err)
	}

	runner := opts.Runner
	if runner == nil {
		runner = defaultToolRunner
	}

	for _, tool := range cfg.StaticAnalysis {
		slug := config.ResolveSAToolSlug(tool)
		matched := filterFiles(files, tool.Glob)
		if len(matched) == 0 {
			if err := logf(logger, "INFO", "skip %s: no files match %q", slug, tool.Glob); err != nil {
				return fmt.Errorf("log static analysis skip: %w", err)
			}
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			if err := logf(logger, "WARN", "skip %s: empty command", slug); err != nil {
				return fmt.Errorf("log static analysis warning: %w", err)
			}
			continue
		}
		if err := logf(logger, "INFO", "run %s image=%s files=%d", slug, image, len(matched)); err != nil {
			return fmt.Errorf("log static analysis run: %w", err)
		}
		if err := runner(opts.Context, scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			return fmt.Errorf("static analysis tool %s failed: %w", slug, err)
		}
	}
	return nil
}

func filterFiles(files []string, glob string) []string {
	glob = strings.TrimSpace(glob)
	if glob == "" {
		return append([]string(nil), files...)
	}
	var out []string
	for _, f := range files {
		if staging.MatchGlob(glob, f) {
			out = append(out, f)
		}
	}
	return out
}

func defaultToolRunner(ctx context.Context, scriptPath, slug, image, command, repoRoot string, files []string) error {
	args := append([]string{slug, image, command, repoRoot}, files...)
	result, err := process.Run(ctx, scriptPath, args, os.Environ(), repoRoot)
	if err != nil {
		return fmt.Errorf("run-sa-tool.sh %s: %w", slug, err)
	}
	if _, err := io.Copy(os.Stdout, strings.NewReader(result.Stdout)); err != nil {
		return fmt.Errorf("write run-sa-tool.sh output: %w", err)
	}
	if _, err := io.Copy(os.Stderr, strings.NewReader(result.Stderr)); err != nil {
		return fmt.Errorf("write run-sa-tool.sh error output: %w", err)
	}
	return nil
}

func resolveScriptsDir(repoRoot string) (string, error) {
	candidates := []string{
		filepath.Join(repoRoot, "pipelines", "scripts"),
		filepath.Join(repoRoot, ".majordomo", "pipelines", "scripts"),
	}
	if v := os.Getenv("MAJORDOMO_SCRIPTS"); v != "" {
		candidates = append([]string{v}, candidates...)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get current directory: %w", err)
	}
	dir := wd
	for i := 0; i < 8 && dir != ""; i++ {
		candidates = append(candidates,
			filepath.Join(dir, "pipelines", "scripts"),
			filepath.Join(dir, ".majordomo", "pipelines", "scripts"),
		)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "run-sa-tool.sh")); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("run-sa-tool.sh not found (set --scripts-dir or MAJORDOMO_SCRIPTS)")
}

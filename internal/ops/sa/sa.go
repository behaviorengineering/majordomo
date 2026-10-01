// Package sa runs staticAnalysis tools from central config against changed files.
package sa

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/ops/executil"
	"github.com/behaviorengineering/majordomo/pkg/platform/config"
	"github.com/behaviorengineering/majordomo/pkg/review/staging"
)

// ToolRunner executes run-sa-tool.sh (tests inject fakes).
type ToolRunner func(scriptPath, slug, image, command, repoRoot string, files []string) error

// Options configures a majordomo sa run.
type Options struct {
	ConfigDir   string
	RepoID      string
	RepoRoot    string
	BaseBranch  string
	ScriptsDir  string
	ImagePrefix string
	Context     context.Context
	Logger      *slog.Logger
	// Runner overrides script execution (tests).
	Runner ToolRunner
	// ChangedFiles injectable for tests; empty → git diff via staging.SetupGit.
	ChangedFiles []string
}

func logf(logger *slog.Logger, level, format string, args ...any) {
	slogLevel := slog.LevelInfo
	if level == "WARN" {
		slogLevel = slog.LevelWarn
	}
	logger.Log(context.Background(), slogLevel, fmt.Sprintf(format, args...), "level", level)
}

// Run executes configured staticAnalysis tools.
func Run(opts Options) error {
	if opts.Logger == nil {
		return errors.New("sa logger is required")
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
		logf(opts.Logger, "INFO", "no staticAnalysis tools configured for %s; skipping", opts.RepoID)
		return nil
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
	logf(opts.Logger, "INFO", "========== majordomo sa ==========")
	logf(opts.Logger, "INFO", "repo %s: %d changed file(s), %d tool(s)", opts.RepoID, len(files), len(cfg.StaticAnalysis))

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
		runner = func(scriptPath, slug, image, command, repoRoot string, files []string) error {
			return defaultToolRunner(opts.Context, scriptPath, slug, image, command, repoRoot, files)
		}
	}

	var failures []error
	for _, tool := range cfg.StaticAnalysis {
		slug := config.ResolveSAToolSlug(tool)
		matched := filterFiles(files, tool.Glob)
		if len(matched) == 0 {
			logf(opts.Logger, "INFO", "skip %s: no files match %q", slug, tool.Glob)
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			logf(opts.Logger, "WARN", "skip %s: empty command", slug)
			continue
		}
		logf(opts.Logger, "INFO", "run %s image=%s files=%d", slug, image, len(matched))
		if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			logf(opts.Logger, "WARN", "%s: %v (continuing)", slug, err)
			failures = append(failures, fmt.Errorf("%s: %w", slug, err))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
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
	_, _, err := executil.Run(ctx, scriptPath, args, os.Environ(), repoRoot)
	if err != nil {
		return fmt.Errorf("run-sa-tool.sh %s: %w", slug, err)
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
	wd, _ := os.Getwd()
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

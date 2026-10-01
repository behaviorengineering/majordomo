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

	"github.com/behaviorengineering/majordomo/internal/ops/process"
	"github.com/behaviorengineering/majordomo/pkg/platform/config"
	"github.com/behaviorengineering/majordomo/pkg/review/staging"
)

// ToolRunner executes run-sa-tool.sh (tests inject fakes).
type ToolRunner func(ctx context.Context, logger *slog.Logger, scriptPath, slug, image, command, repoRoot string, files []string) error

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
	// Logger receives structured operator events.
	Logger *slog.Logger
	// ChangedFiles injectable for tests; empty → git diff via staging.SetupGit.
	ChangedFiles []string
}

// Run executes configured staticAnalysis tools.
func Run(opts Options) error {
	if opts.Context == nil {
		return errors.New("sa requires a context")
	}
	if opts.Logger == nil {
		return errors.New("sa requires a logger")
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
		opts.Logger.Info("no static analysis tools configured", "repo_id", opts.RepoID)
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
	opts.Logger.Info("starting static analysis",
		"repo_id", opts.RepoID, "changed_files", len(files), "tools", len(cfg.StaticAnalysis))

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

	var toolErrors []error
	for _, tool := range cfg.StaticAnalysis {
		slug := config.ResolveSAToolSlug(tool)
		matched := filterFiles(files, tool.Glob)
		if len(matched) == 0 {
			opts.Logger.Info("skipping tool with no matching files",
				"repo_id", opts.RepoID, "tool", slug, "glob", tool.Glob)
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			opts.Logger.Warn("skipping tool with empty command",
				"repo_id", opts.RepoID, "tool", slug)
			continue
		}
		opts.Logger.Info("running static analysis tool",
			"repo_id", opts.RepoID, "tool", slug, "image", image, "files", len(matched))
		if err := runner(opts.Context, opts.Logger, scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			opts.Logger.Warn("static analysis tool failed",
				"repo_id", opts.RepoID, "tool", slug, "error", err)
			toolErrors = append(toolErrors, fmt.Errorf("%s: %w", slug, err))
		}
	}
	return errors.Join(toolErrors...)
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

func defaultToolRunner(ctx context.Context, logger *slog.Logger, scriptPath, slug, image, command, repoRoot string, files []string) error {
	args := append([]string{slug, image, command, repoRoot}, files...)
	result, err := process.Run(ctx, process.Options{
		Name: scriptPath,
		Args: args,
		Dir:  repoRoot,
	})
	if err != nil {
		return fmt.Errorf("run-sa-tool.sh %s: %w", slug, err)
	}
	if result.Stdout != "" {
		logger.Info("static analysis command output", "tool", slug, "stream", "stdout", "output", result.Stdout)
	}
	if result.Stderr != "" {
		logger.Warn("static analysis command output", "tool", slug, "stream", "stderr", "output", result.Stderr)
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
		return "", fmt.Errorf("get current working directory: %w", err)
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

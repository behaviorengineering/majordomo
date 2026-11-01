// Package sa runs staticAnalysis tools from central config against changed files.
package sa

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/ops/execx"
	"github.com/behaviorengineering/majordomo/pkg/platform/config"
	"github.com/behaviorengineering/majordomo/pkg/review/staging"
)

// ToolRunner executes run-sa-tool.sh (tests inject fakes).
type ToolRunner func(scriptPath, slug, image, command, repoRoot string, files []string) error

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
	// Logger receives structured static-analysis progress messages.
	Logger *slog.Logger
	// Executor overrides resilient external command execution.
	Executor *execx.Runner
	// ChangedFiles injectable for tests; empty → git diff via staging.SetupGit.
	ChangedFiles []string
}

// Run executes configured staticAnalysis tools.
func Run(opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
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
		logger.Info("no static analysis tools configured; skipping", "repo_id", opts.RepoID)
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
	logger.Info("static analysis run started", "repo_id", opts.RepoID, "changed_files", len(files), "tools", len(cfg.StaticAnalysis))

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
		if opts.Context == nil {
			return fmt.Errorf("sa requires a context")
		}
		if _, ok := opts.Context.Deadline(); !ok {
			return fmt.Errorf("sa requires a context deadline")
		}
		executor := opts.Executor
		if executor == nil {
			executor = execx.New()
		}
		runner = func(scriptPath, slug, image, command, repoRoot string, files []string) error {
			return defaultToolRunner(opts.Context, executor, scriptPath, slug, image, command, repoRoot, files)
		}
	}

	for _, tool := range cfg.StaticAnalysis {
		slug := config.ResolveSAToolSlug(tool)
		matched := filterFiles(files, tool.Glob)
		if len(matched) == 0 {
			logger.Info("skipping static analysis tool", "tool", slug, "reason", "no matching files", "glob", tool.Glob)
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			logger.Warn("skipping static analysis tool", "tool", slug, "reason", "empty command")
			continue
		}
		logger.Info("running static analysis tool", "tool", slug, "image", image, "files", len(matched))
		if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			logger.Warn("static analysis tool failed; continuing", "tool", slug, "error", err)
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

func defaultToolRunner(ctx context.Context, executor *execx.Runner, scriptPath, slug, image, command, repoRoot string, files []string) error {
	args := append([]string{slug, image, command, repoRoot}, files...)
	stdout, stderr, err := executor.Run(ctx, scriptPath, args, execx.Environment(), repoRoot)
	if stdout != "" {
		if _, writeErr := fmt.Fprint(os.Stdout, stdout); writeErr != nil {
			return fmt.Errorf("write SA tool output: %w", writeErr)
		}
	}
	if stderr != "" {
		if _, writeErr := fmt.Fprint(os.Stderr, stderr); writeErr != nil {
			return fmt.Errorf("write SA tool error output: %w", writeErr)
		}
	}
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
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve scripts directory: get working directory: %w", err)
	}
	for dir := wd; dir != ""; {
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

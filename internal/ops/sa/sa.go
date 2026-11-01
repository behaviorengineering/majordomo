// Package sa runs staticAnalysis tools from central config against changed files.
package sa

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/ops/command"
	"github.com/behaviorengineering/majordomo/pkg/platform/config"
	"github.com/behaviorengineering/majordomo/pkg/review/staging"
)

// ToolRunner executes run-sa-tool.sh. Tests inject fakes.
type ToolRunner func(context.Context, string, string, string, string, string, []string) error

// Options configures a majordomo SA run.
type Options struct {
	Context     context.Context
	ConfigDir   string
	RepoID      string
	RepoRoot    string
	BaseBranch  string
	ScriptsDir  string
	ImagePrefix string
	Logger      *slog.Logger
	// Runner overrides script execution.
	Runner ToolRunner
	// ChangedFiles is injectable for tests; empty means git diff via staging.SetupGit.
	ChangedFiles []string
}

// Run executes configured static-analysis tools.
func Run(opts Options) error {
	if opts.Context == nil {
		return errors.New("sa requires a context")
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
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if len(cfg.StaticAnalysis) == 0 {
		logger.Info("no static-analysis tools configured", "repo_id", opts.RepoID)
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
	logger.Info("static-analysis run started", "repo_id", opts.RepoID, "changed_files", len(files), "tools", len(cfg.StaticAnalysis))

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
			logger.Info("static-analysis tool skipped", "tool", slug, "reason", "no matching files", "glob", tool.Glob)
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			logger.Warn("static-analysis tool skipped", "tool", slug, "reason", "empty command")
			continue
		}
		logger.Info("static-analysis tool started", "tool", slug, "image", image, "files", len(matched))
		if err := runner(opts.Context, scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			logger.Warn("static-analysis tool failed", "tool", slug, "error", err)
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

func defaultToolRunner(ctx context.Context, scriptPath, slug, image, commandName, repoRoot string, files []string) error {
	args := append([]string{slug, image, commandName, repoRoot}, files...)
	result, err := command.Run(ctx, "static-analysis", scriptPath, args, nil, repoRoot, nil)
	if result.Stdout != "" {
		if _, writeErr := io.WriteString(os.Stdout, result.Stdout); writeErr != nil {
			err = errors.Join(err, fmt.Errorf("write static-analysis output: %w", writeErr))
		}
	}
	if result.Stderr != "" {
		if _, writeErr := io.WriteString(os.Stderr, result.Stderr); writeErr != nil {
			err = errors.Join(err, fmt.Errorf("write static-analysis errors: %w", writeErr))
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
		return "", fmt.Errorf("resolve scripts directory: %w", err)
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

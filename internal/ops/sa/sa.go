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

	operrors "github.com/behaviorengineering/majordomo/internal/ops/errors"
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
	// Logger receives structured operator messages.
	Logger *slog.Logger
	// ChangedFiles is injectable for tests; empty means use staging.SetupGit.
	ChangedFiles []string
}

func logf(ctx context.Context, logger *slog.Logger, level, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	switch level {
	case "WARN":
		logger.WarnContext(ctx, message)
	default:
		logger.InfoContext(ctx, message)
	}
}

// Run executes configured staticAnalysis tools.
func Run(opts Options) error {
	if opts.Context == nil {
		return operrors.New(operrors.CodeInvalidArgument, "sa.Run", "context is required")
	}
	if _, ok := opts.Context.Deadline(); !ok {
		return operrors.New(operrors.CodeInvalidArgument, "sa.Run", "context deadline is required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if opts.ConfigDir == "" || opts.RepoID == "" {
		return operrors.New(operrors.CodeInvalidArgument, "sa.Run", "config-dir and repo-id are required")
	}
	if opts.BaseBranch == "" {
		return operrors.New(operrors.CodeInvalidArgument, "sa.Run", "base-branch is required")
	}
	cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
	if err != nil {
		return operrors.Wrap(err, operrors.CodeInternal, "sa.Run", "load static-analysis config")
	}
	if len(cfg.StaticAnalysis) == 0 {
		logf(opts.Context, logger, "INFO", "no staticAnalysis tools configured for %s, skipping", opts.RepoID)
		return nil
	}

	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return operrors.Wrap(err, operrors.CodeInternal, "sa.Run", "get repository root")
		}
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return operrors.Wrap(err, operrors.CodeInvalidArgument, "sa.Run", "resolve repository root")
	}

	files := opts.ChangedFiles
	if files == nil {
		setup, err := staging.SetupGit(opts.BaseBranch, "", repoRoot)
		if err != nil {
			return operrors.Wrap(err, operrors.CodeInternal, "sa.Run", "list changed files")
		}
		files = setup.AllFiles
	}
	logf(opts.Context, logger, "INFO", "========== majordomo sa ==========")
	logf(opts.Context, logger, "INFO", "repo %s: %d changed file(s), %d tool(s)", opts.RepoID, len(files), len(cfg.StaticAnalysis))

	scriptsDir := opts.ScriptsDir
	if scriptsDir == "" {
		scriptsDir, err = resolveScriptsDir(repoRoot)
		if err != nil {
			return operrors.Wrap(err, operrors.CodeNotFound, "sa.Run", "resolve static-analysis scripts")
		}
	}
	scriptPath := filepath.Join(scriptsDir, "run-sa-tool.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return operrors.Wrap(err, operrors.CodeNotFound, "sa.Run", "run-sa-tool.sh not found at "+scriptPath)
	}

	runner := opts.Runner
	if runner == nil {
		runner = defaultToolRunner
	}

	var runErr error
	for _, tool := range cfg.StaticAnalysis {
		slug := config.ResolveSAToolSlug(tool)
		matched := filterFiles(files, tool.Glob)
		if len(matched) == 0 {
			logf(opts.Context, logger, "INFO", "skip %s: no files match %q", slug, tool.Glob)
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			logf(opts.Context, logger, "WARN", "skip %s: empty command", slug)
			continue
		}
		logf(opts.Context, logger, "INFO", "run %s image=%s files=%d", slug, image, len(matched))
		if err := runner(opts.Context, scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			logf(opts.Context, logger, "WARN", "%s: %v", slug, err)
			runErr = errors.Join(runErr, operrors.Wrap(err, operrors.CodeInternal, "sa.Run", "static-analysis tool "+slug))
		}
	}
	return runErr
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
	_, _, err := process.Run(ctx, process.Options{
		Name: scriptPath,
		Args: args,
		Dir:  repoRoot,
	})
	if err != nil {
		return operrors.Wrap(err, operrors.CodeInternal, "sa.defaultToolRunner", "run-sa-tool.sh "+slug)
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
	return "", operrors.New(operrors.CodeNotFound, "sa.resolveScriptsDir", "run-sa-tool.sh not found; set --scripts-dir or MAJORDOMO_SCRIPTS")
}

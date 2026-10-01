// Package sa runs staticAnalysis tools from central config against changed files.
package sa

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	opserrors "github.com/behaviorengineering/majordomo/internal/ops/errors"
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
	Logger      *slog.Logger
	// Runner overrides script execution (tests).
	Runner ToolRunner
	// ChangedFiles injectable for tests; empty → git diff via staging.SetupGit.
	ChangedFiles []string
}

// Run executes configured staticAnalysis tools.
func Run(opts Options) error {
	if opts.Context == nil {
		return opserrors.New(opserrors.CodeInvalidArgument, "sa.Run", "context is required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if opts.ConfigDir == "" || opts.RepoID == "" {
		return opserrors.New(opserrors.CodeInvalidArgument, "sa.Run", "config directory and repository ID are required")
	}
	if opts.BaseBranch == "" {
		return opserrors.New(opserrors.CodeInvalidArgument, "sa.Run", "base branch is required")
	}
	cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
	if err != nil {
		return opserrors.Wrap(err, opserrors.CodeConfiguration, "sa.LoadMerged", "load repository configuration")
	}
	if len(cfg.StaticAnalysis) == 0 {
		logger.Info("no static analysis tools configured", "repo_id", opts.RepoID)
		return nil
	}

	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return opserrors.Wrap(err, opserrors.CodeInternal, "sa.Getwd", "get working directory")
		}
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return opserrors.Wrap(err, opserrors.CodeInternal, "sa.Abs", "resolve repository root")
	}

	files := opts.ChangedFiles
	if files == nil {
		setup, err := staging.SetupGit(opts.BaseBranch, "", repoRoot)
		if err != nil {
			return opserrors.Wrap(err, opserrors.CodeExecution, "sa.SetupGit", "list changed files")
		}
		files = setup.AllFiles
	}
	logger.Info("starting static analysis",
		"repo_id", opts.RepoID,
		"changed_files", len(files),
		"tools", len(cfg.StaticAnalysis),
	)

	scriptsDir := opts.ScriptsDir
	if scriptsDir == "" {
		scriptsDir, err = resolveScriptsDir(repoRoot)
		if err != nil {
			return opserrors.Wrap(err, opserrors.CodeConfiguration, "sa.ResolveScriptsDir", "resolve scripts directory")
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

	var failures []error
	for _, tool := range cfg.StaticAnalysis {
		slug := config.ResolveSAToolSlug(tool)
		matched := filterFiles(files, tool.Glob)
		if len(matched) == 0 {
			logger.Info("skipping static analysis tool",
				"tool", slug,
				"reason", "no files match",
				"glob", tool.Glob,
			)
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			logger.Warn("skipping static analysis tool",
				"tool", slug,
				"reason", "empty command",
			)
			continue
		}
		logger.Info("running static analysis tool",
			"tool", slug,
			"image", image,
			"files", len(matched),
		)
		if err := runner(opts.Context, scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			logger.Warn("static analysis tool failed",
				"tool", slug,
				"error", err,
			)
			failures = append(failures, fmt.Errorf("%s: %w", slug, err))
		}
	}
	if len(failures) > 0 {
		return opserrors.Wrap(stderrors.Join(failures...), opserrors.CodeExecution, "sa.Run", "static analysis failed")
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
	result, err := process.Run(ctx, scriptPath, args, nil, repoRoot, process.Options{})
	if err != nil {
		return fmt.Errorf("run-sa-tool.sh %s: %w", slug, err)
	}
	if _, err := os.Stdout.WriteString(result.Stdout); err != nil {
		return fmt.Errorf("write %s stdout: %w", slug, err)
	}
	if _, err := os.Stderr.WriteString(result.Stderr); err != nil {
		return fmt.Errorf("write %s stderr: %w", slug, err)
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
		return "", fmt.Errorf("get working directory: %w", err)
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

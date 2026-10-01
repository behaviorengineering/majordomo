// Package sa runs staticAnalysis tools from central config against changed files.
package sa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/ops/command"
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
	// ProcessRunner overrides external process execution.
	ProcessRunner *command.Runner
	// ChangedFiles is injectable for tests; an empty slice reads the Git diff.
	ChangedFiles []string
	// Log writes command progress messages.
	Log func(level, format string, args ...any)
}

func defaultLog(level, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", level, fmt.Sprintf(format, args...))
}

// Run executes configured staticAnalysis tools.
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
	logger := opts.Log
	if logger == nil {
		logger = defaultLog
	}
	if len(cfg.StaticAnalysis) == 0 {
		logger("INFO", "no staticAnalysis tools configured for %s, skipping", opts.RepoID)
		return nil
	}

	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve repository root: %w", err)
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
	logger("INFO", "========== majordomo sa ==========")
	logger("INFO", "repo %s: %d changed file(s), %d tool(s)", opts.RepoID, len(files), len(cfg.StaticAnalysis))

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
		processRunner := opts.ProcessRunner
		if processRunner == nil {
			processRunner = command.NewRunner()
		}
		runner = func(ctx context.Context, scriptPath, slug, image, commandName, root string, matched []string) error {
			return defaultToolRunner(ctx, processRunner, scriptPath, slug, image, commandName, root, matched)
		}
	}

	var failures []error
	for _, tool := range cfg.StaticAnalysis {
		slug := config.ResolveSAToolSlug(tool)
		matched := filterFiles(files, tool.Glob)
		if len(matched) == 0 {
			logger("INFO", "skip %s: no files match %q", slug, tool.Glob)
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			logger("WARN", "skip %s: empty command", slug)
			continue
		}
		logger("INFO", "run %s image=%s files=%d", slug, image, len(matched))
		if err := runner(opts.Context, scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			logger("ERROR", "%s failed: %v", slug, err)
			failures = append(failures, fmt.Errorf("%s: %w", slug, err))
		}
	}
	return errors.Join(failures...)
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

func defaultToolRunner(ctx context.Context, processRunner *command.Runner, scriptPath, slug, image, commandName, repoRoot string, files []string) error {
	args := append([]string{slug, image, commandName, repoRoot}, files...)
	_, _, err := processRunner.Run(ctx, command.Request{
		Name:       scriptPath,
		Args:       args,
		Dir:        repoRoot,
		Dependency: "static-analysis",
	})
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

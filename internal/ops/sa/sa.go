// Package sa runs staticAnalysis tools from central config against changed files.
package sa

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	Output      io.Writer
	// Runner overrides script execution (tests).
	Runner ToolRunner
	// ChangedFiles is injectable for tests. An empty value uses git diff via staging.SetupGit.
	ChangedFiles []string
}

func logStaticAnalysis(output io.Writer, level, format string, args ...any) error {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := fmt.Fprintf(output, "[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...)); err != nil {
		return fmt.Errorf("write static-analysis output: %w", err)
	}
	return nil
}

// Run executes configured staticAnalysis tools.
func Run(opts Options) error {
	if opts.Context == nil {
		return fmt.Errorf("sa requires context")
	}
	if _, ok := opts.Context.Deadline(); !ok {
		return fmt.Errorf("sa requires a context deadline")
	}
	if opts.Output == nil {
		return fmt.Errorf("sa requires output")
	}
	if opts.ConfigDir == "" || opts.RepoID == "" {
		return fmt.Errorf("sa requires --config-dir and --repo-id")
	}
	if opts.BaseBranch == "" {
		return fmt.Errorf("sa requires --base-branch")
	}
	cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
	if err != nil {
		return fmt.Errorf("load static-analysis config: %w", err)
	}
	if len(cfg.StaticAnalysis) == 0 {
		if err := logStaticAnalysis(opts.Output, "INFO", "no staticAnalysis tools configured for %s — skipping", opts.RepoID); err != nil {
			return err
		}
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
		return fmt.Errorf("resolve absolute repository root: %w", err)
	}

	files := opts.ChangedFiles
	if files == nil {
		setup, err := staging.SetupGit(opts.BaseBranch, "", repoRoot)
		if err != nil {
			return fmt.Errorf("list changed files: %w", err)
		}
		files = setup.AllFiles
	}
	if err := logStaticAnalysis(opts.Output, "INFO", "========== majordomo sa =========="); err != nil {
		return err
	}
	if err := logStaticAnalysis(opts.Output, "INFO", "repo %s: %d changed file(s), %d tool(s)", opts.RepoID, len(files), len(cfg.StaticAnalysis)); err != nil {
		return err
	}

	scriptsDir := opts.ScriptsDir
	if scriptsDir == "" {
		scriptsDir, err = resolveScriptsDir(repoRoot)
		if err != nil {
			return fmt.Errorf("resolve static-analysis scripts: %w", err)
		}
	}
	scriptPath := filepath.Join(scriptsDir, "run-sa-tool.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("run-sa-tool.sh not found at %s: %w", scriptPath, err)
	}

	runner := opts.Runner
	if runner == nil {
		runner = func(scriptPath, slug, image, command, repoRoot string, files []string) error {
			return defaultToolRunner(opts.Context, opts.Output, scriptPath, slug, image, command, repoRoot, files)
		}
	}

	var toolErrors []error
	for _, tool := range cfg.StaticAnalysis {
		slug := config.ResolveSAToolSlug(tool)
		matched := filterFiles(files, tool.Glob)
		if len(matched) == 0 {
			if err := logStaticAnalysis(opts.Output, "INFO", "skip %s: no files match %q", slug, tool.Glob); err != nil {
				return err
			}
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			if err := logStaticAnalysis(opts.Output, "WARN", "skip %s: empty command", slug); err != nil {
				return err
			}
			continue
		}
		if err := logStaticAnalysis(opts.Output, "INFO", "run %s image=%s files=%d", slug, image, len(matched)); err != nil {
			return err
		}
		if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			if logErr := logStaticAnalysis(opts.Output, "WARN", "%s: %v (continuing)", slug, err); logErr != nil {
				return logErr
			}
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

func defaultToolRunner(ctx context.Context, output io.Writer, scriptPath, slug, image, command, repoRoot string, files []string) error {
	args := append([]string{slug, image, command, repoRoot}, files...)
	stdout, stderr, err := executil.Run(executil.Options{
		Context:   ctx,
		Name:      scriptPath,
		Args:      args,
		Env:       os.Environ(),
		Dir:       repoRoot,
		Networked: true,
	})
	if _, writeErr := io.WriteString(output, stdout+stderr); writeErr != nil {
		return fmt.Errorf("write static-analysis tool output: %w", writeErr)
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
		return "", fmt.Errorf("get current directory while resolving scripts: %w", err)
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

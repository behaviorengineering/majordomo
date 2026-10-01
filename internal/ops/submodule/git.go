// Package submodule ports scripts/submodule.py: interactive .majordomo manager.
package submodule

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/ops/process"
)

const (
	pipelinesBranch = "pipelines"
	worktreeDir     = ".pipelines-worktree"
)

// Options configures the interactive submodule manager.
type Options struct {
	// Context bounds Git and prompt operations.
	Context context.Context
	// StartDir is used to locate the submodule root (default: cwd).
	StartDir string
	// In/Out for prompts (tests).
	In  io.Reader
	Out io.Writer
	// GitRunner overrides git execution (tests). nil → real git.
	GitRunner func(args []string, cwd string, check bool) (string, error)
	// Prompt reads a line after printing prompt (tests). nil → stdin.
	Prompt func(prompt string) (string, error)
	// ReadKey reads a single choice character (tests). nil → line-based fallback.
	ReadKey func(prompt string) (string, error)
	// ProcessExecutor overrides resilient Git execution.
	ProcessExecutor *process.Executor
}

type manager struct {
	opts          Options
	submoduleRoot string
	parentRoot    string // empty if none.
	submoduleName string
	executor      *process.Executor
	outputErr     error
}

// Run launches the interactive submodule manager.
func Run(opts Options) error {
	if opts.Context == nil {
		return fmt.Errorf("submodule context is required")
	}
	if opts.ProcessExecutor == nil {
		opts.ProcessExecutor = process.NewExecutor()
	}
	m := &manager{opts: opts}
	m.executor = opts.ProcessExecutor
	root, err := m.findSubmoduleRoot()
	if err != nil {
		return fmt.Errorf("submodule.findSubmoduleRoot: %w", err)
	}
	m.submoduleRoot = root
	m.parentRoot = m.findParentRepoRoot(root)
	m.submoduleName = m.getSubmoduleName()

	if m.parentRoot != "" {
		parentBranch, err := m.currentBranch(m.parentRoot)
		if err != nil {
			return fmt.Errorf("submodule.currentBranch: %w", err)
		}
		if parentBranch != pipelinesBranch {
			if err := m.promptOffBranchContext(parentBranch); err != nil {
				return fmt.Errorf("submodule.promptOffBranchContext: %w", err)
			}
			return m.outputErr
		}
	}
	if err := m.opsMenuLoop(); err != nil {
		return fmt.Errorf("submodule.opsMenuLoop: %w", err)
	}
	return m.outputErr
}

func (m *manager) findSubmoduleRoot() (string, error) {
	start := m.opts.StartDir
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	out, err := m.git([]string{"rev-parse", "--show-toplevel"}, start, true)
	if err != nil {
		return "", fmt.Errorf("could not determine submodule root — not inside a git repo")
	}
	return out, nil
}

func (m *manager) findParentRepoRoot(submoduleRoot string) string {
	parentCand := filepath.Dir(submoduleRoot)
	out, err := m.git([]string{"rev-parse", "--show-toplevel"}, parentCand, true)
	if err != nil {
		return ""
	}
	parent := out
	if parent == submoduleRoot {
		return ""
	}
	rel, err := filepath.Rel(parent, submoduleRoot)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	indexEntry, err := m.git([]string{"ls-files", "--stage", rel}, parent, false)
	if err != nil {
		return ""
	}
	if strings.HasPrefix(indexEntry, "160000") {
		return parent
	}
	gitDirRaw, err := m.git([]string{"rev-parse", "--git-dir"}, parent, false)
	if err != nil {
		return ""
	}
	if gitDirRaw == "" {
		return ""
	}
	gitDir := gitDirRaw
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(parent, gitDirRaw)
	}
	if st, err := os.Stat(filepath.Join(gitDir, "modules", rel)); err == nil && st.IsDir() {
		return parent
	}
	return ""
}

func (m *manager) getSubmoduleName() string {
	if m.parentRoot == "" {
		return filepath.Base(m.submoduleRoot)
	}
	rel, err := filepath.Rel(m.parentRoot, m.submoduleRoot)
	if err != nil {
		return filepath.Base(m.submoduleRoot)
	}
	return filepath.ToSlash(rel)
}

func (m *manager) isGitlinkInIndex(submoduleName string) bool {
	if m.parentRoot == "" {
		return false
	}
	entry, err := m.git([]string{"ls-files", "--stage", submoduleName}, m.parentRoot, false)
	if err != nil {
		return false
	}
	return strings.HasPrefix(entry, "160000")
}

func (m *manager) currentBranch(repoRoot string) (string, error) {
	out, err := m.git([]string{"symbolic-ref", "--short", "HEAD"}, repoRoot, true)
	if err != nil {
		return "(detached HEAD)", nil
	}
	return out, nil
}

func (m *manager) currentSHA(repoRoot string) (string, error) {
	return m.git([]string{"rev-parse", "--short", "HEAD"}, repoRoot, true)
}

func (m *manager) isDirty(repoRoot string) bool {
	out, err := m.git([]string{"status", "--porcelain"}, repoRoot, false)
	if err != nil {
		return true
	}
	return strings.TrimSpace(out) != ""
}

func (m *manager) gitDir(repoRoot string) (string, error) {
	gitDir, err := m.git([]string{"rev-parse", "--git-dir"}, repoRoot, true)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(gitDir) {
		return gitDir, nil
	}
	return filepath.Join(repoRoot, gitDir), nil
}

func (m *manager) resetWorkingTree(repoRoot string) error {
	if _, err := m.git([]string{"reset", "--hard", "HEAD"}, repoRoot, true); err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}
	_, err := m.git([]string{"clean", "-fd"}, repoRoot, true)
	return err
}

func (m *manager) confirmAndReset() (bool, error) {
	if !m.isDirty(m.submoduleRoot) {
		return true, nil
	}
	m.printf("Warning: submodule has local modifications (possibly from a force push).\n")
	raw, err := m.prompt("Discard local changes and reset to HEAD? (y/N): ")
	if err != nil {
		return false, err
	}
	if strings.ToLower(strings.TrimSpace(raw)) != "y" {
		m.printf("Cancelled — local changes preserved.\n")
		return false, nil
	}
	if err := m.resetWorkingTree(m.submoduleRoot); err != nil {
		return false, err
	}
	return true, nil
}

func (m *manager) remoteTrackingSHA(branch string) string {
	out, err := m.git([]string{"rev-parse", "--verify", "origin/" + branch}, m.submoduleRoot, false)
	if err != nil {
		return ""
	}
	return out
}

func (m *manager) remoteBranches() ([]string, error) {
	m.printf("Fetching remote branches...\n")
	if _, err := m.git([]string{"fetch", "--prune"}, m.submoduleRoot, true); err != nil {
		return nil, fmt.Errorf("git fetch failed: %w", err)
	}
	raw, err := m.git([]string{"branch", "-r"}, m.submoduleRoot, true)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var branches []string
	for _, line := range strings.Split(raw, "\n") {
		stripped := strings.TrimSpace(line)
		if stripped == "" || strings.Contains(stripped, "HEAD") {
			continue
		}
		name := strings.TrimPrefix(stripped, "origin/")
		if !seen[name] {
			seen[name] = true
			branches = append(branches, name)
		}
	}
	return branches, nil
}

func (m *manager) git(args []string, cwd string, check bool) (string, error) {
	if m.opts.GitRunner != nil {
		return m.opts.GitRunner(args, cwd, check)
	}
	if m.executor == nil {
		return "", errors.New("submodule process executor is required")
	}
	var stdout, stderr bytes.Buffer
	err := m.executor.Run(m.opts.Context, process.Spec{
		Name:       "git",
		Args:       args,
		Dir:        cwd,
		Stdout:     &stdout,
		Stderr:     &stderr,
		Dependency: gitDependency(args),
		Retryable:  gitRetryable(args, &stderr),
	})
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		if !check {
			return out, nil
		}
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func gitDependency(args []string) string {
	if len(args) > 0 {
		switch args[0] {
		case "fetch", "pull", "push":
			return "git-remote"
		}
	}
	return "git-local"
}

func gitRetryable(args []string, stderr *bytes.Buffer) func(error) bool {
	if gitDependency(args) != "git-remote" {
		return nil
	}
	return func(err error) bool {
		if err == nil {
			return false
		}
		text := strings.ToLower(stderr.String() + " " + err.Error())
		for _, marker := range []string{
			"connection reset",
			"connection timed out",
			"could not resolve host",
			"network is unreachable",
			"temporary failure",
			"timed out",
			"502",
			"503",
			"429",
		} {
			if strings.Contains(text, marker) {
				return true
			}
		}
		return false
	}
}

func (m *manager) printf(format string, args ...any) {
	if m.outputErr != nil {
		return
	}
	_, m.outputErr = fmt.Fprintf(m.out(), format, args...)
}

func (m *manager) out() io.Writer {
	if m.opts.Out != nil {
		return m.opts.Out
	}
	return os.Stdout
}

func (m *manager) prompt(p string) (string, error) {
	if m.opts.Prompt != nil {
		return m.opts.Prompt(p)
	}
	m.printf("%s", p)
	in := m.opts.In
	if in == nil {
		in = os.Stdin
	}
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", nil
	}
	return scanner.Text(), nil
}

func (m *manager) readKey(p string) (string, error) {
	if m.opts.ReadKey != nil {
		return m.opts.ReadKey(p)
	}
	line, err := m.prompt(p)
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return "", nil
	}
	return string(line[0]), nil
}

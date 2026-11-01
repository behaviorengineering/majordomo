// Package submodule ports scripts/submodule.py: interactive .majordomo manager.
package submodule

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	operrors "github.com/behaviorengineering/majordomo/internal/ops/errors"
	"github.com/behaviorengineering/majordomo/internal/ops/process"
)

const (
	pipelinesBranch = "pipelines"
	worktreeDir     = ".pipelines-worktree"
)

// Options configures the interactive submodule manager.
type Options struct {
	// StartDir is used to locate the submodule root (default: cwd).
	StartDir string
	// Context carries the caller deadline for Git operations.
	Context context.Context
	// In/Out for prompts (tests).
	In  io.Reader
	Out io.Writer
	// GitRunner overrides git execution (tests). Nil means real git.
	GitRunner func(args []string, cwd string, check bool) (string, error)
	// Prompt reads a line after printing prompt (tests). Nil means stdin.
	Prompt func(prompt string) (string, error)
	// ReadKey reads a single choice character (tests). Nil means line-based fallback.
	ReadKey func(prompt string) (string, error)
}

type manager struct {
	opts          Options
	submoduleRoot string
	parentRoot    string // Empty if none.
	submoduleName string
	outputErr     error
}

// Run launches the interactive submodule manager.
func Run(opts Options) error {
	m := &manager{opts: opts}
	root, err := m.findSubmoduleRoot()
	if err != nil {
		return operrors.Wrap(err, operrors.CodeInvalidArgument, "submodule.Run", "find submodule root")
	}
	m.submoduleRoot = root
	m.parentRoot = m.findParentRepoRoot(root)
	m.submoduleName = m.getSubmoduleName()

	if m.parentRoot != "" {
		parentBranch, err := m.currentBranch(m.parentRoot)
		if err != nil {
			return operrors.Wrap(err, operrors.CodeInternal, "submodule.Run", "resolve parent branch")
		}
		if parentBranch != pipelinesBranch {
			if err := m.promptOffBranchContext(parentBranch); err != nil {
				return operrors.Wrap(err, operrors.CodeInternal, "submodule.Run", "handle off-branch context")
			}
			if err := m.outputError(); err != nil {
				return err
			}
			return nil
		}
	}
	if err := m.opsMenuLoop(); err != nil {
		return operrors.Wrap(err, operrors.CodeInternal, "submodule.Run", "run submodule manager")
	}
	if err := m.outputError(); err != nil {
		return err
	}
	return nil
}

func (m *manager) findSubmoduleRoot() (string, error) {
	start := m.opts.StartDir
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", operrors.Wrap(err, operrors.CodeInternal, "submodule.findSubmoduleRoot", "get current directory")
		}
	}
	out, err := m.git([]string{"rev-parse", "--show-toplevel"}, start, true)
	if err != nil {
		return "", operrors.Wrap(err, operrors.CodeInvalidArgument, "submodule.findSubmoduleRoot", "not inside a git repository")
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
	indexEntry, _ := m.git([]string{"ls-files", "--stage", rel}, parent, false)
	if strings.HasPrefix(indexEntry, "160000") {
		return parent
	}
	gitDirRaw, _ := m.git([]string{"rev-parse", "--git-dir"}, parent, false)
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
	entry, _ := m.git([]string{"ls-files", "--stage", submoduleName}, m.parentRoot, false)
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

func (m *manager) isDirty(repoRoot string) (bool, error) {
	out, err := m.git([]string{"status", "--porcelain"}, repoRoot, true)
	if err != nil {
		return false, operrors.Wrap(err, operrors.CodeInternal, "submodule.isDirty", "git status failed")
	}
	return strings.TrimSpace(out) != "", nil
}

func (m *manager) gitDir(repoRoot string) (string, error) {
	gitDir, err := m.git([]string{"rev-parse", "--git-dir"}, repoRoot, true)
	if err != nil {
		return "", operrors.Wrap(err, operrors.CodeInternal, "submodule.gitDir", "resolve Git directory")
	}
	if filepath.IsAbs(gitDir) {
		return gitDir, nil
	}
	return filepath.Join(repoRoot, gitDir), nil
}

func (m *manager) resetWorkingTree(repoRoot string) error {
	if _, err := m.git([]string{"reset", "--hard", "HEAD"}, repoRoot, true); err != nil {
		return operrors.Wrap(err, operrors.CodeInternal, "submodule.resetWorkingTree", "git reset failed")
	}
	if _, err := m.git([]string{"clean", "-fd"}, repoRoot, true); err != nil {
		return operrors.Wrap(err, operrors.CodeInternal, "submodule.resetWorkingTree", "git clean failed")
	}
	return nil
}

func (m *manager) confirmAndReset() (bool, error) {
	dirty, err := m.isDirty(m.submoduleRoot)
	if err != nil {
		return false, err
	}
	if !dirty {
		return true, nil
	}
	m.printf("Warning: submodule has local modifications (possibly from a force push).\n")
	raw, err := m.prompt("Discard local changes and reset to HEAD? (y/N): ")
	if err != nil {
		return false, err
	}
	if strings.ToLower(strings.TrimSpace(raw)) != "y" {
		m.printf("Cancelled, local changes preserved.\n")
		return false, nil
	}
	if err := m.resetWorkingTree(m.submoduleRoot); err != nil {
		return false, err
	}
	return true, nil
}

func (m *manager) remoteTrackingSHA(branch string) (string, error) {
	out, err := m.git([]string{"rev-parse", "--verify", "origin/" + branch}, m.submoduleRoot, true)
	if err != nil {
		return "", operrors.Wrap(err, operrors.CodeNotFound, "submodule.remoteTrackingSHA", "resolve remote tracking branch")
	}
	return out, nil
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
	if m.opts.Context == nil {
		return "", operrors.New(operrors.CodeInvalidArgument, "submodule.git", "context is required")
	}
	stdout, stderr, err := process.Run(m.opts.Context, process.Options{
		Name:      "git",
		Args:      args,
		Dir:       cwd,
		Retryable: retryableGitCommand(args),
	})
	out := strings.TrimSpace(stdout)
	if err != nil {
		if !check {
			return out, nil
		}
		message := "Git command failed"
		if trimmed := strings.TrimSpace(stderr); trimmed != "" {
			message += ": " + trimmed
		}
		return out, operrors.Wrap(err, operrors.CodeInternal, "submodule.git", message)
	}
	return out, nil
}

func retryableGitCommand(args []string) func(error) bool {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "fetch", "push":
		return process.RetryableProcessError
	default:
		return nil
	}
}

func (m *manager) printf(format string, args ...any) {
	if m.outputErr != nil {
		return
	}
	_, m.outputErr = fmt.Fprintf(m.out(), format, args...)
}

func (m *manager) outputError() error {
	if m.outputErr == nil {
		return nil
	}
	return operrors.Wrap(m.outputErr, operrors.CodeInternal, "submodule.output", "write operator output")
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
	if err := m.outputError(); err != nil {
		return "", err
	}
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

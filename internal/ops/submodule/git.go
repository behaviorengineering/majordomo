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

	"github.com/behaviorengineering/majordomo/internal/ops/executil"
)

const (
	pipelinesBranch = "pipelines"
	worktreeDir     = ".pipelines-worktree"
)

// Options configures the interactive submodule manager.
type Options struct {
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
}

type manager struct {
	opts          Options
	submoduleRoot string
	parentRoot    string // Empty when no parent repository exists.
	submoduleName string
	outputErr     error
}

// Run launches the interactive submodule manager.
func Run(opts Options) error {
	m := &manager{opts: opts}
	root, err := m.findSubmoduleRoot()
	if err != nil {
		return err
	}
	m.submoduleRoot = root
	m.parentRoot, err = m.findParentRepoRoot(root)
	if err != nil {
		return err
	}
	m.submoduleName = m.getSubmoduleName()

	if m.parentRoot != "" {
		parentBranch, err := m.currentBranch(m.parentRoot)
		if err != nil {
			return err
		}
		if parentBranch != pipelinesBranch {
			return m.promptOffBranchContext(parentBranch)
		}
	}
	if err := m.opsMenuLoop(); err != nil {
		return err
	}
	if m.outputErr != nil {
		return fmt.Errorf("write submodule output: %w", m.outputErr)
	}
	return nil
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
		return "", fmt.Errorf("could not determine submodule root: %w", err)
	}
	return out, nil
}

func (m *manager) findParentRepoRoot(submoduleRoot string) (string, error) {
	parentCand := filepath.Dir(submoduleRoot)
	out, err := m.git([]string{"rev-parse", "--show-toplevel"}, parentCand, true)
	if err != nil {
		if isNotGitRepository(err) {
			return "", nil
		}
		return "", fmt.Errorf("find parent repository: %w", err)
	}
	parent := out
	if parent == submoduleRoot {
		return "", nil
	}
	rel, err := filepath.Rel(parent, submoduleRoot)
	if err != nil {
		return "", fmt.Errorf("find submodule path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	indexEntry, err := m.git([]string{"ls-files", "--stage", rel}, parent, true)
	if err != nil {
		return "", fmt.Errorf("inspect parent gitlink: %w", err)
	}
	if strings.HasPrefix(indexEntry, "160000") {
		return parent, nil
	}
	gitDirRaw, err := m.git([]string{"rev-parse", "--git-dir"}, parent, true)
	if err != nil {
		return "", fmt.Errorf("find parent git directory: %w", err)
	}
	if gitDirRaw == "" {
		return "", nil
	}
	gitDir := gitDirRaw
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(parent, gitDirRaw)
	}
	if st, err := os.Stat(filepath.Join(gitDir, "modules", rel)); err == nil && st.IsDir() {
		return parent, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect parent submodule metadata: %w", err)
	}
	return "", nil
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

func (m *manager) isGitlinkInIndex(submoduleName string) (bool, error) {
	if m.parentRoot == "" {
		return false, nil
	}
	entry, err := m.git([]string{"ls-files", "--stage", submoduleName}, m.parentRoot, true)
	if err != nil {
		return false, fmt.Errorf("inspect parent gitlink: %w", err)
	}
	return strings.HasPrefix(entry, "160000"), nil
}

func (m *manager) currentBranch(repoRoot string) (string, error) {
	out, err := m.git([]string{"symbolic-ref", "--short", "HEAD"}, repoRoot, true)
	if err != nil {
		if isDetachedHead(err) {
			return "(detached HEAD)", nil
		}
		return "", fmt.Errorf("read current branch: %w", err)
	}
	return out, nil
}

func (m *manager) currentSHA(repoRoot string) (string, error) {
	return m.git([]string{"rev-parse", "--short", "HEAD"}, repoRoot, true)
}

func (m *manager) isDirty(repoRoot string) (bool, error) {
	out, err := m.git([]string{"status", "--porcelain"}, repoRoot, true)
	if err != nil {
		return false, fmt.Errorf("read repository status: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
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
		m.printf("Cancelled: local changes preserved.\n")
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
		if isMissingRef(err) {
			return "", nil
		}
		return "", fmt.Errorf("read remote tracking SHA: %w", err)
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
	stdout, stderr, err := executil.Run(m.opts.Context, "git", args, nil, cwd)
	out := strings.TrimSpace(stdout)
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
	}
	return out, nil
}

func isNotGitRepository(err error) bool {
	return strings.Contains(err.Error(), "not a git repository")
}

func isDetachedHead(err error) bool {
	return strings.Contains(err.Error(), "is not a symbolic ref")
}

func isMissingRef(err error) bool {
	message := err.Error()
	return strings.Contains(message, "needed a single revision") ||
		strings.Contains(message, "unknown revision")
}

func isNothingToCommit(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "nothing to commit") ||
		strings.Contains(message, "no changes added to commit")
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

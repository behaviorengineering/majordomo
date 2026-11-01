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

	"github.com/behaviorengineering/majordomo/internal/ops/command"
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
	parentRoot    string // empty if none
	submoduleName string
	outputErr     error
}

// Run launches the interactive submodule manager.
func Run(opts Options) error {
	if opts.Context == nil {
		return fmt.Errorf("submodule requires a context")
	}
	if _, ok := opts.Context.Deadline(); !ok {
		return fmt.Errorf("submodule requires a context deadline")
	}
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

func (m *manager) findParentRepoRoot(submoduleRoot string) (string, error) {
	parentCand := filepath.Dir(submoduleRoot)
	out, err := m.git([]string{"rev-parse", "--show-toplevel"}, parentCand, true)
	if err != nil {
		if isNotGitRepoError(err) {
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
		return "", fmt.Errorf("resolve submodule path: %w", err)
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
	gitDir := gitDirRaw
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(parent, gitDirRaw)
	}
	modulePath := filepath.Join(gitDir, "modules", rel)
	if st, err := os.Stat(modulePath); err == nil {
		if st.IsDir() {
			return parent, nil
		}
	} else if !os.IsNotExist(err) {
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
		return false, fmt.Errorf("check repository status: %w", err)
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
		m.printf("Cancelled — local changes preserved.\n")
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
		return "", fmt.Errorf("resolve origin/%s: %w", branch, err)
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
	result, err := command.Run(command.Options{
		Context: m.opts.Context,
		Name:    "git",
		Args:    args,
		Dir:     cwd,
		Network: isNetworkGitCommand(args),
	})
	out := strings.TrimSpace(result.Stdout)
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(result.Stderr))
	}
	return out, nil
}

func isNetworkGitCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "fetch", "pull", "push", "clone":
		return true
	default:
		return false
	}
}

func isNotGitRepoError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "not a git repository")
}

func isNothingToCommitError(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "nothing to commit") ||
		strings.Contains(text, "nothing added to commit")
}

func isMissingGitReferenceError(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "needed a single revision") ||
		strings.Contains(text, "unknown revision") ||
		strings.Contains(text, "bad object")
}

func (m *manager) printf(format string, args ...any) {
	if m.outputErr != nil {
		return
	}
	if _, err := fmt.Fprintf(m.out(), format, args...); err != nil {
		m.outputErr = fmt.Errorf("write operator output: %w", err)
	}
}

func (m *manager) out() io.Writer {
	if m.opts.Out != nil {
		return m.opts.Out
	}
	return os.Stdout
}

func (m *manager) prompt(p string) (string, error) {
	if m.outputErr != nil {
		return "", m.outputErr
	}
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
	if m.outputErr != nil {
		return "", m.outputErr
	}
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

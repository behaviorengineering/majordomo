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
	// StartDir is used to locate the submodule root (default: cwd).
	StartDir string
	// Context bounds local and remote git operations.
	Context context.Context
	// In and Out are prompt streams for tests.
	In  io.Reader
	Out io.Writer
	// GitRunner overrides git execution in tests. A nil value runs git.
	GitRunner func(args []string, cwd string, check bool) (string, error)
	// Prompt reads a line after printing the prompt in tests. A nil value uses stdin.
	Prompt func(prompt string) (string, error)
	// ReadKey reads a single choice character in tests. A nil value uses a line-based fallback.
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
	m := &manager{opts: opts}
	if opts.GitRunner == nil {
		if opts.Context == nil {
			return fmt.Errorf("submodule requires context")
		}
		if _, ok := opts.Context.Deadline(); !ok {
			return fmt.Errorf("submodule requires a context deadline")
		}
	}
	root, err := m.findSubmoduleRoot()
	if err != nil {
		return fmt.Errorf("find submodule root: %w", err)
	}
	m.submoduleRoot = root
	m.parentRoot = m.findParentRepoRoot(root)
	m.submoduleName = m.getSubmoduleName()

	if m.parentRoot != "" {
		parentBranch, err := m.currentBranch(m.parentRoot)
		if err != nil {
			return fmt.Errorf("read parent branch: %w", err)
		}
		if parentBranch != pipelinesBranch {
			if err := m.promptOffBranchContext(parentBranch); err != nil {
				return fmt.Errorf("handle off-branch context: %w", err)
			}
			return m.outputErr
		}
	}
	if err := m.opsMenuLoop(); err != nil {
		return fmt.Errorf("run submodule menu: %w", err)
	}
	return m.outputErr
}

func (m *manager) findSubmoduleRoot() (string, error) {
	start := m.opts.StartDir
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get current directory: %w", err)
		}
	}
	out, err := m.git([]string{"rev-parse", "--show-toplevel"}, start, true)
	if err != nil {
		return "", fmt.Errorf("could not determine submodule root: %w", err)
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
	sha, err := m.git([]string{"rev-parse", "--short", "HEAD"}, repoRoot, true)
	if err != nil {
		return "", fmt.Errorf("read current commit: %w", err)
	}
	return sha, nil
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
		return "", fmt.Errorf("resolve git directory: %w", err)
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
	if err != nil {
		return fmt.Errorf("git clean failed: %w", err)
	}
	return nil
}

func (m *manager) confirmAndReset() (bool, error) {
	if !m.isDirty(m.submoduleRoot) {
		return true, nil
	}
	m.printf("Warning: submodule has local modifications (possibly from a force push).\n")
	raw, err := m.prompt("Discard local changes and reset to HEAD? (y/N): ")
	if err != nil {
		return false, fmt.Errorf("read reset confirmation: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(raw)) != "y" {
		m.printf("Cancelled — local changes preserved.\n")
		return false, nil
	}
	if err := m.resetWorkingTree(m.submoduleRoot); err != nil {
		return false, fmt.Errorf("reset working tree: %w", err)
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
		return nil, fmt.Errorf("list remote branches: %w", err)
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
	stdout, stderr, err := executil.Run(executil.Options{
		Context:   m.opts.Context,
		Name:      "git",
		Args:      args,
		Dir:       cwd,
		Networked: isNetworkedGit(args),
	})
	out := strings.TrimSpace(stdout)
	if err != nil {
		if !check {
			return out, nil
		}
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
	}
	return out, nil
}

func isNetworkedGit(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "fetch", "pull", "push":
		return true
	default:
		return false
	}
}

func (m *manager) printf(format string, args ...any) {
	if m.outputErr != nil {
		return
	}
	if _, err := fmt.Fprintf(m.out(), format, args...); err != nil {
		m.outputErr = fmt.Errorf("write submodule output: %w", err)
	}
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
			return "", fmt.Errorf("read prompt input: %w", err)
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
		return "", fmt.Errorf("read key input: %w", err)
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return "", nil
	}
	return string(line[0]), nil
}

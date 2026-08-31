package contextdigest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/githttps"
)

// Git runs git in dir with optional HTTPS auth for the forge.
type Git struct {
	Dir   string
	Token string
	SCM   string // github|gitlab|bitbucket
}

func (g *Git) authConfigArgs() []string {
	return githttps.ExtraHeaderArgs(g.Token, g.SCM)
}

func (g *Git) run(args ...string) (string, error) {
	cmdArgs := append([]string{"-C", g.Dir}, args...)
	cmdArgs = append(g.authConfigArgs(), cmdArgs...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

func (g *Git) runAllowFail(args ...string) (string, int) {
	cmdArgs := append([]string{"-C", g.Dir}, args...)
	cmdArgs = append(g.authConfigArgs(), cmdArgs...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return stdout.String(), ee.ExitCode()
	}
	return stdout.String(), 1
}

func (g *Git) trim(args ...string) (string, error) {
	out, err := g.run(args...)
	return strings.TrimSpace(out), err
}

// ResolveDefaultBranch returns origin's default branch name (without refs/heads/).
func ResolveDefaultBranch(g *Git) (string, error) {
	out, code := g.runAllowFail("symbolic-ref", "refs/remotes/origin/HEAD")
	if code == 0 {
		ref := strings.TrimSpace(out)
		const prefix = "refs/remotes/origin/"
		if strings.HasPrefix(ref, prefix) {
			return strings.TrimPrefix(ref, prefix), nil
		}
	}
	for _, name := range []string{"main", "master"} {
		_, code := g.runAllowFail("rev-parse", "--verify", "origin/"+name)
		if code == 0 {
			return name, nil
		}
	}
	return "", fmt.Errorf("cannot resolve default branch (set origin/HEAD or main/master)")
}

// RemoteBranchExists reports whether refs/heads/branch exists on origin.
func RemoteBranchExists(g *Git, branch string) (bool, error) {
	out, err := g.trim("ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// LocalBranchExists reports whether a local branch ref exists.
func LocalBranchExists(g *Git, branch string) bool {
	_, code := g.runAllowFail("show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return code == 0
}

// FetchOrigin fetches from origin.
func FetchOrigin(g *Git, refspecs ...string) error {
	args := append([]string{"fetch", "origin"}, refspecs...)
	_, err := g.run(args...)
	return err
}

// DeepenDefaultBranch fetches more default-branch history for shallow clones.
func DeepenDefaultBranch(g *Git, defaultBranch string, depth int) error {
	if depth <= 0 {
		depth = 500
	}
	_, err := g.run("fetch", "--deepen="+fmt.Sprint(depth), "origin", defaultBranch)
	return err
}

// IsAncestor reports whether ancestor is an ancestor of desc (same repo).
func IsAncestor(g *Git, ancestor, desc string) (bool, error) {
	if strings.TrimSpace(ancestor) == "" {
		return false, nil
	}
	_, code := g.runAllowFail("merge-base", "--is-ancestor", ancestor, desc)
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("merge-base --is-ancestor %s %s failed", ancestor, desc)
	}
}

// EnsureAncestor verifies ancestor→desc, deepening a shallow clone once when needed.
func EnsureAncestor(g *Git, ancestor, desc, defaultBranch string) error {
	ok, err := IsAncestor(g, ancestor, desc)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if err := DeepenDefaultBranch(g, defaultBranch, 500); err != nil {
		return fmt.Errorf("cursor %s not ancestor of %s (deepen: %w)", ancestor, desc, err)
	}
	ok, err = IsAncestor(g, ancestor, desc)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("cursor %s is not an ancestor of %s", ancestor, desc)
	}
	return nil
}

// FirstParentCommits returns SHAs from fromExclusive to toInclusive along first-parent.
func FirstParentCommits(g *Git, fromExclusive, toInclusive, defaultBranch string) ([]string, error) {
	if fromExclusive == toInclusive {
		return nil, nil
	}
	if err := EnsureAncestor(g, fromExclusive, toInclusive, defaultBranch); err != nil {
		return nil, err
	}
	rangeSpec := fromExclusive + ".." + toInclusive
	out, err := g.trim("rev-list", "--first-parent", "--reverse", rangeSpec)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Fields(out), nil
}

// CheckoutBranch checks out an existing local branch.
func CheckoutBranch(g *Git, branch string) error {
	_, err := g.run("checkout", branch)
	return err
}

// CheckoutOrCreate checks out branch, creating it from base when missing.
func CheckoutOrCreate(g *Git, branch, base string) error {
	if LocalBranchExists(g, branch) {
		return CheckoutBranch(g, branch)
	}
	_, err := g.run("checkout", "-B", branch, base)
	return err
}

// CheckoutUpdateBranch checks out the update branch, fetching from origin before recreating from base.
func CheckoutUpdateBranch(g *Git, baseBranch, updateBranch string) error {
	if LocalBranchExists(g, updateBranch) {
		return CheckoutBranch(g, updateBranch)
	}
	exists, err := RemoteBranchExists(g, updateBranch)
	if err != nil {
		return err
	}
	if exists {
		if err := FetchOrigin(g, updateBranch+":"+updateBranch); err != nil {
			return fmt.Errorf("fetch update branch %s: %w", updateBranch, err)
		}
		return CheckoutBranch(g, updateBranch)
	}
	if err := CheckoutBranch(g, baseBranch); err != nil {
		return err
	}
	return CheckoutOrCreate(g, updateBranch, baseBranch)
}

// CommitAll stages all changes and commits with message. Returns false when the tree is clean.
func CommitAll(g *Git, message string) (bool, error) {
	if _, err := g.run("add", "-A"); err != nil {
		return false, err
	}
	status, err := g.trim("status", "--porcelain")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(status) == "" {
		return false, nil
	}
	_, err = g.run("commit", "-m", message)
	return true, err
}

// Push pushes local HEAD to origin branch.
func Push(g *Git, branch string) error {
	_, err := g.run("push", "origin", "HEAD:"+branch)
	return err
}

// InitOrphan initializes an empty repo and creates branch on an orphan commit.
func InitOrphan(dir, branch string) (*Git, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	g := &Git{Dir: dir}
	if _, err := g.run("init"); err != nil {
		return nil, err
	}
	if _, err := g.run("checkout", "--orphan", branch); err != nil {
		return nil, err
	}
	return g, nil
}

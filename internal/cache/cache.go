package cache

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var cacheBranchRE = regexp.MustCompile(`^majordomo-pr-reviewer-cache/[a-z0-9][a-z0-9-]*$`)
var pollBranchRE = regexp.MustCompile(`(?i)^majordomo-poll-cache/[a-z0-9][a-z0-9._/-]*$`)

// ExitPatternViolation matches push-to-cache.py exit 42.
const ExitPatternViolation = 42

// ValidateReviewCacheBranch returns nil if branch name is allowed.
func ValidateReviewCacheBranch(branch string) error {
	if !cacheBranchRE.MatchString(branch) {
		return fmt.Errorf("cache branch %q does not match majordomo-pr-reviewer-cache/<slug>", branch)
	}
	return nil
}

// ValidatePollCacheBranch returns nil if poll-cache branch name is plausible.
func ValidatePollCacheBranch(branch string) error {
	if !pollBranchRE.MatchString(branch) {
		return fmt.Errorf("poll cache branch %q must match majordomo-poll-cache/<repo-id>", branch)
	}
	return nil
}

// PushOptions configures a constrained cache-branch push.
type PushOptions struct {
	Remote   string
	Branch   string
	Worktree string
	Token    string // BITBUCKET_TOKEN or GITHUB_TOKEN
}

// Push pushes the cache branch with Bearer auth (port of push-to-cache.py).
func Push(opts PushOptions) error {
	if err := ValidateReviewCacheBranch(opts.Branch); err != nil {
		return err
	}
	if opts.Remote == "" || opts.Worktree == "" {
		return fmt.Errorf("--remote and --worktree required")
	}
	token := opts.Token
	if token == "" {
		token = first(os.Getenv("BITBUCKET_TOKEN"), os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))
	}
	if token == "" {
		return fmt.Errorf("token required (BITBUCKET_TOKEN or GITHUB_TOKEN)")
	}
	if !strings.HasPrefix(opts.Remote, "https://") {
		return fmt.Errorf("remote URL must be https")
	}
	info, err := os.Stat(opts.Worktree)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("worktree not found: %s", opts.Worktree)
	}
	auth := "Authorization: Bearer " + token
	run := func(args ...string) error {
		cmd := exec.Command("git", append([]string{"-C", opts.Worktree, "-c", "http.extraHeader=" + auth}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// Best-effort fetch: branch may not exist yet on first push.
	if err := run("fetch", opts.Remote, opts.Branch+":"+opts.Branch); err != nil {
		_ = run("fetch", opts.Remote, opts.Branch)
	}
	err = run("push", opts.Remote, "HEAD:"+opts.Branch)
	if err != nil {
		if ferr := run("fetch", opts.Remote, opts.Branch); ferr != nil {
			return fmt.Errorf("cache push failed (%v); refetch also failed: %w", err, ferr)
		}
		err = run("push", opts.Remote, "HEAD:"+opts.Branch)
		if err != nil {
			return fmt.Errorf("cache push failed after refetch: %w", err)
		}
	}
	return nil
}

// PollCursor is the poll reconciliation cursor stored on the poll-cache branch.
type PollCursor struct {
	RepoID  string            `json:"repo_id"`
	Heads   map[string]string `json:"heads"` // pr_number -> head_sha
	Updated string            `json:"updated,omitempty"`
}

// CursorPath returns the default cursor filename in a worktree.
func CursorPath(worktree string) string {
	return filepath.Join(worktree, "poll-cursor.json")
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

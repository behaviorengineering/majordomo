package cache

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/behaviorengineering/majordomo/pkg/forge/githttps"
)

var inferenceCacheBranchRE = regexp.MustCompile(`^majordomo-inference-cache/[a-z0-9][a-z0-9._/-]*$`)
var pollBranchRE = regexp.MustCompile(`(?i)^majordomo-poll-cache/[a-z0-9][a-z0-9._/-]*$`)

// Path prefixes under majordomo-inference-cache/<repo-id>.
const (
	ReviewCachePrefix = "review"
	// DigestCachePrefix is the open storage convention for factory inference
	// artifacts under the shared inference-cache branch. The open runner does
	// not write this prefix; majordomo-context does.
	DigestCachePrefix = "digest"
)

// ExitPatternViolation matches push-to-cache.py exit 42.
const ExitPatternViolation = 42

// ValidateInferenceCacheBranch returns nil if branch name is allowed.
func ValidateInferenceCacheBranch(branch string) error {
	if !inferenceCacheBranchRE.MatchString(branch) {
		return fmt.Errorf("cache branch %q does not match majordomo-inference-cache/<repo-id>", branch)
	}
	return nil
}

// ValidateReviewCacheBranch is an alias for ValidateInferenceCacheBranch.
func ValidateReviewCacheBranch(branch string) error {
	return ValidateInferenceCacheBranch(branch)
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

// Push pushes the inference-cache branch with forge HTTPS auth (port of push-to-cache.py).
func Push(opts PushOptions) error {
	if err := ValidateInferenceCacheBranch(opts.Branch); err != nil {
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
	authArgs := githttps.ExtraHeaderArgs(token, githttps.InferSCM(opts.Remote))
	run := func(args ...string) error {
		cmdArgs := append([]string{"-C", opts.Worktree}, authArgs...)
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command("git", cmdArgs...)
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

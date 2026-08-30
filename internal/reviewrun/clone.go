package reviewrun

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/config"
)

func ensureServedRepo(opts Options, token, scm, cloneURL, headSHA, baseBranch string) error {
	dir := opts.WorkDir
	if isGitRepo(dir) {
		head, err := gitTrim(dir, token, scm, "rev-parse", "HEAD")
		if err == nil && shaMatch(head, headSHA) {
			logf("INFO", "workdir already at %s; skip clone", shortSHA(head))
			if err := fetchBase(dir, token, scm, baseBranch); err != nil {
				logf("WARN", "%v", err)
			}
			return nil
		}
		if headSHA == "" {
			logf("INFO", "using existing workdir HEAD %s", shortSHA(head))
			if err := fetchBase(dir, token, scm, baseBranch); err != nil {
				logf("WARN", "%v", err)
			}
			return nil
		}
		return checkoutSHA(dir, token, scm, cloneURL, headSHA, baseBranch)
	}
	if cloneURL == "" {
		return fmt.Errorf("clone: %s is not a git repo and clone URL is empty", dir)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("mkdir clone parent: %w", err)
	}
	if _, err := os.Stat(dir); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("clear workdir %s: %w", dir, err)
		}
	}
	logf("INFO", "cloning %s → %s", cloneURL, dir)
	if _, err := git("", token, scm, "clone", "--depth", "50", cloneURL, dir); err != nil {
		return fmt.Errorf("clone: %w", err)
	}
	if _, err := git(dir, "", scm, "remote", "set-url", "origin", cloneURL); err != nil {
		return fmt.Errorf("remote set-url: %w", err)
	}
	return checkoutSHA(dir, token, scm, cloneURL, headSHA, baseBranch)
}

func checkoutSHA(dir, token, scm, cloneURL, headSHA, baseBranch string) error {
	if headSHA != "" {
		if _, err := git(dir, token, scm, "fetch", "--depth", "50", "origin", headSHA); err != nil {
			if _, err2 := git(dir, token, scm, "fetch", "origin", headSHA); err2 != nil {
				return fmt.Errorf("fetch head %s: %w", headSHA, err)
			}
		}
		if _, err := git(dir, token, scm, "checkout", "--force", headSHA); err != nil {
			return fmt.Errorf("checkout %s: %w", headSHA, err)
		}
	}
	if cloneURL != "" {
		_, _ = git(dir, "", scm, "remote", "set-url", "origin", cloneURL)
	}
	return fetchBase(dir, token, scm, baseBranch)
}

func fetchBase(dir, token, scm, baseBranch string) error {
	if baseBranch == "" {
		return nil
	}
	refspec := baseBranch + ":refs/remotes/origin/" + baseBranch
	if _, err := git(dir, token, scm, "fetch", "--depth", "50", "origin", refspec); err != nil {
		if _, err2 := git(dir, token, scm, "fetch", "origin", refspec); err2 != nil {
			return fmt.Errorf("fetch origin/%s: %w", baseBranch, err2)
		}
	}
	return nil
}

func resolveHEAD(dir string) string {
	head, err := gitTrim(dir, "", "", "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return head
}

func resolveDefaultBranch(dir, token, scm string) string {
	out, code := gitAllowFail(dir, token, scm, "symbolic-ref", "refs/remotes/origin/HEAD")
	if code == 0 {
		const prefix = "refs/remotes/origin/"
		if strings.HasPrefix(out, prefix) {
			return strings.TrimPrefix(out, prefix)
		}
	}
	for _, name := range []string{"main", "master"} {
		_, c := gitAllowFail(dir, token, scm, "rev-parse", "--verify", "origin/"+name)
		if c == 0 {
			return name
		}
	}
	return ""
}

func maybeContextDir(opts Options, token, scm, cloneURL, repoID string) string {
	if opts.ContextDir != "" {
		return opts.ContextDir
	}
	if cloneURL == "" {
		return ""
	}
	branch := config.ContextBranch(repoID)
	out, err := gitTrim(opts.WorkDir, token, scm, "ls-remote", "--heads", "origin", branch)
	if err != nil || out == "" {
		return ""
	}
	dest := filepath.Join(filepath.Dir(opts.WorkDir), "majordomo-context-"+repoID)
	if isGitRepo(dest) {
		return dest
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		logf("WARN", "context dir mkdir: %v", err)
		return ""
	}
	logf("INFO", "cloning context branch %s → %s", branch, dest)
	if _, err := git("", token, scm, "clone", "--depth", "1", "--branch", branch, "--single-branch", cloneURL, dest); err != nil {
		logf("WARN", "context branch clone skipped: %v", err)
		return ""
	}
	return dest
}

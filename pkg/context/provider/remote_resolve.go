package contextprovider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// ResolveRemoteContextDir checks out the context branch at a pinned commit for this run.
// The pinned SHA is req.ContextSHA when set, otherwise the current ls-remote branch tip.
func ResolveRemoteContextDir(ctx context.Context, cfg GitRemoteConfig, req Request) (dir, pinnedSHA string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(cfg.CloneURL) == "" || strings.TrimSpace(cfg.WorkDir) == "" {
		return "", "", ErrNoContext
	}
	repoID := strings.TrimSpace(cfg.RepoID)
	if repoID == "" {
		return "", "", ErrNoContext
	}
	branch := strings.TrimSpace(cfg.Branch)
	if branch == "" {
		branch = ContextBranchName(repoID)
	}
	g := cfg.runner()
	scm := strings.TrimSpace(cfg.SCM)

	targetSHA := strings.TrimSpace(req.ContextSHA)
	if targetSHA == "" {
		out, lsErr := g.RunTrim(ctx, cfg.WorkDir, cfg.Token, scm, "ls-remote", "--heads", "origin", branch)
		if lsErr != nil || out == "" {
			return "", "", ErrNoContext
		}
		targetSHA = parseRemoteBranchHead(out)
		if targetSHA == "" {
			return "", "", ErrNoContext
		}
	}

	parent := strings.TrimSpace(cfg.CacheParent)
	if parent == "" {
		parent = filepath.Dir(cfg.WorkDir)
	}
	dest := filepath.Join(parent, "majordomo-context-"+repoID)

	if err := ensureContextCheckout(ctx, g, cfg, dest, branch, targetSHA); err != nil {
		if cfg.Warn != nil {
			cfg.Warn("context checkout: %v", err)
		}
		return "", "", ErrNoContext
	}
	return dest, targetSHA, nil
}

func parseRemoteBranchHead(lsRemoteOut string) string {
	line := strings.TrimSpace(lsRemoteOut)
	if line == "" {
		return ""
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func ensureContextCheckout(ctx context.Context, g GitRunner, cfg GitRemoteConfig, dest, branch, targetSHA string) error {
	scm := strings.TrimSpace(cfg.SCM)
	if !g.IsRepo(dest) {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if cfg.Warn != nil {
			cfg.Warn("cloning context branch %s → %s", branch, dest)
		}
		if _, err := g.Run(ctx, "", cfg.Token, scm, "clone", "--depth", "1", "--branch", branch, "--single-branch", cfg.CloneURL, dest); err != nil {
			return err
		}
	}
	head, err := g.RunTrim(ctx, dest, cfg.Token, scm, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if commitMatch(head, targetSHA) {
		return nil
	}
	if cfg.Warn != nil {
		cfg.Warn("pinning context checkout %s at %s", dest, shortCommit(targetSHA))
	}
	if _, err := g.Run(ctx, dest, cfg.Token, scm, "fetch", "--depth", "1", "origin", targetSHA); err != nil {
		return err
	}
	if _, err := g.Run(ctx, dest, cfg.Token, scm, "checkout", "--force", targetSHA); err != nil {
		return err
	}
	return nil
}

func commitMatch(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	if got == want {
		return true
	}
	if len(want) >= 7 && strings.HasPrefix(got, want) {
		return true
	}
	if len(got) >= 7 && strings.HasPrefix(want, got) {
		return true
	}
	return false
}

func shortCommit(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}

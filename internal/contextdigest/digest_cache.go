package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/majordomo/internal/config"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

// materializeDigestCacheWorktree checks out or seeds majordomo-digest-cache/<repo-id>.
func materializeDigestCacheWorktree(dir string, served *Git, branch, token, scm string) error {
	if err := cache.ValidateDigestCacheBranch(branch); err != nil {
		return err
	}
	g := &Git{Dir: dir, Token: token, SCM: scm}
	if _, err := g.run("init"); err != nil {
		return err
	}
	if err := configureCommitIdentity(g); err != nil {
		return err
	}
	remote, err := served.trim("remote", "get-url", "origin")
	if err != nil {
		return err
	}
	if err := ensureRemote(g, remote); err != nil {
		return err
	}
	exists, err := RemoteBranchExists(served, branch)
	if err != nil {
		return err
	}
	if exists {
		if err := FetchOrigin(g, branch+":"+branch); err != nil {
			return fmt.Errorf("fetch digest cache branch: %w", err)
		}
		if err := CheckoutBranch(g, branch); err != nil {
			return err
		}
		return nil
	}
	if _, err := g.run("checkout", "--orphan", branch); err != nil {
		return err
	}
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Digest inference cache\n\nKeyed inspect/ledger RLM results. Not teaching content.\n"), 0o644); err != nil {
		return err
	}
	if _, err := g.run("add", "-A"); err != nil {
		return err
	}
	if _, err := g.run("commit", "-m", "seed digest inference cache"); err != nil {
		return err
	}
	return nil
}

func pushDigestCacheWorktree(dir, branch, token, scm string, served *Git) error {
	remote, err := served.trim("remote", "get-url", "origin")
	if err != nil {
		return err
	}
	return cache.PushDigest(cache.DigestPushOptions{
		Remote:   remote,
		Branch:   branch,
		Worktree: dir,
		Token:    token,
		SCM:      scm,
	})
}

func digestModelID(cfg config.RepoConfig) string {
	for _, task := range []string{jmodules.TaskTypologyObjectiveGrounding, jmodules.TaskTypologyInspect} {
		provider, ok, err := cfg.ResolveTaskProvider(task)
		if err != nil || !ok {
			continue
		}
		if m := strings.TrimSpace(provider.Model); m != "" {
			return m
		}
	}
	return "unknown"
}

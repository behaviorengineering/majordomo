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

// materializeDigestCacheWorktree checks out or seeds majordomo-inference-cache/<repo-id>.
func materializeDigestCacheWorktree(dir string, served *Git, branch, token, scm string) error {
	if err := cache.ValidateInferenceCacheBranch(branch); err != nil {
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
			return fmt.Errorf("fetch inference cache branch: %w", err)
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
	body := "# Majordomo inference cache\n\n" +
		"Keyed review (`review/`) and digest (`digest/`) artifacts. Not teaching content.\n"
	if err := os.WriteFile(readme, []byte(body), 0o644); err != nil {
		return err
	}
	if _, err := g.run("add", "-A"); err != nil {
		return err
	}
	if _, err := g.run("commit", "-m", "seed inference cache"); err != nil {
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
	for _, task := range []string{jmodules.TaskTypologySliceMeaning, jmodules.TaskTypologyInspect} {
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

package contextdigest

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func resolveToken(cfg config.RepoConfig, scm, owner string) string {
	if v := config.ResolveCredential(cfg.Repository.ID, scm, owner); v != "" {
		return v
	}
	if strings.ToLower(scm) == "bitbucket" {
		return strings.TrimSpace(os.Getenv("BITBUCKET_TOKEN"))
	}
	return ""
}

func splitOwnerName(cloneURL string) (owner, name string) {
	path := strings.TrimSuffix(cloneURL, ".git")
	path = strings.TrimPrefix(path, "https://")
	path = strings.TrimPrefix(path, "http://")
	if i := strings.Index(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i], path[i+1:]
	}
	return "", path
}

func seedOrphan(dir, branch, repoID, defaultHEAD string, at time.Time, token, scm, cloneURL string) error {
	g, err := InitOrphan(dir, branch)
	if err != nil {
		return err
	}
	g.SCM = scm
	if err := configureCommitIdentity(g); err != nil {
		return err
	}
	if err := contextstore.Bootstrap(dir, repoID, defaultHEAD, at); err != nil {
		return err
	}
	if _, err := g.run("add", "-A"); err != nil {
		return err
	}
	if _, err := g.run("commit", "-m", "bootstrap context branch"); err != nil {
		return err
	}
	g.Token = token
	return ensureRemote(g, cloneURL)
}

func configureCommitIdentity(g *Git) error {
	if _, err := g.run("config", "user.email", "majordomo@users.noreply.github.com"); err != nil {
		return err
	}
	_, err := g.run("config", "user.name", "majordomo")
	return err
}

func ensureRemote(g *Git, cloneURL string) error {
	if strings.TrimSpace(cloneURL) == "" {
		return fmt.Errorf("repository.cloneUrl required for context push")
	}
	_, code := g.runAllowFail("remote", "get-url", "origin")
	if code == 0 {
		return nil
	}
	_, err := g.run("remote", "add", "origin", cloneURL)
	return err
}

func materializeContextWorktree(dir string, served *Git, baseBranch, updateBranch, token, scm string) error {
	g := &Git{Dir: dir, Token: token, SCM: scm}
	if _, err := g.run("init"); err != nil {
		return err
	}
	remote, err := served.trim("remote", "get-url", "origin")
	if err != nil {
		return err
	}
	if err := ensureRemote(g, remote); err != nil {
		return err
	}
	if err := FetchOrigin(g, baseBranch+":"+baseBranch, updateBranch+":"+updateBranch); err != nil {
		if err2 := FetchOrigin(g, baseBranch+":"+baseBranch); err2 != nil {
			return fmt.Errorf("fetch context branches: %w", err2)
		}
	}
	return nil
}

func readEffectiveCursor(g *Git, baseBranch, updateBranch string, openPR bool) (string, error) {
	readBranch := baseBranch
	switch {
	case openPR:
		readBranch = updateBranch
		logf("INFO", "open context update PR exists; reading cursor from %s", updateBranch)
	case LocalBranchExists(g, updateBranch):
		readBranch = updateBranch
		logf("INFO", "reading cursor from existing update branch %s", updateBranch)
	default:
		logf("INFO", "reading cursor from merged context branch %s", baseBranch)
	}
	if err := CheckoutBranch(g, readBranch); err != nil {
		return "", err
	}
	return ReadCursor(g.Dir)
}

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}

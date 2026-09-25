package reviewrun

import (
	"context"
	"strings"

	forgeauth "github.com/behaviorengineering/majordomo-forge-clients/pkg/auth"
	forgerun "github.com/behaviorengineering/majordomo-forge-clients/pkg/gitrun"
)

func git(dir, token, scm string, args ...string) (string, error) {
	run := forgerun.Runner{Cred: forgeauth.Credential{Token: token, SCM: scm}}
	return run.Run(contextBackground(), dir, args...)
}

func gitTrim(dir, token, scm string, args ...string) (string, error) {
	out, err := git(dir, token, scm, args...)
	return strings.TrimSpace(out), err
}

func gitAllowFail(dir, token, scm string, args ...string) (string, int) {
	run := forgerun.Runner{Cred: forgeauth.Credential{Token: token, SCM: scm}}
	return run.AllowFail(contextBackground(), dir, args...)
}

func isGitRepo(dir string) bool {
	return forgerun.Runner{}.IsRepo(dir)
}

func shaMatch(got, want string) bool {
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

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
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

// reviewrun git helpers are synchronous; use a non-cancelable background context.
func contextBackground() context.Context {
	return context.Background()
}

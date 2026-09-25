package contextprovider

import (
	"context"
	"strings"

	forgeauth "github.com/behaviorengineering/majordomo-forge-clients/pkg/auth"
	forgerun "github.com/behaviorengineering/majordomo-forge-clients/pkg/gitrun"
)

func trimGitOut(s string) string {
	return strings.TrimSpace(s)
}

func gitWithContext(ctx context.Context, dir, token, scm string, args ...string) (string, error) {
	run := forgerun.Runner{Cred: forgeauth.Credential{Token: token, SCM: scm}}
	return run.Run(ctx, dir, args...)
}

func gitAllowFailWithContext(ctx context.Context, dir, token, scm string, args ...string) (string, int) {
	run := forgerun.Runner{Cred: forgeauth.Credential{Token: token, SCM: scm}}
	return run.AllowFail(ctx, dir, args...)
}

func isGitRepo(dir string) bool {
	return forgerun.Runner{}.IsRepo(dir)
}

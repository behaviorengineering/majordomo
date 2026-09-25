package contextprovider

import (
	"context"
)

// GitRunner runs git subprocesses (injectable for tests and future shared forge kit).
type GitRunner interface {
	Run(ctx context.Context, dir, token, scm string, args ...string) (string, error)
	RunTrim(ctx context.Context, dir, token, scm string, args ...string) (string, error)
	AllowFail(ctx context.Context, dir, token, scm string, args ...string) (string, int)
	IsRepo(dir string) bool
}

// ShellGitRunner executes real git on the host.
type ShellGitRunner struct{}

// DefaultGitRunner returns the production git runner.
func DefaultGitRunner() GitRunner {
	return ShellGitRunner{}
}

func (ShellGitRunner) Run(ctx context.Context, dir, token, scm string, args ...string) (string, error) {
	return gitWithContext(ctx, dir, token, scm, args...)
}

func (ShellGitRunner) RunTrim(ctx context.Context, dir, token, scm string, args ...string) (string, error) {
	out, err := gitWithContext(ctx, dir, token, scm, args...)
	if err != nil {
		return "", err
	}
	return trimGitOut(out), nil
}

func (ShellGitRunner) AllowFail(ctx context.Context, dir, token, scm string, args ...string) (string, int) {
	return gitAllowFailWithContext(ctx, dir, token, scm, args...)
}

func (ShellGitRunner) IsRepo(dir string) bool {
	return isGitRepo(dir)
}

func (cfg GitRemoteConfig) runner() GitRunner {
	if cfg.Git != nil {
		return cfg.Git
	}
	return DefaultGitRunner()
}

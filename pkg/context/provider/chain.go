package contextprovider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// GitRemoteConfig configures optional remote context-branch resolution.
type GitRemoteConfig struct {
	WorkDir     string
	CloneURL    string
	Token       string
	SCM         string
	RepoID      string
	Branch      string // empty → ContextBranchName(RepoID)
	CacheParent string // parent dir for majordomo-context-<repoID>
	Warn        WarnFunc
	Git         GitRunner // nil → DefaultGitRunner()
}

// RemoteProvider resolves context from a remote orphan branch (graceful degradation).
type RemoteProvider struct {
	cfg GitRemoteConfig
}

// NewRemoteProvider returns a provider that shallow-clones the context branch when present.
func NewRemoteProvider(cfg GitRemoteConfig) *RemoteProvider {
	return &RemoteProvider{cfg: cfg}
}

// Resolve implements ContextProvider.
func (p *RemoteProvider) Resolve(ctx context.Context, req Request) (*Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if p == nil {
		return nil, ErrNoContext
	}
	dir, pinnedSHA, err := ResolveRemoteContextDir(ctx, p.cfg, req)
	if err != nil {
		return nil, err
	}
	snap, err := LoadSnapshotFromDir(dir, req.RepoID, false)
	if err != nil {
		if p.cfg.Warn != nil {
			p.cfg.Warn("context tree invalid at %s: %v; proceeding without grounding", dir, err)
		}
		return nil, ErrNoContext
	}
	if pinnedSHA != "" {
		snap.Provenance.ResolvedCommit = pinnedSHA
	}
	return snap, nil
}

// ChainConfig wires explicit filesystem context before optional remote fallback.
type ChainConfig struct {
	ExplicitDir string
	Remote      GitRemoteConfig
	Warn        WarnFunc
}

// Chain tries explicit dir first, then remote branch discovery.
type Chain struct {
	explicitDir string
	remote      *RemoteProvider
}

// NewChain builds the review-runner context provider chain.
func NewChain(cfg ChainConfig) *Chain {
	if cfg.Warn != nil && cfg.Remote.Warn == nil {
		cfg.Remote.Warn = cfg.Warn
	}
	var remote *RemoteProvider
	if strings.TrimSpace(cfg.Remote.CloneURL) != "" {
		remote = NewRemoteProvider(cfg.Remote)
	}
	return &Chain{
		explicitDir: strings.TrimSpace(cfg.ExplicitDir),
		remote:      remote,
	}
}

// Resolve implements ContextProvider.
func (c *Chain) Resolve(ctx context.Context, req Request) (*Snapshot, error) {
	if c == nil {
		return nil, ErrNoContext
	}
	if c.explicitDir != "" {
		return LoadSnapshotFromDir(c.explicitDir, req.RepoID, true)
	}
	if c.remote == nil {
		return nil, ErrNoContext
	}
	return c.remote.Resolve(ctx, req)
}

// Static returns a provider that always yields the same snapshot (tests).
func Static(snap *Snapshot) ContextProvider {
	return staticProvider{snap: snap}
}

type staticProvider struct {
	snap *Snapshot
}

func (s staticProvider) Resolve(context.Context, Request) (*Snapshot, error) {
	if s.snap == nil {
		return nil, ErrNoContext
	}
	return s.snap, nil
}

// ResolveContextSHA returns explicit flag/env context commit pin.
func ResolveContextSHA(flag string) string {
	if s := strings.TrimSpace(flag); s != "" {
		return s
	}
	return strings.TrimSpace(os.Getenv("MAJORDOMO_CONTEXT_SHA"))
}

// ResolveContextDir returns explicit flag/env context directory (same as staging helper).
func ResolveContextDir(flag string) string {
	if s := strings.TrimSpace(flag); s != "" {
		return s
	}
	return strings.TrimSpace(os.Getenv("MAJORDOMO_CONTEXT_DIR"))
}

// FormatResolveError wraps explicit-context failures for operators.
func FormatResolveError(err error) error {
	if err == nil {
		return nil
	}
	if errorsIsInvalidTree(err) {
		return fmt.Errorf("context provider: %w", err)
	}
	return err
}

func errorsIsInvalidTree(err error) bool {
	return errors.Is(err, ErrInvalidContextTree)
}

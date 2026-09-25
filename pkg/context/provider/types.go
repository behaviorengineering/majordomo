package contextprovider

import (
	"context"
	"time"

	"github.com/behaviorengineering/majordomo/pkg/context/agenting"
)

// Request describes context needed for one review prep pass.
type Request struct {
	RepoID     string
	ContextSHA string // optional; when set pins remote checkout to this commit
}

// Provenance records where a snapshot came from (finished packs only).
type Provenance struct {
	SourceCommit     string
	ResolvedCommit   string // git commit pinned for this run (context branch checkout)
	GeneratorVersion string
	GeneratedAt      time.Time
	ClaimAge         time.Duration
}

// Snapshot is a validated, read-only view of agenting packs on a context tree.
type Snapshot struct {
	RootDir    string
	Index      agenting.Index
	Provenance Provenance
}

// StagePacks copies selected grounding files into a batch staging directory.
func (s *Snapshot) StagePacks(batchDir string, packIDs []string) ([]agenting.StagedPack, error) {
	if s == nil {
		return nil, ErrNoContext
	}
	return agenting.Stage(s.RootDir, batchDir, packIDs)
}

// ContextProvider resolves a context snapshot for review grounding.
type ContextProvider interface {
	Resolve(ctx context.Context, req Request) (*Snapshot, error)
}

// WarnFunc logs non-fatal context resolution issues.
type WarnFunc func(format string, args ...any)

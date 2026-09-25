package contextprovider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/pkg/context/agenting"
	contextstore "github.com/behaviorengineering/majordomo/pkg/context/store"
)

// LoadSnapshotFromDir validates dir (when required) and loads agenting index + provenance.
func LoadSnapshotFromDir(dir, repoID string, required bool) (*Snapshot, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		if required {
			return nil, fmt.Errorf("%w: dir is empty", ErrInvalidContextTree)
		}
		return nil, ErrNoContext
	}
	info, err := os.Stat(dir)
	if err != nil {
		if required {
			return nil, fmt.Errorf("%w: %v", ErrInvalidContextTree, err)
		}
		return nil, ErrNoContext
	}
	if !info.IsDir() {
		if required {
			return nil, fmt.Errorf("%w: %q is not a directory", ErrInvalidContextTree, dir)
		}
		return nil, ErrNoContext
	}
	if err := contextstore.ValidateTree(dir); err != nil {
		if required {
			return nil, fmt.Errorf("%w: %v", ErrInvalidContextTree, err)
		}
		return nil, ErrNoContext
	}
	idx, err := agenting.LoadIndex(dir)
	if err != nil {
		if required {
			return nil, fmt.Errorf("%w: agenting index: %v", ErrInvalidContextTree, err)
		}
		return nil, ErrNoContext
	}
	prov, err := provenanceFromMeta(dir, repoID)
	if err != nil && required {
		return nil, fmt.Errorf("%w: %v", ErrInvalidContextTree, err)
	}
	return &Snapshot{RootDir: dir, Index: idx, Provenance: prov}, nil
}

func provenanceFromMeta(dir, repoID string) (Provenance, error) {
	meta, err := contextstore.ParseMeta(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		return Provenance{}, err
	}
	if err := contextstore.ValidateMeta(meta); err != nil {
		return Provenance{}, err
	}
	if repoID != "" && strings.TrimSpace(meta.RepoID) != "" && meta.RepoID != repoID {
		return Provenance{}, fmt.Errorf("meta repo_id %q does not match request %q", meta.RepoID, repoID)
	}
	var generatedAt time.Time
	if ts := strings.TrimSpace(meta.LastDigestAt); ts != "" {
		generatedAt, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return Provenance{}, err
		}
	}
	var claimAge time.Duration
	if !generatedAt.IsZero() {
		claimAge = time.Since(generatedAt)
		if claimAge < 0 {
			claimAge = 0
		}
	}
	return Provenance{
		SourceCommit:     strings.TrimSpace(meta.LastMergedSHA),
		GeneratorVersion: "",
		GeneratedAt:      generatedAt,
		ClaimAge:         claimAge,
	}, nil
}

package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"gopkg.in/yaml.v3"
)

// UpdateMeta writes last_merged_sha and last_digest_at in a context worktree.
func UpdateMeta(dir, cursorSHA string, at time.Time) error {
	path := filepath.Join(dir, "meta.yaml")
	meta, err := contextstore.ParseMeta(path)
	if err != nil {
		return err
	}
	meta.LastMergedSHA = strings.TrimSpace(cursorSHA)
	meta.LastDigestAt = at.UTC().Format(time.RFC3339)
	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta.yaml: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write meta.yaml: %w", err)
	}
	return contextstore.ValidateTree(dir)
}

// ReadCursor reads last_merged_sha from a context worktree meta.yaml.
func ReadCursor(dir string) (string, error) {
	meta, err := contextstore.ParseMeta(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(meta.LastMergedSHA), nil
}

// NeedsMetaUpdate reports whether last_merged_sha differs from cursorSHA.
func NeedsMetaUpdate(dir, cursorSHA string) (bool, error) {
	cur, err := ReadCursor(dir)
	if err != nil {
		return true, err
	}
	return cur != strings.TrimSpace(cursorSHA), nil
}

// IsBehind reports whether cursor is a proper ancestor of head (cursor behind).
func IsBehind(g *Git, cursor, head, defaultBranch string) (bool, error) {
	cursor = strings.TrimSpace(cursor)
	head = strings.TrimSpace(head)
	if cursor == head {
		return false, nil
	}
	if cursor == "" {
		return head != "", nil
	}
	if err := EnsureAncestor(g, cursor, head, defaultBranch); err != nil {
		return false, err
	}
	return true, nil
}

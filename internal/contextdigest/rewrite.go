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

// RewriteInfo describes a detected history rewrite on default.
type RewriteInfo struct {
	CursorBefore string
	NewHead      string
	Why          string
	Pending      bool
}

// DetectRewrite reports when cursor is non-empty and not an ancestor of default HEAD.
func DetectRewrite(g *Git, cursor, head, defaultBranch string) (bool, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" || cursor == strings.TrimSpace(head) {
		return false, nil
	}
	ok, err := IsAncestor(g, cursor, head)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

// readRewriteMeta loads rewrite fields from meta.yaml in dir.
func readRewriteMeta(dir string) (contextstore.Meta, error) {
	return contextstore.ParseMeta(filepath.Join(dir, "meta.yaml"))
}

// writeMeta writes full meta.yaml and validates the tree.
func writeMeta(dir string, meta contextstore.Meta) error {
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.yaml"), data, 0o644); err != nil {
		return err
	}
	return contextstore.ValidateTree(dir)
}

// BeginRewrite records a pending rewrite and appends a chronology event.
func BeginRewrite(ctxDir string, newHead string, at time.Time, actor, evidence string) (RewriteInfo, error) {
	meta, err := readRewriteMeta(ctxDir)
	if err != nil {
		return RewriteInfo{}, err
	}
	meta.RewritePending = true
	meta.RewriteDetectedAt = at.UTC().Format(time.RFC3339)
	meta.RewriteNewHead = strings.TrimSpace(newHead)
	if strings.TrimSpace(evidence) == "" {
		evidence = "default branch history no longer contains cursor " + meta.LastMergedSHA
	}
	ev := contextstore.ChronologyEvent{
		Date:      at,
		Actor:     actor,
		Source:    "history-rewrite",
		Did:       "detected default-branch history rewrite",
		Because:   "last_merged_sha is not an ancestor of current default HEAD",
		InOrderTo: "reshape the teaching story for the new tape",
		Evidence:  evidence,
	}
	if err := contextstore.AppendChronologyEvent(ctxDir, ev); err != nil {
		return RewriteInfo{}, err
	}
	if err := writeMeta(ctxDir, meta); err != nil {
		return RewriteInfo{}, err
	}
	return RewriteInfo{
		CursorBefore: meta.LastMergedSHA,
		NewHead:      newHead,
		Pending:      true,
	}, nil
}

// ApplyRewriteWhy stores the human-provided reason and clears pending when set.
func ApplyRewriteWhy(ctxDir, why string) error {
	meta, err := readRewriteMeta(ctxDir)
	if err != nil {
		return err
	}
	meta.RewriteWhy = strings.TrimSpace(why)
	if meta.RewriteWhy != "" {
		meta.RewritePending = false
	}
	return writeMeta(ctxDir, meta)
}

// CompleteRewrite resets cursor to newHead and clears rewrite flags after reshape.
func CompleteRewrite(ctxDir, newHead string, at time.Time) error {
	meta, err := readRewriteMeta(ctxDir)
	if err != nil {
		return err
	}
	if meta.RewritePending && strings.TrimSpace(meta.RewriteWhy) == "" {
		return fmt.Errorf("rewrite blocked: why is required (@majordomo why … on context PR)")
	}
	meta.LastMergedSHA = strings.TrimSpace(newHead)
	meta.LastDigestAt = at.UTC().Format(time.RFC3339)
	meta.RewritePending = false
	meta.RewriteDetectedAt = ""
	meta.RewriteNewHead = ""
	meta.RewriteWhy = ""
	return writeMeta(ctxDir, meta)
}

package contextstore

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBootstrapValidates(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	if err := Bootstrap(dir, "demo", "abc123", at); err != nil {
		t.Fatal(err)
	}
	meta, err := ParseMeta(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.RepoID != "demo" || meta.LastMergedSHA != "abc123" {
		t.Fatalf("meta = %+v", meta)
	}
	if meta.LastDigestAt != "2026-08-28T03:00:00Z" {
		t.Fatalf("last_digest_at = %q", meta.LastDigestAt)
	}
}

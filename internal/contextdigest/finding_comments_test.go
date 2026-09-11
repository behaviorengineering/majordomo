package contextdigest

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakePRComments struct {
	mu    sync.Mutex
	next  int
	byID  map[string]string
	order []string
}

func (f *fakePRComments) ListPRCommentsWithIDs(string) ([]PRCommentWithID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]PRCommentWithID, 0, len(f.order))
	for _, id := range f.order {
		out = append(out, PRCommentWithID{ID: id, Body: f.byID[id]})
	}
	return out, nil
}

func (f *fakePRComments) PostPRComment(_, body string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	id := fmt.Sprintf("c%d", f.next)
	f.byID[id] = body
	f.order = append(f.order, id)
	return id, nil
}

func (f *fakePRComments) UpdatePRComment(_, commentID, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[commentID]; !ok {
		return fmt.Errorf("missing comment %s", commentID)
	}
	f.byID[commentID] = body
	return nil
}

func TestFindingFingerprintStable(t *testing.T) {
	t.Parallel()
	a := findingFingerprint("Missing SliceBinding `dashboard` -> `config`")
	b := findingFingerprint("missing   slicebinding `dashboard` -> `config`")
	if a == "" || a != b {
		t.Fatalf("a=%q b=%q", a, b)
	}
}

func TestFormatAndParseFindingCommentMarker(t *testing.T) {
	t.Parallel()
	fp := "abcd1234deadbeef"
	body := formatFindingCommentBody(fp, "Keep config as a library.")
	if !strings.Contains(body, findingCommentMarker(fp)) {
		t.Fatalf("missing marker: %s", body)
	}
	if parseFindingFingerprint(body) != fp {
		t.Fatalf("parse=%q", parseFindingFingerprint(body))
	}
}

func TestSyncFindingPRCommentsUpsertAndClear(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evidence := filepath.Join(dir, "evidence", "typology")
	fp := findingFingerprint("finding one about `internal/config`")
	fake := &fakePRComments{byID: map[string]string{}}

	if err := SyncFindingPRComments(fake, "1", dir); err != nil {
		t.Fatal(err)
	}
	if len(fake.byID) != 0 {
		t.Fatalf("no bodies yet, got %d comments", len(fake.byID))
	}

	if err := saveFindingCommentBodies(evidence, []FindingCommentBody{{
		Finding: "finding one about `internal/config`", Fingerprint: fp, Body: "Lean: keep library.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := SyncFindingPRComments(fake, "1", dir); err != nil {
		t.Fatal(err)
	}
	if len(fake.byID) != 1 {
		t.Fatalf("want 1 comment, got %d", len(fake.byID))
	}
	var firstID string
	for id := range fake.byID {
		firstID = id
	}
	if !strings.Contains(fake.byID[firstID], "keep library") {
		t.Fatalf("body=%s", fake.byID[firstID])
	}

	if err := saveFindingCommentBodies(evidence, []FindingCommentBody{{
		Finding: "finding one about `internal/config`", Fingerprint: fp, Body: "Updated lean: fold later.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := SyncFindingPRComments(fake, "1", dir); err != nil {
		t.Fatal(err)
	}
	if len(fake.byID) != 1 {
		t.Fatalf("upsert should keep one comment, got %d", len(fake.byID))
	}
	if !strings.Contains(fake.byID[firstID], "fold later") {
		t.Fatalf("updated=%s", fake.byID[firstID])
	}

	if err := saveFindingCommentBodies(evidence, nil); err != nil {
		t.Fatal(err)
	}
	if err := SyncFindingPRComments(fake, "1", dir); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.byID[firstID], "Cleared by digest") {
		t.Fatalf("cleared=%s", fake.byID[firstID])
	}
}

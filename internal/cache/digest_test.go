package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDigestCacheBranch(t *testing.T) {
	if err := ValidateDigestCacheBranch("majordomo-digest-cache/gitboard"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDigestCacheBranch("majordomo-context/gitboard"); err == nil {
		t.Fatal("expected reject")
	}
}

func TestDigestInspectRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := &DigestStore{Dir: dir}
	fp := InspectFingerprint{
		PackagePath:   "internal/board",
		ContextSHA:    ContentSHA("ctx"),
		ModelID:       "gemma",
		PromptVersion: DigestInspectPromptV1,
		SchemaVersion: DigestInspectSchemaV1,
	}
	role := InspectCachedRole{
		Path: "internal/board", Role: "dto", Confidence: 0.95,
		Evidence: []string{"json tags"}, Agreement: "match", LLMRole: "dto",
	}
	if _, ok, err := store.LookupInspect(fp); err != nil || ok {
		t.Fatalf("want miss, ok=%v err=%v", ok, err)
	}
	if err := store.StoreInspect(fp, role); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.LookupInspect(fp)
	if err != nil || !ok {
		t.Fatalf("want hit, ok=%v err=%v", ok, err)
	}
	if got.Role != "dto" || got.Agreement != "match" {
		t.Fatalf("got %+v", got)
	}
	// Drifted context must miss.
	fp2 := fp
	fp2.ContextSHA = ContentSHA("other")
	if _, ok, err := store.LookupInspect(fp2); err != nil || ok {
		t.Fatalf("want miss on drift, ok=%v err=%v", ok, err)
	}
}

func TestDigestLedgerRoundTripAndRefuseOverclaim(t *testing.T) {
	dir := t.TempDir()
	store := &DigestStore{Dir: dir}
	fp := LedgerFingerprint{
		SliceID:         "board",
		OwnedPathsHash:  OwnedPathsHash([]string{"internal/board"}),
		ContextSHA:      ContentSHA("ctx"),
		ConstraintsHash: ContentSHA("cons"),
		ClusterHash:     ContentSHA("cluster"),
		ModelID:         "gemma",
	}
	entry := LedgerCachedEntry{
		ID: "board", OwnedPaths: []string{"internal/board"},
		Evidence: []string{"BoardPayload"}, Claims: []string{"data_shape"},
		Objective: "Shared board shapes.", Verdict: "grounded", Source: "slice_objective_rlm",
	}
	if err := store.StoreLedger(fp, entry); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.LookupLedger(fp)
	if err != nil || !ok {
		t.Fatalf("want hit, ok=%v err=%v", ok, err)
	}
	if got.Objective != entry.Objective {
		t.Fatalf("got %+v", got)
	}
	bad := entry
	bad.Verdict = "overclaim"
	if err := store.StoreLedger(fp, bad); err == nil {
		t.Fatal("expected refuse overclaim store")
	}
	// Ensure file still grounded.
	path := filepath.Join(dir, "ledger", fp.key()+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestHashDigestPartsStable(t *testing.T) {
	a := HashDigestParts("a", "b")
	b := HashDigestParts("a", "b")
	if a != b || a == "" {
		t.Fatalf("unstable %q %q", a, b)
	}
	if HashDigestParts("a", "b") == HashDigestParts("b", "a") {
		t.Fatal("order should matter")
	}
}

package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDigestCacheBranch(t *testing.T) {
	if err := ValidateDigestCacheBranch("majordomo-inference-cache/gitboard"); err != nil {
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
	wantPath := filepath.Join(dir, DigestCachePrefix, "inspect")
	entries, err := os.ReadDir(wantPath)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected files under %s: %v", wantPath, err)
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
	path := filepath.Join(dir, DigestCachePrefix, "ledger", fp.key()+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestDigestClusterCoTRefineInterventionStoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := &DigestStore{Dir: dir}
	var flushes int
	store.PushFn = func() error {
		flushes++
		return nil
	}

	clusterFP := ClusterCoTFingerprint{
		DraftHash: ContentSHA("draft"), RolesHash: ContentSHA("roles"),
		ConstraintsHash: ContentSHA("cons"), MechanicalHash: ContentSHA("mech"),
		ModelID: "gemma",
	}
	if _, ok, err := store.LookupClusterCoT(clusterFP); err != nil || ok {
		t.Fatalf("cluster miss: ok=%v err=%v", ok, err)
	}
	if err := store.StoreClusterCoT(clusterFP, ClusterCoTCached{
		MergeIDs: "git", MergePackages: "internal/a,internal/b", MergeIntents: "slice", TotalTokens: 10,
	}); err != nil {
		t.Fatal(err)
	}
	gotCluster, ok, err := store.LookupClusterCoT(clusterFP)
	if err != nil || !ok || gotCluster.MergeIDs != "git" {
		t.Fatalf("cluster hit=%v got=%+v err=%v", ok, gotCluster, err)
	}

	refineFP := RefineFingerprint{
		DraftHash: ContentSHA("draft"), RolesHash: ContentSHA("roles"),
		ConstraintsHash: ContentSHA("cons"), LedgerHash: ContentSHA("ledger"),
		VerdictsHash: ContentSHA("verdicts"), MechanicalHash: ContentSHA("mech"),
		ModelID: "gemma",
	}
	if err := store.StoreRefine(refineFP, RefineCached{RefinedCatalogYAML: "id: x\n", TotalTokens: 20}); err != nil {
		t.Fatal(err)
	}
	gotRefine, ok, err := store.LookupRefine(refineFP)
	if err != nil || !ok || gotRefine.RefinedCatalogYAML != "id: x\n" {
		t.Fatalf("refine hit=%v got=%+v err=%v", ok, gotRefine, err)
	}
	if err := store.StoreRefine(refineFP, RefineCached{}); err == nil {
		t.Fatal("expected refuse empty refine")
	}

	intFP := InterventionFingerprint{
		TaskID: "typology_intervention_journey", ArchitectureHash: ContentSHA("arch"),
		RefinedHash: ContentSHA("ref"), VerdictsHash: ContentSHA("v"), FindingsHash: ContentSHA("f"),
		ModelID: "gemma",
	}
	if err := store.StoreIntervention(intFP, InterventionCached{Markdown: "# Journey\n", TotalTokens: 5}); err != nil {
		t.Fatal(err)
	}
	gotInt, ok, err := store.LookupIntervention(intFP)
	if err != nil || !ok || !strings.Contains(gotInt.Markdown, "Journey") {
		t.Fatalf("intervention hit=%v got=%+v err=%v", ok, gotInt, err)
	}

	storyFP := StoryFingerprint{
		SectionID: "mission", RefinedHash: ContentSHA("ref"), LedgerHash: ContentSHA("led"),
		ArchitectureHash: ContentSHA("arch"), ReadmeHash: ContentSHA("readme"), ModelID: "gemma",
	}
	if err := store.StoreStory(storyFP, StoryCached{Markdown: "# Mission\nGitboard serves.\n", TotalTokens: 8}); err != nil {
		t.Fatal(err)
	}
	gotStory, ok, err := store.LookupStory(storyFP)
	if err != nil || !ok || !strings.Contains(gotStory.Markdown, "Gitboard") {
		t.Fatalf("story hit=%v got=%+v err=%v", ok, gotStory, err)
	}
	drift := storyFP
	drift.ReadmeHash = ContentSHA("other")
	if _, ok, err := store.LookupStory(drift); err != nil || ok {
		t.Fatalf("want story miss on drift ok=%v err=%v", ok, err)
	}
	if flushes != 4 {
		t.Fatalf("flushes=%d want 4", flushes)
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

func TestDigestStoreFlushAfterStore(t *testing.T) {
	dir := t.TempDir()
	store := &DigestStore{Dir: dir}
	var flushes int
	store.PushFn = func() error {
		flushes++
		return nil
	}
	fp := InspectFingerprint{
		PackagePath:   "internal/board",
		ContextSHA:    ContentSHA("ctx"),
		ModelID:       "gemma",
		PromptVersion: DigestInspectPromptV1,
		SchemaVersion: DigestInspectSchemaV1,
	}
	if err := store.StoreInspect(fp, InspectCachedRole{
		Path: "internal/board", Role: "dto", Agreement: "match", LLMRole: "dto",
	}); err != nil {
		t.Fatal(err)
	}
	if flushes != 1 {
		t.Fatalf("inspect flushes=%d want 1", flushes)
	}
	ledgerFP := LedgerFingerprint{
		SliceID:         "board",
		OwnedPathsHash:  OwnedPathsHash([]string{"internal/board"}),
		ContextSHA:      ContentSHA("ctx"),
		ConstraintsHash: ContentSHA("cons"),
		ClusterHash:     ContentSHA("cluster"),
		ModelID:         "gemma",
	}
	if err := store.StoreLedger(ledgerFP, LedgerCachedEntry{
		ID: "board", Verdict: "grounded", Objective: "shapes",
	}); err != nil {
		t.Fatal(err)
	}
	if flushes != 2 {
		t.Fatalf("after ledger flushes=%d want 2", flushes)
	}
}

func TestDigestStoreFlushErrorDoesNotDropLocalWrite(t *testing.T) {
	dir := t.TempDir()
	store := &DigestStore{Dir: dir}
	var seen error
	store.OnFlushError = func(err error) { seen = err }
	store.PushFn = func() error {
		return fmt.Errorf("push boom")
	}
	fp := InspectFingerprint{
		PackagePath:   "internal/board",
		ContextSHA:    ContentSHA("ctx"),
		ModelID:       "gemma",
		PromptVersion: DigestInspectPromptV1,
		SchemaVersion: DigestInspectSchemaV1,
	}
	if err := store.StoreInspect(fp, InspectCachedRole{
		Path: "internal/board", Role: "dto", Agreement: "match",
	}); err != nil {
		t.Fatal(err)
	}
	if seen == nil || !strings.Contains(seen.Error(), "push boom") {
		t.Fatalf("OnFlushError=%v", seen)
	}
	got, ok, err := store.LookupInspect(fp)
	if err != nil || !ok || got.Role != "dto" {
		t.Fatalf("local write lost: ok=%v got=%+v err=%v", ok, got, err)
	}
}

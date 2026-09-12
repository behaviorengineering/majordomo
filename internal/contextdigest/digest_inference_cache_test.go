package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/typology/catalog"
)

type countingInspectValidator struct {
	mu    sync.Mutex
	calls int
}

func (c *countingInspectValidator) Validate(_ context.Context, _, _, _, _ string) (string, string, int, int, int, int, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return roleDTO, "json tags", 1, 100, 20, 120, nil
}

func writeBoardPkg(t *testing.T, analysisDir string) {
	t.Helper()
	pkg := filepath.Join(analysisDir, "internal", "board")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "board.go"), []byte("package board\n\ntype Payload struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePackageRolesRLM_cacheHitSkipsLLM(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBoardPkg(t, dir)
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	rolesYAML := `packages:
  - path: internal/board
    role: dto
    confidence: 0.9
    evidence: [json_tags]
    inspected_stage: 1
edges: []
`
	rolesPath := filepath.Join(evidence, packageRolesRel)
	if err := writePackageRoles(rolesPath, mustParseRoles(rolesYAML)); err != nil {
		t.Fatal(err)
	}
	ctxFile := `## ./internal/board
- package: board
- jsonTags: true
- mechanicalRole: dto
`
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(ctxFile), 0o644); err != nil {
		t.Fatal(err)
	}
	srcSHA, err := cache.PackageSourceHash(dir, "internal/board")
	if err != nil {
		t.Fatal(err)
	}

	store := &cache.DigestStore{Dir: filepath.Join(dir, "digest-cache")}
	fp := cache.InspectFingerprint{
		PackagePath:   "internal/board",
		ContextSHA:    srcSHA,
		ModelID:       "test-model",
		PromptVersion: cache.DigestInspectPromptV1,
		SchemaVersion: cache.DigestInspectSchemaV2,
	}
	if err := store.StoreInspect(fp, cache.InspectCachedRole{
		Path:             "internal/board",
		Role:             roleDTO,
		Confidence:       confidenceAgreeMatch,
		Evidence:         []string{"json_tags", "rlm:cached"},
		InspectedStage:   1,
		MechanicalRole:   roleDTO,
		LLMRole:          roleDTO,
		Agreement:        agreementMatch,
		RLMIterations:    0,
		PromptTokens:     100,
		CompletionTokens: 20,
		TotalTokens:      120,
	}); err != nil {
		t.Fatal(err)
	}

	validator := &countingInspectValidator{}
	_, err = validatePackageRolesRLM(context.Background(), validator, dir, evidence, rolesPath, rolesYAML, store, true, "test-model")
	if err != nil {
		t.Fatal(err)
	}
	validator.mu.Lock()
	calls := validator.calls
	validator.mu.Unlock()
	if calls != 0 {
		t.Fatalf("Validate calls=%d want 0 on cache hit", calls)
	}
	st := store.Stats()
	if st.InspectHits != 1 || st.TokensSavedTotal != 120 {
		t.Fatalf("stats=%+v want inspect_hits=1 tokens_saved=120", st)
	}
}

func TestValidatePackageRolesRLM_missCallsLLMAndStoresMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBoardPkg(t, dir)
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	rolesYAML := `packages:
  - path: internal/board
    role: dto
    confidence: 0.9
    evidence: [json_tags]
    inspected_stage: 1
edges: []
`
	rolesPath := filepath.Join(evidence, packageRolesRel)
	if err := writePackageRoles(rolesPath, mustParseRoles(rolesYAML)); err != nil {
		t.Fatal(err)
	}
	ctxFile := `## ./internal/board
- package: board
- mechanicalRole: dto
`
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(ctxFile), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &cache.DigestStore{Dir: filepath.Join(dir, "digest-cache")}
	validator := &countingInspectValidator{}
	_, err := validatePackageRolesRLM(context.Background(), validator, dir, evidence, rolesPath, rolesYAML, store, true, "test-model")
	if err != nil {
		t.Fatal(err)
	}
	validator.mu.Lock()
	calls := validator.calls
	validator.mu.Unlock()
	if calls != 1 {
		t.Fatalf("Validate calls=%d want 1 on miss", calls)
	}
	srcSHA, err := cache.PackageSourceHash(dir, "internal/board")
	if err != nil {
		t.Fatal(err)
	}
	fp := cache.InspectFingerprint{
		PackagePath:   "internal/board",
		ContextSHA:    srcSHA,
		ModelID:       "test-model",
		PromptVersion: cache.DigestInspectPromptV1,
		SchemaVersion: cache.DigestInspectSchemaV2,
	}
	hit, ok, err := store.LookupInspect(fp)
	if err != nil || !ok {
		t.Fatalf("expected inspect store after match, ok=%v err=%v", ok, err)
	}
	if hit.TotalTokens != 120 {
		t.Fatalf("stored usage total=%d want 120", hit.TotalTokens)
	}
}

type overclaimLedgerCaller struct {
	mu    sync.Mutex
	calls int
}

func (c *overclaimLedgerCaller) Complete(_ context.Context, _ any, _ string) (string, int, int, int, int, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return "evidence: x\nclaims: orchestrate\nobjective: does everything\nverdict: overclaim\n", 1, 0, 0, 0, nil
}

func TestBuildSliceObjectiveLedger_cacheHitSkipsLLM(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBoardPkg(t, dir)
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	roles := `packages:
  - path: internal/board
    role: dto
    confidence: 0.9
    evidence: [json_tags]
    inspected_stage: 1
edges: []
`
	if err := os.WriteFile(filepath.Join(evidence, packageRolesRel), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	ctxMD := `## ./internal/board
- package: board
- mechanicalRole: dto
`
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(ctxMD), 0o644); err != nil {
		t.Fatal(err)
	}
	draft := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "board", Owns: []catalog.Component{{ID: "b", Path: "internal/board"}}},
		},
	}
	rolesDoc, err := loadPackageRoles(filepath.Join(evidence, packageRolesRel))
	if err != nil {
		t.Fatal(err)
	}
	constraints := buildCapabilityConstraints(rolesDoc)
	byPath := map[string]packageCapabilityConstraint{}
	for _, row := range constraints.Packages {
		byPath[normalizeRolePath(row.Path)] = row
	}
	constraintBlock := formatConstraintRowsForPaths([]string{"internal/board"}, byPath)
	ownedSrc, err := cache.OwnedPackagesSourceHash(dir, []string{"internal/board"})
	if err != nil {
		t.Fatal(err)
	}
	clusterMD := "# cluster\n"
	store := &cache.DigestStore{Dir: filepath.Join(dir, "digest-cache")}
	fp := cache.LedgerFingerprint{
		SliceID:         "board",
		OwnedPathsHash:  cache.OwnedPathsHash([]string{"internal/board"}),
		ContextSHA:      ownedSrc,
		ConstraintsHash: cache.ContentSHA(constraintBlock),
		ModelID:         "test-model",
		PromptVersion:   cache.DigestLedgerPromptV1,
		SchemaVersion:   cache.DigestLedgerSchemaV2,
	}
	if err := store.StoreLedger(fp, cache.LedgerCachedEntry{
		ID:               "board",
		OwnedPaths:       []string{"internal/board"},
		Evidence:         []string{"json_tags"},
		Claims:           []string{"data_shape"},
		Objective:        "Shared board payload shapes.",
		Verdict:          ledgerVerdictGrounded,
		Source:           "digest_cache",
		PromptTokens:     200,
		CompletionTokens: 40,
		TotalTokens:      240,
	}); err != nil {
		t.Fatal(err)
	}

	caller := &countingLedgerCaller{}
	doc, issues, err := buildSliceObjectiveLedger(context.Background(), caller, sliceLedgerBuildRequest{
		AnalysisDir:   dir,
		EvidenceDir:   evidence,
		DraftTypo:     draft,
		Constraints:   constraints,
		ClusterMD:     clusterMD,
		DigestCache:   store,
		DigestSkips:   true,
		DigestModelID: "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
	caller.mu.Lock()
	calls := len(caller.calls)
	caller.mu.Unlock()
	if calls != 0 {
		t.Fatalf("Complete calls=%d want 0 on cache hit", calls)
	}
	if len(doc.Slices) != 1 || doc.Slices[0].Source != "digest_cache" {
		t.Fatalf("unexpected doc: %+v", doc)
	}
	st := store.Stats()
	if st.LedgerHits != 1 || st.TokensSavedTotal != 240 {
		t.Fatalf("stats=%+v want ledger_hits=1 tokens_saved=240", st)
	}
}

func TestBuildSliceObjectiveLedger_overclaimNotStored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBoardPkg(t, dir)
	evidence := filepath.Join(dir, "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	roles := `packages:
  - path: internal/board
    role: dto
    confidence: 0.9
    evidence: [json_tags]
    inspected_stage: 1
edges: []
`
	if err := os.WriteFile(filepath.Join(evidence, packageRolesRel), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	ctxMD := `## ./internal/board
- package: board
- mechanicalRole: dto
`
	if err := os.WriteFile(filepath.Join(evidence, "package_rlm_context.md"), []byte(ctxMD), 0o644); err != nil {
		t.Fatal(err)
	}
	draft := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{ID: "board", Owns: []catalog.Component{{ID: "b", Path: "internal/board"}}},
		},
	}
	rolesDoc, err := loadPackageRoles(filepath.Join(evidence, packageRolesRel))
	if err != nil {
		t.Fatal(err)
	}
	constraints := buildCapabilityConstraints(rolesDoc)
	store := &cache.DigestStore{Dir: filepath.Join(dir, "digest-cache")}
	caller := &overclaimLedgerCaller{}
	_, issues, err := buildSliceObjectiveLedger(context.Background(), caller, sliceLedgerBuildRequest{
		AnalysisDir:   dir,
		EvidenceDir:   evidence,
		DraftTypo:     draft,
		Constraints:   constraints,
		ClusterMD:     "# cluster\n",
		DigestCache:   store,
		DigestSkips:   true,
		DigestModelID: "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Fatal("expected overclaim issues")
	}
	entries, err := filepath.Glob(filepath.Join(store.Dir, "ledger", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("overclaim must not be stored; found %v", entries)
	}
}

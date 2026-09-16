package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/behaviorengineering/majordomo/internal/githttps"
)

const (
	// DigestInspectSchemaV1 is the legacy inspect schema (hashed RLM markdown).
	DigestInspectSchemaV1 = "inspect-v1"
	// DigestInspectSchemaV2 keys inspect on stable package source hashes.
	DigestInspectSchemaV2 = "inspect-v2"
	// DigestLedgerSchemaV1 is the legacy ledger schema (included cluster markdown).
	DigestLedgerSchemaV1 = "ledger-v1"
	// DigestLedgerSchemaV2 keys ledger on owned-path source + constraints (no cluster prose).
	DigestLedgerSchemaV2 = "ledger-v2"
	// DigestInspectPromptV1 labels the inspect RLM prompt contract.
	DigestInspectPromptV1 = "typology_inspect_rlm_v1"
	// DigestLedgerPromptV1 labels the objective-ledger RLM prompt contract.
	DigestLedgerPromptV1 = "typology_objective_ledger_rlm_v1"
	// DigestClusterAuditSchemaV1 keys full-list cluster merge audits.
	DigestClusterAuditSchemaV1 = "cluster-audit-v1"
	// DigestClusterAuditPromptV1 labels the cluster audit RLM prompt contract.
	DigestClusterAuditPromptV1 = "typology_cluster_audit_rlm_v1"
	// DigestClusterCoTSchemaV1 keys typology cluster CoT merge proposals (full roles YAML).
	DigestClusterCoTSchemaV1 = "cluster-cot-v1"
	// DigestClusterCoTSchemaV2 keys cluster CoT on role identity + mechanical identity
	// (no ephemeral RLM evidence prose).
	DigestClusterCoTSchemaV2 = "cluster-cot-v2"
	// DigestClusterCoTPromptV1 labels the typology_cluster CoT prompt contract.
	DigestClusterCoTPromptV1 = "typology_cluster_cot_v1"
	// DigestRefineSchemaV1 keys typology refine CoT catalogs (full verdicts YAML).
	DigestRefineSchemaV1 = "refine-v1"
	// DigestRefineSchemaV2 keys refine on role/verdict identity hashes (no duration/trace/prose).
	DigestRefineSchemaV2 = "refine-v2"
	// DigestRefinePromptV1 labels the typology_refine CoT prompt contract.
	DigestRefinePromptV1 = "typology_refine_cot_v1"
	// DigestInterventionSchemaV1 keys human-intervention CoT outputs (full verdicts YAML).
	DigestInterventionSchemaV1 = "intervention-v1"
	// DigestInterventionSchemaV2 keys intervention on verdict identity (no duration/trace/prose).
	DigestInterventionSchemaV2 = "intervention-v2"
	// DigestInterventionPromptV1 labels the human-intervention CoT prompt contract.
	DigestInterventionPromptV1 = "typology_intervention_cot_v1"
	// DigestStorySchemaV1 keys bootstrap story RLM sections.
	DigestStorySchemaV1 = "story-v1"
	// DigestStoryPromptV1 labels the bootstrap_story RLM prompt contract.
	DigestStoryPromptV1 = "bootstrap_story_rlm_v1"
)

// ValidateDigestCacheBranch is an alias for ValidateInferenceCacheBranch.
func ValidateDigestCacheBranch(branch string) error {
	return ValidateInferenceCacheBranch(branch)
}

// DigestRunStats counts cache hits/misses and estimated tokens avoided this run.
type DigestRunStats struct {
	InspectHits              int
	InspectMisses            int
	LedgerHits               int
	LedgerMisses             int
	ClusterAuditHits         int
	ClusterAuditMisses       int
	ClusterCoTHits           int
	ClusterCoTMisses         int
	RefineHits               int
	RefineMisses             int
	InterventionHits         int
	InterventionMisses       int
	StoryHits                int
	StoryMisses              int
	TokensSavedPrompt        int
	TokensSavedCompletion    int
	TokensSavedTotal         int
	inspectStoredTotalSum    int
	inspectStoredTotalN      int
	ledgerStoredTotalSum     int
	ledgerStoredTotalN       int
	clusterStoredTotalSum    int
	clusterStoredTotalN      int
	clusterCoTStoredTotalSum int
	clusterCoTStoredTotalN   int
	refineStoredTotalSum     int
	refineStoredTotalN       int
	interventionStoredSum    int
	interventionStoredN      int
	storyStoredTotalSum      int
	storyStoredTotalN        int
}

// DigestStore is a local worktree (or plain directory) of digest inference JSON.
//
// When push is configured, every successful Store* MUST Flush (commit+push) before
// returning. That matches PR review cache practice: durable on the go, not only after
// the whole job succeeds. Flush is serialized; failures are reported via OnFlushError
// and MUST NOT undo the local write.
type DigestStore struct {
	Dir string

	pushMu       sync.Mutex
	pushOpts     *DigestPushOptions
	PushFn       func() error // optional test seam; overrides PushDigest when set
	OnFlushError func(error)

	statsMu sync.Mutex
	stats   DigestRunStats
}

// ConfigurePush enables push-on-the-go after each successful Store*.
func (s *DigestStore) ConfigurePush(opts DigestPushOptions) {
	if s == nil {
		return
	}
	cp := opts
	s.pushOpts = &cp
}

// Flush commits and pushes dirty digest-cache files when push is configured.
func (s *DigestStore) Flush() error {
	if s == nil {
		return nil
	}
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	var err error
	switch {
	case s.PushFn != nil:
		err = s.PushFn()
	case s.pushOpts != nil:
		err = PushDigest(*s.pushOpts)
	default:
		return nil
	}
	if err != nil && s.OnFlushError != nil {
		s.OnFlushError(err)
	}
	return err
}

func (s *DigestStore) flushAfterStore() {
	if s == nil || (s.PushFn == nil && s.pushOpts == nil) {
		return
	}
	_ = s.Flush()
}

// Stats returns a copy of hit/miss and estimated token-savings counters for this run.
func (s *DigestStore) Stats() DigestRunStats {
	if s == nil {
		return DigestRunStats{}
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return s.stats
}

// RecordInspectHit notes an inspect skip and estimated tokens avoided.
func (s *DigestStore) RecordInspectHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.InspectHits++
	if total <= 0 {
		total = s.avgLocked(s.stats.inspectStoredTotalSum, s.stats.inspectStoredTotalN)
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	s.stats.TokensSavedPrompt += prompt
	s.stats.TokensSavedCompletion += completion
	s.stats.TokensSavedTotal += total
}

// RecordInspectMiss notes an inspect provider call.
func (s *DigestStore) RecordInspectMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.InspectMisses++
}

// RecordLedgerHit notes a ledger skip and estimated tokens avoided.
func (s *DigestStore) RecordLedgerHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.LedgerHits++
	if total <= 0 {
		total = s.avgLocked(s.stats.ledgerStoredTotalSum, s.stats.ledgerStoredTotalN)
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	s.stats.TokensSavedPrompt += prompt
	s.stats.TokensSavedCompletion += completion
	s.stats.TokensSavedTotal += total
}

// RecordLedgerMiss notes a ledger provider call.
func (s *DigestStore) RecordLedgerMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.LedgerMisses++
}

// RecordClusterAuditHit notes a cluster-audit skip and estimated tokens avoided.
func (s *DigestStore) RecordClusterAuditHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.ClusterAuditHits++
	if total <= 0 {
		total = s.avgLocked(s.stats.clusterStoredTotalSum, s.stats.clusterStoredTotalN)
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	s.stats.TokensSavedPrompt += prompt
	s.stats.TokensSavedCompletion += completion
	s.stats.TokensSavedTotal += total
}

// RecordClusterAuditMiss notes a cluster-audit provider call.
func (s *DigestStore) RecordClusterAuditMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.ClusterAuditMisses++
}

func (s *DigestStore) recordTokenHit(prompt, completion, total int, bumpHits func(*DigestRunStats), avgSum, avgN *int) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	bumpHits(&s.stats)
	if total <= 0 {
		total = s.avgLocked(*avgSum, *avgN)
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	s.stats.TokensSavedPrompt += prompt
	s.stats.TokensSavedCompletion += completion
	s.stats.TokensSavedTotal += total
}

// RecordClusterCoTHit notes a typology_cluster CoT skip.
func (s *DigestStore) RecordClusterCoTHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.recordTokenHit(prompt, completion, total, func(st *DigestRunStats) { st.ClusterCoTHits++ }, &s.stats.clusterCoTStoredTotalSum, &s.stats.clusterCoTStoredTotalN)
}

// RecordClusterCoTMiss notes a typology_cluster provider call.
func (s *DigestStore) RecordClusterCoTMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.ClusterCoTMisses++
}

// RecordRefineHit notes a typology_refine CoT skip.
func (s *DigestStore) RecordRefineHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.recordTokenHit(prompt, completion, total, func(st *DigestRunStats) { st.RefineHits++ }, &s.stats.refineStoredTotalSum, &s.stats.refineStoredTotalN)
}

// RecordRefineMiss notes a typology_refine provider call.
func (s *DigestStore) RecordRefineMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.RefineMisses++
}

// RecordInterventionHit notes a human-intervention CoT skip.
func (s *DigestStore) RecordInterventionHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.recordTokenHit(prompt, completion, total, func(st *DigestRunStats) { st.InterventionHits++ }, &s.stats.interventionStoredSum, &s.stats.interventionStoredN)
}

// RecordInterventionMiss notes a human-intervention provider call.
func (s *DigestStore) RecordInterventionMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.InterventionMisses++
}

// RecordStoryHit notes a bootstrap_story section skip.
func (s *DigestStore) RecordStoryHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.recordTokenHit(prompt, completion, total, func(st *DigestRunStats) { st.StoryHits++ }, &s.stats.storyStoredTotalSum, &s.stats.storyStoredTotalN)
}

// RecordStoryMiss notes a bootstrap_story provider call.
func (s *DigestStore) RecordStoryMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.StoryMisses++
}

func (s *DigestStore) noteClusterAuditStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.clusterStoredTotalSum += total
	s.stats.clusterStoredTotalN++
}

func (s *DigestStore) noteClusterCoTStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.clusterCoTStoredTotalSum += total
	s.stats.clusterCoTStoredTotalN++
}

func (s *DigestStore) noteRefineStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.refineStoredTotalSum += total
	s.stats.refineStoredTotalN++
}

func (s *DigestStore) noteInterventionStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.interventionStoredSum += total
	s.stats.interventionStoredN++
}

func (s *DigestStore) noteStoryStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.storyStoredTotalSum += total
	s.stats.storyStoredTotalN++
}

func (s *DigestStore) noteInspectStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.inspectStoredTotalSum += total
	s.stats.inspectStoredTotalN++
}

func (s *DigestStore) noteLedgerStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.ledgerStoredTotalSum += total
	s.stats.ledgerStoredTotalN++
}

func (s *DigestStore) avgLocked(sum, n int) int {
	if n <= 0 {
		return 0
	}
	return sum / n
}

// FormatStatsLine returns a one-line operator summary of digest cache reuse.
func FormatStatsLine(st DigestRunStats) string {
	return fmt.Sprintf(
		"digest cache summary inspect_hits=%d inspect_misses=%d ledger_hits=%d ledger_misses=%d cluster_audit_hits=%d cluster_audit_misses=%d cluster_cot_hits=%d cluster_cot_misses=%d refine_hits=%d refine_misses=%d intervention_hits=%d intervention_misses=%d story_hits=%d story_misses=%d estimated_tokens_saved=%d (prompt=%d completion=%d)",
		st.InspectHits, st.InspectMisses, st.LedgerHits, st.LedgerMisses,
		st.ClusterAuditHits, st.ClusterAuditMisses,
		st.ClusterCoTHits, st.ClusterCoTMisses,
		st.RefineHits, st.RefineMisses,
		st.InterventionHits, st.InterventionMisses,
		st.StoryHits, st.StoryMisses,
		st.TokensSavedTotal, st.TokensSavedPrompt, st.TokensSavedCompletion,
	)
}

// InspectFingerprint keys one package role RLM result.
type InspectFingerprint struct {
	PackagePath   string
	ContextSHA    string
	ModelID       string
	PromptVersion string
	SchemaVersion string
}

// LedgerFingerprint keys one slice objective ledger RLM result.
type LedgerFingerprint struct {
	SliceID         string
	OwnedPathsHash  string
	ContextSHA      string
	ConstraintsHash string
	ClusterHash     string
	ModelID         string
	PromptVersion   string
	SchemaVersion   string
}

// InspectCachedRole is the durable inspect payload.
type InspectCachedRole struct {
	Path              string   `json:"path"`
	Role              string   `json:"role"`
	Confidence        float64  `json:"confidence"`
	Evidence          []string `json:"evidence"`
	InspectedStage    int      `json:"inspected_stage"`
	Language          string   `json:"language,omitempty"`
	CandidateRole     string   `json:"candidate_role,omitempty"`
	MechanicalRole    string   `json:"mechanical_role,omitempty"`
	LLMRole           string   `json:"llm_role,omitempty"`
	Agreement         string   `json:"agreement,omitempty"`
	RLMIterations     int      `json:"rlm_iterations,omitempty"`
	PromptTokens      int      `json:"prompt_tokens,omitempty"`
	CompletionTokens  int      `json:"completion_tokens,omitempty"`
	TotalTokens       int      `json:"total_tokens,omitempty"`
}

// LedgerCachedEntry is the durable grounded ledger payload.
type LedgerCachedEntry struct {
	ID               string   `json:"id"`
	OwnedPaths       []string `json:"owned_paths,omitempty"`
	Evidence         []string `json:"evidence"`
	Claims           []string `json:"claims"`
	Objective        string   `json:"objective"`
	Verdict          string   `json:"verdict"`
	Source           string   `json:"source,omitempty"`
	PromptTokens     int      `json:"prompt_tokens,omitempty"`
	CompletionTokens int      `json:"completion_tokens,omitempty"`
	TotalTokens      int      `json:"total_tokens,omitempty"`
}

// ClusterAuditFingerprint keys one full-list cluster merge audit.
type ClusterAuditFingerprint struct {
	MergesHash      string
	RolesHash       string
	ConstraintsHash string
	MechanicalHash  string
	ModelID         string
	PromptVersion   string
	SchemaVersion   string
}

// ClusterAuditCachedMerge is one cached merge verdict.
type ClusterAuditCachedMerge struct {
	ID       string   `json:"id"`
	Packages []string `json:"packages"`
	Verdict  string   `json:"verdict"`
	Reason   string   `json:"reason,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

// ClusterAuditCached is the durable cluster-audit payload.
type ClusterAuditCached struct {
	Merges           []ClusterAuditCachedMerge `json:"merges"`
	RLMIterations    int                       `json:"rlm_iterations,omitempty"`
	PromptTokens     int                       `json:"prompt_tokens,omitempty"`
	CompletionTokens int                       `json:"completion_tokens,omitempty"`
	TotalTokens      int                       `json:"total_tokens,omitempty"`
}

// ClusterCoTFingerprint keys one typology_cluster CoT proposal.
type ClusterCoTFingerprint struct {
	DraftHash       string
	RolesHash       string
	ConstraintsHash string
	MechanicalHash  string
	ModelID         string
	PromptVersion   string
	SchemaVersion   string
}

// ClusterCoTCached is the durable cluster CoT merge-field payload.
type ClusterCoTCached struct {
	MergeIDs         string `json:"merge_ids"`
	MergePackages    string `json:"merge_packages"`
	MergeIntents     string `json:"merge_intents"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
}

// RefineFingerprint keys one typology_refine CoT catalog.
type RefineFingerprint struct {
	DraftHash       string
	RolesHash       string
	ConstraintsHash string
	LedgerHash      string
	VerdictsHash    string
	MechanicalHash  string
	ModelID         string
	PromptVersion   string
	SchemaVersion   string
}

// RefineCached is the durable sanitized refined catalog.
type RefineCached struct {
	RefinedCatalogYAML string `json:"refined_catalog_yaml"`
	PromptTokens       int    `json:"prompt_tokens,omitempty"`
	CompletionTokens   int    `json:"completion_tokens,omitempty"`
	TotalTokens        int    `json:"total_tokens,omitempty"`
}

// InterventionFingerprint keys one human-intervention CoT task output.
type InterventionFingerprint struct {
	TaskID           string
	ArchitectureHash string
	RefinedHash      string
	VerdictsHash     string
	FindingsHash     string
	FindingHash      string // per finding-comment; empty for journey/brief/weaknesses/priority
	ModelID          string
	PromptVersion    string
	SchemaVersion    string
}

// InterventionCached is durable intervention markdown.
type InterventionCached struct {
	Markdown         string `json:"markdown"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
}

// StoryFingerprint keys one bootstrap_story RLM section.
type StoryFingerprint struct {
	SectionID        string
	RefinedHash      string
	LedgerHash       string
	ArchitectureHash string
	ReadmeHash       string
	ModelID          string
	PromptVersion    string
	SchemaVersion    string
}

// StoryCached is durable story-section markdown.
type StoryCached struct {
	Markdown         string `json:"markdown"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
}

type digestRecord struct {
	Kind        string          `json:"kind"`
	Fingerprint string          `json:"fingerprint"`
	CreatedAt   string          `json:"created_at"`
	Payload     json.RawMessage `json:"payload"`
}

// HashDigestParts returns SHA-256 hex of null-separated parts.
func HashDigestParts(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ContentSHA returns SHA-256 hex of raw bytes.
func ContentSHA(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// OwnedPathsHash hashes a stable owned-path list.
func OwnedPathsHash(paths []string) string {
	return ClusterFilesHash(paths)
}

func (fp InspectFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestInspectPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestInspectSchemaV2
	}
	return HashDigestParts("inspect", fp.PackagePath, fp.ContextSHA, fp.ModelID, prompt, schema)
}

func (fp LedgerFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestLedgerPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestLedgerSchemaV2
	}
	if schema == DigestLedgerSchemaV1 {
		return HashDigestParts(
			"ledger", fp.SliceID, fp.OwnedPathsHash, fp.ContextSHA, fp.ConstraintsHash,
			fp.ClusterHash, fp.ModelID, prompt, schema,
		)
	}
	// v2+: cluster proposal prose is not an invalidation input.
	return HashDigestParts(
		"ledger", fp.SliceID, fp.OwnedPathsHash, fp.ContextSHA, fp.ConstraintsHash,
		fp.ModelID, prompt, schema,
	)
}

func (fp ClusterAuditFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestClusterAuditPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestClusterAuditSchemaV1
	}
	return HashDigestParts(
		"cluster_audit", fp.MergesHash, fp.RolesHash, fp.ConstraintsHash, fp.MechanicalHash,
		fp.ModelID, prompt, schema,
	)
}

func (fp ClusterCoTFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestClusterCoTPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestClusterCoTSchemaV2
	}
	return HashDigestParts(
		"cluster_cot", fp.DraftHash, fp.RolesHash, fp.ConstraintsHash, fp.MechanicalHash,
		fp.ModelID, prompt, schema,
	)
}

func (fp RefineFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestRefinePromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestRefineSchemaV2
	}
	return HashDigestParts(
		"refine", fp.DraftHash, fp.RolesHash, fp.ConstraintsHash, fp.LedgerHash, fp.VerdictsHash,
		fp.MechanicalHash, fp.ModelID, prompt, schema,
	)
}

func (fp InterventionFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestInterventionPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestInterventionSchemaV2
	}
	return HashDigestParts(
		"intervention", fp.TaskID, fp.ArchitectureHash, fp.RefinedHash, fp.VerdictsHash,
		fp.FindingsHash, fp.FindingHash, fp.ModelID, prompt, schema,
	)
}

func (fp StoryFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestStoryPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestStorySchemaV1
	}
	return HashDigestParts(
		"story", fp.SectionID, fp.RefinedHash, fp.LedgerHash, fp.ArchitectureHash, fp.ReadmeHash,
		fp.ModelID, prompt, schema,
	)
}

func (s *DigestStore) inspectPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "inspect", key+".json")
}

func (s *DigestStore) ledgerPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "ledger", key+".json")
}

func (s *DigestStore) clusterAuditPath(key string) string {
	return filepath.Join(s.Dir, "cluster_audit", key+".json")
}

func (s *DigestStore) clusterCoTPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "cluster", key+".json")
}

func (s *DigestStore) refinePath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "refine", key+".json")
}

func (s *DigestStore) interventionPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "intervention", key+".json")
}

func (s *DigestStore) storyPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "story", key+".json")
}

// LookupInspect returns a cached inspect role when the fingerprint matches.
func (s *DigestStore) LookupInspect(fp InspectFingerprint) (InspectCachedRole, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return InspectCachedRole{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.inspectPath(key), key)
	if err != nil || !ok {
		return InspectCachedRole{}, false, err
	}
	var out InspectCachedRole
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return InspectCachedRole{}, false, fmt.Errorf("digest inspect payload: %w", err)
	}
	return out, true, nil
}

// StoreInspect writes a successful inspect result.
func (s *DigestStore) StoreInspect(fp InspectFingerprint, role InspectCachedRole) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	key := fp.key()
	payload, err := json.Marshal(role)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.inspectPath(key), digestRecord{
		Kind:        "inspect",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteInspectStoredUsage(role.TotalTokens)
	s.flushAfterStore()
	return nil
}

// LookupLedger returns a cached grounded ledger entry when the fingerprint matches.
func (s *DigestStore) LookupLedger(fp LedgerFingerprint) (LedgerCachedEntry, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return LedgerCachedEntry{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.ledgerPath(key), key)
	if err != nil || !ok {
		return LedgerCachedEntry{}, false, err
	}
	var out LedgerCachedEntry
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return LedgerCachedEntry{}, false, fmt.Errorf("digest ledger payload: %w", err)
	}
	return out, true, nil
}

// StoreLedger writes a grounded ledger entry.
func (s *DigestStore) StoreLedger(fp LedgerFingerprint, entry LedgerCachedEntry) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.Verdict) != "grounded" {
		return fmt.Errorf("digest ledger store refuses verdict %q", entry.Verdict)
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.ledgerPath(key), digestRecord{
		Kind:        "ledger",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteLedgerStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}

// LookupClusterAudit returns a cached full-list cluster audit when the fingerprint matches.
func (s *DigestStore) LookupClusterAudit(fp ClusterAuditFingerprint) (ClusterAuditCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return ClusterAuditCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.clusterAuditPath(key), key)
	if err != nil || !ok {
		return ClusterAuditCached{}, false, err
	}
	var out ClusterAuditCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return ClusterAuditCached{}, false, fmt.Errorf("digest cluster_audit payload: %w", err)
	}
	return out, true, nil
}

// StoreClusterAudit writes a successful cluster-audit result.
func (s *DigestStore) StoreClusterAudit(fp ClusterAuditFingerprint, entry ClusterAuditCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.clusterAuditPath(key), digestRecord{
		Kind:        "cluster_audit",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteClusterAuditStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}

// LookupClusterCoT returns a cached typology_cluster CoT proposal when the fingerprint matches.
func (s *DigestStore) LookupClusterCoT(fp ClusterCoTFingerprint) (ClusterCoTCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return ClusterCoTCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.clusterCoTPath(key), key)
	if err != nil || !ok {
		return ClusterCoTCached{}, false, err
	}
	var out ClusterCoTCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return ClusterCoTCached{}, false, fmt.Errorf("digest cluster_cot payload: %w", err)
	}
	return out, true, nil
}

// StoreClusterCoT writes a successful typology_cluster CoT proposal.
func (s *DigestStore) StoreClusterCoT(fp ClusterCoTFingerprint, entry ClusterCoTCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.MergeIDs) == "" || strings.TrimSpace(entry.MergePackages) == "" || strings.TrimSpace(entry.MergeIntents) == "" {
		return fmt.Errorf("digest cluster_cot store refuses empty merge fields")
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.clusterCoTPath(key), digestRecord{
		Kind:        "cluster_cot",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteClusterCoTStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}

// LookupRefine returns a cached typology_refine catalog when the fingerprint matches.
func (s *DigestStore) LookupRefine(fp RefineFingerprint) (RefineCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return RefineCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.refinePath(key), key)
	if err != nil || !ok {
		return RefineCached{}, false, err
	}
	var out RefineCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return RefineCached{}, false, fmt.Errorf("digest refine payload: %w", err)
	}
	return out, true, nil
}

// StoreRefine writes a validated refined catalog.
func (s *DigestStore) StoreRefine(fp RefineFingerprint, entry RefineCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.RefinedCatalogYAML) == "" {
		return fmt.Errorf("digest refine store refuses empty catalog")
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.refinePath(key), digestRecord{
		Kind:        "refine",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteRefineStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}

// LookupIntervention returns cached human-intervention markdown when the fingerprint matches.
func (s *DigestStore) LookupIntervention(fp InterventionFingerprint) (InterventionCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return InterventionCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.interventionPath(key), key)
	if err != nil || !ok {
		return InterventionCached{}, false, err
	}
	var out InterventionCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return InterventionCached{}, false, fmt.Errorf("digest intervention payload: %w", err)
	}
	return out, true, nil
}

// StoreIntervention writes validated human-intervention markdown.
func (s *DigestStore) StoreIntervention(fp InterventionFingerprint, entry InterventionCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.Markdown) == "" {
		return fmt.Errorf("digest intervention store refuses empty markdown")
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.interventionPath(key), digestRecord{
		Kind:        "intervention",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteInterventionStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}

// LookupStory returns a cached bootstrap_story section when the fingerprint matches.
func (s *DigestStore) LookupStory(fp StoryFingerprint) (StoryCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return StoryCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.storyPath(key), key)
	if err != nil || !ok {
		return StoryCached{}, false, err
	}
	var out StoryCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return StoryCached{}, false, fmt.Errorf("digest story payload: %w", err)
	}
	return out, true, nil
}

// StoreStory writes a validated bootstrap_story section.
func (s *DigestStore) StoreStory(fp StoryFingerprint, entry StoryCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.Markdown) == "" {
		return fmt.Errorf("digest story store refuses empty markdown")
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.storyPath(key), digestRecord{
		Kind:        "story",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteStoryStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}

func readDigestRecord(path, wantKey string) (digestRecord, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return digestRecord{}, false, nil
		}
		return digestRecord{}, false, err
	}
	var rec digestRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return digestRecord{}, false, err
	}
	if rec.Fingerprint != wantKey {
		return digestRecord{}, false, nil
	}
	return rec, true, nil
}

func writeDigestRecord(path string, rec digestRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeJSONFile(path, rec)
}

// DigestPushOptions configures pushing a digest-cache worktree.
type DigestPushOptions struct {
	Remote   string
	Branch   string
	Worktree string
	Token    string
	SCM      string
}

// PushDigest pushes the digest-cache branch (orphan-friendly first push).
func PushDigest(opts DigestPushOptions) error {
	if err := ValidateInferenceCacheBranch(opts.Branch); err != nil {
		return err
	}
	if opts.Remote == "" || opts.Worktree == "" {
		return fmt.Errorf("--remote and --worktree required")
	}
	token := opts.Token
	if token == "" {
		token = first(os.Getenv("BITBUCKET_TOKEN"), os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))
	}
	if token == "" {
		return fmt.Errorf("token required (BITBUCKET_TOKEN or GITHUB_TOKEN)")
	}
	info, err := os.Stat(opts.Worktree)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("worktree not found: %s", opts.Worktree)
	}
	scm := opts.SCM
	if scm == "" {
		scm = githttps.InferSCM(opts.Remote)
	}
	authArgs := githttps.ExtraHeaderArgs(token, scm)
	run := func(args ...string) error {
		cmdArgs := append([]string{"-C", opts.Worktree}, authArgs...)
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command("git", cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	_ = run("add", "-A")
	// Commit only when there is something to commit.
	if err := run("diff", "--cached", "--quiet"); err != nil {
		if err := run("commit", "-m", "digest inference cache"); err != nil {
			return fmt.Errorf("digest cache commit: %w", err)
		}
	}
	_ = run("fetch", opts.Remote, opts.Branch+":"+opts.Branch)
	if err := run("push", opts.Remote, "HEAD:"+opts.Branch); err != nil {
		if ferr := run("fetch", opts.Remote, opts.Branch); ferr != nil {
			return fmt.Errorf("digest cache push failed (%v); refetch also failed: %w", err, ferr)
		}
		if err := run("push", opts.Remote, "HEAD:"+opts.Branch); err != nil {
			return fmt.Errorf("digest cache push failed after refetch: %w", err)
		}
	}
	return nil
}

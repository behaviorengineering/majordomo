package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/behaviorengineering/majordomo/internal/githttps"
)

const (
	// DigestInspectSchemaV1 is the inspect payload / fingerprint schema id.
	DigestInspectSchemaV1 = "inspect-v1"
	// DigestLedgerSchemaV1 is the ledger payload / fingerprint schema id.
	DigestLedgerSchemaV1 = "ledger-v1"
	// DigestInspectPromptV1 labels the inspect RLM prompt contract.
	DigestInspectPromptV1 = "typology_inspect_rlm_v1"
	// DigestLedgerPromptV1 labels the objective-ledger RLM prompt contract.
	DigestLedgerPromptV1 = "typology_objective_ledger_rlm_v1"
)

var digestBranchRE = regexp.MustCompile(`^majordomo-digest-cache/[a-z0-9][a-z0-9._/-]*$`)

// ValidateDigestCacheBranch returns nil if branch name is allowed.
func ValidateDigestCacheBranch(branch string) error {
	if !digestBranchRE.MatchString(branch) {
		return fmt.Errorf("digest cache branch %q does not match majordomo-digest-cache/<repo-id>", branch)
	}
	return nil
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
	Path           string   `json:"path"`
	Role           string   `json:"role"`
	Confidence     float64  `json:"confidence"`
	Evidence       []string `json:"evidence"`
	InspectedStage int      `json:"inspected_stage"`
	Language       string   `json:"language,omitempty"`
	CandidateRole  string   `json:"candidate_role,omitempty"`
	MechanicalRole string   `json:"mechanical_role,omitempty"`
	LLMRole        string   `json:"llm_role,omitempty"`
	Agreement      string   `json:"agreement,omitempty"`
	RLMIterations  int      `json:"rlm_iterations,omitempty"`
}

// LedgerCachedEntry is the durable grounded ledger payload.
type LedgerCachedEntry struct {
	ID         string   `json:"id"`
	OwnedPaths []string `json:"owned_paths,omitempty"`
	Evidence   []string `json:"evidence"`
	Claims     []string `json:"claims"`
	Objective  string   `json:"objective"`
	Verdict    string   `json:"verdict"`
	Source     string   `json:"source,omitempty"`
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
		schema = DigestInspectSchemaV1
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
		schema = DigestLedgerSchemaV1
	}
	return HashDigestParts(
		"ledger", fp.SliceID, fp.OwnedPathsHash, fp.ContextSHA, fp.ConstraintsHash,
		fp.ClusterHash, fp.ModelID, prompt, schema,
	)
}

func (s *DigestStore) inspectPath(key string) string {
	return filepath.Join(s.Dir, "inspect", key+".json")
}

func (s *DigestStore) ledgerPath(key string) string {
	return filepath.Join(s.Dir, "ledger", key+".json")
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
	if err := ValidateDigestCacheBranch(opts.Branch); err != nil {
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

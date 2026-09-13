package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	proposedMergesMachineSection = "## Proposed merges (machine)"
	clusterMergeVerdictsRel      = "cluster_merge_verdicts.yaml"
	maxClusterAuditMerges        = 32

	mergeIntentSlice    = "slice"
	mergeIntentNickname = "nickname"

	verdictAccept  = "accept"
	verdictOverlay = "overlay"
	verdictReject  = "reject"
)

var (
	proposedMergesMachineRE = regexp.MustCompile(`(?im)^##\s*Proposed merges \(machine\)\s*$`)
	markdownH2RE            = regexp.MustCompile(`(?m)^##\s`)
)

// proposedMerge is one machine-parseable cluster fold proposal.
type proposedMerge struct {
	ID       string   `yaml:"id"`
	Packages []string `yaml:"packages"`
	Intent   string   `yaml:"intent"` // slice | nickname
}

// clusterMergeVerdict is one audited merge row.
type clusterMergeVerdict struct {
	ID       string   `yaml:"id"`
	Packages []string `yaml:"packages"`
	Verdict  string   `yaml:"verdict"` // accept | overlay | reject
	Reason   string   `yaml:"reason,omitempty"`
	Evidence []string `yaml:"evidence,omitempty"`
}

// clusterMergeVerdictsDoc is the durable audit sidecar.
type clusterMergeVerdictsDoc struct {
	Attempt       int                    `yaml:"attempt"`
	RLMIterations int                    `yaml:"rlm_iterations,omitempty"`
	DurationMS    int64                  `yaml:"duration_ms,omitempty"`
	PromptTokens  int                    `yaml:"prompt_tokens,omitempty"`
	CompletionTok int                    `yaml:"completion_tokens,omitempty"`
	TotalTokens   int                    `yaml:"total_tokens,omitempty"`
	ProposedRows  int                    `yaml:"proposed_rows"`
	TraceDir      string                 `yaml:"trace_dir,omitempty"`
	GeneratedAt   string                 `yaml:"generated_at,omitempty"`
	Merges        []clusterMergeVerdict  `yaml:"merges"`
}

// packageSetKey fingerprints a merge by sorted package paths (not nickname id).
func packageSetKey(packages []string) string {
	norm := normalizePackageList(packages)
	if len(norm) == 0 {
		return ""
	}
	return strings.Join(norm, "\x00")
}

func normalizePackageList(packages []string) []string {
	seen := make(map[string]struct{}, len(packages))
	out := make([]string, 0, len(packages))
	for _, p := range packages {
		p = normalizeRolePath(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// parseProposedMergesYAML decodes the structured typology_cluster output field.
// Empty list is valid. Missing/blank input is an error (field is mandatory).
func parseProposedMergesYAML(raw string) ([]proposedMerge, error) {
	body := strings.TrimSpace(stripCodeFence(raw))
	if body == "" {
		return nil, fmt.Errorf("proposed_merges_yaml is required")
	}
	if body == "[]" || strings.EqualFold(body, "none") || strings.EqualFold(body, "- []") {
		return nil, nil
	}
	var merges []proposedMerge
	if err := yaml.Unmarshal([]byte(body), &merges); err != nil {
		return nil, fmt.Errorf("proposed_merges_yaml decode: %w", err)
	}
	out := make([]proposedMerge, 0, len(merges))
	seenIDs := make(map[string]struct{}, len(merges))
	for i, row := range merges {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			return nil, fmt.Errorf("proposed_merges_yaml[%d] missing id", i)
		}
		if _, ok := seenIDs[id]; ok {
			return nil, fmt.Errorf("proposed_merges_yaml duplicate id %q", id)
		}
		seenIDs[id] = struct{}{}
		pkgs := normalizePackageList(row.Packages)
		if len(pkgs) == 0 {
			return nil, fmt.Errorf("proposed_merges_yaml %q has no packages", id)
		}
		intent := strings.ToLower(strings.TrimSpace(row.Intent))
		if intent == "" {
			intent = mergeIntentSlice
		}
		if intent != mergeIntentSlice && intent != mergeIntentNickname {
			return nil, fmt.Errorf("proposed_merges_yaml %q intent %q must be slice or nickname", id, row.Intent)
		}
		out = append(out, proposedMerge{ID: id, Packages: pkgs, Intent: intent})
	}
	if len(out) > maxClusterAuditMerges {
		return nil, fmt.Errorf("proposed_merges_yaml has %d rows; max is %d", len(out), maxClusterAuditMerges)
	}
	return out, nil
}

func extractProposedMergesMachineBody(proposalMD string) (string, bool) {
	loc := proposedMergesMachineRE.FindStringIndex(proposalMD)
	if loc == nil {
		return "", false
	}
	rest := proposalMD[loc[1]:]
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	next := markdownH2RE.FindStringIndex(rest)
	if next == nil {
		return rest, true
	}
	return rest[:next[0]], true
}

// stickyVerdictMap keys verdicts by package-set fingerprint.
type stickyVerdictMap map[string]clusterMergeVerdict

func (m stickyVerdictMap) get(packages []string) (clusterMergeVerdict, bool) {
	if m == nil {
		return clusterMergeVerdict{}, false
	}
	v, ok := m[packageSetKey(packages)]
	return v, ok
}

func (m stickyVerdictMap) put(v clusterMergeVerdict) {
	if m == nil {
		return
	}
	key := packageSetKey(v.Packages)
	if key == "" {
		return
	}
	v.Packages = normalizePackageList(v.Packages)
	m[key] = v
}

func (m stickyVerdictMap) applySticky(proposed []proposedMerge) (pending []proposedMerge, frozen []clusterMergeVerdict) {
	for _, p := range proposed {
		if p.Intent == mergeIntentNickname {
			frozen = append(frozen, clusterMergeVerdict{
				ID:       p.ID,
				Packages: p.Packages,
				Verdict:  verdictOverlay,
				Reason:   "proposed as teaching nickname",
			})
			continue
		}
		if prev, ok := m.get(p.Packages); ok {
			prev.ID = p.ID // keep latest nickname for the same package set
			frozen = append(frozen, prev)
			continue
		}
		pending = append(pending, p)
	}
	return pending, frozen
}

// demoteRejectedMergesList rewrites reject/overlay rows to nickname intent.
func demoteRejectedMergesList(merges []proposedMerge, verdicts []clusterMergeVerdict) ([]proposedMerge, []string) {
	byID := make(map[string]clusterMergeVerdict, len(verdicts))
	for _, v := range verdicts {
		byID[v.ID] = v
	}
	out := append([]proposedMerge(nil), merges...)
	var nicknames []string
	for i := range out {
		v, ok := byID[out[i].ID]
		if !ok {
			continue
		}
		switch v.Verdict {
		case verdictReject, verdictOverlay:
			out[i].Intent = mergeIntentNickname
			nicknames = append(nicknames, fmt.Sprintf("- `%s` (%s): %s", out[i].ID, v.Verdict, strings.TrimSpace(v.Reason)))
		}
	}
	return out, nicknames
}

// syncDemotedMergesIntoTeachingMD writes audited merges into the counsel artifact for cold readers.
// Gates MUST read proposed_merges_yaml, not this markdown sync.
func syncDemotedMergesIntoTeachingMD(proposalMD string, merges []proposedMerge, nicknames []string) string {
	proposalMD = replaceProposedMergesMachine(proposalMD, merges)
	if len(nicknames) == 0 {
		return proposalMD
	}
	if strings.Contains(proposalMD, "## Teaching nicknames") {
		return proposalMD
	}
	block := "## Teaching nicknames\n\n" +
		"These labels are cold-reader overlays, not refined-catalog slices.\n\n" +
		strings.Join(nicknames, "\n") + "\n"
	return strings.TrimSpace(proposalMD) + "\n\n" + block + "\n"
}

func replaceProposedMergesMachine(proposalMD string, merges []proposedMerge) string {
	body, err := yaml.Marshal(merges)
	if err != nil {
		return proposalMD
	}
	replacement := proposedMergesMachineSection + "\n" + strings.TrimSpace(string(body)) + "\n"
	loc := proposedMergesMachineRE.FindStringIndex(proposalMD)
	if loc == nil {
		return strings.TrimSpace(proposalMD) + "\n\n" + replacement
	}
	rest := proposalMD[loc[1]:]
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	next := markdownH2RE.FindStringIndex(rest)
	if next == nil {
		return proposalMD[:loc[0]] + replacement
	}
	return proposalMD[:loc[0]] + replacement + "\n" + rest[next[0]:]
}

func formatClusterAuditRejectFeedback(verdicts []clusterMergeVerdict) string {
	var b strings.Builder
	b.WriteString("Cluster merge audit rejects (revise membership; do not re-propose the same package set as intent: slice):\n")
	for _, v := range verdicts {
		if v.Verdict == verdictAccept {
			continue
		}
		fmt.Fprintf(&b, "- id=%s verdict=%s packages=[%s] reason=%s\n",
			v.ID, v.Verdict, strings.Join(v.Packages, ", "), strings.TrimSpace(v.Reason))
	}
	return b.String()
}

func hasOpenClusterRejects(verdicts []clusterMergeVerdict) bool {
	for _, v := range verdicts {
		if v.Verdict == verdictReject {
			return true
		}
	}
	return false
}

func acceptedPackageSets(verdicts []clusterMergeVerdict) map[string]struct{} {
	out := make(map[string]struct{})
	for _, v := range verdicts {
		if v.Verdict != verdictAccept {
			continue
		}
		if key := packageSetKey(v.Packages); key != "" {
			out[key] = struct{}{}
		}
	}
	return out
}

func marshalClusterMergeVerdicts(doc clusterMergeVerdictsDoc) (string, error) {
	if doc.GeneratedAt == "" {
		doc.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return "", fmt.Errorf("cluster_merge_verdicts encode: %w", err)
	}
	return string(data), nil
}

func writeClusterMergeVerdicts(path string, doc clusterMergeVerdictsDoc) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cluster_merge_verdicts mkdir: %w", err)
	}
	raw, err := marshalClusterMergeVerdicts(doc)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		return fmt.Errorf("cluster_merge_verdicts write: %w", err)
	}
	return nil
}

func parseClusterMergeVerdictsYAML(raw string) (clusterMergeVerdictsDoc, error) {
	raw = strings.TrimSpace(stripCodeFence(raw))
	if raw == "" {
		return clusterMergeVerdictsDoc{}, fmt.Errorf("cluster_merge_verdicts_yaml is required")
	}
	var doc clusterMergeVerdictsDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return clusterMergeVerdictsDoc{}, fmt.Errorf("cluster_merge_verdicts decode: %w", err)
	}
	return doc, nil
}

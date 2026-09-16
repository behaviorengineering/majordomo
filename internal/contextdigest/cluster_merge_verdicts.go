package contextdigest

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	clusterMergeVerdictsRel = "cluster_merge_verdicts.yaml"
	clusterMergeProposalRel = "cluster_merge_proposal.yaml"
	maxClusterAuditMerges   = 32

	mergeIntentSlice    = "slice"
	mergeIntentNickname = "nickname"

	verdictAccept  = "accept"
	verdictOverlay = "overlay"
	verdictReject  = "reject"
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
	Attempt       int                   `yaml:"attempt"`
	RLMIterations int                   `yaml:"rlm_iterations,omitempty"`
	DurationMS    int64                 `yaml:"duration_ms,omitempty"`
	PromptTokens  int                   `yaml:"prompt_tokens,omitempty"`
	CompletionTok int                   `yaml:"completion_tokens,omitempty"`
	TotalTokens   int                   `yaml:"total_tokens,omitempty"`
	ProposedRows  int                   `yaml:"proposed_rows"`
	TraceDir      string                `yaml:"trace_dir,omitempty"`
	GeneratedAt   string                `yaml:"generated_at,omitempty"`
	Merges        []clusterMergeVerdict `yaml:"merges"`
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

// mergesFromClusterOut zips parallel cluster CoT fields into proposed merges.
// Fields are flat strings (comma / semicolon separated). Literal "none" means no folds.
// Strop rejects empty mandatory arrays, so empty-list proposals use the none sentinel.
func mergesFromClusterOut(out map[string]interface{}) ([]proposedMerge, error) {
	idsRaw := strings.TrimSpace(stringField(out, "merge_ids"))
	pkgsRaw := strings.TrimSpace(stringField(out, "merge_packages"))
	intentsRaw := strings.TrimSpace(stringField(out, "merge_intents"))

	// Backward compatible: accept []string from older stubs / XML array parse.
	if idsRaw == "" {
		idsRaw = strings.Join(stringListField(out, "merge_ids"), ",")
	}
	if pkgsRaw == "" {
		pkgsRaw = strings.Join(stringListField(out, "merge_packages"), ";")
	}
	if intentsRaw == "" {
		intentsRaw = strings.Join(stringListField(out, "merge_intents"), ",")
	}

	if isMergeNoneSentinel(idsRaw) && isMergeNoneSentinel(pkgsRaw) && isMergeNoneSentinel(intentsRaw) {
		return nil, nil
	}
	if isMergeNoneSentinel(idsRaw) || isMergeNoneSentinel(pkgsRaw) || isMergeNoneSentinel(intentsRaw) {
		return nil, fmt.Errorf("merge_ids / merge_packages / merge_intents must all be none or all carry the same number of rows")
	}

	ids := splitCommaTokens(idsRaw)
	pkgGroups := splitSemicolonGroups(pkgsRaw)
	intents := splitCommaTokens(intentsRaw)
	if len(ids) == 0 && len(pkgGroups) == 0 && len(intents) == 0 {
		return nil, nil
	}
	if len(ids) != len(pkgGroups) || len(ids) != len(intents) {
		return nil, fmt.Errorf("merge_ids (%d), merge_packages (%d), merge_intents (%d) must be the same length",
			len(ids), len(pkgGroups), len(intents))
	}
	rows := make([]proposedMerge, 0, len(ids))
	seenIDs := make(map[string]struct{}, len(ids))
	for i := range ids {
		id := strings.TrimSpace(ids[i])
		if id == "" || strings.EqualFold(id, "none") {
			return nil, fmt.Errorf("merge_ids[%d] is empty", i)
		}
		if _, ok := seenIDs[id]; ok {
			return nil, fmt.Errorf("merge_ids duplicate id %q", id)
		}
		seenIDs[id] = struct{}{}
		pkgList := normalizePackageList(splitCommaPackages(pkgGroups[i]))
		if len(pkgList) == 0 {
			return nil, fmt.Errorf("merge %q has no packages", id)
		}
		intent := strings.ToLower(strings.TrimSpace(intents[i]))
		if intent == "" {
			intent = mergeIntentSlice
		}
		if intent != mergeIntentSlice && intent != mergeIntentNickname {
			return nil, fmt.Errorf("merge %q intent %q must be slice or nickname", id, intents[i])
		}
		rows = append(rows, proposedMerge{ID: id, Packages: pkgList, Intent: intent})
	}
	if len(rows) > maxClusterAuditMerges {
		return nil, fmt.Errorf("cluster merge proposal has %d rows; max is %d", len(rows), maxClusterAuditMerges)
	}
	return rows, nil
}

func isMergeNoneSentinel(raw string) bool {
	s := strings.TrimSpace(strings.ToLower(raw))
	return s == "" || s == "none" || s == "-" || s == "[]"
}

// flattenMergesForCache encodes proposed merges back into CoT flat string fields.
func flattenMergesForCache(merges []proposedMerge) (ids, pkgs, intents string) {
	if len(merges) == 0 {
		return "none", "none", "none"
	}
	idParts := make([]string, 0, len(merges))
	pkgParts := make([]string, 0, len(merges))
	intentParts := make([]string, 0, len(merges))
	for _, m := range merges {
		idParts = append(idParts, m.ID)
		pkgParts = append(pkgParts, strings.Join(m.Packages, ","))
		intentParts = append(intentParts, m.Intent)
	}
	return strings.Join(idParts, ","), strings.Join(pkgParts, ";"), strings.Join(intentParts, ",")
}

func splitCommaTokens(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitSemicolonGroups(raw string) []string {
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func stringListField(out map[string]interface{}, key string) []string {
	if out == nil {
		return nil
	}
	raw, ok := out[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, strings.TrimSpace(fmt.Sprint(item)))
		}
		return out
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}

func splitCommaPackages(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// marshalClusterMergeProposal encodes the proposal evidence file body.
func marshalClusterMergeProposal(merges []proposedMerge) (string, error) {
	if merges == nil {
		merges = []proposedMerge{}
	}
	raw, err := yaml.Marshal(merges)
	if err != nil {
		return "", fmt.Errorf("cluster_merge_proposal encode: %w", err)
	}
	return string(raw), nil
}

// parseProposedMergesYAML decodes cluster_merge_proposal.yaml (or equivalent YAML body).
func parseProposedMergesYAML(raw string) ([]proposedMerge, error) {
	body := strings.TrimSpace(stripCodeFence(raw))
	if body == "" || body == "[]" || strings.EqualFold(body, "none") || strings.EqualFold(body, "- []") {
		return nil, nil
	}
	var merges []proposedMerge
	if err := yaml.Unmarshal([]byte(body), &merges); err != nil {
		return nil, fmt.Errorf("cluster_merge_proposal decode: %w", err)
	}
	out := make([]proposedMerge, 0, len(merges))
	seenIDs := make(map[string]struct{}, len(merges))
	for i, row := range merges {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			return nil, fmt.Errorf("cluster_merge_proposal[%d] missing id", i)
		}
		if _, ok := seenIDs[id]; ok {
			return nil, fmt.Errorf("cluster_merge_proposal duplicate id %q", id)
		}
		seenIDs[id] = struct{}{}
		pkgs := normalizePackageList(row.Packages)
		if len(pkgs) == 0 {
			return nil, fmt.Errorf("cluster_merge_proposal %q has no packages", id)
		}
		intent := strings.ToLower(strings.TrimSpace(row.Intent))
		if intent == "" {
			intent = mergeIntentSlice
		}
		if intent != mergeIntentSlice && intent != mergeIntentNickname {
			return nil, fmt.Errorf("cluster_merge_proposal %q intent %q must be slice or nickname", id, row.Intent)
		}
		out = append(out, proposedMerge{ID: id, Packages: pkgs, Intent: intent})
	}
	if len(out) > maxClusterAuditMerges {
		return nil, fmt.Errorf("cluster_merge_proposal has %d rows; max is %d", len(out), maxClusterAuditMerges)
	}
	return out, nil
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
			prev.ID = p.ID
			frozen = append(frozen, prev)
			continue
		}
		pending = append(pending, p)
	}
	return pending, frozen
}

// demoteRejectedMergesList sets reject/overlay rows to nickname intent on the structured list.
func demoteRejectedMergesList(merges []proposedMerge, verdicts []clusterMergeVerdict) []proposedMerge {
	if len(merges) == 0 {
		return merges
	}
	byID := make(map[string]clusterMergeVerdict, len(verdicts))
	for _, v := range verdicts {
		byID[v.ID] = v
	}
	out := make([]proposedMerge, len(merges))
	copy(out, merges)
	for i := range out {
		v, ok := byID[out[i].ID]
		if !ok {
			continue
		}
		switch v.Verdict {
		case verdictReject, verdictOverlay:
			out[i].Intent = mergeIntentNickname
		}
	}
	return out
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

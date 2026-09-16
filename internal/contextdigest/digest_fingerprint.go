package contextdigest

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/cache"
	"gopkg.in/yaml.v3"
)

// rolesIdentitySHA hashes role decisions without ephemeral RLM evidence prose,
// llm_role quotes, or rlm_iterations. Used for cluster/refine cache keys.
func rolesIdentitySHA(rolesYAML string) string {
	doc, err := parseRolesYAML(rolesYAML)
	if err != nil {
		return cache.ContentSHA(rolesYAML)
	}
	parts := make([]string, 0, len(doc.Packages)*6+len(doc.Edges)*3)
	pkgs := append([]packageRoleNode(nil), doc.Packages...)
	sort.Slice(pkgs, func(i, j int) bool {
		return normalizeRolePath(pkgs[i].Path) < normalizeRolePath(pkgs[j].Path)
	})
	for _, n := range pkgs {
		parts = append(parts,
			normalizeRolePath(n.Path),
			strings.TrimSpace(n.Role),
			strings.TrimSpace(n.MechanicalRole),
			strings.TrimSpace(n.Agreement),
			strings.TrimSpace(n.CandidateRole),
			strconv.FormatFloat(n.Confidence, 'f', 4, 64),
			strconv.Itoa(n.InspectedStage),
		)
	}
	edges := append([]packageRoleEdge(nil), doc.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		a := edges[i].From + "\x00" + edges[i].To + "\x00" + edges[i].Kind
		b := edges[j].From + "\x00" + edges[j].To + "\x00" + edges[j].Kind
		return a < b
	})
	for _, e := range edges {
		parts = append(parts, e.From, e.To, e.Kind)
	}
	return cache.HashDigestParts(parts...)
}

// mechanicalIdentitySHA builds mechanical grouping from role identity fields only
// (no evidence / llm_role / rlm_iterations) so the cluster key stays stable across
// reseeds that regenerate RLM prose for the same role decisions.
func mechanicalIdentitySHA(rolesYAML string) (string, error) {
	doc, err := parseRolesYAML(rolesYAML)
	if err != nil {
		return "", err
	}
	stable := packageRolesDoc{
		Packages: make([]packageRoleNode, 0, len(doc.Packages)),
		Edges:    append([]packageRoleEdge(nil), doc.Edges...),
	}
	for _, n := range doc.Packages {
		stable.Packages = append(stable.Packages, packageRoleNode{
			Path:           n.Path,
			Role:           n.Role,
			Confidence:     n.Confidence,
			InspectedStage: n.InspectedStage,
			CandidateRole:  n.CandidateRole,
			MechanicalRole: n.MechanicalRole,
			Agreement:      n.Agreement,
			Language:       n.Language,
		})
	}
	yamlOut, err := mechanicalPreClusterYAML(stable)
	if err != nil {
		return "", err
	}
	return cache.ContentSHA(yamlOut), nil
}

// draftCatalogIdentitySHA hashes a draft typology catalog without ephemeral
// worktree ids (majordomo-typology-<rand>) and with sorted sliceBindings.
func draftCatalogIdentitySHA(draftYAML string) string {
	raw := strings.TrimSpace(draftYAML)
	if raw == "" {
		return cache.ContentSHA("")
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return cache.ContentSHA(raw)
	}
	normalizeEphemeralCatalogDoc(doc)
	out, err := yaml.Marshal(doc)
	if err != nil {
		return cache.ContentSHA(raw)
	}
	return cache.ContentSHA(string(out))
}

// architectureIdentitySHA hashes an architecture brief without generated_at stamps.
func architectureIdentitySHA(architectureMD string) string {
	return cache.ContentSHA(stripGeneratedAtLines(architectureMD))
}

func stripGeneratedAtLines(raw string) string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "generated_at:") {
			continue
		}
		// Ledger/claim provenance labels flip between aligned/raw across reseeds.
		if strings.HasPrefix(trim, "source: slice_objective_rlm") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			out = append(out, indent+"source: slice_objective_rlm")
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func normalizeEphemeralCatalogDoc(doc map[string]interface{}) {
	if id, ok := doc["id"].(string); ok {
		doc["id"] = normalizeDraftCatalogID(id)
	}
	delete(doc, "generated_at")
	if bindings, ok := doc["sliceBindings"].([]interface{}); ok {
		sort.SliceStable(bindings, func(i, j int) bool {
			return bindingSortKey(bindings[i]) < bindingSortKey(bindings[j])
		})
		doc["sliceBindings"] = bindings
	}
}

func normalizeDraftCatalogID(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "majordomo-typology-") {
		return "majordomo-typology"
	}
	return id
}

func bindingSortKey(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Sprint(v)
	}
	return strings.Join([]string{
		fmt.Sprint(m["from"]),
		fmt.Sprint(m["to"]),
		fmt.Sprint(m["kind"]),
	}, "\x00")
}

// clusterVerdictsIdentitySHA hashes accept/overlay/reject decisions only.
// Duration, tokens, trace_dir, generated_at, and free-form reason/evidence are omitted.
func clusterVerdictsIdentitySHA(verdictsYAML string) string {
	raw := strings.TrimSpace(verdictsYAML)
	if raw == "" {
		return cache.ContentSHA("")
	}
	var doc clusterMergeVerdictsDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return cache.ContentSHA(raw)
	}
	merges := append([]clusterMergeVerdict(nil), doc.Merges...)
	sort.Slice(merges, func(i, j int) bool {
		return packageSetKey(merges[i].Packages) < packageSetKey(merges[j].Packages)
	})
	parts := make([]string, 0, len(merges)*3)
	for _, m := range merges {
		parts = append(parts,
			strings.TrimSpace(m.ID),
			packageSetKey(m.Packages),
			strings.TrimSpace(m.Verdict),
		)
	}
	return cache.HashDigestParts(parts...)
}

func parseRolesYAML(rolesYAML string) (packageRolesDoc, error) {
	raw := strings.TrimSpace(rolesYAML)
	if raw == "" {
		return packageRolesDoc{}, fmt.Errorf("empty roles yaml")
	}
	var doc packageRolesDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return packageRolesDoc{}, err
	}
	return doc, nil
}

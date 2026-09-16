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

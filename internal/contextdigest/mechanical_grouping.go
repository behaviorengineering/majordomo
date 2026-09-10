package contextdigest

import "github.com/behaviorengineering/typology/roles"

// mechanicalPreCluster formats Typology's deterministic grouping seed for the
// cluster LLM. Grouping math lives in Typology; Majordomo only renders it.
func mechanicalPreCluster(doc packageRolesDoc) string {
	topo := roles.Topology{
		Packages: make([]roles.Node, 0, len(doc.Packages)),
		Edges:    make([]roles.Edge, 0, len(doc.Edges)),
	}
	for _, n := range doc.Packages {
		topo.Packages = append(topo.Packages, roles.Node{
			Path:           n.Path,
			Role:           n.Role,
			Confidence:     n.Confidence,
			Evidence:       n.Evidence,
			InspectedStage: n.InspectedStage,
			CandidateRole:  n.CandidateRole,
		})
	}
	for _, e := range doc.Edges {
		topo.Edges = append(topo.Edges, roles.Edge{
			From: e.From,
			To:   e.To,
			Kind: e.Kind,
		})
	}
	return roles.FormatGroupingMarkdown(roles.BuildGrouping(topo))
}

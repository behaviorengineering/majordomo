package contextdigest

import (
	"fmt"

	"github.com/behaviorengineering/typology/roles"
	"gopkg.in/yaml.v3"
)

const mechanicalGroupingRel = "mechanical_grouping.yaml"

// mechanicalPreClusterYAML returns Typology's deterministic grouping seed as YAML.
// Grouping math lives in Typology; Majordomo only marshals it for evidence + LLM inputs.
func mechanicalPreClusterYAML(doc packageRolesDoc) (string, error) {
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
	raw, err := yaml.Marshal(roles.BuildGrouping(topo))
	if err != nil {
		return "", fmt.Errorf("mechanical_grouping encode: %w", err)
	}
	return string(raw), nil
}

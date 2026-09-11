package contextdigest

import (
	"fmt"
	"path/filepath"
	"strings"

	typologypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/typology"
)

// loadPackageRolesFromEvidenceDir reads package_roles.yaml from the evidence dir.
// Fail-closed when the file is missing or empty of packages.
func loadPackageRolesFromEvidenceDir(evidenceDir string) (packageRolesDoc, error) {
	path := filepath.Join(evidenceDir, packageRolesRel)
	doc, err := loadPackageRoles(path)
	if err != nil {
		return packageRolesDoc{}, fmt.Errorf("%s required for claim entailment: %w", packageRolesRel, err)
	}
	if len(doc.Packages) == 0 {
		return packageRolesDoc{}, fmt.Errorf("%s has no packages", packageRolesRel)
	}
	return doc, nil
}

// rejectUnentailedClaims returns issue strings for claims not justified by is, edges, or evidence flags.
// must_not intersection is checked separately; this is the positive entailment gate.
func rejectUnentailedClaims(
	sliceID string,
	claims []string,
	ownedPaths []string,
	constraints packageCapabilityConstraintsDoc,
	roles packageRolesDoc,
) []string {
	byPath := constraintsByPath(constraints)
	rolesByPath := roleByPath(roles)
	isUnion := sliceIsUnion(ownedPaths, byPath)
	fillsDTOFrom := fillsDTOFromSet(roles.Edges)

	var issues []string
	for _, raw := range claims {
		claim := strings.TrimSpace(raw)
		if claim == "" {
			continue
		}
		if claimEntailed(claim, ownedPaths, isUnion, fillsDTOFrom, byPath, rolesByPath) {
			continue
		}
		issues = append(issues, fmt.Sprintf(
			"%s: slice %q claim %q is not entailed by package is/edges/evidence",
			typologypack.CriterionIDRoleGrounding, strings.TrimSpace(sliceID), claim,
		))
	}
	return issues
}

func claimEntailed(
	claim string,
	ownedPaths []string,
	isUnion map[string]struct{},
	fillsDTOFrom map[string]struct{},
	byPath map[string]packageCapabilityConstraint,
	rolesByPath map[string]packageRoleNode,
) bool {
	if _, ok := isUnion[claim]; ok {
		return true
	}
	switch claim {
	case capFillDTO:
		for _, p := range ownedPaths {
			if _, ok := fillsDTOFrom[normalizeRolePath(p)]; ok {
				return true
			}
		}
		return false
	case capServeHTTP, capWireHandlers:
		return ownedEvidenceHasAny(ownedPaths, rolesByPath,
			"delivery:http", "delivery:grpc") ||
			ownedEvidenceHasPrefix(ownedPaths, rolesByPath, "imports_net_http") ||
			ownedEvidenceHasPrefix(ownedPaths, rolesByPath, "imports_grpc")
	case capRunCLI:
		return ownedEvidenceHasAny(ownedPaths, rolesByPath, "has_main")
	case capExecProcess:
		return ownedEvidenceHasAny(ownedPaths, rolesByPath, "imports_os_exec")
	case capObservability:
		return ownedEvidenceHasAny(ownedPaths, rolesByPath, "imports_otel", "imports_prometheus")
	case capConfig:
		// Soft path: config role already puts config in is. Extra evidence-only path
		// allows config-shaped packages without otel/prom contradiction flags.
		for _, p := range ownedPaths {
			n, ok := rolesByPath[normalizeRolePath(p)]
			if !ok {
				continue
			}
			if strings.TrimSpace(n.Role) == roleConfig {
				return true
			}
			if evidenceHasAny(n.Evidence, "imports_otel", "imports_prometheus") {
				continue
			}
			if evidenceHasAny(n.Evidence, "config_keys", "env_config") {
				return true
			}
		}
		return false
	case capOwnDomainRules:
		for _, p := range ownedPaths {
			n, ok := rolesByPath[normalizeRolePath(p)]
			if !ok {
				n = packageRoleNode{Path: p, Role: roleUnknown}
			}
			role := strings.TrimSpace(n.Role)
			if role != "" && role != roleUnknown {
				continue
			}
			if isDTOShapedEvidence(n.Evidence) {
				continue
			}
			if hasDomainWorkEvidence(n.Evidence) {
				return true
			}
		}
		return false
	case capSynchronizeState, capMergeAdapters:
		return false
	case capOrchestrate:
		for _, p := range ownedPaths {
			c, ok := byPath[normalizeRolePath(p)]
			role := ""
			if ok {
				role = strings.TrimSpace(c.Role)
			}
			if role == "" {
				if n, rok := rolesByPath[normalizeRolePath(p)]; rok {
					role = strings.TrimSpace(n.Role)
				}
			}
			if role == roleEntrypoint {
				return true
			}
		}
		return false
	case capAggregateViews, capAdaptExternal, capDataShape:
		return false
	default:
		return false
	}
}

func sliceIsUnion(paths []string, byPath map[string]packageCapabilityConstraint) map[string]struct{} {
	out := make(map[string]struct{})
	for _, p := range paths {
		c, ok := byPath[normalizeRolePath(p)]
		if !ok {
			continue
		}
		for _, code := range c.Is {
			code = strings.TrimSpace(code)
			if code != "" {
				out[code] = struct{}{}
			}
		}
	}
	return out
}

func fillsDTOFromSet(edges []packageRoleEdge) map[string]struct{} {
	out := make(map[string]struct{})
	for _, e := range edges {
		if strings.ToLower(strings.TrimSpace(e.Kind)) != edgeFillsDTO {
			continue
		}
		from := normalizeRolePath(e.From)
		if from != "" {
			out[from] = struct{}{}
		}
	}
	return out
}

func ownedEvidenceHasAny(paths []string, rolesByPath map[string]packageRoleNode, flags ...string) bool {
	for _, p := range paths {
		n, ok := rolesByPath[normalizeRolePath(p)]
		if !ok {
			continue
		}
		if evidenceHasAny(n.Evidence, flags...) {
			return true
		}
	}
	return false
}

func ownedEvidenceHasPrefix(paths []string, rolesByPath map[string]packageRoleNode, prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return false
	}
	for _, p := range paths {
		n, ok := rolesByPath[normalizeRolePath(p)]
		if !ok {
			continue
		}
		for _, e := range n.Evidence {
			if strings.HasPrefix(strings.TrimSpace(e), prefix) {
				return true
			}
		}
	}
	return false
}

func evidenceHasAny(evidence []string, flags ...string) bool {
	want := make(map[string]struct{}, len(flags))
	for _, f := range flags {
		f = strings.TrimSpace(f)
		if f != "" {
			want[f] = struct{}{}
		}
	}
	for _, e := range evidence {
		if _, ok := want[strings.TrimSpace(e)]; ok {
			return true
		}
	}
	return false
}

func isDTOShapedEvidence(evidence []string) bool {
	return evidenceHasAny(evidence, "json_tags") && !hasDomainWorkEvidence(evidence)
}

func hasDomainWorkEvidence(evidence []string) bool {
	for _, raw := range evidence {
		e := strings.TrimSpace(raw)
		switch e {
		case "has_main", "go_embed", "embeds_static",
			"imports_os_exec", "imports_otel", "imports_prometheus",
			"exported_funcs", "exported_methods":
			return true
		}
		if strings.HasPrefix(e, "delivery:") || strings.HasPrefix(e, "imports_") {
			return true
		}
	}
	return false
}

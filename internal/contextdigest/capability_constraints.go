package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Portable capability codes for is / must_not priors. No product nouns.
const (
	capDataShape        = "data_shape"
	capSynchronizeState = "synchronize_state"
	capMergeAdapters    = "merge_adapters"
	capServeHTTP        = "serve_http"
	capWireHandlers     = "wire_handlers"
	capOrchestrate      = "orchestrate"
	capOwnDomainRules   = "own_domain_rules"
	capFillDTO          = "fill_dto"
	capRunCLI           = "run_cli"
	capAggregateViews   = "aggregate_views"
	capExecProcess      = "exec_process"
	capObservability    = "observability"
	capAdaptExternal    = "adapt_external"
	capConfig           = "config"

	edgeFillsDTO     = "fills_dto"
	edgeUsesRunner   = "uses_runner"
	edgeServesServer = "serves_server"

	packageCapabilityConstraintsRel = "package_capability_constraints.yaml"
	sliceObjectiveClaimsRel         = "slice_objective_claims.yaml"
	packageRolesRel                 = "package_roles.yaml"
	refinedSnapshotRel              = "refined_snapshot.yaml"

	capabilityConstraintsSection = "## Capability constraints (is / is-not)"
)

// packageCapabilityConstraintsDoc is durable is/is-not evidence for later stages.
type packageCapabilityConstraintsDoc struct {
	Packages []packageCapabilityConstraint `yaml:"packages"`
}

type packageCapabilityConstraint struct {
	Path     string   `yaml:"path"`
	Is       []string `yaml:"is,omitempty"`
	MustNot  []string `yaml:"must_not,omitempty"`
	FilledBy []string `yaml:"filled_by,omitempty"`
	Source   string   `yaml:"source,omitempty"`
	Role     string   `yaml:"role,omitempty"`
}

// sliceObjectiveClaimsDoc is the Majordomo sidecar of structured claims per slice.
type sliceObjectiveClaimsDoc struct {
	Slices []sliceObjectiveClaim `yaml:"slices"`
}

type sliceObjectiveClaim struct {
	ID     string   `yaml:"id"`
	Claims []string `yaml:"claims"`
}

type roleCapabilityDefaults struct {
	Is      []string
	MustNot []string
}

func roleCapabilityTable() map[string]roleCapabilityDefaults {
	return map[string]roleCapabilityDefaults{
		roleDTO: {
			Is: []string{capDataShape},
			MustNot: []string{
				capSynchronizeState, capMergeAdapters, capServeHTTP,
				capOrchestrate, capOwnDomainRules, capRunCLI, capWireHandlers,
			},
		},
		roleHTTPSurface: {
			Is:      []string{capServeHTTP, capWireHandlers},
			MustNot: []string{capOwnDomainRules, capOrchestrate},
		},
		roleEntrypoint: {
			// orchestrate is entrypoint-only; kept in is so constraint rows show the prior.
			Is:      []string{capRunCLI, capOrchestrate},
			MustNot: []string{capOwnDomainRules, capServeHTTP},
		},
		roleAggregator: {
			Is:      []string{capAggregateViews},
			MustNot: []string{capServeHTTP, capRunCLI, capOrchestrate},
		},
		roleExecRunner: {
			Is:      []string{capExecProcess},
			MustNot: []string{capRunCLI, capOwnDomainRules, capOrchestrate},
		},
		roleAdapter: {
			Is:      []string{capAdaptExternal, capFillDTO},
			MustNot: []string{capOrchestrate},
		},
		roleConfig: {
			Is:      []string{capConfig},
			MustNot: []string{capOwnDomainRules, capOrchestrate},
		},
		roleObservability: {
			Is:      []string{capObservability},
			MustNot: []string{capConfig, capOwnDomainRules, capOrchestrate},
		},
		roleUnknown: {
			Is:      []string{},
			MustNot: []string{capOrchestrate},
		},
	}
}

func knownCapabilityCodes() map[string]struct{} {
	codes := []string{
		capDataShape, capSynchronizeState, capMergeAdapters, capServeHTTP,
		capWireHandlers, capOrchestrate, capOwnDomainRules, capFillDTO,
		capRunCLI, capAggregateViews, capExecProcess, capObservability,
		capAdaptExternal, capConfig,
	}
	out := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		out[c] = struct{}{}
	}
	return out
}

// buildCapabilityConstraints maps roles + edges into portable is/must_not priors.
func buildCapabilityConstraints(doc packageRolesDoc) packageCapabilityConstraintsDoc {
	table := roleCapabilityTable()
	filledBy := map[string][]string{}
	for _, e := range doc.Edges {
		kind := strings.ToLower(strings.TrimSpace(e.Kind))
		from := normalizeRolePath(e.From)
		to := normalizeRolePath(e.To)
		if from == "" || to == "" {
			continue
		}
		switch kind {
		case edgeFillsDTO:
			filledBy[to] = append(filledBy[to], from)
		}
	}

	out := packageCapabilityConstraintsDoc{}
	for _, n := range doc.Packages {
		path := normalizeRolePath(n.Path)
		if path == "" {
			continue
		}
		role := strings.TrimSpace(n.Role)
		if role == "" {
			role = roleUnknown
		}
		defs, ok := table[role]
		if !ok {
			defs = table[roleUnknown]
		}
		c := packageCapabilityConstraint{
			Path:    path,
			Is:      append([]string(nil), defs.Is...),
			MustNot: append([]string(nil), defs.MustNot...),
			Source:  "role_table",
			Role:    role,
		}
		// Fail-closed: orchestrate is entrypoint-only even for unknown/custom roles.
		if role != roleEntrypoint {
			c.MustNot = uniqueStrings(append(c.MustNot, capOrchestrate))
		}
		if fillers := uniqueStrings(filledBy[path]); len(fillers) > 0 {
			c.FilledBy = fillers
			// Inbound fills_dto: the DTO package must not claim to merge adapters.
			c.MustNot = uniqueStrings(append(c.MustNot, capMergeAdapters, capSynchronizeState))
		}
		if n.Agreement == agreementMatch || n.Agreement == agreementDisagree {
			c.Source = "role_rlm"
		}
		out.Packages = append(out.Packages, c)
	}
	sort.Slice(out.Packages, func(i, j int) bool {
		return out.Packages[i].Path < out.Packages[j].Path
	})
	return out
}

func writeCapabilityConstraints(path string, doc packageCapabilityConstraintsDoc) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("capability constraints mkdir: %w", err)
	}
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("capability constraints encode: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("capability constraints write: %w", err)
	}
	return nil
}

func loadCapabilityConstraints(path string) (packageCapabilityConstraintsDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return packageCapabilityConstraintsDoc{}, err
	}
	var doc packageCapabilityConstraintsDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return packageCapabilityConstraintsDoc{}, fmt.Errorf("capability constraints decode: %w", err)
	}
	return doc, nil
}

func parseCapabilityConstraintsYAML(raw string) (packageCapabilityConstraintsDoc, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return packageCapabilityConstraintsDoc{}, fmt.Errorf("capability constraints YAML is empty")
	}
	var doc packageCapabilityConstraintsDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return packageCapabilityConstraintsDoc{}, fmt.Errorf("capability constraints decode: %w", err)
	}
	return doc, nil
}

func constraintsByPath(doc packageCapabilityConstraintsDoc) map[string]packageCapabilityConstraint {
	out := make(map[string]packageCapabilityConstraint, len(doc.Packages))
	for _, p := range doc.Packages {
		out[normalizeRolePath(p.Path)] = p
	}
	return out
}

func formatCapabilityConstraintsMarkdown(doc packageCapabilityConstraintsDoc) string {
	var b strings.Builder
	b.WriteString(capabilityConstraintsSection)
	b.WriteString("\n\n")
	b.WriteString("Factual priors for refine. MUST NOT contradict.\n\n")
	for _, p := range doc.Packages {
		fmt.Fprintf(&b, "- `%s` role=%s is=[%s] must_not=[%s]",
			p.Path, p.Role, strings.Join(p.Is, ", "), strings.Join(p.MustNot, ", "))
		if len(p.FilledBy) > 0 {
			fmt.Fprintf(&b, " filled_by=[%s]", strings.Join(p.FilledBy, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func clusterProposalHasCapabilityConstraints(proposalMD string, doc packageCapabilityConstraintsDoc) (bool, string) {
	if !strings.Contains(proposalMD, capabilityConstraintsSection) {
		return false, fmt.Sprintf("cluster proposal missing %q section", capabilityConstraintsSection)
	}
	if len(doc.Packages) == 0 {
		return true, ""
	}
	lower := proposalMD
	quoted := 0
	for _, p := range doc.Packages {
		if strings.Contains(lower, p.Path) || strings.Contains(lower, "`"+p.Path+"`") {
			quoted++
		}
	}
	if quoted == 0 {
		return false, "cluster capability constraints section quotes no package paths from package_capability_constraints.yaml"
	}
	return true, ""
}

func ensureClusterCapabilityConstraintsSection(proposalMD string, doc packageCapabilityConstraintsDoc) string {
	if strings.Contains(proposalMD, capabilityConstraintsSection) {
		return proposalMD
	}
	block := formatCapabilityConstraintsMarkdown(doc)
	return strings.TrimSpace(proposalMD) + "\n\n" + block + "\n"
}

func parseObjectiveClaimsYAML(raw string) (sliceObjectiveClaimsDoc, error) {
	raw = strings.TrimSpace(stripCodeFence(raw))
	if raw == "" {
		return sliceObjectiveClaimsDoc{}, fmt.Errorf("objective_claims_yaml is required")
	}
	var doc sliceObjectiveClaimsDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return sliceObjectiveClaimsDoc{}, fmt.Errorf("objective_claims_yaml decode: %w", err)
	}
	if len(doc.Slices) == 0 {
		return sliceObjectiveClaimsDoc{}, fmt.Errorf("objective_claims_yaml has no slices")
	}
	known := knownCapabilityCodes()
	for i, s := range doc.Slices {
		if strings.TrimSpace(s.ID) == "" {
			return sliceObjectiveClaimsDoc{}, fmt.Errorf("objective_claims_yaml slice[%d] missing id", i)
		}
		if len(s.Claims) == 0 {
			return sliceObjectiveClaimsDoc{}, fmt.Errorf("objective_claims_yaml slice %q has empty claims", s.ID)
		}
		for _, c := range s.Claims {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if _, ok := known[c]; !ok {
				return sliceObjectiveClaimsDoc{}, fmt.Errorf("objective_claims_yaml slice %q unknown claim code %q", s.ID, c)
			}
		}
	}
	return doc, nil
}

func writeObjectiveClaims(path string, doc sliceObjectiveClaimsDoc) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("objective claims mkdir: %w", err)
	}
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("objective claims encode: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("objective claims write: %w", err)
	}
	return nil
}

func claimsBySliceID(doc sliceObjectiveClaimsDoc) map[string][]string {
	out := make(map[string][]string, len(doc.Slices))
	for _, s := range doc.Slices {
		out[strings.TrimSpace(s.ID)] = append([]string(nil), s.Claims...)
	}
	return out
}

// sliceMustNotUnion returns must_not codes for packages owned by the slice.
func sliceMustNotUnion(paths []string, byPath map[string]packageCapabilityConstraint) []string {
	var out []string
	for _, p := range paths {
		c, ok := byPath[normalizeRolePath(p)]
		if !ok {
			continue
		}
		out = append(out, c.MustNot...)
		if len(c.FilledBy) > 0 {
			out = append(out, capMergeAdapters, capSynchronizeState)
		}
	}
	return uniqueStrings(out)
}

func packageUnconstrained(c packageCapabilityConstraint) bool {
	if strings.TrimSpace(c.Role) == "" || c.Role == roleUnknown {
		return true
	}
	return len(c.Is) == 0 && len(c.MustNot) == 0
}

func intersectStrings(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	set := map[string]struct{}{}
	for _, s := range a {
		set[strings.TrimSpace(s)] = struct{}{}
	}
	var out []string
	for _, s := range b {
		s = strings.TrimSpace(s)
		if _, ok := set[s]; ok {
			out = append(out, s)
		}
	}
	return uniqueStrings(out)
}

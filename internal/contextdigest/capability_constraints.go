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
				capExecProcess, capFillDTO, capAdaptExternal, capAggregateViews,
				capObservability, capConfig,
			},
		},
		roleHTTPSurface: {
			Is: []string{capServeHTTP, capWireHandlers},
			MustNot: []string{
				capOwnDomainRules, capOrchestrate, capExecProcess, capFillDTO,
				capAdaptExternal, capAggregateViews, capDataShape, capObservability,
				capConfig, capRunCLI, capSynchronizeState, capMergeAdapters,
			},
		},
		roleEntrypoint: {
			// orchestrate is entrypoint-only; kept in is so constraint rows show the prior.
			Is: []string{capRunCLI, capOrchestrate},
			MustNot: []string{
				capOwnDomainRules, capServeHTTP, capExecProcess, capFillDTO,
				capAdaptExternal, capAggregateViews, capDataShape, capObservability,
				capConfig, capWireHandlers, capSynchronizeState, capMergeAdapters,
			},
		},
		roleAggregator: {
			Is: []string{capAggregateViews},
			MustNot: []string{
				capServeHTTP, capRunCLI, capOrchestrate, capExecProcess, capFillDTO,
				capAdaptExternal, capDataShape, capObservability, capConfig,
				capSynchronizeState, capMergeAdapters,
			},
		},
		roleExecRunner: {
			Is: []string{capExecProcess},
			MustNot: []string{
				capRunCLI, capOwnDomainRules, capOrchestrate, capFillDTO,
				capAdaptExternal, capAggregateViews, capDataShape, capObservability,
				capConfig, capServeHTTP, capWireHandlers, capSynchronizeState, capMergeAdapters,
			},
		},
		roleAdapter: {
			Is: []string{capAdaptExternal, capFillDTO},
			MustNot: []string{
				capOrchestrate, capExecProcess, capOwnDomainRules, capAggregateViews,
				capDataShape, capObservability, capConfig, capServeHTTP, capWireHandlers,
				capRunCLI, capSynchronizeState, capMergeAdapters,
			},
		},
		roleConfig: {
			Is: []string{capConfig},
			MustNot: []string{
				capOwnDomainRules, capOrchestrate, capExecProcess, capFillDTO,
				capAdaptExternal, capAggregateViews, capDataShape, capObservability,
				capServeHTTP, capWireHandlers, capRunCLI, capSynchronizeState, capMergeAdapters,
			},
		},
		roleObservability: {
			Is: []string{capObservability},
			MustNot: []string{
				capConfig, capOwnDomainRules, capOrchestrate, capExecProcess, capFillDTO,
				capAdaptExternal, capAggregateViews, capDataShape, capServeHTTP,
				capWireHandlers, capRunCLI, capSynchronizeState, capMergeAdapters,
			},
		},
		roleUnknown: {
			Is: []string{},
			MustNot: []string{
				capOrchestrate, capExecProcess, capFillDTO, capOwnDomainRules,
				capAdaptExternal, capAggregateViews, capDataShape, capObservability,
				capConfig, capServeHTTP, capWireHandlers, capRunCLI,
				capSynchronizeState, capMergeAdapters,
			},
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
	fillsDTOFrom := map[string]struct{}{}
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
			fillsDTOFrom[from] = struct{}{}
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
		// Fail-closed post-pass is the authority for unknown/custom roles.
		if role != roleEntrypoint {
			c.MustNot = uniqueStrings(append(c.MustNot, capOrchestrate))
		}
		if role != roleExecRunner && !evidenceHasAny(n.Evidence, "imports_os_exec") {
			c.MustNot = uniqueStrings(append(c.MustNot, capExecProcess))
		}
		if role != roleAdapter {
			if _, ok := fillsDTOFrom[path]; !ok {
				c.MustNot = uniqueStrings(append(c.MustNot, capFillDTO))
			}
		}
		// own_domain_rules: aggregator-only (entrypoint/http already forbid in table).
		if role != roleAggregator {
			c.MustNot = uniqueStrings(append(c.MustNot, capOwnDomainRules))
		}
		if role != roleAdapter {
			c.MustNot = uniqueStrings(append(c.MustNot, capAdaptExternal))
		}
		if role != roleAggregator {
			c.MustNot = uniqueStrings(append(c.MustNot, capAggregateViews))
		}
		if role != roleDTO {
			c.MustNot = uniqueStrings(append(c.MustNot, capDataShape))
		}
		if role != roleObservability &&
			!evidenceHasAny(n.Evidence, "imports_otel", "imports_prometheus") {
			c.MustNot = uniqueStrings(append(c.MustNot, capObservability))
		}
		allowHTTP := role == roleHTTPSurface ||
			evidenceHasAny(n.Evidence, "delivery:http", "delivery:grpc") ||
			evidenceHasAnyPrefix(n.Evidence, "imports_net_http", "imports_grpc")
		if !allowHTTP {
			c.MustNot = uniqueStrings(append(c.MustNot, capServeHTTP, capWireHandlers))
		}
		if role != roleEntrypoint && !evidenceHasAny(n.Evidence, "has_main") {
			c.MustNot = uniqueStrings(append(c.MustNot, capRunCLI))
		}
		allowConfig := role == roleConfig
		if !allowConfig &&
			!evidenceHasAny(n.Evidence, "imports_otel", "imports_prometheus") &&
			evidenceHasAny(n.Evidence, "config_keys", "env_config") {
			allowConfig = true
		}
		if !allowConfig {
			c.MustNot = uniqueStrings(append(c.MustNot, capConfig))
		}
		// No role puts these in is; entailment always rejects.
		c.MustNot = uniqueStrings(append(c.MustNot, capSynchronizeState, capMergeAdapters))
		if fillers := uniqueStrings(filledBy[path]); len(fillers) > 0 {
			c.FilledBy = fillers
		}
		// Evidence/role exceptions must clear table defaults, not only skip appends.
		c.MustNot = dropAllowedCapabilityMustNot(c.MustNot, role, n.Evidence, path, fillsDTOFrom)
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

// dropAllowedCapabilityMustNot removes codes that role or evidence explicitly allows,
// so table defaults cannot override fail-closed exceptions.
func dropAllowedCapabilityMustNot(
	mustNot []string,
	role string,
	evidence []string,
	path string,
	fillsDTOFrom map[string]struct{},
) []string {
	drop := map[string]struct{}{}
	if role == roleEntrypoint {
		drop[capOrchestrate] = struct{}{}
		drop[capRunCLI] = struct{}{}
	}
	if role == roleExecRunner || evidenceHasAny(evidence, "imports_os_exec") {
		drop[capExecProcess] = struct{}{}
	}
	if role == roleAdapter {
		drop[capFillDTO] = struct{}{}
		drop[capAdaptExternal] = struct{}{}
	}
	if _, ok := fillsDTOFrom[path]; ok {
		drop[capFillDTO] = struct{}{}
	}
	if role == roleAggregator {
		drop[capOwnDomainRules] = struct{}{}
		drop[capAggregateViews] = struct{}{}
	}
	if role == roleDTO {
		drop[capDataShape] = struct{}{}
	}
	if role == roleObservability || evidenceHasAny(evidence, "imports_otel", "imports_prometheus") {
		drop[capObservability] = struct{}{}
	}
	if role == roleHTTPSurface ||
		evidenceHasAny(evidence, "delivery:http", "delivery:grpc") ||
		evidenceHasAnyPrefix(evidence, "imports_net_http", "imports_grpc") {
		drop[capServeHTTP] = struct{}{}
		drop[capWireHandlers] = struct{}{}
	}
	if evidenceHasAny(evidence, "has_main") {
		drop[capRunCLI] = struct{}{}
	}
	if role == roleConfig ||
		(!evidenceHasAny(evidence, "imports_otel", "imports_prometheus") &&
			evidenceHasAny(evidence, "config_keys", "env_config")) {
		drop[capConfig] = struct{}{}
	}
	if len(drop) == 0 {
		return mustNot
	}
	out := make([]string, 0, len(mustNot))
	for _, code := range mustNot {
		if _, ok := drop[code]; ok {
			continue
		}
		out = append(out, code)
	}
	return out
}

// claimPolicyPromptRules is the ledger RLM instruction block that mirrors Go gates.
// Keep aligned with buildCapabilityConstraints fail-closed passes and claimEntailed.
func claimPolicyPromptRules() string {
	return `Claim policy (deterministic; MUST follow):
- Prefer claim codes that already appear in owned package is=[] rows.
- NEVER emit a code listed in owned must_not=[].
- Codes outside is=[] are allowed ONLY with matching entailment evidence:
  - orchestrate: owned package role is entrypoint
  - exec_process: role is exec_runner OR evidence includes imports_os_exec
  - fill_dto: role is adapter OR an outbound fills_dto edge from an owned package
  - serve_http / wire_handlers: role is http_surface OR delivery:http|grpc OR imports_net_http / imports_grpc
  - run_cli: role is entrypoint OR evidence has_main
  - observability: role is observability OR imports_otel / imports_prometheus
  - config: role is config OR config_keys / env_config (without otel/prom-only packages)
  - own_domain_rules: role is aggregator
  - adapt_external / aggregate_views / data_shape: only when already in owned is=[]
  - synchronize_state / merge_adapters: never (always must_not)
- If owned is=[] and every code is in must_not, emit verdict: grounded with empty claims (or claims: none), quote package symbols as evidence, and write a cautious objective. Do not invent a claim code.
- Prestige English ("orchestrates", "runs git via a helper", "builds a card DTO") is NOT a claim code.
- When unsure, drop the claim or set verdict: overclaim.`
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
			// Empty claims are valid for unclassified packages (fail-closed must_not covers all codes).
			continue
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

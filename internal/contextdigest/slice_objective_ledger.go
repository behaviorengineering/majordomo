package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	typologypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/typology"
	"github.com/behaviorengineering/typology/catalog"
	"gopkg.in/yaml.v3"
)

const (
	sliceObjectiveLedgerRel   = "slice_objective_ledger.yaml"
	ledgerVerdictGrounded     = "grounded"
	ledgerVerdictOverclaim    = "overclaim"
	ledgerSourceRLM           = "slice_objective_rlm"
	ledgerSourceUnclaimed     = "slice_objective_rlm_unclaimed"
	ledgerUnclaimedObjective  = "Package role is unclassified; no portable capability claim is justified yet."
)

var (
	ledgerEvidenceRE  = regexp.MustCompile(`(?i)^evidence\s*[:=]\s*(.+)$`)
	ledgerClaimsRE    = regexp.MustCompile(`(?i)^claims\s*[:=]\s*(.+)$`)
	ledgerObjectiveRE = regexp.MustCompile(`(?i)^objective\s*[:=]\s*(.+)$`)
)

// sliceObjectiveLedgerDoc is the evidence-first meaning ledger per slice.
type sliceObjectiveLedgerDoc struct {
	Slices []sliceObjectiveLedgerEntry `yaml:"slices"`
}

type sliceObjectiveLedgerEntry struct {
	ID         string   `yaml:"id"`
	OwnedPaths []string `yaml:"owned_paths,omitempty"`
	Evidence   []string `yaml:"evidence"`
	Claims     []string `yaml:"claims"`
	Objective  string   `yaml:"objective"`
	Verdict    string   `yaml:"verdict"`
	Source     string   `yaml:"source,omitempty"`
}

func writeObjectiveLedger(path string, doc sliceObjectiveLedgerDoc) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("objective ledger mkdir: %w", err)
	}
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("objective ledger encode: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("objective ledger write: %w", err)
	}
	return nil
}

func marshalLedger(doc sliceObjectiveLedgerDoc) (string, error) {
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return "", fmt.Errorf("objective ledger encode: %w", err)
	}
	return string(data), nil
}

func parseObjectiveLedgerYAML(raw string) (sliceObjectiveLedgerDoc, error) {
	raw = strings.TrimSpace(stripCodeFence(raw))
	if raw == "" {
		return sliceObjectiveLedgerDoc{}, fmt.Errorf("slice_objective_ledger_yaml is required")
	}
	var doc sliceObjectiveLedgerDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return sliceObjectiveLedgerDoc{}, fmt.Errorf("slice_objective_ledger_yaml decode: %w", err)
	}
	if err := validateObjectiveLedgerDoc(doc); err != nil {
		return sliceObjectiveLedgerDoc{}, err
	}
	return doc, nil
}

func validateObjectiveLedgerDoc(doc sliceObjectiveLedgerDoc) error {
	if len(doc.Slices) == 0 {
		return fmt.Errorf("slice_objective_ledger_yaml has no slices")
	}
	known := knownCapabilityCodes()
	for i, s := range doc.Slices {
		id := strings.TrimSpace(s.ID)
		if id == "" {
			return fmt.Errorf("slice_objective_ledger_yaml slice[%d] missing id", i)
		}
		if strings.TrimSpace(s.Objective) == "" {
			return fmt.Errorf("slice_objective_ledger_yaml slice %q missing objective", id)
		}
		verdict := strings.ToLower(strings.TrimSpace(s.Verdict))
		if verdict != ledgerVerdictGrounded {
			return fmt.Errorf("slice_objective_ledger_yaml slice %q verdict %q is not grounded", id, s.Verdict)
		}
		if len(normalizeEvidenceList(s.Evidence)) == 0 {
			return fmt.Errorf("slice_objective_ledger_yaml slice %q has empty evidence", id)
		}
		if len(s.Claims) == 0 {
			if strings.TrimSpace(s.Source) != ledgerSourceUnclaimed {
				return fmt.Errorf("slice_objective_ledger_yaml slice %q has empty claims", id)
			}
		} else {
			for _, c := range s.Claims {
				c = strings.TrimSpace(c)
				if c == "" {
					continue
				}
				if _, ok := known[c]; !ok {
					return fmt.Errorf("slice_objective_ledger_yaml slice %q unknown claim code %q", id, c)
				}
			}
		}
	}
	return nil
}

func claimsDocFromLedger(ledger sliceObjectiveLedgerDoc) sliceObjectiveClaimsDoc {
	out := sliceObjectiveClaimsDoc{}
	for _, s := range ledger.Slices {
		out.Slices = append(out.Slices, sliceObjectiveClaim{
			ID:     strings.TrimSpace(s.ID),
			Claims: append([]string(nil), s.Claims...),
		})
	}
	return out
}

func normalizeObjectiveText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func normalizeEvidenceList(in []string) []string {
	var out []string
	for _, e := range in {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// alignLedgerToRefinedCatalog remaps draft-keyed ledger rows onto refined slices by
// owned package overlap. Refine may merge draft neighborhoods; meaning travels with packages.
func alignLedgerToRefinedCatalog(
	typo catalog.Typology,
	ledger sliceObjectiveLedgerDoc,
) (sliceObjectiveLedgerDoc, sliceObjectiveClaimsDoc, []string) {
	var issues []string
	byPath := map[string][]sliceObjectiveLedgerEntry{}
	for _, e := range ledger.Slices {
		for _, p := range e.OwnedPaths {
			np := normalizeRolePath(p)
			if np == "" {
				continue
			}
			byPath[np] = append(byPath[np], e)
		}
	}
	aligned := sliceObjectiveLedgerDoc{}
	claims := sliceObjectiveClaimsDoc{}
	for _, s := range typo.Slices {
		id := strings.TrimSpace(s.ID)
		paths := slicePackagePaths(s)
		if id == "" || len(paths) == 0 {
			continue
		}
		var contributors []sliceObjectiveLedgerEntry
		seen := map[string]struct{}{}
		for _, p := range paths {
			for _, e := range byPath[normalizeRolePath(p)] {
				eid := strings.TrimSpace(e.ID)
				if eid == "" {
					continue
				}
				if _, ok := seen[eid]; ok {
					continue
				}
				seen[eid] = struct{}{}
				contributors = append(contributors, e)
			}
		}
		if len(contributors) == 0 {
			issues = append(issues, fmt.Sprintf(
				"%s: slice %q owns packages but no slice_objective_ledger entry covers those paths; rebuild meaning from evidence before teaching",
				typologypack.CriterionIDRoleGrounding, id,
			))
			continue
		}
		obj := normalizeObjectiveText(s.Objective)
		allowed := map[string]string{}
		var evidence []string
		var claimCodes []string
		for _, c := range contributors {
			allowed[normalizeObjectiveText(c.Objective)] = c.Objective
			evidence = append(evidence, c.Evidence...)
			claimCodes = append(claimCodes, c.Claims...)
		}
		if _, ok := allowed[obj]; !ok {
			var opts []string
			for _, v := range allowed {
				opts = append(opts, v)
			}
			sort.Strings(opts)
			issues = append(issues, fmt.Sprintf(
				"%s: slice %q catalog objective %q does not match any contributing ledger objective %v; copy one contributing ledger objective verbatim",
				typologypack.CriterionIDRoleGrounding, id, s.Objective, opts,
			))
		}
		entry := sliceObjectiveLedgerEntry{
			ID:         id,
			OwnedPaths: append([]string(nil), paths...),
			Evidence:   uniqueStrings(normalizeEvidenceList(evidence)),
			Claims:     uniqueStrings(claimCodes),
			Objective:  s.Objective,
			Verdict:    ledgerVerdictGrounded,
			Source:     "slice_objective_rlm_aligned",
		}
		if entry.Objective == "" {
			entry.Objective = contributors[0].Objective
		}
		aligned.Slices = append(aligned.Slices, entry)
		claims.Slices = append(claims.Slices, sliceObjectiveClaim{
			ID:     id,
			Claims: append([]string(nil), entry.Claims...),
		})
	}
	sort.Slice(aligned.Slices, func(i, j int) bool { return aligned.Slices[i].ID < aligned.Slices[j].ID })
	sort.Slice(claims.Slices, func(i, j int) bool { return claims.Slices[i].ID < claims.Slices[j].ID })
	return aligned, claims, issues
}

// validateLedgerAgainstConstraints rejects claim∩must_not, unentailed claims, and empty evidence for runtime claims.
func validateLedgerAgainstConstraints(ledger sliceObjectiveLedgerDoc, constraints packageCapabilityConstraintsDoc, roles packageRolesDoc) []string {
	byPath := constraintsByPath(constraints)
	var issues []string
	for _, s := range ledger.Slices {
		id := strings.TrimSpace(s.ID)
		mustNot := sliceMustNotUnion(s.OwnedPaths, byPath)
		if hit := intersectStrings(s.Claims, mustNot); len(hit) > 0 {
			issues = append(issues, fmt.Sprintf(
				"%s: ledger slice %q claims %v intersect must_not %v",
				typologypack.CriterionIDRoleGrounding, id, s.Claims, hit,
			))
		}
		issues = append(issues, rejectUnentailedClaims(id, s.Claims, s.OwnedPaths, constraints, roles)...)
		if claimsImplyRuntimeWork(s.Claims) && len(normalizeEvidenceList(s.Evidence)) == 0 {
			issues = append(issues, fmt.Sprintf(
				"%s: ledger slice %q claims runtime work without evidence quotes",
				typologypack.CriterionIDRoleGrounding, id,
			))
		}
	}
	return issues
}

func claimsImplyRuntimeWork(claims []string) bool {
	for _, c := range claims {
		switch strings.TrimSpace(c) {
		case "", capDataShape:
			continue
		default:
			return true
		}
	}
	return false
}

func formatConstraintRowsForPaths(paths []string, byPath map[string]packageCapabilityConstraint) string {
	norm := append([]string(nil), paths...)
	sort.Strings(norm)
	var b strings.Builder
	b.WriteString("Capability constraints for owned packages:\n")
	for _, p := range norm {
		c, ok := byPath[normalizeRolePath(p)]
		if !ok {
			fmt.Fprintf(&b, "- `%s` (no constraint row)\n", p)
			continue
		}
		fmt.Fprintf(&b, "- `%s` role=%s is=[%s] must_not=[%s]",
			c.Path, c.Role, strings.Join(c.Is, ", "), strings.Join(c.MustNot, ", "))
		if len(c.FilledBy) > 0 {
			fmt.Fprintf(&b, " filled_by=[%s]", strings.Join(c.FilledBy, ", "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func parseSliceObjectiveLedgerAnswer(text string) (evidence []string, claims []string, objective, verdict string, err error) {
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if m := objectiveVerdictRE.FindStringSubmatch(trim); len(m) == 2 {
			verdict = strings.ToLower(m[1])
			continue
		}
		if m := ledgerEvidenceRE.FindStringSubmatch(trim); len(m) == 2 {
			evidence = append(evidence, splitLedgerList(m[1])...)
			continue
		}
		if m := ledgerClaimsRE.FindStringSubmatch(trim); len(m) == 2 {
			claims = append(claims, splitLedgerList(m[1])...)
			continue
		}
		if m := ledgerObjectiveRE.FindStringSubmatch(trim); len(m) == 2 {
			objective = strings.TrimSpace(m[1])
		}
	}
	evidence = uniqueStrings(normalizeEvidenceList(evidence))
	claims = uniqueStrings(claims)
	objective = strings.TrimSpace(objective)
	verdict = strings.ToLower(strings.TrimSpace(verdict))
	if verdict != ledgerVerdictGrounded && verdict != ledgerVerdictOverclaim {
		return nil, nil, "", "", fmt.Errorf("slice objective ledger missing verdict")
	}
	if verdict == ledgerVerdictGrounded {
		if objective == "" {
			return nil, nil, "", "", fmt.Errorf("slice objective ledger grounded answer missing objective")
		}
		if len(evidence) == 0 {
			return nil, nil, "", "", fmt.Errorf("slice objective ledger grounded answer missing evidence")
		}
		known := knownCapabilityCodes()
		for _, c := range claims {
			if _, ok := known[c]; !ok {
				return nil, nil, "", "", fmt.Errorf("slice objective ledger unknown claim code %q", c)
			}
		}
		// Empty claims are allowed only when the caller accepts an unclaimed slice
		// (every portable code is in must_not). That check lives in buildSliceObjectiveLedger.
	}
	return evidence, claims, objective, verdict, nil
}

func splitLedgerList(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "[]")
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '|'
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "`\"'")
		if p == "" || strings.EqualFold(p, "none") || p == "-" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// sliceAllCapabilityCodesMustNot reports whether every portable claim code is forbidden
// for the owned packages (typical for role=unknown with empty is[]).
func sliceAllCapabilityCodesMustNot(paths []string, byPath map[string]packageCapabilityConstraint) bool {
	if len(paths) == 0 {
		return false
	}
	blocked := map[string]struct{}{}
	for _, code := range sliceMustNotUnion(paths, byPath) {
		blocked[code] = struct{}{}
	}
	for code := range knownCapabilityCodes() {
		if _, ok := blocked[code]; !ok {
			return false
		}
	}
	return true
}

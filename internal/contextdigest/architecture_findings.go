package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/behaviorengineering/typology/catalog"
)

const (
	humanInterventionRel = "evidence/typology/human_intervention.md"
	prPriorityRel        = "evidence/typology/pr_priority.md"
)

var missingSliceBindingRE = regexp.MustCompile(`(?i)SliceBinding\s+([A-Za-z0-9_./-]+)\s*->\s*([A-Za-z0-9_./-]+)\s+missing`)

// isArchitectureFindingsHeading reports headings that list open architecture issues.
func isArchitectureFindingsHeading(line string) bool {
	low := strings.ToLower(strings.TrimSpace(line))
	if !strings.HasPrefix(low, "#") {
		return false
	}
	return strings.Contains(low, "finding") ||
		strings.Contains(low, "drift") ||
		strings.Contains(low, "design question")
}

// extractArchitectureFindings returns bullet/content lines under a findings heading.
func extractArchitectureFindings(architectureMD string) []string {
	lines := strings.Split(architectureMD, "\n")
	inFindings := false
	var out []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		low := strings.ToLower(trim)
		if isArchitectureFindingsHeading(trim) {
			inFindings = true
			continue
		}
		if inFindings && strings.HasPrefix(trim, "#") {
			break
		}
		if !inFindings || trim == "" {
			continue
		}
		// Skip instructional prose that is not a finding bullet.
		if strings.HasPrefix(trim, "The following findings") {
			continue
		}
		if strings.HasPrefix(trim, "1.") || strings.HasPrefix(trim, "2.") || strings.HasPrefix(trim, "3.") {
			// Numbered remediation steps after the list, not findings.
			if strings.Contains(low, "read the relevant") || strings.Contains(low, "fix the code") || strings.Contains(low, "record a temporary") {
				break
			}
		}
		finding := ""
		switch {
		case strings.HasPrefix(trim, "- "), strings.HasPrefix(trim, "* "):
			finding = strings.TrimSpace(trim[2:])
		case strings.HasPrefix(trim, "|") && !strings.Contains(trim, "---"):
			finding = strings.TrimSpace(trim)
		default:
			if strings.Contains(trim, "`") && (strings.Contains(low, "slicebinding") || strings.Contains(low, "unmapped") || strings.Contains(low, "missing")) {
				finding = trim
			}
		}
		if finding == "" {
			continue
		}
		out = append(out, finding)
	}
	return out
}

// parseMissingSliceBinding extracts from/to ids from a missing-binding finding line.
func parseMissingSliceBinding(finding string) (from, to string, ok bool) {
	m := missingSliceBindingRE.FindStringSubmatch(finding)
	if len(m) != 3 {
		return "", "", false
	}
	from = strings.TrimSpace(m[1])
	to = strings.TrimSpace(m[2])
	if from == "" || to == "" {
		return "", "", false
	}
	return from, to, true
}

// applyEvidencedLibraryBindings adds slice→library SliceBindings for missing-binding
// architecture findings where to is an existing library id. Slice→slice edges are skipped.
func applyEvidencedLibraryBindings(t catalog.Typology, findings []string) (catalog.Typology, bool) {
	libs := make(map[string]struct{}, len(t.Libraries))
	for _, lib := range t.Libraries {
		if id := strings.TrimSpace(lib.ID); id != "" {
			libs[id] = struct{}{}
		}
	}
	if len(libs) == 0 {
		return t, false
	}
	slices := make(map[string]struct{}, len(t.Slices))
	for _, s := range t.Slices {
		if id := strings.TrimSpace(s.ID); id != "" {
			slices[id] = struct{}{}
		}
	}
	hasBinding := func(from, to string) bool {
		for _, b := range t.SliceBindings {
			if b.From == from && b.To == to {
				return true
			}
		}
		return false
	}
	changed := false
	for _, f := range findings {
		from, to, ok := parseMissingSliceBinding(f)
		if !ok {
			continue
		}
		if _, isLib := libs[to]; !isLib {
			continue
		}
		if _, isSlice := slices[from]; !isSlice {
			continue
		}
		if hasBinding(from, to) {
			continue
		}
		t.SliceBindings = append(t.SliceBindings, catalog.SliceBinding{
			From: from,
			To:   to,
			Kind: catalog.SliceReads,
		})
		changed = true
	}
	return t, changed
}

// formatFindingsList renders findings as a plain numbered list for LLM input.
func formatFindingsList(findings []string) string {
	if len(findings) == 0 {
		return ""
	}
	var b strings.Builder
	for i, f := range findings {
		fmt.Fprintf(&b, "%d. %s\n", i+1, f)
	}
	return strings.TrimSpace(b.String())
}

// validateHumanInterventionOutputs fails closed when open findings are under-reported.
func validateHumanInterventionOutputs(findings []string, journeyMD, humanInterventionMD, prPriorityMD string) error {
	if err := validateJourneyFindings(findings, journeyMD); err != nil {
		return err
	}
	if err := validateNamedFindingCoverage(findings, "human_intervention_md", humanInterventionMD); err != nil {
		return err
	}
	return validateNamedFindingCoverage(findings, "pr_priority_md", prPriorityMD)
}

func validateJourneyFindings(findings []string, journeyMD string) error {
	if len(findings) == 0 {
		return nil
	}
	if journeyStatusClaimsComplete(journeyMD) {
		return fmt.Errorf("human intervention: journey Status must not claim complete while architecture findings remain")
	}
	if err := validateNamedFindingCoverage(findings, "journey debt", journeyMD); err != nil {
		return err
	}
	if !journeyHasDebtTable(journeyMD) {
		return fmt.Errorf("human intervention: journey must include a technical debt table when findings remain")
	}
	return nil
}

func validateNamedFindingCoverage(findings []string, name, body string) error {
	if len(findings) == 0 {
		return nil
	}
	for _, f := range findings {
		needle := findingMatchNeedle(f)
		if needle == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(body), strings.ToLower(needle)) {
			return fmt.Errorf("human intervention: finding %q missing from %s", f, name)
		}
	}
	return nil
}

// findingMatchNeedle picks a stable substring for coverage checks.
func findingMatchNeedle(finding string) string {
	f := strings.TrimSpace(finding)
	if f == "" {
		return ""
	}
	// Prefer a backticked package/slice id when present.
	if i := strings.Index(f, "`"); i >= 0 {
		rest := f[i+1:]
		if j := strings.Index(rest, "`"); j > 0 {
			return rest[:j]
		}
	}
	if len(f) > 48 {
		return f[:48]
	}
	return f
}

func loadPRPriorityMarkdown(ctxDir string) string {
	path := filepath.Join(ctxDir, prPriorityRel)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func emptyHumanInterventionNote() string {
	return "# Human intervention\n\nNo open architecture findings after typology refine. No human boundary decisions required for this seed.\n"
}

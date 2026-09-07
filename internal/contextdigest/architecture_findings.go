package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	humanInterventionRel = "evidence/typology/human_intervention.md"
	prPriorityRel        = "evidence/typology/pr_priority.md"
)

// extractArchitectureFindings returns bullet/content lines under a findings heading.
func extractArchitectureFindings(architectureMD string) []string {
	lines := strings.Split(architectureMD, "\n")
	inFindings := false
	var out []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		low := strings.ToLower(trim)
		if strings.HasPrefix(low, "#") && strings.Contains(low, "finding") {
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
	if len(findings) == 0 {
		return nil
	}
	if journeyStatusClaimsComplete(journeyMD) {
		return fmt.Errorf("human intervention: journey Status must not claim complete while architecture findings remain")
	}
	targets := []struct {
		name, body string
	}{
		{"journey debt", journeyMD},
		{"human_intervention_md", humanInterventionMD},
		{"pr_priority_md", prPriorityMD},
	}
	for _, f := range findings {
		needle := findingMatchNeedle(f)
		if needle == "" {
			continue
		}
		for _, t := range targets {
			if !strings.Contains(strings.ToLower(t.body), strings.ToLower(needle)) {
				return fmt.Errorf("human intervention: finding %q missing from %s", f, t.name)
			}
		}
	}
	if !journeyHasDebtTable(journeyMD) {
		return fmt.Errorf("human intervention: journey must include a technical debt table when findings remain")
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

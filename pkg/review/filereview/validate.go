package filereview

import (
	"fmt"
	"strings"
)

// ValidateReports checks every reviewable has a report and every finding is well-formed.
func ValidateReports(reviewables []Reviewable, reports map[string]Report) error {
	var errs []string
	for _, r := range reviewables {
		rep, ok := reports[r.Slug]
		if !ok {
			errs = append(errs, fmt.Sprintf("missing report for slug %q (file %q)", r.Slug, r.File))
			continue
		}
		if strings.TrimSpace(rep.File) == "" {
			errs = append(errs, fmt.Sprintf("%s: empty file field", r.Slug))
		}
		if strings.TrimSpace(rep.Slug) == "" {
			errs = append(errs, fmt.Sprintf("%s: empty slug", r.Slug))
		}
		if len(rep.Findings) == 0 && !rep.NoIssues {
			errs = append(errs, fmt.Sprintf("%s: no findings and no explicit no-issues marker", r.Slug))
		}
		for i, f := range rep.Findings {
			if f.Severity != SeverityCritical && f.Severity != SeverityWarn && f.Severity != SeverityInfo {
				errs = append(errs, fmt.Sprintf("%s finding[%d]: bad severity %q", r.Slug, i, f.Severity))
			}
			if strings.TrimSpace(f.Text) == "" {
				errs = append(errs, fmt.Sprintf("%s finding[%d]: empty text", r.Slug, i))
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("filereview validate: %s", strings.Join(errs, "; "))
}

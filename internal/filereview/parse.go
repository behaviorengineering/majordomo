package filereview

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	findingRe    = regexp.MustCompile(`^\s*-\s+\[(CRITICAL|WARN|INFO)\]\s+(.+)$`)
	fileHeaderRe = regexp.MustCompile(`^#\s+(.+)$`)
	noIssuesRe   = regexp.MustCompile(`(?i)^\s*No issues found\.?\s*$`)
)

// ParseMarkdownReport reads a per-file markdown report into a Report.
func ParseMarkdownReport(path, fallbackSlug string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if fallbackSlug != "" {
		slug = fallbackSlug
	}
	rep := Report{Slug: slug, File: slug}
	noIssues := false
	for _, line := range strings.Split(string(data), "\n") {
		if m := fileHeaderRe.FindStringSubmatch(line); m != nil {
			rep.File = strings.TrimSpace(m[1])
			continue
		}
		if noIssuesRe.MatchString(line) {
			noIssues = true
			continue
		}
		if m := findingRe.FindStringSubmatch(line); m != nil {
			sev, err := ParseSeverity(m[1])
			if err != nil {
				return Report{}, err
			}
			text := strings.TrimSpace(m[2])
			if text == "" {
				return Report{}, fmt.Errorf("filereview: empty finding text in %s", path)
			}
			rep.Findings = append(rep.Findings, Finding{Severity: sev, Text: text})
		}
	}
	rep.NoIssues = noIssues && len(rep.Findings) == 0
	return rep, nil
}

// FormatMarkdown renders a Report to the display markdown shape.
func FormatMarkdown(rep Report) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(rep.File)
	b.WriteString("\n\n")
	if len(rep.Findings) == 0 {
		b.WriteString("No issues found.\n")
		return b.String()
	}
	for _, f := range rep.Findings {
		b.WriteString("- [")
		b.WriteString(f.Severity.Tag())
		b.WriteString("] ")
		b.WriteString(f.Text)
		b.WriteString("\n")
	}
	return b.String()
}

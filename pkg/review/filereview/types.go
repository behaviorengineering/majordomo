package filereview

import "fmt"

// Severity is a structured finding severity.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarn     Severity = "warn"
	SeverityInfo     Severity = "info"
)

// ParseSeverity maps markdown tags to Severity.
func ParseSeverity(tag string) (Severity, error) {
	switch tag {
	case "CRITICAL", "critical":
		return SeverityCritical, nil
	case "WARN", "warn":
		return SeverityWarn, nil
	case "INFO", "info":
		return SeverityInfo, nil
	default:
		return "", fmt.Errorf("filereview: unknown severity %q", tag)
	}
}

func (s Severity) Tag() string {
	switch s {
	case SeverityCritical:
		return "CRITICAL"
	case SeverityWarn:
		return "WARN"
	case SeverityInfo:
		return "INFO"
	default:
		return string(s)
	}
}

// Finding is one structured finding.
type Finding struct {
	Severity Severity `json:"severity"`
	Text     string   `json:"text"`
}

// Report is one reviewable's Judge output.
type Report struct {
	File     string    `json:"file"`
	Slug     string    `json:"slug"`
	Findings []Finding `json:"findings"`
	// NoIssues is true when the report explicitly has no findings (allowed).
	NoIssues bool `json:"no_issues,omitempty"`
}

// Reviewable is one Prepare input from the batch manifest.
type Reviewable struct {
	File string `json:"file"`
	Slug string `json:"slug"`
}

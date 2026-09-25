package poll

import (
	"bytes"
	"fmt"
	"text/template"
)

// repoOutcome is one served-repo result for the poll summary diagram.
type repoOutcome struct {
	RepoID     string
	SCM        string
	Owner      string
	Name       string
	Status     string // polled | no_credential | poll_disabled | list_error
	Open       int
	Pending    int
	Skipped    int // open but not queued (cursor)
	Detail     string
	Continuous bool
}

func (o repoOutcome) path() string {
	if o.Owner == "" && o.Name == "" {
		return "-"
	}
	if o.Name == "" {
		return o.Owner
	}
	if o.Owner == "" {
		return o.Name
	}
	return o.Owner + "/" + o.Name
}

func (o repoOutcome) continuousLabel() string {
	if o.Continuous {
		return "continuous=on"
	}
	return "continuous=off"
}

// pollSummaryData is the view model for pollSummaryTmpl.
type pollSummaryData struct {
	Configured   int
	Polled       int
	NoCredential int
	Disabled     int
	ListErrors   int
	OpenTotal    int
	SkipTotal    int
	PendingTotal int
	CredOK       int
	Listed       int
	Repos        []repoRow
}

type repoRow struct {
	RepoID          string
	SCM             string
	Path            string
	Status          string
	Open            int
	Pending         int
	Skipped         int
	Detail          string
	ContinuousLabel string
}

const pollSummaryTmpl = `========== poll summary ==========
 repos configured : {{.Configured}}
   polled         : {{.Polled}}
   no credential  : {{.NoCredential}}
   poll disabled  : {{.Disabled}}
   list errors    : {{.ListErrors}}
 open PRs/MRs     : {{.OpenTotal}}
   cursor skip    : {{.SkipTotal}}
 pending reviews  : {{.PendingTotal}}

 flow:
   [config {{.Configured}}] -> [cred ok {{.CredOK}}] -> [listed {{.Listed}}] -> [pending {{.PendingTotal}}]

 per repo:
{{- range .Repos}}
{{- if eq .Status "polled"}}
   {{printf "%-24s %-7s %-28s open=%-3d pending=%-3d skip=%-3d %s" .RepoID .SCM .Path .Open .Pending .Skipped .ContinuousLabel}}
{{- else if eq .Status "no_credential"}}
   {{printf "%-24s %-7s %-28s SKIP no credential (%s)" .RepoID .SCM .Path .Detail}}
{{- else if eq .Status "poll_disabled"}}
   {{printf "%-24s %-7s %-28s SKIP poll disabled" .RepoID .SCM .Path}}
{{- else if eq .Status "list_error"}}
   {{printf "%-24s %-7s %-28s ERROR list failed: %s" .RepoID .SCM .Path .Detail}}
{{- else}}
   {{printf "%-24s %-7s %-28s %s" .RepoID .SCM .Path .Status}}
{{- end}}
{{- end}}
==================================
`

var pollSummaryTemplate = template.Must(template.New("pollSummary").Parse(pollSummaryTmpl))

func formatASCIISummary(outcomes []repoOutcome, pendingTotal int) string {
	data := pollSummaryData{
		Configured:   len(outcomes),
		PendingTotal: pendingTotal,
		Repos:        make([]repoRow, 0, len(outcomes)),
	}
	for _, o := range outcomes {
		switch o.Status {
		case "polled":
			data.Polled++
			data.OpenTotal += o.Open
			data.SkipTotal += o.Skipped
		case "no_credential":
			data.NoCredential++
		case "poll_disabled":
			data.Disabled++
		case "list_error":
			data.ListErrors++
		}
		data.Repos = append(data.Repos, repoRow{
			RepoID:          o.RepoID,
			SCM:             o.SCM,
			Path:            o.path(),
			Status:          o.Status,
			Open:            o.Open,
			Pending:         o.Pending,
			Skipped:         o.Skipped,
			Detail:          o.Detail,
			ContinuousLabel: o.continuousLabel(),
		})
	}
	data.CredOK = data.Polled + data.ListErrors
	data.Listed = data.Polled

	var b bytes.Buffer
	if err := pollSummaryTemplate.Execute(&b, data); err != nil {
		return fmt.Sprintf("poll summary render failed: %v\n", err)
	}
	return b.String()
}

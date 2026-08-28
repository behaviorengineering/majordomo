package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

// CompactOptions configures chronology compaction.
type CompactOptions struct {
	MaxEntries   int
	KeepRecent   int
	ForceCompact bool
}

// DefaultCompactOptions returns plan defaults.
func DefaultCompactOptions(cfgMax int) CompactOptions {
	max := cfgMax
	if max <= 0 {
		max = 40
	}
	return CompactOptions{MaxEntries: max, KeepRecent: 10}
}

// CompactChronology merges older entries when over threshold (newest K locked).
func CompactChronology(ctxDir string, opts CompactOptions) (bool, error) {
	path := filepath.Join(ctxDir, "chronology.md")
	events, err := contextstore.ParseChronologyFile(path)
	if err != nil {
		return false, err
	}
	if !opts.ForceCompact && len(events) <= opts.MaxEntries {
		return false, nil
	}
	if len(events) <= opts.KeepRecent {
		return false, nil
	}
	keep := events[:opts.KeepRecent]
	older := events[opts.KeepRecent:]
	if len(older) == 0 {
		return false, nil
	}
	summary := contextstore.ChronologyEvent{
		Actor:     "majordomo",
		Source:    "compaction",
		Did:       fmt.Sprintf("compacted %d older chronology entries", len(older)),
		Because:   "teaching document readability threshold exceeded",
		InOrderTo: "keep chronology scannable for newcomers",
		Evidence:  fmt.Sprintf("entries %d–%d merged without inventing new claims", opts.KeepRecent+1, len(events)),
	}
	if err := rewriteChronology(path, append([]contextstore.ChronologyEvent{summary}, keep...)); err != nil {
		return false, err
	}
	_, err = contextstore.ParseChronologyFile(path)
	return true, err
}

func rewriteChronology(path string, events []contextstore.ChronologyEvent) error {
	var b strings.Builder
	b.WriteString("# Chronology\n\nNewest first.\n")
	for _, ev := range events {
		if ev.Date.IsZero() {
			continue
		}
		if strings.TrimSpace(ev.Actor) == "" {
			ev.Actor = "majordomo"
		}
		if strings.TrimSpace(ev.Source) == "" {
			ev.Source = "compaction"
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "### %s - %s - %s\n\n", ev.Date.Format("2006-01-02"), ev.Actor, ev.Source)
		fmt.Fprintf(&b, "- **Did:** %s\n", ev.Did)
		fmt.Fprintf(&b, "- **Because:** %s\n", ev.Because)
		fmt.Fprintf(&b, "- **In order to:** %s\n", ev.InOrderTo)
		fmt.Fprintf(&b, "- **Evidence:** %s\n", ev.Evidence)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

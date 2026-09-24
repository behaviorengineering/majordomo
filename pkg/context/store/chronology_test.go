package contextstore

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseChronologyGolden(t *testing.T) {
	events, err := ParseChronologyFile(filepath.Join("testdata", "valid", "chronology.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events", len(events))
	}
	if events[0].Actor != "Alice" || events[0].Source != "PR #412" {
		t.Fatalf("first event = %+v", events[0])
	}
	if events[0].Did == "" || events[0].Because == "" || events[0].InOrderTo == "" || events[0].Evidence == "" {
		t.Fatalf("first event missing fields: %+v", events[0])
	}
	if events[1].Actor != "Bob" {
		t.Fatalf("second event = %+v", events[1])
	}
}

func TestParseChronologyEmDashHeading(t *testing.T) {
	src := `# Chronology

### 2026-08-28 — Alice — PR #412

- **Did:** extract auth into middleware
- **Because:** token checks were duplicated in three handlers
- **In order to:** make session expiry consistent
- **Evidence:** PR #412, merge ` + "`abc123def`" + `
`
	events, err := ParseChronology(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Actor != "Alice" {
		t.Fatalf("got %+v", events)
	}
}

func TestParseChronologyEmpty(t *testing.T) {
	events, err := ParseChronology(strings.NewReader("# Chronology\n\nNewest first.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("got %d events", len(events))
	}
}

func TestParseChronologyMissingBecause(t *testing.T) {
	_, err := ParseChronologyFile(filepath.Join("testdata", "bad-chronology", "chronology.md"))
	if err == nil {
		t.Fatal("expected missing Because")
	}
	if !strings.Contains(err.Error(), "Because") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseChronologyNewestFirst(t *testing.T) {
	src := `### 2026-08-01 - Bob - PR #400

- **Did:** a
- **Because:** b
- **In order to:** c
- **Evidence:** d

### 2026-08-28 - Alice - PR #412

- **Did:** a
- **Because:** b
- **In order to:** c
- **Evidence:** d
`
	_, err := ParseChronology(strings.NewReader(src))
	if err == nil {
		t.Fatal("expected newest-first error")
	}
}

func TestParseChronologyBadHeading(t *testing.T) {
	_, err := ParseChronology(strings.NewReader("### not-a-date - x - y\n"))
	if err == nil {
		t.Fatal("expected heading error")
	}
}

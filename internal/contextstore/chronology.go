package contextstore

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	fieldDid       = "Did"
	fieldBecause   = "Because"
	fieldInOrderTo = "In order to"
	fieldEvidence  = "Evidence"
)

var (
	headingRE      = regexp.MustCompile(`^###\s+(\d{4}-\d{2}-\d{2})\s+[—–-]\s+(.+?)\s+[—–-]\s+(.+)\s*$`)
	bulletRE       = regexp.MustCompile(`^-\s+\*\*(Did|Because|In order to|Evidence):\*\*\s+(.+)\s*$`)
	requiredFields = []string{fieldDid, fieldBecause, fieldInOrderTo, fieldEvidence}
)

// ChronologyEvent is one append-only chronology.md entry.
type ChronologyEvent struct {
	Date      time.Time
	Actor     string
	Source    string
	Did       string
	Because   string
	InOrderTo string
	Evidence  string
	Heading   string
}

// ParseChronologyFile reads chronology.md from path.
func ParseChronologyFile(path string) ([]ChronologyEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read chronology.md: %w", err)
	}
	defer f.Close()
	events, err := ParseChronology(f)
	if err != nil {
		return nil, err
	}
	return events, nil
}

// ParseChronology parses chronology.md from r.
func ParseChronology(r io.Reader) ([]ChronologyEvent, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var events []ChronologyEvent
	var cur *pendingEvent
	lineNo := 0

	flush := func() error {
		if cur == nil {
			return nil
		}
		ev, err := cur.finish()
		if err != nil {
			return err
		}
		events = append(events, ev)
		cur = nil
		return nil
	}

	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), " \t")
		if strings.HasPrefix(line, "### ") {
			if err := flush(); err != nil {
				return nil, err
			}
			m := headingRE.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("chronology.md:%d: heading must be ### YYYY-MM-DD - actor - source", lineNo)
			}
			day, err := time.Parse("2006-01-02", m[1])
			if err != nil {
				return nil, fmt.Errorf("chronology.md:%d: invalid date %q: %w", lineNo, m[1], err)
			}
			cur = &pendingEvent{
				line:    lineNo,
				heading: line,
				date:    day,
				actor:   strings.TrimSpace(m[2]),
				source:  strings.TrimSpace(m[3]),
				fields:  map[string]string{},
			}
			continue
		}
		if cur == nil {
			continue
		}
		if line == "" {
			continue
		}
		bm := bulletRE.FindStringSubmatch(line)
		if bm == nil {
			return nil, fmt.Errorf("chronology.md:%d: expected a Did/Because/In order to/Evidence bullet", lineNo)
		}
		key, val := bm[1], strings.TrimSpace(bm[2])
		if _, ok := cur.fields[key]; ok {
			return nil, fmt.Errorf("chronology.md:%d: duplicate %s", lineNo, key)
		}
		if val == "" {
			return nil, fmt.Errorf("chronology.md:%d: %s is empty", lineNo, key)
		}
		cur.fields[key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read chronology.md: %w", err)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if err := validateNewestFirst(events); err != nil {
		return nil, err
	}
	return events, nil
}

type pendingEvent struct {
	line    int
	heading string
	date    time.Time
	actor   string
	source  string
	fields  map[string]string
}

func (p *pendingEvent) finish() (ChronologyEvent, error) {
	for _, key := range requiredFields {
		if strings.TrimSpace(p.fields[key]) == "" {
			return ChronologyEvent{}, fmt.Errorf("chronology.md:%d: missing **%s:**", p.line, key)
		}
	}
	return ChronologyEvent{
		Date:      p.date,
		Actor:     p.actor,
		Source:    p.source,
		Did:       p.fields[fieldDid],
		Because:   p.fields[fieldBecause],
		InOrderTo: p.fields[fieldInOrderTo],
		Evidence:  p.fields[fieldEvidence],
		Heading:   p.heading,
	}, nil
}

func validateNewestFirst(events []ChronologyEvent) error {
	for i := 1; i < len(events); i++ {
		if events[i].Date.After(events[i-1].Date) {
			return fmt.Errorf("chronology.md: events must be newest first (entry %d is dated after entry %d)", i+1, i)
		}
	}
	return nil
}

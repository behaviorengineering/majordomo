package contextstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AppendChronologyEvent prepends a validated event to chronology.md (newest first).
func AppendChronologyEvent(dir string, ev ChronologyEvent) error {
	if strings.TrimSpace(ev.Did) == "" || strings.TrimSpace(ev.Because) == "" ||
		strings.TrimSpace(ev.InOrderTo) == "" || strings.TrimSpace(ev.Evidence) == "" {
		return fmt.Errorf("chronology event requires Did, Because, In order to, and Evidence")
	}
	if ev.Date.IsZero() {
		ev.Date = time.Now().UTC()
	}
	if strings.TrimSpace(ev.Actor) == "" {
		ev.Actor = "majordomo"
	}
	if strings.TrimSpace(ev.Source) == "" {
		ev.Source = "digest"
	}
	heading := fmt.Sprintf("### %s - %s - %s", ev.Date.Format("2006-01-02"), ev.Actor, ev.Source)
	block := heading + "\n\n" +
		fmt.Sprintf("- **Did:** %s\n", ev.Did) +
		fmt.Sprintf("- **Because:** %s\n", ev.Because) +
		fmt.Sprintf("- **In order to:** %s\n", ev.InOrderTo) +
		fmt.Sprintf("- **Evidence:** %s\n", ev.Evidence)

	path := filepath.Join(dir, "chronology.md")
	existing, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read chronology.md: %w", err)
	}
	body := strings.TrimRight(string(existing), " \t\n")
	if !strings.HasPrefix(body, "# Chronology") {
		body = "# Chronology\n\nNewest first.\n"
	}
	var out strings.Builder
	out.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		out.WriteByte('\n')
	}
	out.WriteByte('\n')
	out.WriteString(block)
	if err := os.WriteFile(path, []byte(out.String()), 0o644); err != nil {
		return err
	}
	_, err = ParseChronologyFile(path)
	return err
}

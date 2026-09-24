package diff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildAllOptions configures concatenation of per-file staging diffs.
type BuildAllOptions struct {
	Manifest string
	Output   string
	// Cap truncates each file's diff to at most Cap lines. Nil means no cap.
	Cap *int
}

type manifestFile struct {
	Reviewable []reviewableEntry `json:"reviewable"`
}

type reviewableEntry struct {
	File         string          `json:"file"`
	InputFile    string          `json:"input_file"`
	AgentContext json.RawMessage `json:"agent_context"`
}

// BuildAll concatenates per-file diffs from a staging manifest into one file.
// Port of pipelines/scripts/build-all-diffs.py.
func BuildAll(opts BuildAllOptions) error {
	if opts.Manifest == "" || opts.Output == "" {
		return fmt.Errorf("manifest and output paths required")
	}
	raw, err := os.ReadFile(opts.Manifest)
	if err != nil {
		return fmt.Errorf("cannot read manifest %s: %w", opts.Manifest, err)
	}
	var m manifestFile
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("cannot parse manifest %s: %w", opts.Manifest, err)
	}
	if m.Reviewable == nil {
		return fmt.Errorf("manifest %s has no 'reviewable' array", opts.Manifest)
	}

	if err := os.MkdirAll(filepath.Dir(opts.Output), 0o755); err != nil {
		return err
	}
	f, err := os.Create(opts.Output)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range m.Reviewable {
		if err := writeEntry(f, entry, opts.Cap); err != nil {
			return err
		}
	}
	return nil
}

func writeEntry(f *os.File, entry reviewableEntry, capLines *int) error {
	if _, err := fmt.Fprintf(f, "=== FILE: %s ===\n", entry.File); err != nil {
		return err
	}
	if ctx := serializeAgentContext(entry.AgentContext); ctx != "" {
		if _, err := fmt.Fprintf(f, "=== AGENT CONTEXT: %s ===\n", ctx); err != nil {
			return err
		}
	}

	data, err := os.ReadFile(entry.InputFile)
	var lines []string
	if err != nil {
		lines = nil
	} else {
		text := string(data)
		if text == "" {
			lines = []string{}
		} else {
			lines = strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
			// Python splitlines() drops a trailing empty line from a final newline.
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
		}
	}

	if capLines == nil || len(lines) <= *capLines {
		if len(lines) > 0 {
			if _, err := f.WriteString(strings.Join(lines, "\n")); err != nil {
				return err
			}
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
	} else {
		capN := *capLines
		if _, err := f.WriteString(strings.Join(lines[:capN], "\n")); err != nil {
			return err
		}
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
		omitted := len(lines) - capN
		if _, err := fmt.Fprintf(f, "[... %d lines omitted — diff cap is %d lines]\n", omitted, capN); err != nil {
			return err
		}
	}
	_, err = f.WriteString("\n")
	return err
}

func serializeAgentContext(raw json.RawMessage) string {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj) == 0 {
		return ""
	}
	// encoding/json sorts map keys, matching Python json.dumps(..., sort_keys=True).
	out, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return string(out)
}

package contextdigest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolishTypologyArchitectureBriefFillsEmptyFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "architecture.md")
	raw := `# Typology Architecture Brief

#### ` + "`pruneagent`" + `

Automated pruning of stale resources.

- Packages: ./internal/pruneagent
- Surfaces:
- Programs:

## Observed topology

The repository contains **13 Go packages** in the inspected modules:
- ` + "`.`" + `

### High-coupling packages

| Package | In-degree | Out-degree |
|---------|-----------|------------|
| ` + "`./cmd/gitboard`" + ` | 0 | 11 |
`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "graph.txt"), []byte(`=== Typology Import Graph ===
Hubs (high coupling):
  - ./cmd/gitboard (in: 0, out: 11)
  - ./internal/dashboard (in: 2, out: 4)
Leaves (out-degree 0):
  - ./internal/board (in: 2)
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := polishTypologyArchitectureBrief(path); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	if !strings.Contains(body, "- Surfaces: _(none)_") {
		t.Fatalf("surfaces not polished:\n%s", body)
	}
	if !strings.Contains(body, "- Programs: _(none)_") {
		t.Fatalf("programs not polished:\n%s", body)
	}
	if strings.Contains(body, "- `.`\n") {
		t.Fatalf("module-scope inventory still present:\n%s", body)
	}
	if !strings.Contains(body, "- `./cmd/gitboard`") {
		t.Fatalf("expected graph package inventory:\n%s", body)
	}
	if !strings.Contains(body, typologyArchitectureRoleMarker) {
		t.Fatalf("expected Typology evidence banner:\n%s", body)
	}
}

func TestPolishTypologyArchitectureBriefAddsBannerWhenAlreadyGood(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "architecture.md")
	raw := `## Observed topology

The repository contains **3 Go packages** in the inspected modules:
- ` + "`./internal/board`" + `
- ` + "`./cmd/gitboard`" + `

- Surfaces: gitboard-cli (cli)
- Programs: _(none)_
`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := polishTypologyArchitectureBrief(path); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), typologyArchitectureRoleMarker) {
		t.Fatalf("expected Typology evidence banner:\n%s", after)
	}
}

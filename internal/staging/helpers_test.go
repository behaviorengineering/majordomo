package staging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildStagingFilenameKeepsByteLimit(t *testing.T) {
	longSlug := string(make([]byte, 600))
	for i := range longSlug {
		longSlug = longSlug[:i] + "x" + longSlug[i+1:]
	}
	// simpler:
	b := make([]byte, 600)
	for i := range b {
		b[i] = 'x'
	}
	fileName := BuildStagingFilename(string(b), "")
	if filepath.Ext(fileName) != ".txt" {
		t.Fatalf("expected .txt suffix, got %q", fileName)
	}
	if len([]byte(fileName)) > MaxStageFilenameBytes {
		t.Fatalf("filename too long: %d", len([]byte(fileName)))
	}
	if !containsDash(fileName) {
		t.Fatalf("expected hash dash in truncated name: %s", fileName)
	}
}

func containsDash(s string) bool {
	for _, c := range s {
		if c == '-' {
			return true
		}
	}
	return false
}

func TestIsExcludedMatchesExpectedPatterns(t *testing.T) {
	if !IsExcluded("uv.lock") {
		t.Fatal("uv.lock should be excluded")
	}
	if !IsExcluded("frontend/app.min.js") {
		t.Fatal("min.js should be excluded")
	}
	if !IsExcluded("dist/package.js") {
		t.Fatal("dist/ should be excluded")
	}
	if IsExcluded("src/app.py") {
		t.Fatal("src/app.py should not be excluded")
	}
}

func TestLoadAgentContextConfigSupportsLegacyFlatForm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-context.json")
	data := `{"techStack":["python"],"customRules":["prefer explicit error handling"]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := LoadAgentContextConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Scoped) != 0 {
		t.Fatalf("scoped should be empty: %v", parsed.Scoped)
	}
	stack, _ := parsed.Global["techStack"].([]any)
	if len(stack) != 1 || stack[0] != "python" {
		t.Fatalf("techStack: %v", parsed.Global["techStack"])
	}
}

func TestResolveRoutingPersonasMissingFileExits(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveRoutingPersonas(map[string]string{"pr-review-code": "agents/missing.persona.md"}, dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*ErrFatal); !ok {
		t.Fatalf("expected ErrFatal, got %T: %v", err, err)
	}
}

func TestContextForFileInvalidScopedContextExits(t *testing.T) {
	dir := t.TempDir()
	ctx := AgentContext{
		Global: map[string]any{"customRules": []any{"rule-a"}},
		Scoped: map[string]any{"src/**": "invalid"},
	}
	_, err := ContextForFile("src/app.py", ctx, dir)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseNameStatusUsesDestinationForRenameAndCopy(t *testing.T) {
	raw := "R100\x00src/old.py\x00src/new.py\x00C075\x00a.txt\x00b.txt\x00"
	parsed := ParseNameStatus(raw)
	if len(parsed) != 2 || parsed[0] != [2]string{"R", "src/new.py"} || parsed[1] != [2]string{"C", "b.txt"} {
		t.Fatalf("got %#v", parsed)
	}
}

func TestParseNameStatusHandlesRegularStatusEntries(t *testing.T) {
	raw := "A\x00src/add.py\x00M\x00src/mod.py\x00D\x00src/del.py\x00"
	parsed := ParseNameStatus(raw)
	want := [][2]string{{"A", "src/add.py"}, {"M", "src/mod.py"}, {"D", "src/del.py"}}
	if len(parsed) != len(want) {
		t.Fatalf("got %#v", parsed)
	}
	for i := range want {
		if parsed[i] != want[i] {
			t.Fatalf("got %#v", parsed)
		}
	}
}

func TestGetSubmoduleExclusionsParsesCachedStatus(t *testing.T) {
	stdout := " abc123 .majordomo (heads/main)\n-fff111 vendor/lib\n"
	patterns := ParseSubmoduleStatusLines(stdout)
	matchedMajordomo := false
	matchedVendor := false
	for _, p := range patterns {
		if p.MatchString(".majordomo/stages/copilot-review.groovy") {
			matchedMajordomo = true
		}
		if p.MatchString("vendor/lib/src/main.py") {
			matchedVendor = true
		}
	}
	if !matchedMajordomo || !matchedVendor {
		t.Fatalf("patterns did not match: majordomo=%v vendor=%v", matchedMajordomo, matchedVendor)
	}
}

func TestLoadRoutingFallsBackToDefaultsForInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")
	if err := os.WriteFile(path, []byte("{not-valid-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, personas, err := LoadRouting(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(personas) != 0 {
		t.Fatal("expected empty personas")
	}
	if len(rules) != len(DefaultRouting) {
		t.Fatalf("expected default routing len %d got %d", len(DefaultRouting), len(rules))
	}
}

func TestLoadRoutingExitsForInvalidRuleShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")
	data, _ := json.Marshal(map[string]any{
		"pr-review-code": map[string]any{"globs": "**/*.py"},
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadRouting(path)
	if err == nil {
		t.Fatal("expected error")
	}
}

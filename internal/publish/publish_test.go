package publish

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasBodyContent(t *testing.T) {
	if HasBodyContent("# Title\n\n<!-- x -->\n") {
		t.Fatal("expected false")
	}
	if !HasBodyContent("# Title\n\nHello world\n") {
		t.Fatal("expected true")
	}
}

func TestPublishGitHubViaCLI(t *testing.T) {
	dir := t.TempDir()
	summary := filepath.Join(dir, "summary.md")
	if err := os.WriteFile(summary, []byte("# Review\n\nFinding one.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls [][]string
	err := Run(Options{
		SCM: "github", PRNumber: "9", SummaryFile: summary, Mode: ModeComment,
		GitHubToken: "tok", GitHubOwner: "acme", GitHubRepo: "demo",
		Runner: func(name string, args []string, env []string) (string, error) {
			if name != "gh" {
				t.Fatalf("want gh, got %s", name)
			}
			calls = append(calls, append([]string{name}, args...))
			return "", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0][1] != "pr" || calls[0][2] != "comment" {
		t.Fatalf("calls=%v", calls)
	}
	joined := strings.Join(calls[0], " ")
	if !strings.Contains(joined, "--body-file") || !strings.Contains(joined, "-R") {
		t.Fatalf("unexpected args %v", calls[0])
	}
}

func TestPublishGitLabAutoClaimsDescription(t *testing.T) {
	dir := t.TempDir()
	summary := filepath.Join(dir, "summary.md")
	if err := os.WriteFile(summary, []byte("# Review\n\nFinding one.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls []string
	err := Run(Options{
		SCM: "gitlab", PRNumber: "7", SummaryFile: summary, Mode: ModeAuto,
		GitLabToken: "glpat", GitHubOwner: "acme", GitHubRepo: "pay",
		Runner: func(name string, args []string, env []string) (string, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			if name == "glab" && len(args) >= 2 && args[0] == "mr" && args[1] == "view" {
				return `{"description":""}`, nil
			}
			return "", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) < 2 {
		t.Fatalf("expected view+update, got %v", calls)
	}
	if !strings.Contains(calls[1], "mr update") {
		t.Fatalf("expected update after empty description, got %v", calls)
	}
}

func TestPublishGitLabMissingCLIReported(t *testing.T) {
	dir := t.TempDir()
	summary := filepath.Join(dir, "summary.md")
	_ = os.WriteFile(summary, []byte("# R\n\nbody\n"), 0o644)
	err := Run(Options{
		SCM: "gitlab", PRNumber: "1", SummaryFile: summary, Mode: ModeComment,
		GitLabToken: "t", GitHubOwner: "a", GitHubRepo: "b",
		Runner: func(name string, args []string, env []string) (string, error) {
			return "", fmt.Errorf("%s not found on PATH", name)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
}

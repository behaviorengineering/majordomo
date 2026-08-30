package poll

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/config"
)

func TestFormatASCIISummary(t *testing.T) {
	got := formatASCIISummary([]repoOutcome{
		{RepoID: "polypus", SCM: "github", Owner: "behaviorengineering", Name: "polypus", Status: "polled", Open: 0, Pending: 0, Continuous: true},
		{RepoID: "consilium", SCM: "gitlab", Owner: "behaviorengineering", Name: "consilium", Status: "polled", Open: 2, Pending: 1, Skipped: 1, Continuous: true},
		{RepoID: "example-github", SCM: "github", Owner: "YOUR_ORG", Name: "YOUR_REPO", Status: "no_credential", Detail: "GH_TOKEN_YOUR_ORG"},
	}, 1)
	for _, want := range []string{
		"repos configured : 3",
		"polled         : 2",
		"no credential  : 1",
		"open PRs/MRs     : 2",
		"pending reviews  : 1",
		"[config 3] -> [cred ok 2] -> [listed 2] -> [pending 1]",
		"polypus",
		"consilium",
		"SKIP no credential",
		"continuous=on",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRunWritesPollSummary(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	cursorDir := filepath.Join(dir, "cursors")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("trigger:\n  poll: true\n"), 0o644)
	_ = os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte(`
scm: github
repository:
  id: demo
  owner: acme
  name: demo
  cloneUrl: https://github.com/acme/demo.git
`), 0o644)

	t.Setenv("GH_TOKEN_ACME", "tok")
	err := Run(Options{
		ConfigDir: cfgDir,
		CursorDir: cursorDir,
		OutPath:   filepath.Join(dir, "pending.json"),
		ListPRs: func(cfg config.RepoConfig, token string) ([]openPR, error) {
			return []openPR{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cursorDir, "poll-summary.txt"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "demo") || !strings.Contains(s, "pending reviews  : 0") {
		t.Fatalf("unexpected summary:\n%s", s)
	}
}

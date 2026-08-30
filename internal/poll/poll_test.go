package poll

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/config"
)

func TestRunEmitsPending(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("trigger:\n  poll: true\n"), 0o644)
	_ = os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte(`
scm: github
repository:
  id: demo
  owner: acme
  name: demo
  cloneUrl: https://github.com/acme/demo.git
trigger:
  poll: true
publishMode: auto
`), 0o644)

	out := filepath.Join(dir, "pending.json")
	err := Run(Options{
		ConfigDir: cfgDir,
		CursorDir: filepath.Join(dir, "cursors"),
		OutPath:   out,
		ListPRs: func(cfg config.RepoConfig, token string) ([]openPR, error) {
			return []openPR{{Number: 3, HeadSHA: "abc1234deadbeef", BaseBranch: "main"}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), `"pr": "3"`) && !strings.Contains(string(data), `"pr":"3"`) {
		t.Fatalf("missing pr in %s", data)
	}
}

func TestRunSkipsMissingCredentialWithoutCallingList(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("trigger:\n  poll: true\n"), 0o644)
	_ = os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte(`
scm: github
repository:
  id: demo
  owner: acme
  name: demo
trigger:
  poll: true
`), 0o644)

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GH_TOKEN_ACME", "")
	t.Setenv("MAJORDOMO_CREDENTIAL_DEMO", "")

	err := Run(Options{
		ConfigDir: cfgDir,
		CursorDir: filepath.Join(dir, "cursors"),
		OutPath:   filepath.Join(dir, "pending.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "pending.json"))
	if strings.Contains(string(data), `"pr"`) {
		t.Fatalf("expected no reviews without credential, got %s", data)
	}
}

func TestRunUsesOrgCredential(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("trigger:\n  poll: true\n"), 0o644)
	_ = os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte(`
scm: github
repository:
  id: demo
  owner: acme
  name: demo
trigger:
  poll: true
`), 0o644)

	t.Setenv("MAJORDOMO_CREDENTIAL_DEMO", "")
	t.Setenv("GH_TOKEN_ACME", "org-secret")

	var sawToken string
	err := Run(Options{
		ConfigDir: cfgDir,
		CursorDir: filepath.Join(dir, "cursors"),
		OutPath:   filepath.Join(dir, "pending.json"),
		ListPRs: func(cfg config.RepoConfig, token string) ([]openPR, error) {
			sawToken = token
			return []openPR{{Number: 1, HeadSHA: "abc", BaseBranch: "main"}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sawToken != "org-secret" {
		t.Fatalf("token=%q", sawToken)
	}
}

func TestRunFailsWhenListErrors(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("trigger:\n  poll: true\n"), 0o644)
	_ = os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte(`
scm: github
repository:
  id: demo
  owner: acme
  name: demo
trigger:
  poll: true
`), 0o644)

	err := Run(Options{
		ConfigDir: cfgDir,
		CursorDir: filepath.Join(dir, "cursors"),
		OutPath:   filepath.Join(dir, "pending.json"),
		ListPRs: func(cfg config.RepoConfig, token string) ([]openPR, error) {
			return nil, fmt.Errorf("boom")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "poll incomplete") {
		t.Fatalf("expected poll incomplete error, got %v", err)
	}
}

func TestIsMajordomoInternalBranch(t *testing.T) {
	cases := []struct {
		base, head string
		want       bool
	}{
		{"main", "feature/x", false},
		{"majordomo-context/demo", "majordomo-context/demo-update", true},
		{"main", "majordomo-context/demo-update", true},
		{"majordomo-pr-reviewer-cache/demo", "bot/cache", true},
		{"main", "majordomo-poll-cache/demo", true},
		{"main", "feature/majordomo-context/nope", false},
	}
	for _, tc := range cases {
		if got := isMajordomoInternalBranch(tc.base, tc.head); got != tc.want {
			t.Fatalf("base=%q head=%q got %v want %v", tc.base, tc.head, got, tc.want)
		}
	}
}

func TestRunSkipsMajordomoInternalBranches(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("trigger:\n  poll: true\n"), 0o644)
	_ = os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte(`
scm: github
repository:
  id: demo
  owner: acme
  name: demo
trigger:
  poll: true
`), 0o644)

	out := filepath.Join(dir, "pending.json")
	err := Run(Options{
		ConfigDir: cfgDir,
		CursorDir: filepath.Join(dir, "cursors"),
		OutPath:   out,
		ListPRs: func(cfg config.RepoConfig, token string) ([]openPR, error) {
			return []openPR{
				{
					Number:     10,
					HeadSHA:    "ctxsha",
					BaseBranch: "majordomo-context/demo",
					HeadBranch: "majordomo-context/demo-update",
				},
				{
					Number:     11,
					HeadSHA:    "cachesha",
					BaseBranch: "main",
					HeadBranch: "majordomo-pr-reviewer-cache/demo",
				},
				{
					Number:     3,
					HeadSHA:    "abc1234deadbeef",
					BaseBranch: "main",
					HeadBranch: "feature/ok",
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	s := string(data)
	if strings.Contains(s, `"pr": "10"`) || strings.Contains(s, `"pr":"10"`) {
		t.Fatalf("expected context PR skipped, got %s", s)
	}
	if strings.Contains(s, `"pr": "11"`) || strings.Contains(s, `"pr":"11"`) {
		t.Fatalf("expected cache PR skipped, got %s", s)
	}
	if !strings.Contains(s, `"pr": "3"`) && !strings.Contains(s, `"pr":"3"`) {
		t.Fatalf("expected normal PR pending, got %s", s)
	}
}

func TestListGitHubPRsPaginated(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages = append(pages, r.URL.RawQuery)
		if !strings.Contains(r.URL.Path, "/repos/acme/demo/pulls") {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("auth %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("page") == "2" || strings.Contains(r.URL.RawQuery, "page=2") {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"number": 2, "head": map[string]string{"sha": "bbb", "ref": "feat-b"}, "base": map[string]string{"ref": "main"}},
			})
			return
		}
		w.Header().Set("Link", `<http://`+r.Host+r.URL.Path+`?state=open&per_page=100&page=2>; rel="next"`)
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"number": 1, "head": map[string]string{"sha": "aaa", "ref": "feat-a"}, "base": map[string]string{"ref": "main"}},
		})
	}))
	defer srv.Close()

	prs, err := listGitHubPRs(config.RepoConfig{
		SCM:        "github",
		Repository: config.Repository{Owner: "acme", Name: "demo"},
		SCMAPI:     config.SCMAPI{BaseURL: srv.URL},
	}, "tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 2 || prs[0].Number != 1 || prs[1].Number != 2 {
		t.Fatalf("got %#v pages=%v", prs, pages)
	}
	if prs[0].HeadBranch != "feat-a" || prs[1].HeadBranch != "feat-b" {
		t.Fatalf("head branches %#v", prs)
	}
}

func TestListGitLabMRs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "glpat-x" {
			t.Fatalf("token header %q", r.Header.Get("PRIVATE-TOKEN"))
		}
		if !strings.Contains(r.URL.RequestURI(), "acme%2Fdemo") {
			t.Fatalf("expected encoded project in %s (path=%s)", r.URL.RequestURI(), r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"iid": 9, "sha": "deadbeef", "source_branch": "feat", "target_branch": "main"},
		})
	}))
	defer srv.Close()

	prs, err := listGitLabMRs(config.RepoConfig{
		SCM: "gitlab",
		Repository: config.Repository{
			ID:       "demo",
			CloneURL: "https://gitlab.com/acme/demo.git",
		},
		SCMAPI: config.SCMAPI{BaseURL: srv.URL},
	}, "glpat-x")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 || prs[0].Number != 9 || prs[0].HeadSHA != "deadbeef" {
		t.Fatalf("got %#v", prs)
	}
	if prs[0].HeadBranch != "feat" || prs[0].BaseBranch != "main" {
		t.Fatalf("branches %#v", prs[0])
	}
}

func TestParseClonePathNested(t *testing.T) {
	path := parseClonePath("https://gitlab.com/acme/team/demo.git")
	owner, name := splitOwnerName(path)
	if path != "acme/team/demo" || owner != "acme/team" || name != "demo" {
		t.Fatalf("path=%q owner=%q name=%q", path, owner, name)
	}
}

func TestRunSkipsAlreadyReviewedWhenContinuousOff(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	cursorDir := filepath.Join(dir, "cursors")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte(`
trigger:
  poll: true
review:
  publishMode: auto
  enableContinuousRuns: false
`), 0o644)
	_ = os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte(`
scm: github
repository:
  id: demo
  owner: acme
  name: demo
`), 0o644)

	t.Setenv("GH_TOKEN_ACME", "tok")
	// Seed cursor as if PR #1 was already reviewed at an older head.
	_ = os.MkdirAll(filepath.Join(cursorDir, "demo"), 0o755)
	_ = os.WriteFile(filepath.Join(cursorDir, "demo", "poll-cursor.json"), []byte(`{
  "repo_id": "demo",
  "heads": {"1": "oldsha"}
}
`), 0o644)

	out := filepath.Join(dir, "pending.json")
	err := Run(Options{
		ConfigDir: cfgDir,
		CursorDir: cursorDir,
		OutPath:   out,
		ListPRs: func(cfg config.RepoConfig, token string) ([]openPR, error) {
			return []openPR{{Number: 1, HeadSHA: "newsha", BaseBranch: "main"}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if strings.Contains(string(data), `"pr"`) {
		t.Fatalf("expected skip when continuous off and PR already in cursor, got %s", data)
	}
}

func TestRunRequeuesOnNewHeadWhenContinuousOn(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	cursorDir := filepath.Join(dir, "cursors")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte(`
trigger:
  poll: true
review:
  enableContinuousRuns: true
  publishMode: comment
`), 0o644)
	_ = os.WriteFile(filepath.Join(cfgDir, "demo.yaml"), []byte(`
scm: github
repository:
  id: demo
  owner: acme
  name: demo
`), 0o644)

	t.Setenv("GH_TOKEN_ACME", "tok")
	_ = os.MkdirAll(filepath.Join(cursorDir, "demo"), 0o755)
	_ = os.WriteFile(filepath.Join(cursorDir, "demo", "poll-cursor.json"), []byte(`{
  "repo_id": "demo",
  "heads": {"1": "oldsha"}
}
`), 0o644)

	out := filepath.Join(dir, "pending.json")
	err := Run(Options{
		ConfigDir: cfgDir,
		CursorDir: cursorDir,
		OutPath:   out,
		ListPRs: func(cfg config.RepoConfig, token string) ([]openPR, error) {
			return []openPR{{Number: 1, HeadSHA: "newsha", BaseBranch: "main"}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), `"pr": "1"`) && !strings.Contains(string(data), `"pr":"1"`) {
		t.Fatalf("expected re-queue, got %s", data)
	}
	if !strings.Contains(string(data), `"publish_mode": "comment"`) && !strings.Contains(string(data), `"publish_mode":"comment"`) {
		t.Fatalf("expected review.publishMode, got %s", data)
	}
}

func TestRunGitLabPending(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "cfg")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "_defaults.yaml"), []byte("trigger:\n  poll: true\n"), 0o644)
	_ = os.WriteFile(filepath.Join(cfgDir, "pay.yaml"), []byte(`
scm: gitlab
repository:
  id: pay
  cloneUrl: https://gitlab.com/acme/pay.git
scmApi:
  baseUrl: https://gitlab.com
  projectId: "42"
trigger:
  poll: true
`), 0o644)

	out := filepath.Join(dir, "pending.json")
	err := Run(Options{
		ConfigDir: cfgDir,
		CursorDir: filepath.Join(dir, "cursors"),
		OutPath:   out,
		ListPRs: func(cfg config.RepoConfig, token string) ([]openPR, error) {
			if cfg.SCM != "gitlab" {
				t.Fatalf("scm %s", cfg.SCM)
			}
			return []openPR{{Number: 7, HeadSHA: "cafe", BaseBranch: "main"}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	s := string(data)
	if !strings.Contains(s, `"scm": "gitlab"`) || !strings.Contains(s, `"pr": "7"`) {
		t.Fatalf("unexpected %s", s)
	}
}

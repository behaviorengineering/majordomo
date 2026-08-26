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
	t.Setenv("MAJORDOMO_CREDENTIAL__DEMO", "")

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
				{"number": 2, "head": map[string]string{"sha": "bbb"}, "base": map[string]string{"ref": "main"}},
			})
			return
		}
		w.Header().Set("Link", `<http://`+r.Host+r.URL.Path+`?state=open&per_page=100&page=2>; rel="next"`)
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"number": 1, "head": map[string]string{"sha": "aaa"}, "base": map[string]string{"ref": "main"}},
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
			{"iid": 9, "sha": "deadbeef", "target_branch": "main"},
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
}

func TestParseClonePathNested(t *testing.T) {
	path := parseClonePath("https://gitlab.com/acme/team/demo.git")
	owner, name := splitOwnerName(path)
	if path != "acme/team/demo" || owner != "acme/team" || name != "demo" {
		t.Fatalf("path=%q owner=%q name=%q", path, owner, name)
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

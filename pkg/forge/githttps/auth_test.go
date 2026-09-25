package githttps

import (
	"encoding/base64"
	"testing"
)

func TestExtraHeaderArgsEmpty(t *testing.T) {
	t.Parallel()
	if got := ExtraHeaderArgs("", "github"); got != nil {
		t.Fatalf("empty token: got %#v", got)
	}
	if got := ExtraHeaderArgs("   ", "github"); got != nil {
		t.Fatalf("whitespace token: got %#v", got)
	}
}

func TestExtraHeaderArgsGitHubBasic(t *testing.T) {
	t.Parallel()
	got := ExtraHeaderArgs("tok", "github")
	wantBasic := base64.StdEncoding.EncodeToString([]byte("x-access-token:tok"))
	want := []string{"-c", "http.extraHeader=Authorization: Basic " + wantBasic}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("github: got %#v want %#v", got, want)
	}
	// Default SCM is github.
	gotDef := ExtraHeaderArgs("tok", "")
	if gotDef[1] != want[1] {
		t.Fatalf("default scm: got %q want %q", gotDef[1], want[1])
	}
}

func TestExtraHeaderArgsGitLab(t *testing.T) {
	t.Parallel()
	got := ExtraHeaderArgs("glpat", "gitlab")
	wantBasic := base64.StdEncoding.EncodeToString([]byte("oauth2:glpat"))
	want := []string{"-c", "http.extraHeader=Authorization: Basic " + wantBasic}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("gitlab: got %#v want %#v", got, want)
	}
}

func TestExtraHeaderArgsBitbucket(t *testing.T) {
	t.Parallel()
	got := ExtraHeaderArgs("bb", "bitbucket")
	wantBasic := base64.StdEncoding.EncodeToString([]byte("x-token-auth:bb"))
	want := "http.extraHeader=Authorization: Basic " + wantBasic
	if len(got) != 2 || got[1] != want {
		t.Fatalf("bitbucket: got %#v want header %q", got, want)
	}
}

func TestInferSCM(t *testing.T) {
	t.Parallel()
	cases := []struct {
		url  string
		want string
	}{
		{"https://github.com/o/r.git", "github"},
		{"https://gitlab.com/o/r.git", "gitlab"},
		{"https://bitbucket.org/o/r.git", "bitbucket"},
		{"https://gitlab.example.com/o/r.git", "gitlab"},
	}
	for _, tc := range cases {
		if got := InferSCM(tc.url); got != tc.want {
			t.Fatalf("InferSCM(%q)=%q want %q", tc.url, got, tc.want)
		}
	}
}

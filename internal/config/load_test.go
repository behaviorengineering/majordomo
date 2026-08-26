package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAllAndCredentialEnv(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "_defaults.yaml"), []byte(`
trigger:
  poll: true
  interval: 5m
review:
  publishMode: auto
  enableContinuousRuns: false
`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "payments-api.yaml"), []byte(`
scm: github
repository:
  owner: acme
  name: payments-api
  cloneUrl: https://github.com/acme/payments-api.git
trigger:
  poll: true
`), 0o644)
	all, err := LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Repository.ID != "payments-api" {
		t.Fatalf("%#v", all)
	}
	if got := CredentialEnvName("payments-api"); got != "MAJORDOMO_CREDENTIAL_PAYMENTS_API" {
		t.Fatalf("got %s", got)
	}
	cfg := all[0]
	if cfg.EffectivePublishMode() != "auto" {
		t.Fatalf("publishMode=%q", cfg.EffectivePublishMode())
	}
	if cfg.Review.ContinuousRunsEnabled() {
		t.Fatal("expected continuous runs false from defaults")
	}
}

func TestReviewMergeAndEffectivePublishMode(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "_defaults.yaml"), []byte(`
review:
  publishMode: auto
  enableContinuousRuns: false
`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "demo.yaml"), []byte(`
scm: github
repository:
  owner: acme
  name: demo
review:
  publishMode: comment
  enableContinuousRuns: true
`), 0o644)
	all, err := LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := all[0]
	if cfg.EffectivePublishMode() != "comment" {
		t.Fatalf("got %q", cfg.EffectivePublishMode())
	}
	if !cfg.Review.ContinuousRunsEnabled() {
		t.Fatal("expected continuous true from override")
	}
}

func TestLegacyTopLevelPublishMode(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "_defaults.yaml"), []byte("{}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "demo.yaml"), []byte(`
scm: github
repository:
  owner: acme
  name: demo
publishMode: description
`), 0o644)
	all, err := LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if all[0].EffectivePublishMode() != "description" {
		t.Fatalf("got %q", all[0].EffectivePublishMode())
	}
}

func TestOrgCredentialEnvName(t *testing.T) {
	if got := OrgCredentialEnvName("github", "xynova"); got != "GH_TOKEN_XYNOVA" {
		t.Fatalf("github: %s", got)
	}
	if got := OrgCredentialEnvName("gitlab", "behaviorengineering"); got != "GITLAB_TOKEN_BEHAVIORENGINEERING" {
		t.Fatalf("gitlab: %s", got)
	}
	if got := OrgCredentialEnvName("gitlab", "acme/team"); got != "GITLAB_TOKEN_ACME_TEAM" {
		t.Fatalf("nested: %s", got)
	}
}

func TestResolveCredentialOrder(t *testing.T) {
	t.Setenv("MAJORDOMO_CREDENTIAL_DEMO", "")
	t.Setenv("GH_TOKEN_ACME", "")
	t.Setenv("GITLAB_TOKEN_ACME", "")

	if got := ResolveCredential("demo", "github", "acme"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	t.Setenv("GH_TOKEN_ACME", "org-tok")
	if got := ResolveCredential("demo", "github", "acme"); got != "org-tok" {
		t.Fatalf("org: %q", got)
	}

	t.Setenv("MAJORDOMO_CREDENTIAL_DEMO", "repo-tok")
	if got := ResolveCredential("demo", "github", "acme"); got != "repo-tok" {
		t.Fatalf("per-repo override: %q", got)
	}

	// Unqualified tokens must not be used.
	t.Setenv("MAJORDOMO_CREDENTIAL_DEMO", "")
	t.Setenv("GH_TOKEN_ACME", "")
	t.Setenv("GITHUB_TOKEN", "bare-gh")
	t.Setenv("GH_TOKEN", "bare-gh2")
	if got := ResolveCredential("demo", "github", "acme"); got != "" {
		t.Fatalf("must ignore unqualified github tokens, got %q", got)
	}

	t.Setenv("GITLAB_TOKEN", "bare-gl")
	t.Setenv("PRIVATE_TOKEN", "bare-priv")
	if got := ResolveCredential("demo", "gitlab", "acme"); got != "" {
		t.Fatalf("must ignore unqualified gitlab tokens, got %q", got)
	}
}

func TestCredentialHint(t *testing.T) {
	got := CredentialHint("consilium", "gitlab", "behaviorengineering")
	want := "MAJORDOMO_CREDENTIAL_CONSILIUM or GITLAB_TOKEN_BEHAVIORENGINEERING"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

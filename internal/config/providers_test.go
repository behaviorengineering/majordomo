package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTaskProviderAndMerge(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "_defaults.yaml"), []byte(`
ai_providers:
  digest_default:
    api_schema: openai
    api_key: "local"
    model: "@cf/google/gemma-4-26b-a4b-it"
    base_url: "http://127.0.0.1:1320/v1"
    timeout: "60s"
  review_default:
    api_schema: openai
    api_key: gateway
    model: claude-sonnet-4-20250514
    base_url: gateway
job_configs:
  context-digest:
    modules:
      bootstrap_story:
        provider: digest_default
      digest_story:
        provider: digest_default
  pr-review:
    modules:
      filereview:
        provider: review_default
`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "demo.yaml"), []byte(`
scm: github
repository:
  owner: acme
  name: demo
ai_providers:
  digest_strong:
    api_schema: openai
    api_key: "local"
    model: "@cf/google/gemma-4-26b-a4b-it"
    base_url: "http://127.0.0.1:1320/v1"
job_configs:
  context-digest:
    modules:
      bootstrap_story:
        provider: digest_strong
`), 0o644)

	cfg, err := LoadMerged(dir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	p, ok, err := cfg.ResolveTaskProvider("bootstrap_story")
	if err != nil || !ok {
		t.Fatalf("bootstrap_story: ok=%v err=%v", ok, err)
	}
	if p.Model != "@cf/google/gemma-4-26b-a4b-it" || p.BaseURL != "http://127.0.0.1:1320/v1" {
		t.Fatalf("unexpected bootstrap provider: %#v", p)
	}
	if !cfg.AIProviders["review_default"].UsesEmbeddedGateway() {
		t.Fatal("expected review_default to target embedded gateway")
	}
	if cfg.AIProviders["digest_default"].UsesEmbeddedGateway() {
		t.Fatal("digest_default should not use embedded gateway")
	}
	p2, ok, err := cfg.ResolveTaskProvider("digest_story")
	if err != nil || !ok {
		t.Fatalf("digest_story: ok=%v err=%v", ok, err)
	}
	if p2.Model != "@cf/google/gemma-4-26b-a4b-it" {
		t.Fatalf("digest_story model=%q", p2.Model)
	}
	p3, ok, err := cfg.ResolveTaskProvider("filereview")
	if err != nil || !ok {
		t.Fatalf("filereview: ok=%v err=%v", ok, err)
	}
	if !p3.UsesEmbeddedGateway() {
		t.Fatalf("filereview should use gateway, got %#v", p3)
	}
	_, ok, err = cfg.ResolveTaskProvider("summary")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("summary should have no configured provider")
	}
}

func TestGetAIProviderExpandsEnv(t *testing.T) {
	t.Setenv("TEST_POLY_KEY", "secret-value")
	cfg := RepoConfig{
		AIProviders: map[string]AIProviderConfig{
			"poly": {
				APISchema: "openai",
				APIKey:    "${TEST_POLY_KEY}",
				Model:     "m1",
				BaseURL:   "http://127.0.0.1:1320/v1",
			},
		},
	}
	p, err := cfg.GetAIProvider("poly")
	if err != nil {
		t.Fatal(err)
	}
	if p.APIKey != "secret-value" {
		t.Fatalf("api_key=%q", p.APIKey)
	}
	strop := p.ToStrop()
	if strop.APISchema != "openai" || strop.Model != "m1" {
		t.Fatalf("%#v", strop)
	}
}

func TestGetAIProviderFailsUnresolvedKey(t *testing.T) {
	t.Setenv("MISSING_PROVIDER_KEY", "")
	cfg := RepoConfig{
		AIProviders: map[string]AIProviderConfig{
			"poly": {
				APISchema: "openai",
				APIKey:    "${MISSING_PROVIDER_KEY}",
				Model:     "m1",
				BaseURL:   "http://127.0.0.1:1320/v1",
			},
		},
	}
	_, err := cfg.GetAIProvider("poly")
	if err == nil {
		t.Fatal("expected unresolved key error")
	}
}

func TestJobForTask(t *testing.T) {
	if JobForTask("bootstrap_story") != JobContextDigest {
		t.Fatal("bootstrap")
	}
	if JobForTask("typology_inspect") != JobContextDigest {
		t.Fatal("typology_inspect")
	}
	if JobForTask("typology_human_intervention") != JobContextDigest {
		t.Fatal("typology_human_intervention")
	}
	if JobForTask("typology_finding_comment") != JobContextDigest {
		t.Fatal("typology_finding_comment")
	}
	if JobForTask("typology_intervention_brief") != JobContextDigest {
		t.Fatal("typology_intervention_brief")
	}
	if JobForTask("filereview") != JobPRReview {
		t.Fatal("filereview")
	}
	if JobForTask("unknown") != "" {
		t.Fatal("unknown")
	}
}

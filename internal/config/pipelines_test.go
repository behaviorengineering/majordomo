package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/staging"
)

func TestPipelinesAndStaticAnalysisLoadMerge(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "_defaults.yaml"), []byte(`
pipelines:
  pr-review:
    model: default-model
    routing:
      pr-review-docs:
        - "**/*.md"
      pr-review-code:
        - "**"
staticAnalysis:
  - tool: ruff
    image: registry/sa-ruff:1
    command: check
    glob: "**/*.py"
`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "demo.yaml"), []byte(`
scm: github
repository:
  owner: acme
  name: demo
pipelines:
  pr-review:
    model: override-model
    agentContext:
      global:
        customRules:
          - "no secrets"
    routing:
      pr-review-conf:
        - "**/*.yml"
      pr-review-code:
        - "**"
staticAnalysis:
  - tool: eslint
    image: registry/sa-eslint:1
    command: lint
    glob: "**/*.js"
`), 0o644)

	cfg, err := LoadMerged(dir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	pipe, ok := cfg.PipelineNamed("pr-review")
	if !ok {
		t.Fatal("missing pipeline")
	}
	if pipe.Model != "override-model" {
		t.Fatalf("model=%q", pipe.Model)
	}
	if pipe.Routing.Empty() || pipe.Routing.Keys[0] != "pr-review-conf" {
		t.Fatalf("routing keys=%v", pipe.Routing.Keys)
	}
	if pipe.AgentContext == nil {
		t.Fatal("expected agentContext")
	}
	if len(cfg.StaticAnalysis) != 1 || cfg.StaticAnalysis[0].Tool != "eslint" {
		t.Fatalf("staticAnalysis replaced: %#v", cfg.StaticAnalysis)
	}
}

func TestMaterializePrepRoundTrip(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "_defaults.yaml"), []byte("{}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "demo.yaml"), []byte(`
scm: github
repository:
  owner: acme
  name: demo
pipelines:
  pr-review:
    routing:
      pr-review-docs:
        - "**/*.md"
      pr-review-code:
        globs:
          - "**"
        persona: agents/personas/pr-review-code.persona.md
    agentContext:
      global:
        customRules:
          - "keep it simple"
      scoped: {}
`), 0o644)
	cfg, err := LoadMerged(dir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "mat")
	mat, err := MaterializePrep(cfg, "pr-review", outDir)
	if err != nil {
		t.Fatal(err)
	}
	if mat.RoutingPath == "" || mat.AgentContextPath == "" {
		t.Fatalf("%#v", mat)
	}
	rules, personas, err := staging.LoadRouting(mat.RoutingPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 || rules[0].Agent != "pr-review-docs" {
		t.Fatalf("rules=%#v", rules)
	}
	if personas["pr-review-code"] == "" {
		t.Fatalf("persona missing: %#v", personas)
	}
	ctx, err := staging.LoadAgentContextConfig(mat.AgentContextPath)
	if err != nil {
		t.Fatal(err)
	}
	rulesAny, _ := ctx.Global["customRules"].([]any)
	if len(rulesAny) != 1 {
		t.Fatalf("agent context %#v", ctx.Global)
	}
}

func TestResolveSAToolSlugAndImage(t *testing.T) {
	slug := ResolveSAToolSlug(StaticAnalysisTool{Dockerfile: "dockerfiles/sa-tools/ruff.Dockerfile"})
	if slug != "ruff" {
		t.Fatalf("slug=%q", slug)
	}
	t.Setenv("MAJORDOMO_SA_IMAGE_PREFIX", "")
	t.Setenv("MAJORDOMO_SA_IMAGE_TAG", "abc")
	img := ResolveSAImage(StaticAnalysisTool{Tool: "ruff"}, "ghcr.io/org")
	if img != "ghcr.io/org/sa-ruff:abc" {
		t.Fatalf("img=%q", img)
	}
}

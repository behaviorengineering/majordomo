package contextdigest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/majordomo/internal/judge"
)

func TestLocalBootstrapSurveyRunnerFallback(t *testing.T) {
	analysis := t.TempDir()
	if err := os.WriteFile(filepath.Join(analysis, "README.md"), []byte("# Demo\n\nHello world.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(t.TempDir(), "evidence", "typology")
	runner := LocalBootstrapSurveyRunner{}
	if err := runner.Survey(context.Background(), BootstrapSurveyInput{
		AnalysisDir: analysis,
		EvidenceDir: evidence,
		SourceSHA:   "abc123",
		RepoID:      "demo",
		GeneratedAt: time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	manifest, err := contextstore.ParseTypologyManifest(filepath.Join(evidence, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Mode != contextstore.TypologyModeFallback {
		t.Fatalf("mode=%q", manifest.Mode)
	}
	if manifest.RepoID != "demo" {
		t.Fatalf("repo_id=%q", manifest.RepoID)
	}
	if _, err := os.Stat(filepath.Join(evidence, contextstore.TypologyArchitectureBriefPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(evidence, "snapshot.yaml")); !os.IsNotExist(err) {
		t.Fatalf("snapshot should not exist in fallback path: %v", err)
	}
}

func TestLocalBootstrapSurveyRunnerReusesExistingCatalog(t *testing.T) {
	analysis := t.TempDir()
	if err := os.WriteFile(filepath.Join(analysis, "README.md"), []byte("# Demo\n\nHello world.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(analysis, "go.mod"), []byte("module example.com/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(analysis, ".typology"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(analysis, ".typology", "typology.yaml"), []byte(`id: demo
slices:
  - id: demo
    objective: Demo bounded context for stub survey.
    owns:
      - id: demo-core
        path: internal/demo
`), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := writeTypologyStub(t)
	evidence := filepath.Join(t.TempDir(), "evidence", "typology")
	runner := LocalBootstrapSurveyRunner{}
	if err := runner.Survey(context.Background(), BootstrapSurveyInput{
		AnalysisDir:    analysis,
		EvidenceDir:    evidence,
		SourceSHA:      "abc123",
		RepoID:         "demo",
		TypologyBinary: binary,
		ModuleScope:    ".",
		GeneratedAt:    time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	manifest, err := contextstore.ParseTypologyManifest(filepath.Join(evidence, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Mode != contextstore.TypologyModeReuse {
		t.Fatalf("mode=%q", manifest.Mode)
	}
	if manifest.RepoID != "demo" {
		t.Fatalf("repo_id=%q", manifest.RepoID)
	}
	if manifest.TypologyVersion != "typology v9.9.9" {
		t.Fatalf("version=%q", manifest.TypologyVersion)
	}
	if manifest.RefineStatus != contextstore.TypologyRefinePending {
		t.Fatalf("refine_status=%q", manifest.RefineStatus)
	}
	draft, err := os.ReadFile(filepath.Join(analysis, "tmp", "typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(draft), "id: demo") {
		t.Fatalf("draft=%s", draft)
	}
	if _, err := os.Stat(filepath.Join(evidence, "graph.txt")); err != nil {
		t.Fatal(err)
	}
	contracts, err := os.ReadFile(filepath.Join(evidence, "package_contracts.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contracts), "hasMain: false") {
		t.Fatalf("contracts=%s", contracts)
	}
	if manifest.PackageContractsPath != "package_contracts.md" {
		t.Fatalf("package_contracts_path=%q", manifest.PackageContractsPath)
	}
	if manifest.PackageRolesPath != "package_roles.yaml" {
		t.Fatalf("package_roles_path=%q", manifest.PackageRolesPath)
	}
	roles, err := os.ReadFile(filepath.Join(evidence, "package_roles.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(roles), "packages:") {
		t.Fatalf("roles=%s", roles)
	}
	if _, err := os.Stat(filepath.Join(evidence, "draft_snapshot.yaml")); !os.IsNotExist(err) {
		t.Fatalf("draft snapshot must not be committed evidence: %v", err)
	}
	architecture, err := os.ReadFile(filepath.Join(analysis, "tmp", "typology", "architecture_draft.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(architecture), "Typology Architecture") {
		t.Fatalf("architecture=%s", architecture)
	}
}

func TestBootstrapContextBranchWritesLLMStory(t *testing.T) {
	served := initServedRemote(t)
	cfgDir, workDir := writeDigestConfig(t, served.remote, "demo")
	_ = cfgDir
	ctxDir := t.TempDir()
	at := time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC)
	branch := config.ContextBranch("demo")
	if err := seedOrphan(ctxDir, branch, "demo", served.head, at, "", "github", served.remote); err != nil {
		t.Fatal(err)
	}
	fakeGen := bootstrapStoryGeneratorFunc(func(_ context.Context, input BootstrapStoryInput) (BootstrapStoryOutput, error) {
		if input.RepoID != "demo" {
			t.Fatalf("repo_id=%q", input.RepoID)
		}
		return BootstrapStoryOutput{
			ReadmeMD:       "# Context branch\n\nSeeded.\n",
			MissionMD:      "# Mission\n\nSeeded.\n",
			ArchitectureMD: "# Architecture\n\nSeeded.\n",
			ConventionsMD:  "# Conventions\n\nSeeded.\n",
			WeaknessesMD:   "# Weaknesses\n\nSeeded.\n",
			ChronologyMD:   "# Chronology\n\nSeeded.\n",
			GroundingMD:    "# Overview\n\nSeeded.\n",
		}, nil
	})
	fakeRefine := typologyRefineGeneratorFunc(func(_ context.Context, _ TypologyRefineInput) (TypologyRefineOutput, error) {
		t.Fatal("fallback survey must skip refine")
		return TypologyRefineOutput{}, nil
	})
	if err := bootstrapContextBranch(ctxDir, Options{
		WorkDir:                 workDir,
		BootstrapSurveyPolicy:   "auto",
		BootstrapStoryGenerator: fakeGen,
		TypologyRefineGenerator: fakeRefine,
	}, "demo", served.head, at); err != nil {
		t.Fatal(err)
	}
	if err := contextstore.ValidateTree(ctxDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ctxDir, "evidence", "typology", "manifest.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ctxDir, "evidence", "typology", "README.md")); err != nil {
		t.Fatal(err)
	}
	assertFileContains(t, filepath.Join(ctxDir, "README.md"), "## Reading order")
	assertFileContains(t, filepath.Join(ctxDir, "mission.md"), "Seeded.")
	assertFileContains(t, filepath.Join(ctxDir, "agenting", "overview", "GROUNDING.md"), "Seeded.")
}

func TestJudgeBootstrapStoryGeneratorFailsClosedWithoutLLM(t *testing.T) {
	judge.ResetRegistryForTests()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENERATIVE_AI_API_KEY", "")
	_, err := JudgeBootstrapStoryGenerator{}.Generate(context.Background(), BootstrapStoryInput{})
	if err == nil {
		t.Fatal("expected LLM readiness error")
	}
}

type bootstrapStoryGeneratorFunc func(context.Context, BootstrapStoryInput) (BootstrapStoryOutput, error)

func (f bootstrapStoryGeneratorFunc) Generate(ctx context.Context, input BootstrapStoryInput) (BootstrapStoryOutput, error) {
	return f(ctx, input)
}

type typologyRefineGeneratorFunc func(context.Context, TypologyRefineInput) (TypologyRefineOutput, error)

func (f typologyRefineGeneratorFunc) Refine(ctx context.Context, input TypologyRefineInput) (TypologyRefineOutput, error) {
	return f(ctx, input)
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %s", path, data)
	}
}

func TestPilotGitboardDraftFreeSurvey(t *testing.T) {
	analysis := strings.TrimSpace(os.Getenv("MAJORDOMO_PILOT_ANALYSIS"))
	typo := strings.TrimSpace(os.Getenv("MAJORDOMO_PILOT_TYPOLOGY"))
	if analysis == "" || typo == "" {
		t.Skip("set MAJORDOMO_PILOT_ANALYSIS and MAJORDOMO_PILOT_TYPOLOGY to run pilot survey")
	}
	evidence := filepath.Join(t.TempDir(), "evidence", "typology")
	runner := LocalBootstrapSurveyRunner{}
	if err := runner.Survey(context.Background(), BootstrapSurveyInput{
		AnalysisDir:    analysis,
		EvidenceDir:    evidence,
		SourceSHA:      "deadbeef",
		RepoID:         "gitboard",
		TypologyBinary: typo,
		GeneratedAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"draft_snapshot.yaml", "architecture_draft.md"} {
		if _, err := os.Stat(filepath.Join(evidence, banned)); !os.IsNotExist(err) {
			t.Fatalf("banned evidence file %s: %v", banned, err)
		}
	}
	for _, required := range []string{"graph.txt", "manifest.yaml"} {
		if _, err := os.Stat(filepath.Join(evidence, required)); err != nil {
			t.Fatalf("%s: %v", required, err)
		}
	}
	if _, err := os.Stat(filepath.Join(analysis, "tmp", "typology", "typology.yaml")); err != nil {
		t.Fatalf("ephemeral draft missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(analysis, "tmp", "typology", "architecture_draft.md")); err != nil {
		t.Fatalf("ephemeral architecture draft missing: %v", err)
	}
	manifest, err := contextstore.ParseTypologyManifest(filepath.Join(evidence, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.RefineStatus != contextstore.TypologyRefinePending {
		t.Fatalf("refine_status=%q", manifest.RefineStatus)
	}
}

func TestPilotSanitizeRefinedSurfaces(t *testing.T) {
	path := strings.TrimSpace(os.Getenv("MAJORDOMO_PILOT_REFINED"))
	if path == "" {
		t.Skip("set MAJORDOMO_PILOT_REFINED to run pilot sanitize")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := validateRefinedCatalogYAML(string(raw), string(raw), "demo", "")
	if err != nil {
		t.Fatal(err)
	}
	if outPath := strings.TrimSpace(os.Getenv("MAJORDOMO_PILOT_SANITIZED_OUT")); outPath != "" {
		if err := os.WriteFile(outPath, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lower := strings.ToLower(out)
	if strings.Contains(string(raw), "cmd/") || strings.Contains(string(raw), "dashboard") {
		if !strings.Contains(lower, "surfaces:") {
			t.Fatalf("expected surfaces after sanitize when interaction paths exist, got:\n%s", out)
		}
	}
	ok, feedback := evaluateTypologyBoundaries(out, "# Journey\n\n## Technical debt & boundary violations\n\n| Violation | Severity | Notes |\n| --- | --- | --- |\n| sample | low | recorded |\n", "## Findings\n\n- sample finding\n", "")
	if !ok {
		t.Fatalf("boundary eval after sanitize: %s", feedback)
	}
}

func writeTypologyStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "typology")
	script := `#!/bin/sh
set -eu
cmd="$1"
case "$cmd" in
  version)
    echo "typology v9.9.9"
    ;;
  discover)
    repo="$2"
    mkdir -p "$repo/tmp/typology"
    cat > "$repo/tmp/typology/typology.yaml" <<'EOF'
id: demo
slices:
  - id: demo
    objective: Demo bounded context for stub survey.
    owns:
      - id: demo-core
        path: internal/demo
EOF
    ;;
  contracts)
    out=""
    prev=""
    repo="$2"
    for arg in "$@"; do
      if [ "$prev" = "--out" ]; then
        out="$arg"
      fi
      prev="$arg"
    done
    if [ -z "$out" ]; then
      out="$repo/tmp/typology/package_contracts.md"
    fi
    mkdir -p "$(dirname "$out")"
    cat > "$out" <<'EOF'
# Package public contracts

## ./internal/demo
- package: demo
- hasMain: false
- role: unknown
- confidence: 0.00
- exportedDecls: (none)
- exportedFuncs: Run
EOF
    mkdir -p "$repo/tmp/typology"
    cat > "$repo/tmp/typology/package_roles.yaml" <<'EOF'
packages:
  - path: internal/demo
    role: unknown
    confidence: 0
    inspected_stage: 2
edges: []
EOF
    cat > "$repo/tmp/typology/package_rlm_context.md" <<'EOF'
# Package RLM context index

## ./internal/demo
- mechanicalRole: unknown
EOF
    ;;
  show)
    echo "graph: demo"
    ;;
  architecture)
    out=""
    prev=""
    for arg in "$@"; do
      if [ "$prev" = "--out" ]; then
        out="$arg"
      fi
      prev="$arg"
    done
    repo="$2"
    body="# Typology Architecture

Demo architecture brief."
    if [ -n "$out" ]; then
      mkdir -p "$(dirname "$out")"
      printf '%s\n' "$body" > "$out"
    else
      mkdir -p "$repo/docs/architecture"
      printf '%s\n' "$body" > "$repo/docs/architecture/typology.md"
    fi
    ;;
  *)
    echo "unexpected command: $cmd" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscoverSurveyRoots(t *testing.T) {
	t.Parallel()

	t.Run("neither", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		roots, err := discoverSurveyRoots(dir)
		if err != nil {
			t.Fatal(err)
		}
		if roots.HasGo || roots.HasPython {
			t.Fatalf("roots=%+v want neither", roots)
		}
	})

	t.Run("go_only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.22\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		roots, err := discoverSurveyRoots(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !roots.HasGo || roots.HasPython {
			t.Fatalf("roots=%+v want go only", roots)
		}
	})

	t.Run("python_only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname=\"x\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		roots, err := discoverSurveyRoots(dir)
		if err != nil {
			t.Fatal(err)
		}
		if roots.HasGo || !roots.HasPython {
			t.Fatalf("roots=%+v want python only", roots)
		}
	})
}

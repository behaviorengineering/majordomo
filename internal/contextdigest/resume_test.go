package contextdigest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/majordomo/internal/judge"
	"github.com/behaviorengineering/strop/evaluation"
)

func TestValidateResumeOptions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		opts    Options
		wantErr string
	}{
		{name: "disabled", opts: Options{}},
		{name: "ok refine", opts: Options{ResumePR: 42, FromStage: "refine"}},
		{name: "ok story", opts: Options{ResumePR: 7, FromStage: "STORY"}},
		{name: "missing pr", opts: Options{FromStage: "story"}, wantErr: "--resume-pr"},
		{name: "missing stage", opts: Options{ResumePR: 1}, wantErr: "--from-stage"},
		{name: "bad stage", opts: Options{ResumePR: 1, FromStage: "survey"}, wantErr: "unsupported"},
		{name: "skip story conflict", opts: Options{ResumePR: 1, FromStage: "story", SkipStory: true}, wantErr: "--skip-story"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateResumeOptions(tc.opts)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateResumeEvidencePerStage(t *testing.T) {
	t.Parallel()

	t.Run("refine ok", func(t *testing.T) {
		t.Parallel()
		ctxDir := writeResumeEvidenceFixture(t, resumeFixtureKindRefine)
		if err := validateResumeEvidence(ctxDir, ResumeStageRefine); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("refine missing graph", func(t *testing.T) {
		t.Parallel()
		ctxDir := writeResumeEvidenceFixture(t, resumeFixtureKindRefine)
		_ = os.Remove(filepath.Join(ctxDir, "evidence", "typology", "graph.txt"))
		err := validateResumeEvidence(ctxDir, ResumeStageRefine)
		if err == nil || !strings.Contains(err.Error(), "graph_path") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("story missing ledger fails closed", func(t *testing.T) {
		t.Parallel()
		ctxDir := writeResumeEvidenceFixture(t, resumeFixtureKindStory)
		_ = os.Remove(filepath.Join(ctxDir, "evidence", "typology", "slice_objective_ledger.yaml"))
		err := validateResumeEvidence(ctxDir, ResumeStageStory)
		if err == nil || !strings.Contains(err.Error(), "slice_objective_ledger") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("story ok", func(t *testing.T) {
		t.Parallel()
		ctxDir := writeResumeEvidenceFixture(t, resumeFixtureKindStory)
		if err := validateResumeEvidence(ctxDir, ResumeStageStory); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("intervention ok", func(t *testing.T) {
		t.Parallel()
		ctxDir := writeResumeEvidenceFixture(t, resumeFixtureKindIntervention)
		if err := validateResumeEvidence(ctxDir, ResumeStageIntervention); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRunBootstrapFromStageStorySkipsSurveyAndRefine(t *testing.T) {
	t.Parallel()
	ctxDir := writeResumeEvidenceFixture(t, resumeFixtureKindStory)
	analysis := t.TempDir()
	if err := os.WriteFile(filepath.Join(analysis, "README.md"), []byte("# Demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var surveyCalls, refineCalls, interventionCalls, storyCalls atomic.Int32
	opts := Options{
		ResumePR:  99,
		FromStage: ResumeStageStory,
		BootstrapSurveyRunner: bootstrapSurveyRunnerFunc(func(context.Context, BootstrapSurveyInput) error {
			surveyCalls.Add(1)
			return nil
		}),
		TypologyRefineGenerator: typologyRefineGeneratorFunc(func(context.Context, TypologyRefineInput) (TypologyRefineOutput, error) {
			refineCalls.Add(1)
			return TypologyRefineOutput{}, nil
		}),
		HumanInterventionGenerator: humanInterventionGeneratorFunc(func(context.Context, HumanInterventionInput) (HumanInterventionOutput, error) {
			interventionCalls.Add(1)
			return HumanInterventionOutput{
				JourneyMD:           "# Journey\n",
				HumanInterventionMD: "# HI\n",
			}, nil
		}),
		BootstrapStoryGenerator: bootstrapStoryGeneratorFunc(func(_ context.Context, in BootstrapStoryInput) (BootstrapStoryOutput, error) {
			storyCalls.Add(1)
			if !strings.Contains(in.TypologyRefinedCatalog, "id: demo") {
				t.Fatalf("refined=%q", in.TypologyRefinedCatalog)
			}
			return BootstrapStoryOutput{
				ReadmeMD:       "# README\n",
				MissionMD:      "# Mission\n",
				ArchitectureMD: "# Architecture\n\nDemo bounded context.\n",
				ConventionsMD:  "# Conventions\n",
				WeaknessesMD:   "# Weaknesses\n",
				ChronologyMD:   "# Chronology\n",
				GroundingMD:    "# Grounding\n",
			}, nil
		}),
	}

	at := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	if err := runBootstrapFromStage(context.Background(), ctxDir, analysis, "abc123", at, opts, ResumeStageStory); err != nil {
		t.Fatal(err)
	}
	if surveyCalls.Load() != 0 {
		t.Fatalf("survey calls=%d want 0", surveyCalls.Load())
	}
	if refineCalls.Load() != 0 {
		t.Fatalf("refine calls=%d want 0", refineCalls.Load())
	}
	if interventionCalls.Load() != 0 {
		t.Fatalf("intervention calls=%d want 0", interventionCalls.Load())
	}
	if storyCalls.Load() != 1 {
		t.Fatalf("story calls=%d want 1", storyCalls.Load())
	}
}

func TestRunBootstrapFromStageInterventionThenStory(t *testing.T) {
	t.Parallel()
	ctxDir := writeResumeEvidenceFixture(t, resumeFixtureKindIntervention)
	analysis := t.TempDir()
	if err := os.WriteFile(filepath.Join(analysis, "README.md"), []byte("# Demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var refineCalls, interventionCalls, storyCalls atomic.Int32
	opts := Options{
		ResumePR:  12,
		FromStage: ResumeStageIntervention,
		TypologyRefineGenerator: typologyRefineGeneratorFunc(func(context.Context, TypologyRefineInput) (TypologyRefineOutput, error) {
			refineCalls.Add(1)
			return TypologyRefineOutput{}, nil
		}),
		HumanInterventionGenerator: humanInterventionGeneratorFunc(func(context.Context, HumanInterventionInput) (HumanInterventionOutput, error) {
			interventionCalls.Add(1)
			return HumanInterventionOutput{
				JourneyMD:           "# Journey\n\nUpdated.\n",
				HumanInterventionMD: "# Human intervention\n",
				WeaknessesSeedMD:    "# Weaknesses\n",
			}, nil
		}),
		BootstrapStoryGenerator: bootstrapStoryGeneratorFunc(func(context.Context, BootstrapStoryInput) (BootstrapStoryOutput, error) {
			storyCalls.Add(1)
			return BootstrapStoryOutput{
				ReadmeMD:       "# README\n",
				MissionMD:      "# Mission\n",
				ArchitectureMD: "# Architecture\n\nDemo bounded context.\n",
				ConventionsMD:  "# Conventions\n",
				WeaknessesMD:   "# Weaknesses\n",
				ChronologyMD:   "# Chronology\n",
				GroundingMD:    "# Grounding\n",
			}, nil
		}),
	}
	at := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	if err := runBootstrapFromStage(context.Background(), ctxDir, analysis, "abc123", at, opts, ResumeStageIntervention); err != nil {
		t.Fatal(err)
	}
	if refineCalls.Load() != 0 {
		t.Fatalf("refine calls=%d want 0", refineCalls.Load())
	}
	if interventionCalls.Load() != 1 {
		t.Fatalf("intervention calls=%d want 1", interventionCalls.Load())
	}
	if storyCalls.Load() != 1 {
		t.Fatalf("story calls=%d want 1", storyCalls.Load())
	}
}

func TestStageAnalysisDraftsFromEvidence(t *testing.T) {
	t.Parallel()
	ctxDir := writeResumeEvidenceFixture(t, resumeFixtureKindRefine)
	evidence := filepath.Join(ctxDir, "evidence", "typology")
	manifest, err := contextstore.ParseTypologyManifest(filepath.Join(evidence, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	analysis := t.TempDir()
	if err := stageAnalysisDraftsFromEvidence(analysis, evidence, manifest); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tmp/typology/typology.yaml", "tmp/typology/architecture_draft.md"} {
		if _, err := os.Stat(filepath.Join(analysis, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestRunResumeFromPRLocalOnly(t *testing.T) {
	ctxDirSrc := writeResumeEvidenceFixture(t, resumeFixtureKindStory)
	workStory := t.TempDir()
	analysisSrc := t.TempDir()
	if err := os.WriteFile(filepath.Join(analysisSrc, "README.md"), []byte("# Gitboard\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	served := t.TempDir()
	sg := &Git{Dir: served}
	if _, err := sg.run("init"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(served, "README.md"), []byte("# Gitboard\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := configureCommitIdentity(sg); err != nil {
		t.Fatal(err)
	}
	if _, err := sg.run("add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := sg.run("commit", "-m", "init"); err != nil {
		t.Fatal(err)
	}
	bare := t.TempDir()
	if _, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatal(err)
	}
	if _, err := sg.run("remote", "add", "origin", bare); err != nil {
		t.Fatal(err)
	}

	prev := materializePRHeadForResume
	materializePRHeadForResume = func(dir string, _ *Git, _ *Forge, _ string, _ PRHead, _, _ string) error {
		if err := copyTree(ctxDirSrc, dir); err != nil {
			return err
		}
		g := &Git{Dir: dir}
		if _, err := g.run("init"); err != nil {
			return err
		}
		if err := configureCommitIdentity(g); err != nil {
			return err
		}
		if _, err := g.run("add", "-A"); err != nil {
			return err
		}
		_, err := g.run("commit", "-m", "fixture pr head")
		return err
	}
	t.Cleanup(func() { materializePRHeadForResume = prev })

	prevClone := cloneAnalysisRepoFn
	cloneAnalysisRepoFn = func(_ context.Context, _ string) (string, error) {
		dst := t.TempDir()
		if err := copyTree(analysisSrc, dst); err != nil {
			return "", err
		}
		return dst, nil
	}
	t.Cleanup(func() { cloneAnalysisRepoFn = prevClone })

	var storyCalls atomic.Int32
	forge := &Forge{
		SCM: "github", Owner: "acme", Name: "demo", Token: "t", RepoID: "demo",
		Runner: func(name string, args []string, _ []string) (string, error) {
			if name == "gh" && strings.Contains(strings.Join(args, " "), "pr view") {
				return `{"headRefOid":"abcdef0123456789","headRefName":"majordomo-context/demo-update"}`, nil
			}
			t.Fatalf("unexpected CLI %s %v", name, args)
			return "", nil
		},
	}
	opts := Options{
		RepoID:       "demo",
		WorkDir:      served,
		ResumePR:     41,
		FromStage:    ResumeStageStory,
		WorkStoryDir: workStory,
		Judge:        resumeNoopJudge{},
		BootstrapStoryGenerator: bootstrapStoryGeneratorFunc(func(context.Context, BootstrapStoryInput) (BootstrapStoryOutput, error) {
			storyCalls.Add(1)
			return BootstrapStoryOutput{
				ReadmeMD:       "# README\n",
				MissionMD:      "# Mission\n",
				ArchitectureMD: "# Architecture\n\nDemo bounded context.\n",
				ConventionsMD:  "# Conventions\n",
				WeaknessesMD:   "# Weaknesses\n",
				ChronologyMD:   "# Chronology\n",
				GroundingMD:    "# Grounding\n",
			}, nil
		}),
	}
	cfg := config.RepoConfig{Repository: config.Repository{ID: "demo", CloneURL: bare}}
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	ctxDir := t.TempDir()
	res, err := runResumeFromPR(opts, cfg, forge, sg, "t", "github", ctxDir, "main", "deadbeef", now, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "resume" {
		t.Fatalf("action=%q", res.Action)
	}
	if res.ResumePR != 41 || res.FromStage != ResumeStageStory {
		t.Fatalf("provenance=%+v", res)
	}
	if storyCalls.Load() != 1 {
		t.Fatalf("story calls=%d", storyCalls.Load())
	}
	if _, err := os.Stat(filepath.Join(res.LocalOutDir, "evidence", "typology", "manifest.yaml")); err != nil {
		t.Fatalf("local out missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workStory, "resume_provenance.json")); err != nil {
		t.Fatal(err)
	}
	out, _ := exec.Command("git", "-C", bare, "show-ref").CombinedOutput()
	if strings.Contains(string(out), "majordomo-context") {
		t.Fatalf("context ref pushed to bare: %s", out)
	}
}

type resumeNoopJudge struct{}

func (resumeNoopJudge) Generate(context.Context, string, map[string]interface{}, int) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (resumeNoopJudge) Evaluate(context.Context, string, map[string]interface{}, map[string]interface{}, int) (*evaluation.AggregatedEvaluation, error) {
	return &evaluation.AggregatedEvaluation{}, nil
}

func (resumeNoopJudge) Ready() bool { return true }

func (resumeNoopJudge) TaskModel(string) string { return "noop" }

var _ judge.Generator = resumeNoopJudge{}

func TestResolveGitHubPRHeadViaRunner(t *testing.T) {
	t.Parallel()
	f := &Forge{
		SCM: "github", Owner: "acme", Name: "demo", Token: "t", RepoID: "demo",
		Runner: func(name string, args []string, _ []string) (string, error) {
			if name != "gh" {
				t.Fatalf("cli=%s", name)
			}
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, "pr view 41") {
				t.Fatalf("args=%v", args)
			}
			return `{"headRefOid":"deadbeef0123456789","headRefName":"majordomo-context/demo-update"}`, nil
		},
	}
	head, err := f.ResolvePRHead("41")
	if err != nil {
		t.Fatal(err)
	}
	if head.SHA != "deadbeef0123456789" || head.RefName != "majordomo-context/demo-update" {
		t.Fatalf("head=%+v", head)
	}
}

type resumeFixtureKind int

const (
	resumeFixtureKindRefine resumeFixtureKind = iota
	resumeFixtureKindIntervention
	resumeFixtureKindStory
)

type bootstrapSurveyRunnerFunc func(context.Context, BootstrapSurveyInput) error

func (f bootstrapSurveyRunnerFunc) Survey(ctx context.Context, input BootstrapSurveyInput) error {
	return f(ctx, input)
}

func writeResumeEvidenceFixture(t *testing.T, kind resumeFixtureKind) string {
	t.Helper()
	ctxDir := t.TempDir()
	evidence := filepath.Join(ctxDir, "evidence", "typology")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(evidence, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog := `id: demo
slices:
  - id: demo
    owns:
      - path: internal/demo
`
	write("snapshot.yaml", catalog)
	write("graph.txt", "graph: demo\n")
	write("package_contracts.md", "# contracts\n")
	write("package_roles.yaml", "packages:\n  - path: internal/demo\n    role: unknown\n")
	write(contextstore.TypologyArchitectureBriefPath, "# Architecture brief\n\nDemo bounded context.\n")

	manifest := contextstore.TypologyManifest{
		RepoID:               "demo",
		SourceSHA:            "abc123",
		GeneratedAt:          time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Mode:                 contextstore.TypologyModeDiscover,
		ModuleScope:          ".",
		SnapshotPath:         "snapshot.yaml",
		ArchitecturePath:     contextstore.TypologyArchitectureBriefPath,
		RefineStatus:         contextstore.TypologyRefinePending,
		GraphPath:            "graph.txt",
		PackageContractsPath: "package_contracts.md",
		PackageRolesPath:     "package_roles.yaml",
		ClusterProposalPath:  "cluster_merge_proposal.yaml",
		RefinedSnapshotPath:  "refined_snapshot.yaml",
		JourneyPath:          "journey.md",
	}

	if kind == resumeFixtureKindIntervention || kind == resumeFixtureKindStory {
		write("refined_snapshot.yaml", catalog)
		write("journey.md", "# Journey\n\nOpen.\n")
		write("human_intervention.md", "# HI\n")
		write("cluster_merge_proposal.yaml", "[]\n")
		write("package_capability_constraints.yaml", "packages: []\n")
		write("slice_objective_claims.yaml", "slices:\n  - id: demo\n    claims: [data_shape]\n")
		write("slice_objective_ledger.yaml", `slices:
  - id: demo
    owned_paths: [internal/demo]
    evidence: [DemoType]
    claims: [data_shape]
    objective: Demo bounded context.
    verdict: grounded
`)
		manifest.RefineStatus = contextstore.TypologyRefineComplete
		manifest.PackageCapabilityConstraintsPath = "package_capability_constraints.yaml"
		manifest.SliceObjectiveClaimsPath = "slice_objective_claims.yaml"
		manifest.SliceObjectiveLedgerPath = "slice_objective_ledger.yaml"
		manifest.HumanInterventionPath = "human_intervention.md"
	}

	for _, name := range []string{"README.md", "mission.md", "architecture.md", "conventions.md", "weaknesses.md", "chronology.md"} {
		if err := os.WriteFile(filepath.Join(ctxDir, name), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(ctxDir, "agenting", "overview"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "agenting", "overview", "GROUNDING.md"), []byte("# Grounding\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctxDir, "meta.yaml"), []byte("repo_id: demo\ncursor: abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeTypologyManifest(evidence, manifest); err != nil {
		t.Fatal(err)
	}
	return ctxDir
}

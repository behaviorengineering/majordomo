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
)

func TestValidateLocalSeedOptions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		opts    Options
		wantErr string
	}{
		{name: "ok empty from-stage", opts: Options{LocalSeedDir: "/tmp/seed"}},
		{name: "ok survey", opts: Options{LocalSeedDir: "/tmp/seed", FromStage: "survey"}},
		{name: "conflict resume-pr", opts: Options{LocalSeedDir: "/tmp/seed", ResumePR: 1}, wantErr: "conflicts"},
		{name: "bad stage", opts: Options{LocalSeedDir: "/tmp/seed", FromStage: "catchup"}, wantErr: "unsupported"},
		{name: "skip story conflict", opts: Options{LocalSeedDir: "/tmp/seed", FromStage: "story", SkipStory: true}, wantErr: "--skip-story"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateLocalSeedOptions(tc.opts)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("err=%v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v want %q", err, tc.wantErr)
			}
		})
	}
}

func TestOpenLocalSeedWorkspaceIdentityAndLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	ws, err := OpenLocalSeedWorkspace(root, "demo", "abc123", ".", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.Release(); err != nil {
		t.Fatal(err)
	}

	_, err = OpenLocalSeedWorkspace(root, "other", "abc123", ".", false, now)
	if err == nil || !strings.Contains(err.Error(), "repo_id") {
		t.Fatalf("err=%v", err)
	}

	_, err = OpenLocalSeedWorkspace(root, "demo", "deadbeef", ".", false, now)
	if err == nil || !strings.Contains(err.Error(), "source_sha") {
		t.Fatalf("err=%v", err)
	}

	ws2, err := OpenLocalSeedWorkspace(root, "demo", "deadbeef", ".", true, now)
	if err != nil {
		t.Fatal(err)
	}
	if ws2.Manifest.SourceSHA != "deadbeef" {
		t.Fatalf("source=%q", ws2.Manifest.SourceSHA)
	}
	// Hold lock; second open must fail.
	_, err = OpenLocalSeedWorkspace(root, "demo", "deadbeef", ".", true, now)
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("err=%v", err)
	}
	_ = ws2.Release()
}

func TestLocalSeedWorkspaceCheckpointAtomic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	ws, err := OpenLocalSeedWorkspace(root, "demo", "abc", ".", false, now)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Release()
	if err := ws.BeginStage(LocalStageSurvey, now); err != nil {
		t.Fatal(err)
	}
	if err := ws.RecordFailure(LocalStageSurvey, context.Canceled, now); err != nil {
		t.Fatal(err)
	}
	if ws.Manifest.CompletedStage != "" {
		t.Fatalf("completed advanced on failure: %q", ws.Manifest.CompletedStage)
	}
	if ws.Manifest.LastError == "" {
		t.Fatal("expected last_error")
	}
	if err := ws.SaveCheckpoint(LocalStageSurvey, now); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, localWorkspaceManifest))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "completed_stage: survey") {
		t.Fatalf("manifest=%s", raw)
	}
}

func TestRunLocalSeedFreshAndResumeNoPush(t *testing.T) {
	served := t.TempDir()
	sg := &Git{Dir: served}
	if _, err := sg.run("init"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(served, "README.md"), []byte("# Demo\n"), 0o644); err != nil {
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
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("bare: %v %s", err, out)
	}
	if _, err := sg.run("remote", "add", "origin", bare); err != nil {
		t.Fatal(err)
	}

	seedDir := t.TempDir()
	configDir := writeMinimalLocalConfig(t)
	var surveyCalls, refineCalls, interventionCalls, storyCalls atomic.Int32

	opts := Options{
		ConfigDir:    configDir,
		RepoID:       "demo",
		WorkDir:      served,
		LocalSeedDir: seedDir,
		FromStage:    LocalStageSurvey,
		BootstrapSurveyRunner: bootstrapSurveyRunnerFunc(func(_ context.Context, in BootstrapSurveyInput) error {
			surveyCalls.Add(1)
			if err := os.MkdirAll(in.EvidenceDir, 0o755); err != nil {
				return err
			}
			catalog := "id: demo\nslices:\n  - id: demo\n    owns:\n      - path: internal/demo\n"
			_ = os.WriteFile(filepath.Join(in.EvidenceDir, "snapshot.yaml"), []byte(catalog), 0o644)
			_ = os.WriteFile(filepath.Join(in.EvidenceDir, "graph.txt"), []byte("g\n"), 0o644)
			_ = os.WriteFile(filepath.Join(in.EvidenceDir, "package_contracts.md"), []byte("# c\n"), 0o644)
			_ = os.WriteFile(filepath.Join(in.EvidenceDir, "package_roles.yaml"), []byte("packages: []\n"), 0o644)
			_ = os.WriteFile(filepath.Join(in.EvidenceDir, contextstore.TypologyArchitectureBriefPath), []byte("# brief\nDemo bounded context.\n"), 0o644)
			_ = os.MkdirAll(filepath.Join(in.AnalysisDir, "tmp", "typology"), 0o755)
			_ = os.WriteFile(filepath.Join(in.AnalysisDir, analysisDraftCatalogRel), []byte(catalog), 0o644)
			_ = os.WriteFile(filepath.Join(in.AnalysisDir, analysisDraftArchRel), []byte("# draft\n"), 0o644)
			manifest := contextstore.TypologyManifest{
				RepoID: "demo", SourceSHA: "x", GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Mode: contextstore.TypologyModeDiscover, SnapshotPath: "snapshot.yaml",
				ArchitecturePath: contextstore.TypologyArchitectureBriefPath,
				RefineStatus:     contextstore.TypologyRefinePending,
				GraphPath:        "graph.txt", PackageContractsPath: "package_contracts.md",
				PackageRolesPath: "package_roles.yaml", RefinedSnapshotPath: "refined_snapshot.yaml",
				JourneyPath: "journey.md", ClusterProposalPath: "slice_grouping_proposal.yaml",
			}
			return writeTypologyManifest(in.EvidenceDir, manifest)
		}),
		TypologySlicePipeline: typologySlicePipelineFunc(func(context.Context, TypologySlicePipelineInput) (TypologySlicePipelineOutput, error) {
			refineCalls.Add(1)
			return TypologySlicePipelineOutput{
				MechanicalGroupingYAML:   "entrypoint_paths: []\n",
				ClusterMergeProposalYAML: "[]\n",
				RefinedCatalogYAML:       "id: demo\nslices:\n  - id: demo\n    owns:\n      - path: internal/demo\n",
				ObjectiveLedgerYAML: `slices:
  - id: demo
    owned_paths: [internal/demo]
    evidence: [DemoType]
    claims: [data_shape]
    objective: Demo bounded context.
    verdict: grounded
`,
				ObjectiveClaimsYAML: "slices:\n  - id: demo\n    claims: [data_shape]\n",
			}, nil
		}),
		HumanInterventionGenerator: humanInterventionGeneratorFunc(func(context.Context, HumanInterventionInput) (HumanInterventionOutput, error) {
			interventionCalls.Add(1)
			return HumanInterventionOutput{JourneyMD: "# Journey\n", HumanInterventionMD: "# HI\n"}, nil
		}),
		BootstrapStoryGenerator: bootstrapStoryGeneratorFunc(func(context.Context, BootstrapStoryInput) (BootstrapStoryOutput, error) {
			storyCalls.Add(1)
			return BootstrapStoryOutput{
				ReadmeMD: "# README\n", MissionMD: "# Mission\n",
				ArchitectureMD: "# Architecture\n\nDemo bounded context.\n",
				ConventionsMD:  "# Conventions\n", WeaknessesMD: "# Weaknesses\n",
				ChronologyMD: "# Chronology\n", GroundingMD: "# Grounding\n",
			}, nil
		}),
		Judge:          resumeNoopJudge{},
		TypologyBinary: writeTypologyStub(t),
	}

	// Force survey-only first by skipping story and injecting stop after survey via from-stage survey + SkipStory
	// and a refine that we won't reach: use from-stage survey with SkipStory and BootstrapSurveyPolicy never?
	// Better: run with stubs but stop after survey by setting FromStage survey and a custom path.
	// runLocalStages with fromStage=survey runs survey+refine+story. To test resume, run full once then
	// second call with from-stage story after manually setting checkpoint.

	cfg := config.RepoConfig{Repository: config.Repository{ID: "demo"}}
	now := time.Date(2026, 9, 17, 2, 0, 0, 0, time.UTC)

	// First run: survey only by using policy always and interrupting after survey via SkipStory +
	// replace refine with failure? Use FromStage survey and mock refine that records then we save checkpoint mid-way.
	opts.SkipStory = true
	opts.TypologySlicePipeline = typologySlicePipelineFunc(func(context.Context, TypologySlicePipelineInput) (TypologySlicePipelineOutput, error) {
		refineCalls.Add(1)
		return TypologySlicePipelineOutput{}, context.Canceled
	})
	_, err := runLocalSeed(opts, cfg, now)
	if err == nil {
		t.Fatal("expected refine cancel")
	}
	if surveyCalls.Load() != 1 {
		t.Fatalf("survey=%d", surveyCalls.Load())
	}
	manifestRaw, _ := os.ReadFile(filepath.Join(seedDir, localWorkspaceManifest))
	if !strings.Contains(string(manifestRaw), "completed_stage: survey") {
		t.Fatalf("expected survey checkpoint, got %s", manifestRaw)
	}
	if strings.Contains(string(manifestRaw), "completed_stage: intervention") {
		t.Fatal("intervention should not complete after refine cancel")
	}

	// Resume from refine with working refine/story stubs.
	opts.FromStage = LocalStageRefine
	opts.SkipStory = false
	opts.TypologySlicePipeline = typologySlicePipelineFunc(func(context.Context, TypologySlicePipelineInput) (TypologySlicePipelineOutput, error) {
		refineCalls.Add(1)
		return TypologySlicePipelineOutput{
			MechanicalGroupingYAML:   "entrypoint_paths: []\n",
			ClusterMergeProposalYAML: "[]\n",
			RefinedCatalogYAML:       "id: demo\nslices:\n  - id: demo\n    owns:\n      - path: internal/demo\n",
			ObjectiveLedgerYAML: `slices:
  - id: demo
    owned_paths: [internal/demo]
    evidence: [DemoType]
    claims: [data_shape]
    objective: Demo bounded context.
    verdict: grounded
`,
			ObjectiveClaimsYAML: "slices:\n  - id: demo\n    claims: [data_shape]\n",
		}, nil
	})
	// refineTypologyEvidence needs typology binary stub that writes architecture.
	res, err := runLocalSeed(opts, cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "local" || res.CacheMode != "local" {
		t.Fatalf("res=%+v", res)
	}
	if res.CompletedStage != LocalStageStory {
		t.Fatalf("completed=%q", res.CompletedStage)
	}
	if surveyCalls.Load() != 1 {
		t.Fatalf("survey rerun=%d", surveyCalls.Load())
	}
	if storyCalls.Load() != 1 {
		t.Fatalf("story=%d", storyCalls.Load())
	}
	if _, err := os.Stat(filepath.Join(seedDir, "context", "mission.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(seedDir, "inference-cache")); err != nil {
		t.Fatal(err)
	}
	out, _ := exec.Command("git", "-C", bare, "show-ref").CombinedOutput()
	if strings.Contains(string(out), "majordomo-context") || strings.Contains(string(out), "majordomo-inference-cache") {
		t.Fatalf("remote refs pushed: %s", out)
	}
}

func writeMinimalLocalConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	defaults := filepath.Join(dir, "_defaults.yaml")
	if err := os.WriteFile(defaults, []byte("scm: github\ncache:\n  disableSkips: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "demo.yaml")
	if err := os.WriteFile(repo, []byte("repository:\n  id: demo\n  cloneUrl: https://example.com/demo.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

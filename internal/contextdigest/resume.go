package contextdigest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

func isResumeMode(opts Options) bool {
	// Legacy helper: true for PR-seeded replay (not local filesystem seed).
	return isPRResumeMode(opts)
}

func normalizeResumeStage(stage string) string {
	s := strings.ToLower(strings.TrimSpace(stage))
	if s == ResumeStageRefine || s == LocalStageRefine {
		return ResumeStageCatalog
	}
	return s
}

func validateResumeOptions(opts Options) error {
	pr := opts.ResumePR
	stage := normalizeResumeStage(opts.FromStage)
	if pr <= 0 && stage == "" {
		return nil
	}
	if pr <= 0 {
		return fmt.Errorf("--resume-pr required with --from-stage (or use --local-seed-dir for filesystem seed)")
	}
	if stage == "" {
		return fmt.Errorf("--from-stage required with --resume-pr (catalog|intervention|story; refine still accepted)")
	}
	switch stage {
	case ResumeStageCatalog, ResumeStageIntervention, ResumeStageStory:
	default:
		return fmt.Errorf("unsupported --from-stage %q (want catalog|intervention|story)", opts.FromStage)
	}
	if opts.SkipStory && stage == ResumeStageStory {
		return fmt.Errorf("--skip-story conflicts with --from-stage story")
	}
	return nil
}

// ResumeProvenance records which context PR head seeded a local stage replay.
type ResumeProvenance struct {
	ResumePR  int    `json:"resume_pr"`
	HeadSHA   string `json:"head_sha"`
	FromStage string `json:"from_stage"`
	RepoID    string `json:"repo_id"`
	At        string `json:"at"`
}

func runResumeFromPR(
	opts Options,
	cfg config.RepoConfig,
	forge *Forge,
	servedGit *Git,
	token, scm, ctxDir, defaultBranch, defaultHEAD string,
	now time.Time,
	digestDir, digestBranch string,
) (Result, error) {
	stage := normalizeResumeStage(opts.FromStage)
	prNum := fmt.Sprint(opts.ResumePR)

	closeTrace, err := prepareWorkStory(&opts, now)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = closeTrace() }()

	if err := ensureDigestJudge(&opts, cfg); err != nil {
		return Result{}, err
	}

	head, err := forge.ResolvePRHead(prNum)
	if err != nil {
		return Result{}, fmt.Errorf("resume-pr %s: %w", prNum, err)
	}
	logf("INFO", "resume_pr=%s head=%s from_stage=%s (local-only; no context push/PR)", prNum, shortSHA(head.SHA), stage)

	if err := materializePRHeadForResume(ctxDir, servedGit, forge, prNum, head, token, scm); err != nil {
		return Result{}, err
	}
	if err := validateResumeEvidence(ctxDir, stage); err != nil {
		return Result{}, err
	}

	ctxGit := &Git{Dir: ctxDir, Token: token, SCM: scm}
	if err := configureCommitIdentity(ctxGit); err != nil {
		return Result{}, err
	}
	if _, err := CommitAll(ctxGit, fmt.Sprintf("resume baseline: PR %s @ %s", prNum, shortSHA(head.SHA))); err != nil {
		return Result{}, err
	}

	analysisDir, err := cloneAnalysisRepoFn(opts.Context, opts.WorkDir)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(analysisDir)

	sourceSHA := defaultHEAD
	if m, merr := contextstore.ParseTypologyManifest(filepath.Join(ctxDir, "evidence", "typology", "manifest.yaml")); merr == nil && strings.TrimSpace(m.SourceSHA) != "" {
		sourceSHA = m.SourceSHA
	}

	if err := runBootstrapFromStage(opts.Context, ctxDir, analysisDir, sourceSHA, now, opts, stage); err != nil {
		return Result{}, err
	}
	if _, err := CommitAll(ctxGit, fmt.Sprintf("resume %s from PR %s", stage, prNum)); err != nil {
		return Result{}, err
	}

	localOut, err := persistResumeLocalOutputs(ctxDir, opts.WorkStoryDir, ResumeProvenance{
		ResumePR:  opts.ResumePR,
		HeadSHA:   head.SHA,
		FromStage: stage,
		RepoID:    opts.RepoID,
		At:        now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return Result{}, err
	}

	if opts.DigestCache != nil && servedGit != nil && strings.TrimSpace(digestDir) != "" {
		if err := pushDigestCacheWorktree(digestDir, digestBranch, token, scm, servedGit); err != nil {
			logf("WARN", "digest inference cache push: %v", err)
		} else {
			logf("INFO", "digest inference cache pushed branch=%s", digestBranch)
		}
	}

	msg := fmt.Sprintf("resume from PR %s at %s complete (local-only)", prNum, stage)
	return Result{
		Action:        "resume",
		DefaultBranch: defaultBranch,
		DefaultHEAD:   defaultHEAD,
		CursorAfter:   sourceSHA,
		ContextPR:     prNum,
		Message:       msg,
		WorkStoryDir:  opts.WorkStoryDir,
		ResumePR:      opts.ResumePR,
		ResumeHead:    head.SHA,
		FromStage:     stage,
		LocalOutDir:   localOut,
	}, nil
}

func persistResumeLocalOutputs(ctxDir, workStoryDir string, prov ResumeProvenance) (string, error) {
	if strings.TrimSpace(workStoryDir) == "" {
		return "", fmt.Errorf("resume local outputs: work story dir is empty")
	}
	outDir := filepath.Join(workStoryDir, "local-context")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("resume local-context mkdir: %w", err)
	}
	if err := copyTree(ctxDir, outDir); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(prov, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(workStoryDir, "resume_provenance.json"), raw, 0o644); err != nil {
		return "", fmt.Errorf("write resume provenance: %w", err)
	}
	g := &Git{Dir: ctxDir}
	if diff, err := g.trim("diff", "HEAD~1", "HEAD"); err == nil && strings.TrimSpace(diff) != "" {
		if err := os.WriteFile(filepath.Join(workStoryDir, "local.diff"), []byte(diff+"\n"), 0o644); err != nil {
			return "", fmt.Errorf("write local.diff: %w", err)
		}
	} else if status, serr := g.trim("diff", "HEAD"); serr == nil && strings.TrimSpace(status) != "" {
		if err := os.WriteFile(filepath.Join(workStoryDir, "local.diff"), []byte(status+"\n"), 0o644); err != nil {
			return "", fmt.Errorf("write local.diff: %w", err)
		}
	}
	logf("INFO", "resume local outputs dir=%s provenance=resume_provenance.json", outDir)
	return outDir, nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Skip nested .git; local-context is a file dump, not a clone.
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

package reviewrun

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/observability"
	"github.com/behaviorengineering/majordomo/internal/orchestrate"
	"github.com/behaviorengineering/majordomo/internal/publish"
	"github.com/behaviorengineering/majordomo/internal/sa"
)

// Options configures majordomo run review.
type Options struct {
	ConfigDir   string
	RepoID      string
	PRNumber    string
	HeadSHA     string
	BaseBranch  string
	CloneURL    string
	WorkDir     string
	StagingDir  string
	OutputDir   string
	ScriptsDir  string
	ContextDir  string
	CursorDir   string
	Until       string
	Publish     bool
	SkipDeep    bool
	SkipReport  bool
	Concurrency int

	// Injectables for tests.
	Clone       func() error
	SA          func(sa.Options) error
	Orchestrate func(orchestrate.Options) error
	PublishFn   func(publish.Options) error
}

func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}

// Run executes clone, SA, orchestrate, and optional publish for one PR.
func Run(opts Options) (err error) {
	if strings.TrimSpace(opts.ConfigDir) == "" || strings.TrimSpace(opts.RepoID) == "" {
		return fmt.Errorf("run review requires --config-dir and --repo-id")
	}
	if strings.TrimSpace(opts.PRNumber) == "" {
		return fmt.Errorf("run review requires --pr")
	}

	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join("review-output", opts.RepoID, "pr-review")
	}
	otelCfg := observability.ResolveConfig(opts.OutputDir)
	if _, otelErr := observability.Init(otelCfg); otelErr != nil {
		logf("WARN", "otel init: %v", otelErr)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observability.Flush(flushCtx)
		_ = observability.Shutdown(flushCtx)
	}()

	ctx, span := observability.StartChainSpan(context.Background(), otelCfg.ServiceName, "majordomo.run.review")
	_ = ctx
	defer observability.EndSpanWithStatus(span, &err)

	until, err := ParseUntil(opts.Until)
	if err != nil {
		return err
	}
	opts.Until = until

	cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	scm := strings.ToLower(strings.TrimSpace(cfg.SCM))
	if scm == "" {
		scm = "github"
	}
	owner, name := cfg.Repository.Owner, cfg.Repository.Name
	cloneURL := strings.TrimSpace(opts.CloneURL)
	if cloneURL == "" {
		cloneURL = strings.TrimSpace(cfg.Repository.CloneURL)
	}
	if owner == "" || name == "" {
		o, n := splitOwnerName(cloneURL)
		if owner == "" {
			owner = o
		}
		if name == "" {
			name = n
		}
	}
	token := config.ResolveCredential(cfg.Repository.ID, scm, owner)
	if token == "" && strings.ToLower(scm) == "bitbucket" {
		token = strings.TrimSpace(os.Getenv("BITBUCKET_TOKEN"))
	}

	workdir := strings.TrimSpace(opts.WorkDir)
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("workdir: %w", err)
		}
		if isGitRepo(wd) {
			workdir = wd
		} else {
			workdir = filepath.Join(wd, "served", "repo")
		}
	}
	workdir, err = filepath.Abs(workdir)
	if err != nil {
		return fmt.Errorf("workdir abs: %w", err)
	}
	opts.WorkDir = workdir

	if opts.StagingDir == "" {
		opts.StagingDir = filepath.Join("staging", opts.RepoID+"-pr-"+opts.PRNumber)
	}
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join("review-output", opts.RepoID, "pr-review")
	}
	opts.StagingDir, err = filepath.Abs(opts.StagingDir)
	if err != nil {
		return fmt.Errorf("staging-dir: %w", err)
	}
	opts.OutputDir, err = filepath.Abs(opts.OutputDir)
	if err != nil {
		return fmt.Errorf("output-dir: %w", err)
	}
	if err := os.MkdirAll(opts.StagingDir, 0o755); err != nil {
		return fmt.Errorf("mkdir staging: %w", err)
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir output: %w", err)
	}

	logf("INFO", "========== majordomo run review ==========")
	logf("INFO", "repo=%s pr=%s until=%s publish=%v", opts.RepoID, opts.PRNumber, emptyUntil(opts.Until), opts.Publish)

	if err := runClone(opts, token, scm, cloneURL); err != nil {
		return err
	}
	if opts.HeadSHA == "" {
		opts.HeadSHA = resolveHEAD(opts.WorkDir)
	}
	if opts.BaseBranch == "" && isGitRepo(opts.WorkDir) {
		opts.BaseBranch = resolveDefaultBranch(opts.WorkDir, token, scm)
	}

	if !shouldRun(opts.Until, StageSA) {
		logf("INFO", "until=%s: stopping after clone", opts.Until)
		return nil
	}

	if err := runSA(opts); err != nil {
		logf("WARN", "sa: %v (continuing)", err)
	}
	if !shouldRun(opts.Until, StagePrep) {
		logf("INFO", "until=%s: stopping after sa", opts.Until)
		return nil
	}

	contextDir := maybeContextDir(opts, token, scm, cloneURL, opts.RepoID)
	if err := runOrchestrate(opts, contextDir); err != nil {
		return err
	}
	if !shouldRun(opts.Until, StagePublish) {
		logf("INFO", "until=%s: stopping before publish", opts.Until)
		return nil
	}

	if opts.Publish {
		if err := runPublish(opts, cfg, scm, owner, name); err != nil {
			return err
		}
		if err := recordCursor(opts); err != nil {
			return err
		}
	}
	logf("INFO", "run review complete")
	return nil
}

func emptyUntil(s string) string {
	if s == "" {
		return "(full)"
	}
	return s
}

func runClone(opts Options, token, scm, cloneURL string) error {
	if opts.Clone != nil {
		return opts.Clone()
	}
	head := opts.HeadSHA
	return ensureServedRepo(opts, token, scm, cloneURL, head, opts.BaseBranch)
}

func runSA(opts Options) error {
	if opts.SA != nil {
		return opts.SA(sa.Options{
			ConfigDir:  opts.ConfigDir,
			RepoID:     opts.RepoID,
			RepoRoot:   opts.WorkDir,
			BaseBranch: opts.BaseBranch,
			ScriptsDir: opts.ScriptsDir,
		})
	}
	if opts.BaseBranch == "" {
		logf("INFO", "sa skipped: no base branch")
		return nil
	}
	return sa.Run(sa.Options{
		ConfigDir:  opts.ConfigDir,
		RepoID:     opts.RepoID,
		RepoRoot:   opts.WorkDir,
		BaseBranch: opts.BaseBranch,
		ScriptsDir: opts.ScriptsDir,
	})
}

func runOrchestrate(opts Options, contextDir string) error {
	oUntil := orchestrateUntil(opts.Until)
	fn := opts.Orchestrate
	if fn == nil {
		fn = orchestrate.Run
	}
	return fn(orchestrate.Options{
		PRNumber:    opts.PRNumber,
		BaseBranch:  opts.BaseBranch,
		StagingDir:  opts.StagingDir,
		OutputDir:   opts.OutputDir,
		Pipeline:    "pr-review",
		Concurrency: opts.Concurrency,
		ScriptsDir:  opts.ScriptsDir,
		SkipDeep:    opts.SkipDeep,
		SkipReport:  opts.SkipReport,
		Until:       oUntil,
		RepoRoot:    opts.WorkDir,
		ConfigDir:   opts.ConfigDir,
		RepoID:      opts.RepoID,
		ContextDir:  contextDir,
	})
}

func runPublish(opts Options, cfg config.RepoConfig, scm, owner, name string) error {
	summary, err := findSummary(opts.OutputDir)
	if err != nil || summary == "" {
		logf("INFO", "no summary.md; skip publish")
		return nil
	}
	mode := publish.Mode(cfg.EffectivePublishMode())
	fn := opts.PublishFn
	if fn == nil {
		fn = publish.Run
	}
	return fn(publish.Options{
		SCM:         scm,
		PRNumber:    opts.PRNumber,
		SummaryFile: summary,
		Mode:        mode,
		RepoID:      opts.RepoID,
		GitHubOwner: owner,
		GitHubRepo:  name,
	})
}

func recordCursor(opts Options) error {
	if opts.HeadSHA == "" {
		return nil
	}
	cursorDir := opts.CursorDir
	if cursorDir == "" {
		cursorDir = ".poll-cache"
	}
	path := filepath.Join(cursorDir, opts.RepoID, "poll-cursor.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("poll cursor dir: %w", err)
	}
	c, err := cache.ReadPollCursor(path)
	if err != nil {
		return fmt.Errorf("read poll cursor: %w", err)
	}
	cache.RecordHead(c, opts.PRNumber, opts.HeadSHA)
	if err := cache.WritePollCursor(path, c); err != nil {
		return fmt.Errorf("write poll cursor: %w", err)
	}
	logf("INFO", "poll cursor recorded %s=%s", opts.PRNumber, shortSHA(opts.HeadSHA))
	return nil
}

func findSummary(outputDir string) (string, error) {
	candidates := []string{
		filepath.Join(outputDir, "summary.md"),
		filepath.Join(outputDir, "pr-review-summary", "summary.md"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	var found string
	walkErr := filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "summary.md") {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		return "", walkErr
	}
	return found, nil
}

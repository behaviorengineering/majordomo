package contextdigest

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextgate"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

// Result describes one digest run outcome.
type Result struct {
	Action        string `json:"action"` // noop | skipped | seed | catchup | rewrite | rewrite_blocked | gate_regen
	DefaultBranch string `json:"default_branch,omitempty"`
	DefaultHEAD   string `json:"default_head,omitempty"`
	CursorBefore  string `json:"cursor_before,omitempty"`
	CursorAfter   string `json:"cursor_after,omitempty"`
	CommitsWalked int    `json:"commits_walked,omitempty"`
	ContextPR     string `json:"context_pr,omitempty"`
	GateStatus    string `json:"gate_status,omitempty"`
	Message       string `json:"message,omitempty"`
}

// Options configures majordomo context digest.
type Options struct {
	ConfigDir    string
	RepoID       string
	WorkDir      string // served-repo clone with origin remote
	Now          time.Time
	Forge        *Forge // optional inject for tests
	SkipStory    bool
	SkipCompact  bool
	ForceCompact bool
}

func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}

// Run executes the context digest catch-up job for one served repo.
func Run(opts Options) (Result, error) {
	if opts.ConfigDir == "" || opts.RepoID == "" {
		return Result{}, fmt.Errorf("--config-dir and --repo-id required")
	}
	if opts.WorkDir == "" {
		return Result{}, fmt.Errorf("--workdir required (served-repo clone with origin)")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	defaults, err := config.LoadDefaults(opts.ConfigDir)
	if err != nil {
		return Result{}, err
	}
	cfg, err := config.LoadRepoFile(opts.ConfigDir, opts.RepoID, defaults)
	if err != nil {
		return Result{}, err
	}
	scm := strings.ToLower(strings.TrimSpace(cfg.SCM))
	if scm == "" {
		scm = "github"
	}
	if scm == "generic" {
		logf("INFO", "scm=generic: skipping context digest for %s", opts.RepoID)
		return Result{Action: "skipped", Message: "generic scm skips context PR"}, nil
	}

	owner, name := cfg.Repository.Owner, cfg.Repository.Name
	if owner == "" || name == "" {
		owner, name = splitOwnerName(cfg.Repository.CloneURL)
	}
	token := resolveToken(cfg, scm, owner)
	if token == "" {
		return Result{}, fmt.Errorf("forge token required (%s)", config.CredentialHint(cfg.Repository.ID, scm, owner))
	}

	baseBranch := cfg.Context.Branch
	if baseBranch == "" {
		baseBranch = config.ContextBranch(cfg.Repository.ID)
	}
	updateBranch := config.ContextUpdateBranch(cfg.Repository.ID)
	if err := contextstore.ValidateContextBranch(baseBranch); err != nil {
		return Result{}, err
	}

	servedGit := &Git{Dir: opts.WorkDir, Token: token, SCM: scm}
	if err := FetchOrigin(servedGit); err != nil {
		return Result{}, fmt.Errorf("fetch served repo: %w", err)
	}
	defaultBranch, err := ResolveDefaultBranch(servedGit)
	if err != nil {
		return Result{}, err
	}
	defaultHEAD, err := servedGit.trim("rev-parse", "origin/"+defaultBranch)
	if err != nil {
		return Result{}, fmt.Errorf("resolve default HEAD: %w", err)
	}

	forge := opts.Forge
	if forge == nil {
		forge = &Forge{
			SCM: scm, RepoID: cfg.Repository.ID, Owner: owner, Name: name,
			Token: token, BaseURL: cfg.SCMAPI.BaseURL,
		}
	}

	ctxDir, err := os.MkdirTemp("", "majordomo-context-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(ctxDir)

	baseExists, err := RemoteBranchExists(servedGit, baseBranch)
	if err != nil {
		return Result{}, err
	}

	var cursorBefore string
	var commits []string
	var commitCtxs []CommitContext
	action := "catchup"
	needsWrite := false
	gatePrefix := cfg.Context.GatePrefix()
	var gateSidecar contextgate.Sidecar
	var openPRNum string
	cursorAfter := defaultHEAD

	if !baseExists {
		action = "seed"
		logf("INFO", "context base branch %s missing; seeding orphan", baseBranch)
		if err := seedOrphan(ctxDir, baseBranch, cfg.Repository.ID, defaultHEAD, now, token, scm, cfg.Repository.CloneURL); err != nil {
			return Result{}, err
		}
		needsWrite = true
	} else {
		ctxGit := &Git{Dir: ctxDir, Token: token, SCM: scm}
		if err := materializeContextWorktree(ctxDir, servedGit, baseBranch, updateBranch, token, scm); err != nil {
			return Result{}, err
		}
		openPR := false
		if _, open, err := forge.WriteTipBranch(baseBranch, updateBranch); err != nil {
			return Result{}, err
		} else if open {
			openPR = true
		}
		cursorBefore, err = readEffectiveCursor(ctxGit, baseBranch, updateBranch, openPR)
		if err != nil {
			return Result{}, err
		}
		if openPR {
			openPRNum, _ = forge.findOpenPRNumber(baseBranch, updateBranch)
			if openPRNum != "" {
				comments, err := forge.ListPRComments(openPRNum)
				if err != nil {
					logf("WARN", "list PR comments: %v", err)
				} else {
					meta, _ := readRewriteMeta(ctxDir)
					if _, _, why := contextgate.ApplyComments(comments, gatePrefix); why != "" && meta.RewritePending {
						_ = ApplyRewriteWhy(ctxDir, why)
					}
					gateSidecar, err = contextgate.SyncFromComments(ctxDir, openPRNum, gatePrefix, comments, meta.RewritePending, meta.RewriteWhy)
					if err != nil {
						return Result{}, err
					}
				}
			}
		}

		caughtUp := strings.TrimSpace(cursorBefore) == defaultHEAD
		var behind bool
		if caughtUp {
			behind = false
		} else if strings.TrimSpace(cursorBefore) == "" {
			behind = defaultHEAD != ""
		} else {
			var err error
			behind, err = IsBehind(servedGit, cursorBefore, defaultHEAD, defaultBranch)
			if err != nil {
				rewrite, rerr := DetectRewrite(servedGit, cursorBefore, defaultHEAD, defaultBranch)
				if rerr != nil {
					return Result{}, rerr
				}
				if rewrite {
					action = "rewrite"
					if err := handleRewrite(ctxDir, servedGit, cursorBefore, defaultHEAD, now, opts); err != nil {
						if strings.Contains(err.Error(), "why is required") {
							if err := CheckoutUpdateBranch(ctxGit, baseBranch, updateBranch); err != nil {
								return Result{}, err
							}
							needsWrite = true
							return finishDigestRun(finishParams{
								cfg: cfg, forge: forge, ctxDir: ctxDir, ctxGit: ctxGit,
								token: token, scm: scm, baseBranch: baseBranch, updateBranch: updateBranch,
								action: "rewrite_blocked", defaultBranch: defaultBranch, defaultHEAD: defaultHEAD,
								cursorBefore: cursorBefore, cursorAfter: cursorBefore, commits: commits, needsWrite: needsWrite,
								gateSidecar: gateSidecar, openPRNum: openPRNum, cloneURL: cfg.Repository.CloneURL,
								message: err.Error(),
							})
						}
						return Result{}, err
					}
					cursorBefore, _ = ReadCursor(ctxDir)
					caughtUp = false
					behind = false
				} else {
					return Result{}, err
				}
			}
		}

		if caughtUp && !gateSidecar.RegenRequested() {
			logf("INFO", "context cursor caught up at %s", cursorBefore)
			res := Result{
				Action: "noop", DefaultBranch: defaultBranch, DefaultHEAD: defaultHEAD,
				CursorBefore: cursorBefore, CursorAfter: cursorBefore,
				GateStatus: string(gateSidecar.Status), Message: "cursor caught up",
			}
			if cfg.Context.AutoMergeEnabled() && gateSidecar.ReadyToMerge() && openPRNum != "" {
				if err := forge.MergeUpdatePR(openPRNum); err != nil {
					logf("WARN", "autoMerge: %v", err)
				} else {
					res.Message = "cursor caught up; context PR merged"
				}
			}
			return res, nil
		}

		if !caughtUp && behind {
			commits, err = FirstParentCommits(servedGit, cursorBefore, defaultHEAD, defaultBranch)
			if err != nil {
				return Result{}, err
			}
			if cap := cfg.Context.MaxCommitsPerRunLimit(); cap > 0 && len(commits) > cap {
				logf("INFO", "catch-up capped to %d of %d commits (maxCommitsPerRun)", cap, len(commits))
				commits = commits[:cap]
			}
			logf("INFO", "catch-up: %d commit(s) from %s toward %s", len(commits), cursorBefore, defaultHEAD)
		}

		cursorAfter = defaultHEAD
		if len(commits) > 0 {
			cursorAfter = commits[len(commits)-1]
		} else if strings.TrimSpace(cursorBefore) != "" {
			cursorAfter = cursorBefore
		}

		if err := CheckoutUpdateBranch(ctxGit, baseBranch, updateBranch); err != nil {
			return Result{}, err
		}

		regenFeedback := ""
		if gateSidecar.RegenRequested() {
			g := contextgate.NewGate(gatePrefix)
			regenFeedback, _ = g.NormalizeReject(gateSidecar.RejectReason)
			action = "gate_regen"
		}

		if !opts.SkipStory && (len(commits) > 0 || gateSidecar.RegenRequested()) {
			for _, sha := range commits {
				cc, err := LoadCommitContext(servedGit, sha)
				if err != nil {
					return Result{}, err
				}
				commitCtxs = append(commitCtxs, cc)
			}
			if err := walkCommitContexts(ctxDir, commitCtxs, now, regenFeedback); err != nil {
				return Result{}, err
			}
		}

		if !opts.SkipCompact {
			copts := DefaultCompactOptions(cfg.Context.Compaction.MaxChronologyEntries)
			if cfg.Context.Compaction.KeepRecentEntries > 0 {
				copts.KeepRecent = cfg.Context.Compaction.KeepRecentEntries
			}
			copts.ForceCompact = opts.ForceCompact
			if _, err := CompactChronology(ctxDir, copts); err != nil {
				return Result{}, err
			}
		}

		if !opts.SkipStory {
			if err := MaterializeAgenting(ctxDir, CollectChangedFiles(commitCtxs)); err != nil {
				return Result{}, err
			}
		}

		needs, err := NeedsMetaUpdate(ctxDir, cursorAfter)
		if err != nil {
			return Result{}, err
		}
		treeDirty := treeHasChanges(ctxDir)
		if needs || treeDirty || gateSidecar.RegenRequested() {
			if needs || treeDirty {
				if err := UpdateMeta(ctxDir, cursorAfter, now); err != nil {
					return Result{}, err
				}
			}
			if err := configureCommitIdentity(ctxGit); err != nil {
				return Result{}, err
			}
			msg := fmt.Sprintf("context digest: advance cursor to %s", shortSHA(cursorAfter))
			if action == "rewrite" {
				msg = fmt.Sprintf("context digest: history rewrite to %s", shortSHA(defaultHEAD))
			}
			if gateSidecar.RegenRequested() {
				msg = fmt.Sprintf("context digest: regen after gate reject")
			}
			committed, err := CommitAll(ctxGit, msg)
			if err != nil {
				return Result{}, err
			}
			needsWrite = committed || needsWrite
			if gateSidecar.RegenRequested() {
				gateSidecar.Status = contextgate.StatusOpen
				gateSidecar.RejectReason = ""
				_ = contextgate.SaveSidecar(ctxDir, gateSidecar)
			}
		}
	}

	return finishDigestRun(finishParams{
		cfg: cfg, forge: forge, ctxDir: ctxDir,
		ctxGit: &Git{Dir: ctxDir, Token: token, SCM: scm},
		token: token, scm: scm, baseBranch: baseBranch, updateBranch: updateBranch,
		action: action, defaultBranch: defaultBranch, defaultHEAD: defaultHEAD,
		cursorBefore: cursorBefore, cursorAfter: cursorAfter, commits: commits, needsWrite: needsWrite,
		gateSidecar: gateSidecar, openPRNum: openPRNum, cloneURL: cfg.Repository.CloneURL,
	})
}

type finishParams struct {
	cfg              config.RepoConfig
	forge            *Forge
	ctxDir           string
	ctxGit           *Git
	token, scm       string
	baseBranch       string
	updateBranch     string
	action           string
	defaultBranch    string
	defaultHEAD      string
	cursorBefore     string
	cursorAfter      string
	commits          []string
	needsWrite       bool
	gateSidecar      contextgate.Sidecar
	openPRNum        string
	cloneURL         string
	message          string
}

func finishDigestRun(p finishParams) (Result, error) {
	cfg := p.cfg
	if err := ensureRemote(p.ctxGit, p.cloneURL); err != nil {
		return Result{}, err
	}
	if p.needsWrite {
		if p.action == "seed" {
			if err := Push(p.ctxGit, p.baseBranch); err != nil {
				return Result{}, fmt.Errorf("push context base: %w", err)
			}
			if err := CheckoutOrCreate(p.ctxGit, p.updateBranch, p.baseBranch); err != nil {
				return Result{}, err
			}
		}
		if err := Push(p.ctxGit, p.updateBranch); err != nil {
			return Result{}, fmt.Errorf("push context update: %w", err)
		}
	}

	title := fmt.Sprintf("context digest: %s", cfg.Repository.ID)
	body := digestPRBody(len(p.commits), p.defaultHEAD, p.gateSidecar)
	pr, err := p.forge.OpenUpdatePR(p.baseBranch, p.updateBranch, title, body)
	if err != nil {
		return Result{}, err
	}
	if cfg.Context.AutoMergeEnabled() && p.gateSidecar.ReadyToMerge() && pr != "" {
		if err := p.forge.MergeUpdatePR(pr); err != nil {
			logf("WARN", "autoMerge: %v", err)
		}
	}

	msg := p.message
	if msg == "" {
		msg = "digest complete"
	}
	return Result{
		Action:        p.action,
		DefaultBranch: p.defaultBranch,
		DefaultHEAD:   p.defaultHEAD,
		CursorBefore:  p.cursorBefore,
		CursorAfter:   p.cursorAfter,
		CommitsWalked: len(p.commits),
		ContextPR:     pr,
		GateStatus:    string(p.gateSidecar.Status),
		Message:       msg,
	}, nil
}

func handleRewrite(ctxDir string, g *Git, cursor, newHead string, at time.Time, opts Options) error {
	meta, err := readRewriteMeta(ctxDir)
	if err != nil {
		return err
	}
	if !meta.RewritePending {
		if _, err := BeginRewrite(ctxDir, newHead, at, "majordomo", "cursor "+cursor+" not ancestor of "+newHead); err != nil {
			return err
		}
		meta, _ = readRewriteMeta(ctxDir)
	}
	if meta.RewritePending && strings.TrimSpace(meta.RewriteWhy) == "" {
		return fmt.Errorf("rewrite blocked: why is required (@majordomo why … on context PR)")
	}
	why := meta.RewriteWhy
	if !opts.SkipStory {
		if err := ReshapeStory(ctxDir, newHead, why); err != nil {
			return err
		}
	}
	if err := CompleteRewrite(ctxDir, newHead, at); err != nil {
		return err
	}
	return nil
}

func treeHasChanges(dir string) bool {
	g := &Git{Dir: dir}
	status, err := g.trim("status", "--porcelain")
	return err == nil && strings.TrimSpace(status) != ""
}

func (f *Forge) findOpenPRNumber(baseBranch, headBranch string) (string, error) {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.findGitHubOpen(baseBranch, headBranch)
	case "gitlab":
		env := f.glabEnv()
		return f.findGitLabOpen(baseBranch, headBranch, env, glabRepoArgs(f.Owner, f.Name))
	case "bitbucket":
		return f.findBitbucketOpen(baseBranch, headBranch)
	default:
		return "", nil
	}
}

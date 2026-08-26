package poll

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/majordomo/internal/config"
)

// PendingReview is one PR/MR that needs a review run.
type PendingReview struct {
	RepoID      string `json:"repo_id"`
	SCM         string `json:"scm"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	PRNumber    string `json:"pr"`
	HeadSHA     string `json:"head_sha"`
	BaseBranch  string `json:"base_branch"`
	CloneURL    string `json:"clone_url"`
	ReviewID    string `json:"review_id"`
	PublishMode string `json:"publish_mode,omitempty"`
}

// Result is the JSON document written by majordomo poll.
type Result struct {
	GeneratedAt string          `json:"generated_at"`
	Reviews     []PendingReview `json:"reviews"`
}

// Options configures a poll run.
type Options struct {
	ConfigDir string
	CursorDir string // local cursor store (Actions cache); default .poll-cache
	OutPath   string // write JSON here; empty → stdout
	// ListPRs injectable for tests
	ListPRs func(cfg config.RepoConfig, token string) ([]openPR, error)
}

type openPR struct {
	Number     int
	HeadSHA    string
	BaseBranch string
}

func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}

// Run discovers PRs/MRs that need review and writes Result JSON.
func Run(opts Options) error {
	if opts.ConfigDir == "" {
		return fmt.Errorf("--config-dir required")
	}
	if opts.CursorDir == "" {
		opts.CursorDir = ".poll-cache"
	}
	// Capture before defaulting so production empty-token skips still work.
	listInjected := opts.ListPRs != nil
	if opts.ListPRs == nil {
		opts.ListPRs = listOpenPRs
	}

	cfgs, err := config.LoadAll(opts.ConfigDir)
	if err != nil {
		return err
	}
	logf("INFO", "========== majordomo poll ==========")
	logf("INFO", "config dir: %s (%d repo(s))", opts.ConfigDir, len(cfgs))

	result := Result{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Reviews:     []PendingReview{},
	}

	var listErrs []string
	for _, cfg := range cfgs {
		if !cfg.Trigger.PollEnabled() {
			logf("INFO", "skip %s (poll disabled)", cfg.Repository.ID)
			continue
		}
		scm := strings.ToLower(cfg.SCM)
		if scm == "" {
			scm = "github"
		}

		owner, name := cfg.Repository.Owner, cfg.Repository.Name
		if owner == "" || name == "" {
			o2, n2 := splitOwnerName(parseClonePath(cfg.Repository.CloneURL))
			if owner == "" {
				owner = o2
			}
			if name == "" {
				name = n2
			}
		}

		token := config.ResolveCredential(cfg.Repository.ID, scm, owner)
		// When ListPRs is injected (tests), allow empty token.
		if token == "" && !listInjected {
			logf("WARN", "%s: no credential (set %s) — skipping",
				cfg.Repository.ID, config.CredentialHint(cfg.Repository.ID, scm, owner))
			continue
		}

		prs, err := opts.ListPRs(cfg, token)
		if err != nil {
			msg := fmt.Sprintf("%s: list PRs failed: %v", cfg.Repository.ID, err)
			logf("WARN", "%s", msg)
			listErrs = append(listErrs, msg)
			continue
		}

		cursorPath := filepath.Join(opts.CursorDir, cfg.Repository.ID, "poll-cursor.json")
		cursor, err := cache.ReadPollCursor(cursorPath)
		if err != nil {
			return err
		}
		cursor.RepoID = cfg.Repository.ID

		for _, pr := range prs {
			prNum := fmt.Sprintf("%d", pr.Number)
			continuous := cfg.Review.ContinuousRunsEnabled()
			if !cache.ShouldReview(cursor, prNum, pr.HeadSHA, continuous) {
				if continuous {
					logf("INFO", "%s#%s: head unchanged (%s)", cfg.Repository.ID, prNum, short(pr.HeadSHA))
				} else {
					logf("INFO", "%s#%s: already reviewed (enableContinuousRuns=false)", cfg.Repository.ID, prNum)
				}
				continue
			}
			clone := cfg.Repository.CloneURL
			if clone == "" {
				clone = defaultCloneURL(scm, cfg.SCMAPI.BaseURL, owner, name)
			}
			reviewID := fmt.Sprintf("%s:%s/%s:%s:%s", scm, owner, name, prNum, pr.HeadSHA)
			result.Reviews = append(result.Reviews, PendingReview{
				RepoID:      cfg.Repository.ID,
				SCM:         scm,
				Owner:       owner,
				Name:        name,
				PRNumber:    prNum,
				HeadSHA:     pr.HeadSHA,
				BaseBranch:  pr.BaseBranch,
				CloneURL:    clone,
				ReviewID:    reviewID,
				PublishMode: cfg.EffectivePublishMode(),
			})
			logf("INFO", "%s#%s: needs review @ %s", cfg.Repository.ID, prNum, short(pr.HeadSHA))
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if opts.OutPath == "" {
		if _, err = os.Stdout.Write(data); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(opts.OutPath), 0o755); err != nil {
			return err
		}
		logf("INFO", "wrote %d pending review(s) → %s", len(result.Reviews), opts.OutPath)
		if err := os.WriteFile(opts.OutPath, data, 0o644); err != nil {
			return err
		}
	}
	if len(listErrs) > 0 {
		return fmt.Errorf("poll incomplete: %d repo list failure(s): %s", len(listErrs), strings.Join(listErrs, "; "))
	}
	return nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// parseClonePath returns the repo path after the host (no .git), e.g. "acme/demo" or "g/s/demo".
func parseClonePath(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "ssh://")
	u = strings.TrimPrefix(u, "git@")
	u = strings.Replace(u, ":", "/", 1)
	if i := strings.Index(u, "/"); i >= 0 {
		return strings.Trim(u[i+1:], "/")
	}
	return ""
}

func splitOwnerName(path string) (owner, name string) {
	path = strings.Trim(path, "/")
	if path == "" {
		return "", ""
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		return "", parts[0]
	}
	return strings.Join(parts[:len(parts)-1], "/"), parts[len(parts)-1]
}

func defaultCloneURL(scm, apiBase, owner, name string) string {
	if owner == "" || name == "" {
		return ""
	}
	host := "github.com"
	if scm == "gitlab" {
		host = "gitlab.com"
	}
	if apiBase != "" {
		b := strings.TrimRight(apiBase, "/")
		b = strings.TrimPrefix(b, "https://")
		b = strings.TrimPrefix(b, "http://")
		if i := strings.Index(b, "/"); i >= 0 {
			b = b[:i]
		}
		if b != "" {
			host = b
		}
	}
	return fmt.Sprintf("https://%s/%s/%s.git", host, owner, name)
}

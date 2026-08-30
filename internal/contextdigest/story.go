package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
)

const maxDiffLines = 120

// CommitContext is one first-parent node on default.
type CommitContext struct {
	SHA     string
	Subject string
	Body    string
	Diff    string
	Files   []string
}

// LoadCommitContext loads metadata and capped diff for one commit.
func LoadCommitContext(g *Git, sha string) (CommitContext, error) {
	subject, err := g.trim("show", "-s", "--format=%s", sha)
	if err != nil {
		return CommitContext{}, err
	}
	body, _ := g.trim("show", "-s", "--format=%b", sha)
	diff, err := g.trim("show", "--format=", "--no-color", "-U3", sha)
	if err != nil {
		return CommitContext{}, err
	}
	lines := strings.Split(diff, "\n")
	if len(lines) > maxDiffLines {
		diff = strings.Join(lines[:maxDiffLines], "\n") + fmt.Sprintf("\n[... %d lines omitted — digest diff cap]\n", len(lines)-maxDiffLines)
	}
	namesOut, err := g.trim("show", "--name-only", "--format=", sha)
	if err != nil {
		return CommitContext{}, err
	}
	var files []string
	for _, f := range strings.Split(namesOut, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, f)
		}
	}
	return CommitContext{SHA: sha, Subject: subject, Body: body, Diff: diff, Files: files}, nil
}

// ProcessCommit applies evidenced story updates for one commit (chronology when supportable).
func ProcessCommit(ctxDir string, cc CommitContext, at time.Time) error {
	if !evidenceForChronology(cc) {
		return nil
	}
	ev := contextstore.ChronologyEvent{
		Date:      at,
		Actor:     "majordomo",
		Source:    shortSHA(cc.SHA),
		Did:       cc.Subject,
		Because:   firstLine(cc.Body),
		InOrderTo: "advance context cursor on default first-parent tape",
		Evidence:  "commit " + cc.SHA + "; files: " + strings.Join(cc.Files, ", "),
	}
	if strings.TrimSpace(ev.Because) == "" {
		ev.Because = "shown in commit diff on default branch"
	}
	return contextstore.AppendChronologyEvent(ctxDir, ev)
}

func evidenceForChronology(cc CommitContext) bool {
	if strings.TrimSpace(cc.Subject) == "" {
		return false
	}
	if len(cc.Files) == 0 && strings.TrimSpace(cc.Diff) == "" {
		return false
	}
	return true
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

// WalkCommits processes each SHA in order, advancing partial cursor in meta when opts allow.
func WalkCommits(ctxDir string, g *Git, commits []string, at time.Time, regenFeedback string) error {
	_ = regenFeedback
	for _, sha := range commits {
		cc, err := LoadCommitContext(g, sha)
		if err != nil {
			return fmt.Errorf("commit %s: %w", sha, err)
		}
		if err := ProcessCommit(ctxDir, cc, at); err != nil {
			return err
		}
		if err := touchStorySections(ctxDir, cc); err != nil {
			return err
		}
	}
	return nil
}

// walkCommitContexts applies story updates for pre-loaded commit contexts.
func walkCommitContexts(ctxDir string, commits []CommitContext, at time.Time, regenFeedback string) error {
	for _, cc := range commits {
		if err := ProcessCommit(ctxDir, cc, at); err != nil {
			return err
		}
	}
	if err := applyStoryLLM(ctxDir, commits, regenFeedback); err != nil {
		logf("WARN", "LLM story digest: %v; using rule-based fallback", err)
		for _, cc := range commits {
			if err := touchStorySections(ctxDir, cc); err != nil {
				return err
			}
		}
	}
	return nil
}

func touchStorySections(ctxDir string, cc CommitContext) error {
	if len(cc.Files) == 0 {
		return nil
	}
	for _, name := range []string{"weaknesses.md"} {
		if !anyPathMatches(cc.Files, []string{"**/*.go", "**/*.py", "**/internal/**"}) {
			continue
		}
		path := filepath.Join(ctxDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		note := fmt.Sprintf("\n- Observed in `%s`: %s\n", shortSHA(cc.SHA), cc.Subject)
		if !strings.Contains(string(data), cc.SHA) {
			if err := os.WriteFile(path, append(data, []byte(note)...), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func anyPathMatches(files, globs []string) bool {
	for _, f := range files {
		for _, g := range globs {
			if matchSimpleGlob(g, f) {
				return true
			}
		}
	}
	return false
}

func matchSimpleGlob(pattern, name string) bool {
	pattern = strings.ReplaceAll(pattern, "**/", "")
	pattern = strings.TrimPrefix(pattern, "**")
	pattern = strings.TrimPrefix(pattern, "*")
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return strings.HasPrefix(name, prefix+"/") || name == prefix
	}
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(name, pattern[1:])
	}
	return pattern == "" || strings.Contains(name, pattern)
}

// ReshapeStory rewrites teaching files after history rewrite (bootstrap-from-last grain).
func ReshapeStory(ctxDir, newHead, why string) error {
	note := fmt.Sprintf("# Architecture\n\nReshaped after history rewrite (HEAD `%s`).\n\n**Why:** %s\n\nRe-read the codebase on default; prior first-parent tape is obsolete.\n",
		shortSHA(newHead), strings.TrimSpace(why))
	if err := os.WriteFile(filepath.Join(ctxDir, "architecture.md"), []byte(note), 0o644); err != nil {
		return err
	}
	return nil
}

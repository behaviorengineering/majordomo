package contextdigest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextgate"
	"github.com/behaviorengineering/majordomo/internal/outbound"
	"github.com/behaviorengineering/majordomo/internal/publish"
)

const contextPRMarker = "<!-- majordomo-context-digest -->"

// Forge opens or restacks context update PRs.
type Forge struct {
	SCM     string
	RepoID  string
	Owner   string
	Name    string
	Token   string
	BaseURL string
	Runner  publish.CLIRunner
	Client  *http.Client
}

func (f *Forge) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return outbound.Client(60 * time.Second)
}

func (f *Forge) runCLI(name string, args []string, env []string) (string, error) {
	runner := f.Runner
	if runner == nil {
		runner = defaultRunner
	}
	return runner(name, args, env)
}

func defaultRunner(name string, args []string, env []string) (string, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", fmt.Errorf("%s not found on PATH: %w", name, err)
	}
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

// OpenUpdatePR ensures one open context update PR exists; returns PR id/number.
func (f *Forge) OpenUpdatePR(baseBranch, headBranch, title, body string) (string, error) {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.openGitHub(baseBranch, headBranch, title, body)
	case "gitlab":
		return f.openGitLab(baseBranch, headBranch, title, body)
	case "bitbucket":
		return f.openBitbucket(baseBranch, headBranch, title, body)
	default:
		return "", fmt.Errorf("unsupported scm %q for context PR", scm)
	}
}

func (f *Forge) ghEnv() []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GH_TOKEN="+f.Token, "GITHUB_TOKEN="+f.Token)
	return env
}

func (f *Forge) glabEnv() []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GITLAB_TOKEN="+f.Token, "GLAB_TOKEN="+f.Token)
	return env
}

func (f *Forge) repoSlug() string {
	return f.Owner + "/" + f.Name
}

func (f *Forge) openGitHub(baseBranch, headBranch, title, body string) (string, error) {
	if f.Token == "" || f.Owner == "" || f.Name == "" {
		return "", fmt.Errorf("github context PR requires token and owner/name (%s)",
			config.CredentialHint(f.RepoID, "github", f.Owner))
	}
	repo := f.repoSlug()
	env := f.ghEnv()
	if existing, err := f.findGitHubOpen(baseBranch, headBranch); err != nil {
		return "", err
	} else if existing != "" {
		if err := f.updateGitHubPR(existing, title, body); err != nil {
			return existing, err
		}
		return existing, nil
	}
	path, cleanup, err := writeTempBody(body)
	if err != nil {
		return "", err
	}
	defer cleanup()
	args := []string{
		"pr", "create",
		"--base", baseBranch,
		"--head", headBranch,
		"--title", title,
		"--body-file", path,
		"-R", repo,
	}
	out, err := f.runCLI("gh", args, env)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (f *Forge) findGitHubOpen(baseBranch, headBranch string) (string, error) {
	repo := f.repoSlug()
	env := f.ghEnv()
	head := f.Owner + ":" + headBranch
	args := []string{
		"pr", "list",
		"--state", "open",
		"--base", baseBranch,
		"--head", head,
		"--json", "number",
		"-q", ".[0].number",
		"-R", repo,
	}
	out, err := f.runCLI("gh", args, env)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (f *Forge) updateGitHubPR(number, title, body string) error {
	repo := f.repoSlug()
	env := f.ghEnv()
	path, cleanup, err := writeTempBody(body)
	if err != nil {
		return err
	}
	defer cleanup()
	args := []string{
		"pr", "edit", number,
		"--title", title,
		"--body-file", path,
		"-R", repo,
	}
	_, err = f.runCLI("gh", args, env)
	return err
}

func (f *Forge) openGitLab(baseBranch, headBranch, title, body string) (string, error) {
	if f.Token == "" {
		return "", fmt.Errorf("gitlab context MR requires token (%s)",
			config.CredentialHint(f.RepoID, "gitlab", f.Owner))
	}
	env := f.glabEnv()
	repoArgs := glabRepoArgs(f.Owner, f.Name)
	if existing, err := f.findGitLabOpen(baseBranch, headBranch, env, repoArgs); err != nil {
		return "", err
	} else if existing != "" {
		if err := f.updateGitLabMR(existing, title, body, env, repoArgs); err != nil {
			return existing, err
		}
		return existing, nil
	}
	// glab accepts --description (inline), not --description-file (unlike gh --body-file).
	args := append([]string{
		"mr", "create",
		"--target-branch", baseBranch,
		"--source-branch", headBranch,
		"--title", title,
		"--description", body,
		"-y",
	}, repoArgs...)
	out, err := f.runCLI("glab", args, env)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func glabRepoArgs(owner, name string) []string {
	if strings.Contains(owner, "/") {
		return []string{"--repo", owner + "/" + name}
	}
	return []string{"--repo", owner + "/" + name}
}

func (f *Forge) findGitLabOpen(baseBranch, headBranch string, env, repoArgs []string) (string, error) {
	// glab mr list defaults to open MRs; it has no --state flag (unlike gh pr list).
	args := append([]string{
		"mr", "list",
		"--target-branch", baseBranch,
		"--source-branch", headBranch,
		"-F", "json",
	}, repoArgs...)
	out, err := f.runCLI("glab", args, env)
	if err != nil {
		return "", err
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return "", fmt.Errorf("decode glab mr list: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	if iid, ok := rows[0]["iid"]; ok {
		return fmt.Sprint(iid), nil
	}
	return "", nil
}

func (f *Forge) updateGitLabMR(iid, title, body string, env []string, repoArgs []string) error {
	args := append([]string{
		"mr", "update", iid,
		"--title", title,
		"--description", body,
		"-y",
	}, repoArgs...)
	_, err := f.runCLI("glab", args, env)
	return err
}

func (f *Forge) openBitbucket(baseBranch, headBranch, title, body string) (string, error) {
	if f.Token == "" || f.BaseURL == "" || f.Owner == "" || f.Name == "" {
		return "", fmt.Errorf("bitbucket context PR requires BITBUCKET_URL, token, project, repo")
	}
	if existing, err := f.findBitbucketOpen(baseBranch, headBranch); err != nil {
		return "", err
	} else if existing != "" {
		if err := f.updateBitbucketPR(existing, title, body); err != nil {
			return existing, err
		}
		return existing, nil
	}
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests"
	payload := map[string]any{
		"title":       title,
		"description": body,
		"state":       "OPEN",
		"open":        true,
		"closed":      false,
		"fromRef": map[string]any{
			"id": "refs/heads/" + headBranch,
			"repository": map[string]any{
				"slug":    f.Name,
				"project": map[string]string{"key": f.Owner},
			},
		},
		"toRef": map[string]any{
			"id": "refs/heads/" + baseBranch,
			"repository": map[string]any{
				"slug":    f.Name,
				"project": map[string]string{"key": f.Owner},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, api, strings.NewReader(string(raw)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read bitbucket create PR response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("bitbucket create PR HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	if id, ok := out["id"]; ok {
		return fmt.Sprint(id), nil
	}
	return "", nil
}

func (f *Forge) findBitbucketOpen(baseBranch, headBranch string) (string, error) {
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests?state=OPEN"
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Accept", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read bitbucket PR list response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("bitbucket list PR HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Values []struct {
			ID      int `json:"id"`
			FromRef struct {
				ID string `json:"id"`
			} `json:"fromRef"`
			ToRef struct {
				ID string `json:"id"`
			} `json:"toRef"`
		} `json:"values"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	wantFrom := "refs/heads/" + headBranch
	wantTo := "refs/heads/" + baseBranch
	for _, pr := range out.Values {
		if pr.FromRef.ID == wantFrom && pr.ToRef.ID == wantTo {
			return fmt.Sprint(pr.ID), nil
		}
	}
	return "", nil
}

func (f *Forge) updateBitbucketPR(id, title, body string) error {
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests/" + url.PathEscape(id)
	payload := map[string]any{
		"title":       title,
		"description": body,
		"version":     0,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, api, strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read bitbucket update PR response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("bitbucket update PR HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// WriteTipBranch reports whether headBranch has an open update PR (write tip beats merged base).
func (f *Forge) WriteTipBranch(baseBranch, headBranch string) (string, bool, error) {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		n, err := f.findGitHubOpen(baseBranch, headBranch)
		return headBranch, n != "", err
	case "gitlab":
		env := f.glabEnv()
		repoArgs := glabRepoArgs(f.Owner, f.Name)
		n, err := f.findGitLabOpen(baseBranch, headBranch, env, repoArgs)
		return headBranch, n != "", err
	case "bitbucket":
		n, err := f.findBitbucketOpen(baseBranch, headBranch)
		return headBranch, n != "", err
	default:
		return "", false, fmt.Errorf("unsupported scm %q", scm)
	}
}

func writeTempBody(content string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "majordomo-context-*.md")
	if err != nil {
		return "", nil, err
	}
	path = f.Name()
	cleanup = func() { _ = os.Remove(path) }
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

// digestPRBodyData is the view model for the context-PR summary envelope.
type digestPRBodyData struct {
	Marker             string
	PriorityMD         string
	PriorityHasHeading bool
	Seed               bool
	Commits            int
	HeadSHA            string
	GateDone           bool
	GateRejected       bool
	GateBlockedWhy     bool
	RejectReason       string
}

const digestPRBodyTmpl = `{{.Marker}}

This pull request updates the teaching context for this repository. That briefing is what Majordomo keeps so a reviewer who has never seen this codebase can still understand what the project is, how it is shaped, and what still needs a human call. Treat the files here as that briefing, not as a changelog of code the bot already fixed.
{{if .PriorityMD}}
{{if not .PriorityHasHeading}}## Priority: human decisions

{{end}}{{.PriorityMD}}
{{end}}
## What this catch-up did

{{if .Seed -}}
Majordomo started the context cursor at ` + "`{{.HeadSHA}}`" + ` (today's default-branch HEAD). It did not replay older history.
{{else -}}
Majordomo walked {{.Commits}} default-branch commit(s) and parked the context cursor at ` + "`{{.HeadSHA}}`" + `.
{{end}}
If the story and the priorities look right, merge. If they do not, comment ` + "`@majordomo reject <reason>`" + ` and digest will regenerate. Comment ` + "`@majordomo done`" + ` when the conversation is finished.
{{if .GateDone}}
**Gate:** the conversation is complete. A human may merge.
{{else if .GateRejected}}
**Gate:** rejected. Regen is scheduled ({{.RejectReason}}).
{{else if .GateBlockedWhy}}
**Gate:** blocked. Supply ` + "`@majordomo why <reason>`" + ` before rewrite completes.
{{end}}`

var digestPRBodyTemplate = template.Must(template.New("digestPRBody").Parse(digestPRBodyTmpl))

// digestPRBody renders the context update PR summary envelope.
func digestPRBody(commits int, headSHA string, gate contextgate.Sidecar, priorityMD string) (string, error) {
	priority := strings.TrimSpace(priorityMD)
	data := &digestPRBodyData{
		Marker:             contextPRMarker,
		PriorityMD:         priority,
		PriorityHasHeading: strings.HasPrefix(strings.ToLower(priority), "## priority"),
		Seed:               commits <= 0,
		Commits:            commits,
		HeadSHA:            headSHA,
		GateDone:           gate.Status == contextgate.StatusDone,
		GateRejected:       gate.Status == contextgate.StatusRejected,
		GateBlockedWhy:     gate.Status == contextgate.StatusBlockedWhy,
		RejectReason:       gate.RejectReason,
	}
	var b bytes.Buffer
	if err := digestPRBodyTemplate.Execute(&b, data); err != nil {
		return "", fmt.Errorf("render context PR body: %w", err)
	}
	return b.String(), nil
}

// ListPRComments returns PR/MR comments oldest-first.
func (f *Forge) ListPRComments(prNumber string) ([]contextgate.Comment, error) {
	withIDs, err := f.ListPRCommentsWithIDs(prNumber)
	if err != nil {
		return nil, err
	}
	out := make([]contextgate.Comment, 0, len(withIDs))
	for _, c := range withIDs {
		out = append(out, contextgate.Comment{Body: c.Body, Author: c.Author, PostedAt: c.PostedAt})
	}
	return out, nil
}

// MergeUpdatePR merges an open context update PR when autoMerge is enabled.
func (f *Forge) MergeUpdatePR(prNumber string) error {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		env := f.ghEnv()
		_, err := f.runCLI("gh", []string{"pr", "merge", prNumber, "--merge", "-R", f.repoSlug()}, env)
		return err
	case "gitlab":
		env := f.glabEnv()
		args := append([]string{"mr", "merge", prNumber}, glabRepoArgs(f.Owner, f.Name)...)
		_, err := f.runCLI("glab", args, env)
		return err
	default:
		return fmt.Errorf("autoMerge not supported for scm %q", scm)
	}
}

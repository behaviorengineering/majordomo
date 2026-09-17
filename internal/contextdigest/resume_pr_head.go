package contextdigest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/config"
)

// PRHead is the resolved head commit of a pull/merge request.
type PRHead struct {
	SHA     string // commit SHA
	RefName string // head branch name when known (bitbucket/gitlab fetch)
}

// ResolvePRHead returns the head SHA (and ref when available) for a PR/MR by number.
func (f *Forge) ResolvePRHead(prNumber string) (PRHead, error) {
	prNumber = strings.TrimSpace(prNumber)
	if prNumber == "" {
		return PRHead{}, fmt.Errorf("PR number is required")
	}
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.resolveGitHubPRHead(prNumber)
	case "gitlab":
		return f.resolveGitLabPRHead(prNumber)
	case "bitbucket":
		return f.resolveBitbucketPRHead(prNumber)
	default:
		return PRHead{}, fmt.Errorf("unsupported scm %q for resume PR head", scm)
	}
}

func (f *Forge) resolveGitHubPRHead(prNumber string) (PRHead, error) {
	if f.Token == "" || f.Owner == "" || f.Name == "" {
		return PRHead{}, fmt.Errorf("github resume PR requires token and owner/name (%s)",
			config.CredentialHint(f.RepoID, "github", f.Owner))
	}
	args := []string{
		"pr", "view", prNumber,
		"--json", "headRefOid,headRefName",
		"-R", f.repoSlug(),
	}
	out, err := f.runCLI("gh", args, f.ghEnv())
	if err != nil {
		return PRHead{}, err
	}
	var row struct {
		HeadRefOid  string `json:"headRefOid"`
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		return PRHead{}, fmt.Errorf("decode gh pr view: %w", err)
	}
	if strings.TrimSpace(row.HeadRefOid) == "" {
		return PRHead{}, fmt.Errorf("github PR %s has empty head SHA", prNumber)
	}
	return PRHead{SHA: row.HeadRefOid, RefName: row.HeadRefName}, nil
}

func (f *Forge) resolveGitLabPRHead(prNumber string) (PRHead, error) {
	if f.Token == "" {
		return PRHead{}, fmt.Errorf("gitlab resume MR requires token (%s)",
			config.CredentialHint(f.RepoID, "gitlab", f.Owner))
	}
	env := f.glabEnv()
	args := append([]string{"mr", "view", prNumber, "-F", "json"}, glabRepoArgs(f.Owner, f.Name)...)
	out, err := f.runCLI("glab", args, env)
	if err != nil {
		return PRHead{}, err
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		return PRHead{}, fmt.Errorf("decode glab mr view: %w", err)
	}
	sha := ""
	if v, ok := row["sha"]; ok {
		sha = fmt.Sprint(v)
	}
	if sha == "" {
		if diff, ok := row["diff_refs"].(map[string]any); ok {
			sha = fmt.Sprint(diff["head_sha"])
		}
	}
	ref := ""
	if v, ok := row["source_branch"]; ok {
		ref = fmt.Sprint(v)
	}
	if strings.TrimSpace(sha) == "" {
		return PRHead{}, fmt.Errorf("gitlab MR %s has empty head SHA", prNumber)
	}
	return PRHead{SHA: sha, RefName: ref}, nil
}

func (f *Forge) resolveBitbucketPRHead(prNumber string) (PRHead, error) {
	if f.Token == "" || f.BaseURL == "" || f.Owner == "" || f.Name == "" {
		return PRHead{}, fmt.Errorf("bitbucket resume PR requires BITBUCKET_URL, token, project, repo")
	}
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests/" + url.PathEscape(prNumber)
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return PRHead{}, err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Accept", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return PRHead{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return PRHead{}, fmt.Errorf("read bitbucket PR response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return PRHead{}, fmt.Errorf("bitbucket get PR HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		FromRef struct {
			ID           string `json:"id"`
			LatestCommit string `json:"latestCommit"`
			DisplayID    string `json:"displayId"`
		} `json:"fromRef"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return PRHead{}, err
	}
	sha := strings.TrimSpace(out.FromRef.LatestCommit)
	if sha == "" {
		return PRHead{}, fmt.Errorf("bitbucket PR %s has empty head SHA", prNumber)
	}
	ref := strings.TrimSpace(out.FromRef.DisplayID)
	if ref == "" {
		ref = strings.TrimPrefix(out.FromRef.ID, "refs/heads/")
	}
	return PRHead{SHA: sha, RefName: ref}, nil
}

// materializePRHeadForResume fetches a context PR head into dir.
// Tests may swap this to copy a fixture tree without forge network.
var materializePRHeadForResume = materializePRHeadTree

// materializePRHeadTree fetches the PR head into dir as a local context worktree.
func materializePRHeadTree(dir string, served *Git, _ *Forge, prNumber string, head PRHead, token, scm string) error {
	g := &Git{Dir: dir, Token: token, SCM: scm}
	if _, err := g.run("init"); err != nil {
		return err
	}
	remote, err := served.trim("remote", "get-url", "origin")
	if err != nil {
		return err
	}
	if err := ensureRemote(g, remote); err != nil {
		return err
	}
	localRef := "pr-resume-" + prNumber
	scmLower := strings.ToLower(strings.TrimSpace(scm))
	var fetchErr error
	switch scmLower {
	case "github":
		fetchErr = FetchOrigin(g, fmt.Sprintf("pull/%s/head:%s", prNumber, localRef))
	case "gitlab":
		fetchErr = FetchOrigin(g, fmt.Sprintf("merge-requests/%s/head:%s", prNumber, localRef))
	case "bitbucket":
		if strings.TrimSpace(head.RefName) == "" {
			return fmt.Errorf("bitbucket resume PR %s missing head branch name", prNumber)
		}
		fetchErr = FetchOrigin(g, head.RefName+":"+localRef)
	default:
		return fmt.Errorf("unsupported scm %q for resume PR fetch", scm)
	}
	if fetchErr != nil {
		if ferr := FetchOrigin(g, head.SHA); ferr != nil {
			return fmt.Errorf("fetch PR %s head: %w (sha fallback: %v)", prNumber, fetchErr, ferr)
		}
		if _, err := g.run("checkout", "-B", localRef, head.SHA); err != nil {
			return err
		}
		return nil
	}
	if err := CheckoutBranch(g, localRef); err != nil {
		return err
	}
	got, err := g.trim("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	want := strings.TrimSpace(head.SHA)
	if want != "" && got != want && !strings.HasPrefix(got, want) && !strings.HasPrefix(want, got) {
		logf("WARN", "resume PR %s HEAD %s != resolved %s", prNumber, shortSHA(got), shortSHA(want))
	}
	return nil
}

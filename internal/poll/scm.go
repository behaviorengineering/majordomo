package poll

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/outbound"
)

func listOpenPRs(cfg config.RepoConfig, token string) ([]openPR, error) {
	scm := strings.ToLower(cfg.SCM)
	if scm == "" {
		scm = "github"
	}
	switch scm {
	case "github":
		return listGitHubPRs(cfg, token)
	case "gitlab":
		return listGitLabMRs(cfg, token)
	default:
		return nil, fmt.Errorf("scm %q not supported in poll (github|gitlab)", scm)
	}
}

func listGitHubPRs(cfg config.RepoConfig, token string) ([]openPR, error) {
	owner, name := cfg.Repository.Owner, cfg.Repository.Name
	if owner == "" || name == "" {
		owner, name = splitOwnerName(parseClonePath(cfg.Repository.CloneURL))
	}
	if owner == "" || name == "" {
		return nil, fmt.Errorf("github owner/name required")
	}
	base := "https://api.github.com"
	if cfg.SCMAPI.BaseURL != "" {
		base = strings.TrimRight(cfg.SCMAPI.BaseURL, "/")
	}

	var out []openPR
	next := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&per_page=100", base, owner, name)
	client := outbound.Client(60 * time.Second)
	for next != "" {
		req, err := http.NewRequest(http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := outbound.DoWithRetry(client, req, 3)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GitHub API HTTP %d: %s", resp.StatusCode, string(body))
		}
		var raw []struct {
			Number int `json:"number"`
			Head   struct {
				SHA string `json:"sha"`
				Ref string `json:"ref"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("decode GitHub pulls: %w", err)
		}
		for _, p := range raw {
			out = append(out, openPR{
				Number:     p.Number,
				HeadSHA:    p.Head.SHA,
				BaseBranch: p.Base.Ref,
				HeadBranch: p.Head.Ref,
			})
		}
		next = linkRelNext(resp.Header.Get("Link"))
	}
	return out, nil
}

func listGitLabMRs(cfg config.RepoConfig, token string) ([]openPR, error) {
	project, err := gitlabProjectRef(cfg)
	if err != nil {
		return nil, err
	}
	api := gitlabAPIBase(cfg)
	var out []openPR
	client := outbound.Client(60 * time.Second)
	page := 1
	for {
		req, err := newGitLabMRListRequest(api, project, page)
		if err != nil {
			return nil, err
		}
		req.Header.Set("PRIVATE-TOKEN", token)
		req.Header.Set("Accept", "application/json")
		resp, err := outbound.DoWithRetry(client, req, 3)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GitLab API HTTP %d: %s", resp.StatusCode, string(body))
		}
		var raw []struct {
			IID          int    `json:"iid"`
			SHA          string `json:"sha"`
			SourceBranch string `json:"source_branch"`
			TargetBranch string `json:"target_branch"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("decode GitLab merge_requests: %w", err)
		}
		for _, mr := range raw {
			out = append(out, openPR{
				Number:     mr.IID,
				HeadSHA:    mr.SHA,
				BaseBranch: mr.TargetBranch,
				HeadBranch: mr.SourceBranch,
			})
		}
		nextPage := strings.TrimSpace(resp.Header.Get("X-Next-Page"))
		if nextPage == "" {
			break
		}
		n, err := strconv.Atoi(nextPage)
		if err != nil || n <= page {
			break
		}
		page = n
	}
	return out, nil
}

// newGitLabMRListRequest builds GET .../projects/:id/merge_requests with %2F preserved for nested paths.
func newGitLabMRListRequest(apiBase, project string, page int) (*http.Request, error) {
	base, err := url.Parse(strings.TrimRight(apiBase, "/") + "/")
	if err != nil {
		return nil, err
	}
	esc := encodeGitLabProject(project)
	path := strings.TrimSuffix(base.EscapedPath(), "/") + "/projects/" + esc + "/merge_requests"
	req, err := http.NewRequest(http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	req.URL.Scheme = base.Scheme
	req.URL.Host = base.Host
	req.URL.Path = ""
	req.URL.RawPath = ""
	req.URL.Opaque = "//" + base.Host + path
	req.URL.RawQuery = fmt.Sprintf("state=opened&per_page=100&page=%d", page)
	return req, nil
}

func encodeGitLabProject(id string) string {
	// PathEscape does not encode "/"; GitLab requires %2F for group/project paths.
	return strings.ReplaceAll(url.PathEscape(id), "/", "%2F")
}

func gitlabAPIBase(cfg config.RepoConfig) string {
	base := strings.TrimRight(cfg.SCMAPI.BaseURL, "/")
	if base == "" {
		base = "https://gitlab.com"
	}
	if strings.HasSuffix(base, "/api/v4") {
		return base
	}
	return base + "/api/v4"
}

func gitlabProjectRef(cfg config.RepoConfig) (string, error) {
	if id := strings.TrimSpace(cfg.SCMAPI.ProjectID); id != "" {
		return id, nil
	}
	if cfg.Repository.Owner != "" && cfg.Repository.Name != "" {
		return cfg.Repository.Owner + "/" + cfg.Repository.Name, nil
	}
	path := parseClonePath(cfg.Repository.CloneURL)
	if path == "" {
		return "", fmt.Errorf("gitlab projectId or repository owner/name/cloneUrl required")
	}
	return path, nil
}

// linkRelNext returns the URL for rel="next" from a GitHub Link header.
func linkRelNext(link string) string {
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

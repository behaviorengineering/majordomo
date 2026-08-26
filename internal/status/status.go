package status

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// State is a forge build/commit status.
type State string

const (
	StateInProgress State = "INPROGRESS"
	StateSuccessful State = "SUCCESSFUL"
	StateFailed     State = "FAILED"
)

// Options configures a status post.
type Options struct {
	SCM        string // github | bitbucket
	CommitSHA  string
	State      State
	// Bitbucket
	BBBaseURL   string
	BBProject   string
	BBRepo      string
	BBToken     string
	BuildURL    string
	BuildKey    string
	BuildName   string
	Description string
	BuildNumber string
	BuildRef    string
	// GitHub
	GitHubToken string
	GitHubOwner string
	GitHubRepo  string
	Context     string // GitHub status context
	HTTPClient  *http.Client
}

func (o *Options) client() *http.Client {
	if o.HTTPClient != nil {
		return o.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func logf(level, format string, args ...any) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}

// Run posts commit/build status.
func Run(opts Options) error {
	opts.State = State(strings.ToUpper(string(opts.State)))
	switch opts.State {
	case StateInProgress, StateSuccessful, StateFailed:
	default:
		return fmt.Errorf("state must be INPROGRESS|SUCCESSFUL|FAILED, got %q", opts.State)
	}
	if opts.CommitSHA == "" {
		return fmt.Errorf("commit SHA required")
	}
	opts.fillFromEnv()
	switch strings.ToLower(opts.SCM) {
	case "github":
		return postGitHub(opts)
	case "bitbucket":
		return postBitbucket(opts)
	default:
		return fmt.Errorf("unsupported scm %q (github|bitbucket)", opts.SCM)
	}
}

func (o *Options) fillFromEnv() {
	if o.BBBaseURL == "" {
		o.BBBaseURL = first(os.Getenv("BB_BASE_URL"), os.Getenv("BITBUCKET_URL"))
	}
	if o.BBProject == "" {
		o.BBProject = first(os.Getenv("BB_PROJECT_KEY"), os.Getenv("BB_PROJECT"))
	}
	if o.BBRepo == "" {
		o.BBRepo = first(os.Getenv("BB_REPO_SLUG"), os.Getenv("BB_REPO"))
	}
	if o.BBToken == "" {
		o.BBToken = os.Getenv("BITBUCKET_TOKEN")
	}
	if o.BuildURL == "" {
		o.BuildURL = os.Getenv("BUILD_URL")
	}
	if o.BuildKey == "" {
		o.BuildKey = os.Getenv("BB_BUILD_KEY")
	}
	if o.BuildName == "" {
		o.BuildName = os.Getenv("BB_BUILD_NAME")
	}
	if o.Description == "" {
		o.Description = os.Getenv("BB_BUILD_DESCRIPTION")
	}
	if o.BuildNumber == "" {
		o.BuildNumber = os.Getenv("BB_BUILD_NUMBER")
	}
	if o.BuildRef == "" {
		o.BuildRef = os.Getenv("BB_BUILD_REF")
	}
	if o.GitHubToken == "" {
		o.GitHubToken = first(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))
	}
	if o.GitHubOwner == "" || o.GitHubRepo == "" {
		if repo := os.Getenv("GITHUB_REPOSITORY"); strings.Contains(repo, "/") {
			parts := strings.SplitN(repo, "/", 2)
			if o.GitHubOwner == "" {
				o.GitHubOwner = parts[0]
			}
			if o.GitHubRepo == "" {
				o.GitHubRepo = parts[1]
			}
		}
	}
	if o.Context == "" {
		o.Context = first(os.Getenv("GITHUB_STATUS_CONTEXT"), "majordomo")
	}
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func postBitbucket(opts Options) error {
	if opts.BBBaseURL == "" || opts.BBProject == "" || opts.BBRepo == "" || opts.BBToken == "" {
		return fmt.Errorf("bitbucket status requires BB_BASE_URL, BB_PROJECT_KEY, BB_REPO_SLUG, BITBUCKET_TOKEN")
	}
	if opts.BuildURL == "" || opts.BuildKey == "" || opts.BuildName == "" || opts.Description == "" {
		return fmt.Errorf("bitbucket status requires BUILD_URL, BB_BUILD_KEY, BB_BUILD_NAME, BB_BUILD_DESCRIPTION")
	}
	parent := opts.BuildKey
	key := opts.BuildKey
	if opts.BuildNumber != "" {
		key = opts.BuildKey + "#" + opts.BuildNumber
	}
	url := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/commits/%s/builds",
		strings.TrimRight(opts.BBBaseURL, "/"), opts.BBProject, opts.BBRepo, opts.CommitSHA)
	payload := map[string]string{
		"state":       string(opts.State),
		"key":         key,
		"parent":      parent,
		"name":        opts.BuildName,
		"url":         opts.BuildURL,
		"description": opts.Description,
	}
	if opts.BuildNumber != "" {
		payload["buildNumber"] = opts.BuildNumber
	}
	if opts.BuildRef != "" {
		payload["ref"] = opts.BuildRef
	}
	logf("INFO", "========== Bitbucket build status %s @ %s ==========", opts.State, opts.CommitSHA[:min(7, len(opts.CommitSHA))])
	return doJSON(opts.client(), "POST", url, opts.BBToken, payload)
}

func postGitHub(opts Options) error {
	if opts.GitHubToken == "" || opts.GitHubOwner == "" || opts.GitHubRepo == "" {
		return fmt.Errorf("github status requires GITHUB_TOKEN and GITHUB_REPOSITORY")
	}
	ghState := "pending"
	switch opts.State {
	case StateSuccessful:
		ghState = "success"
	case StateFailed:
		ghState = "failure"
	case StateInProgress:
		ghState = "pending"
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/statuses/%s", opts.GitHubOwner, opts.GitHubRepo, opts.CommitSHA)
	payload := map[string]string{
		"state":       ghState,
		"context":     opts.Context,
		"description": first(opts.Description, string(opts.State)),
	}
	if opts.BuildURL != "" {
		payload["target_url"] = opts.BuildURL
	}
	logf("INFO", "========== GitHub status %s @ %s ==========", ghState, opts.CommitSHA[:min(7, len(opts.CommitSHA))])
	return doJSON(opts.client(), "POST", url, opts.GitHubToken, payload)
}

func doJSON(client *http.Client, method, url, token string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

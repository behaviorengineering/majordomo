package publish

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
)

const Marker = "<!-- majordomo-review -->"
const legacyMarker = "<!-- copilot-review -->"

// Mode is how the summary is published.
type Mode string

const (
	ModeAuto        Mode = "auto"
	ModeComment     Mode = "comment"
	ModeDescription Mode = "description"
)

// CLIRunner executes an external command (tests inject fakes).
// stdout is captured; stderr is inherited unless the runner redirects it.
type CLIRunner func(name string, args []string, env []string) (stdout string, err error)

// Options configures a publish run.
type Options struct {
	SCM         string // github | gitlab | bitbucket
	PRNumber    string
	SummaryFile string
	Mode        Mode
	// Bitbucket Server (HTTP)
	BitbucketURL   string
	BitbucketToken string
	BBProject      string
	BBRepo         string
	// RepoID is the majordomo-central-config id (optional; enables MAJORDOMO_CREDENTIAL_ override).
	RepoID string
	// GitHub (gh CLI) — owner/repo also used for GitLab path projects
	GitHubToken string
	GitHubOwner string
	GitHubRepo  string
	// GitLab (glab CLI)
	GitLabToken     string
	GitLabHost      string // e.g. gitlab.com or gitlab.example.com
	GitLabProjectID string // optional numeric id; else owner/name
	// Artifact URLs (optional links footer)
	SummaryArtifactURL     string
	SummaryHTMLArtifactURL string
	TechReviewArtifactURL  string
	TechDeepArtifactURL    string
	SAArtifactURLsJSON     string
	HTTPClient             *http.Client
	// Runner overrides CLI execution (tests). Empty → exec on PATH.
	Runner CLIRunner
}

func (o *Options) client() *http.Client {
	if o.HTTPClient != nil {
		return o.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (o *Options) runCLI(name string, args []string, env []string) (string, error) {
	runner := o.Runner
	if runner == nil {
		runner = defaultCLIRunner
	}
	return runner(name, args, env)
}

func defaultCLIRunner(name string, args []string, env []string) (string, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", fmt.Errorf("%s not found on PATH (run publish inside majordomo-%s job container): %w", name, name, err)
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

func logf(level, format string, args ...any) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}

// HasBodyContent reports whether summary has non-heading body text.
func HasBodyContent(summary string) bool {
	commentLine := regexp.MustCompile(`^<!--.*-->$`)
	for _, line := range strings.Split(summary, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") || commentLine.MatchString(s) {
			continue
		}
		return true
	}
	return false
}

func (o *Options) reviewLinks(fallbackSummary string) []string {
	var links []string
	summaryURL := firstNonEmpty(o.SummaryHTMLArtifactURL, fallbackSummary)
	if summaryURL != "" {
		links = append(links, "[ 👁 PR Summary ]("+summaryURL+")")
	}
	if o.TechReviewArtifactURL != "" {
		links = append(links, "[ 🔍 Technical Review ]("+o.TechReviewArtifactURL+")")
	}
	if o.TechDeepArtifactURL != "" {
		links = append(links, "[ 🧪 Technical Deep Review ]("+o.TechDeepArtifactURL+")")
	}
	if o.SAArtifactURLsJSON != "" {
		var sa []map[string]string
		if json.Unmarshal([]byte(o.SAArtifactURLsJSON), &sa) == nil {
			for _, e := range sa {
				if e["slug"] != "" && e["url"] != "" {
					links = append(links, "[ 🔬 "+e["slug"]+" ]("+e["url"]+")")
				}
			}
		}
	}
	return links
}

func (o *Options) withLinks(text string) string {
	links := o.reviewLinks("")
	if len(links) == 0 {
		return text
	}
	return text + "\n\n---\n" + strings.Join(links, " · ")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func ownedByMajordomo(body string) bool {
	return body == "" || strings.Contains(body, Marker) || strings.Contains(body, legacyMarker)
}

// Run publishes the summary to the configured SCM.
func Run(opts Options) error {
	opts.Mode = Mode(strings.ToLower(string(opts.Mode)))
	switch opts.Mode {
	case ModeAuto, ModeComment, ModeDescription:
	default:
		return fmt.Errorf("mode must be auto|comment|description, got %q", opts.Mode)
	}
	data, err := os.ReadFile(opts.SummaryFile)
	if err != nil {
		return fmt.Errorf("summary file not found: %w", err)
	}
	summary := string(data)
	if !HasBodyContent(summary) {
		logf("INFO", "summary.md contains no body content — skipping publish")
		return nil
	}
	opts.fillFromEnv()
	switch strings.ToLower(opts.SCM) {
	case "github":
		return publishGitHub(opts, summary)
	case "gitlab":
		return publishGitLab(opts, summary)
	case "bitbucket":
		return publishBitbucket(opts, summary)
	default:
		return fmt.Errorf("unsupported scm %q (github|gitlab|bitbucket)", opts.SCM)
	}
}

func (o *Options) fillFromEnv() {
	if o.BitbucketURL == "" {
		o.BitbucketURL = os.Getenv("BITBUCKET_URL")
	}
	if o.BitbucketToken == "" {
		o.BitbucketToken = os.Getenv("BITBUCKET_TOKEN")
	}
	if o.BBProject == "" {
		o.BBProject = os.Getenv("BB_PROJECT")
	}
	if o.BBRepo == "" {
		o.BBRepo = os.Getenv("BB_REPO")
	}
	if o.RepoID == "" {
		o.RepoID = firstNonEmpty(os.Getenv("MAJORDOMO_REPO_ID"), os.Getenv("REPO_ID"))
	}
	if o.GitHubOwner == "" {
		o.GitHubOwner = os.Getenv("GITHUB_REPOSITORY_OWNER")
		if o.GitHubOwner == "" {
			if repo := os.Getenv("GITHUB_REPOSITORY"); strings.Contains(repo, "/") {
				parts := strings.SplitN(repo, "/", 2)
				o.GitHubOwner, o.GitHubRepo = parts[0], parts[1]
			}
		}
	}
	if o.GitHubRepo == "" {
		if repo := os.Getenv("GITHUB_REPOSITORY"); strings.Contains(repo, "/") {
			o.GitHubRepo = strings.SplitN(repo, "/", 2)[1]
		}
	}
	if o.GitLabHost == "" {
		o.GitLabHost = firstNonEmpty(os.Getenv("GITLAB_HOST"), os.Getenv("GLAB_HOST"))
	}
	if o.GitLabProjectID == "" {
		o.GitLabProjectID = firstNonEmpty(os.Getenv("GITLAB_PROJECT_ID"), os.Getenv("CI_PROJECT_ID"))
	}
	if o.GitHubOwner == "" || o.GitHubRepo == "" {
		if path := firstNonEmpty(os.Getenv("GITLAB_REPO"), os.Getenv("CI_PROJECT_PATH")); strings.Contains(path, "/") {
			parts := strings.Split(path, "/")
			if o.GitHubOwner == "" {
				o.GitHubOwner = strings.Join(parts[:len(parts)-1], "/")
			}
			if o.GitHubRepo == "" {
				o.GitHubRepo = parts[len(parts)-1]
			}
		}
	}
	// Served-repo tokens: per-repo override, then per-org (no unqualified GH_TOKEN / GITLAB_TOKEN).
	if o.GitHubToken == "" {
		o.GitHubToken = config.ResolveCredential(o.RepoID, "github", o.GitHubOwner)
	}
	if o.GitLabToken == "" {
		o.GitLabToken = config.ResolveCredential(o.RepoID, "gitlab", o.GitHubOwner)
	}
	if o.SummaryArtifactURL == "" {
		o.SummaryArtifactURL = os.Getenv("SUMMARY_ARTIFACT_URL")
	}
	if o.SummaryHTMLArtifactURL == "" {
		o.SummaryHTMLArtifactURL = os.Getenv("SUMMARY_HTML_ARTIFACT_URL")
	}
	if o.TechReviewArtifactURL == "" {
		o.TechReviewArtifactURL = os.Getenv("TECH_REVIEW_ARTIFACT_URL")
	}
	if o.TechDeepArtifactURL == "" {
		o.TechDeepArtifactURL = os.Getenv("TECH_DEEP_ARTIFACT_URL")
	}
	if o.SAArtifactURLsJSON == "" {
		o.SAArtifactURLsJSON = os.Getenv("SA_ARTIFACT_URLS")
	}
}

func writeTempBody(content string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "majordomo-publish-*.md")
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

func httpJSON(client *http.Client, method, url, token string, body any) (map[string]any, int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, url, string(raw))
	}
	if len(raw) == 0 {
		return map[string]any{}, resp.StatusCode, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, resp.StatusCode, err
	}
	return out, resp.StatusCode, nil
}

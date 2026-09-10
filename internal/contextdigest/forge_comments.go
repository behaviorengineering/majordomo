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
	"strconv"
	"strings"
)

// PRCommentWithID is a forge PR/MR comment including a stable update id.
type PRCommentWithID struct {
	ID       string
	Body     string
	Author   string
	PostedAt string
}

// ListPRCommentsWithIDs returns PR/MR comments oldest-first with forge ids.
func (f *Forge) ListPRCommentsWithIDs(prNumber string) ([]PRCommentWithID, error) {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.listGitHubCommentsWithIDs(prNumber)
	case "gitlab":
		return f.listGitLabCommentsWithIDs(prNumber)
	case "bitbucket":
		return f.listBitbucketCommentsWithIDs(prNumber)
	default:
		return nil, fmt.Errorf("unsupported scm %q for comments", scm)
	}
}

// PostPRComment creates a PR/MR comment and returns its forge id.
func (f *Forge) PostPRComment(prNumber, body string) (string, error) {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.postGitHubComment(prNumber, body)
	case "gitlab":
		return f.postGitLabComment(prNumber, body)
	case "bitbucket":
		return f.postBitbucketComment(prNumber, body)
	default:
		return "", fmt.Errorf("unsupported scm %q for post comment", scm)
	}
}

// UpdatePRComment replaces a PR/MR comment body by forge id.
func (f *Forge) UpdatePRComment(prNumber, commentID, body string) error {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.updateGitHubComment(commentID, body)
	case "gitlab":
		return f.updateGitLabComment(prNumber, commentID, body)
	case "bitbucket":
		return f.updateBitbucketComment(prNumber, commentID, body)
	default:
		return fmt.Errorf("unsupported scm %q for update comment", scm)
	}
}

func (f *Forge) listGitHubCommentsWithIDs(prNumber string) ([]PRCommentWithID, error) {
	repo := f.repoSlug()
	env := f.ghEnv()
	args := []string{
		"api", "repos/" + repo + "/issues/" + prNumber + "/comments",
		"--jq", ".[] | {id: .id, body: .body, user: .user.login, created_at: .created_at}",
	}
	out, err := f.runCLI("gh", args, env)
	if err != nil {
		return nil, err
	}
	return decodeCommentIDLines(out)
}

func (f *Forge) postGitHubComment(prNumber, body string) (string, error) {
	repo := f.repoSlug()
	env := f.ghEnv()
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return "", err
	}
	args := []string{
		"api", "-X", "POST", "repos/" + repo + "/issues/" + prNumber + "/comments",
		"--input", "-",
		"--jq", ".id",
	}
	out, err := f.runCLIWithStdin("gh", args, env, string(payload))
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(out)
	if id == "" {
		return "", fmt.Errorf("github post comment: empty id")
	}
	return id, nil
}

func (f *Forge) updateGitHubComment(commentID, body string) error {
	repo := f.repoSlug()
	env := f.ghEnv()
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	args := []string{
		"api", "-X", "PATCH", "repos/" + repo + "/issues/comments/" + commentID,
		"--input", "-",
	}
	_, err = f.runCLIWithStdin("gh", args, env, string(payload))
	return err
}

func (f *Forge) listGitLabCommentsWithIDs(iid string) ([]PRCommentWithID, error) {
	env := f.glabEnv()
	repoArgs := glabRepoArgs(f.Owner, f.Name)
	args := append([]string{
		"api", "projects/" + url.PathEscape(f.Owner+"/"+f.Name) + "/merge_requests/" + iid + "/notes",
	}, repoArgs...)
	out, err := f.runCLI("glab", args, env)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Username string `json:"username"`
		} `json:"author"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, fmt.Errorf("decode gitlab notes: %w", err)
	}
	var comments []PRCommentWithID
	for _, r := range rows {
		comments = append(comments, PRCommentWithID{
			ID: strconv.Itoa(r.ID), Body: r.Body, Author: r.User.Username, PostedAt: r.CreatedAt,
		})
	}
	return comments, nil
}

func (f *Forge) postGitLabComment(iid, body string) (string, error) {
	env := f.glabEnv()
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return "", err
	}
	path := "projects/" + url.PathEscape(f.Owner+"/"+f.Name) + "/merge_requests/" + iid + "/notes"
	args := []string{"api", "-X", "POST", path, "--input", "-"}
	out, err := f.runCLIWithStdin("glab", args, env, string(payload))
	if err != nil {
		return "", err
	}
	var row struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		return "", fmt.Errorf("decode gitlab note create: %w", err)
	}
	if row.ID == 0 {
		return "", fmt.Errorf("gitlab post comment: empty id")
	}
	return strconv.Itoa(row.ID), nil
}

func (f *Forge) updateGitLabComment(iid, noteID, body string) error {
	env := f.glabEnv()
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	path := "projects/" + url.PathEscape(f.Owner+"/"+f.Name) + "/merge_requests/" + iid + "/notes/" + noteID
	args := []string{"api", "-X", "PUT", path, "--input", "-"}
	_, err = f.runCLIWithStdin("glab", args, env, string(payload))
	return err
}

func (f *Forge) listBitbucketCommentsWithIDs(prNumber string) ([]PRCommentWithID, error) {
	base := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests/" + url.PathEscape(prNumber) + "/activities"
	var comments []PRCommentWithID
	start := 0
	const pageSize = 50
	for {
		api := fmt.Sprintf("%s?start=%d&limit=%d", base, start, pageSize)
		req, err := http.NewRequest(http.MethodGet, api, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+f.Token)
		req.Header.Set("Accept", "application/json")
		resp, err := f.client().Do(req)
		if err != nil {
			return nil, err
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("bitbucket list activities HTTP %d: %s", resp.StatusCode, string(raw))
		}
		var out struct {
			Values []struct {
				Action      string `json:"action"`
				CreatedDate int64  `json:"createdDate"`
				User        struct {
					Name string `json:"name"`
				} `json:"user"`
				Comment struct {
					ID   int    `json:"id"`
					Text string `json:"text"`
				} `json:"comment"`
			} `json:"values"`
			IsLastPage    bool `json:"isLastPage"`
			NextPageStart *int `json:"nextPageStart"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("decode bitbucket activities: %w", err)
		}
		for _, row := range out.Values {
			if row.Action != "COMMENTED" || strings.TrimSpace(row.Comment.Text) == "" {
				continue
			}
			comments = append(comments, PRCommentWithID{
				ID:       strconv.Itoa(row.Comment.ID),
				Body:     row.Comment.Text,
				Author:   row.User.Name,
				PostedAt: fmt.Sprint(row.CreatedDate),
			})
		}
		if out.IsLastPage || out.NextPageStart == nil {
			break
		}
		start = *out.NextPageStart
	}
	return comments, nil
}

func (f *Forge) postBitbucketComment(prNumber, body string) (string, error) {
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests/" + url.PathEscape(prNumber) + "/comments"
	payload, err := json.Marshal(map[string]string{"text": body})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, api, strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("bitbucket post comment HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var row struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return "", fmt.Errorf("decode bitbucket comment: %w", err)
	}
	if row.ID == 0 {
		return "", fmt.Errorf("bitbucket post comment: empty id")
	}
	return strconv.Itoa(row.ID), nil
}

func (f *Forge) updateBitbucketComment(prNumber, commentID, body string) error {
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests/" + url.PathEscape(prNumber) +
		"/comments/" + url.PathEscape(commentID)
	payload, err := json.Marshal(map[string]string{"text": body})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, api, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("bitbucket update comment HTTP %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

func decodeCommentIDLines(out string) ([]PRCommentWithID, error) {
	var comments []PRCommentWithID
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var row struct {
			ID        any    `json:"id"`
			Body      string `json:"body"`
			User      string `json:"user"`
			CreatedAt string `json:"created_at"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		comments = append(comments, PRCommentWithID{
			ID: fmt.Sprint(row.ID), Body: row.Body, Author: row.User, PostedAt: row.CreatedAt,
		})
	}
	return comments, nil
}

func (f *Forge) runCLIWithStdin(name string, args []string, env []string, stdin string) (string, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", fmt.Errorf("%s not found on PATH: %w", name, err)
	}
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	cmd.Stdin = strings.NewReader(stdin)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

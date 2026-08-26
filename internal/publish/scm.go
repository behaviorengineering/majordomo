package publish

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func publishGitHub(opts Options, summary string) error {
	if opts.GitHubToken == "" || opts.GitHubOwner == "" || opts.GitHubRepo == "" {
		return fmt.Errorf("github publish requires GITHUB_TOKEN and owner/repo (GITHUB_REPOSITORY)")
	}
	repo := opts.GitHubOwner + "/" + opts.GitHubRepo
	env := append([]string{}, os.Environ()...)
	env = append(env, "GH_TOKEN="+opts.GitHubToken, "GITHUB_TOKEN="+opts.GitHubToken)

	body := Marker + "\n" + opts.withLinks(summary)
	logf("INFO", "========== Publishing PR summary to GitHub via gh (mode: %s) ==========", opts.Mode)

	switch opts.Mode {
	case ModeComment:
		return ghComment(opts, env, repo, body)
	case ModeDescription:
		return ghEditBody(opts, env, repo, body)
	case ModeAuto:
		current, err := ghViewBody(opts, env, repo)
		if err != nil {
			return err
		}
		if ownedByMajordomo(current) {
			logf("INFO", "claiming/updating PR description")
			return ghEditBody(opts, env, repo, body)
		}
		logf("INFO", "PR description has user content — posting link comment")
		artifact := opts.SummaryArtifactURL
		if artifact == "" {
			return fmt.Errorf("SUMMARY_ARTIFACT_URL required when PR description is owned by someone else")
		}
		links := opts.reviewLinks(artifact)
		msg := "🤖 **Majordomo PR Review complete** — " + strings.Join(links, " · ")
		return ghComment(opts, env, repo, msg)
	}
	return nil
}

func ghRepoArgs(repo string) []string {
	return []string{"-R", repo}
}

func ghComment(opts Options, env []string, repo, body string) error {
	path, cleanup, err := writeTempBody(body)
	if err != nil {
		return err
	}
	defer cleanup()
	args := append([]string{"pr", "comment", opts.PRNumber, "--body-file", path}, ghRepoArgs(repo)...)
	_, err = opts.runCLI("gh", args, env)
	return err
}

func ghEditBody(opts Options, env []string, repo, body string) error {
	path, cleanup, err := writeTempBody(body)
	if err != nil {
		return err
	}
	defer cleanup()
	args := append([]string{"pr", "edit", opts.PRNumber, "--body-file", path}, ghRepoArgs(repo)...)
	_, err = opts.runCLI("gh", args, env)
	return err
}

func ghViewBody(opts Options, env []string, repo string) (string, error) {
	args := append([]string{"pr", "view", opts.PRNumber, "--json", "body", "-q", ".body"}, ghRepoArgs(repo)...)
	out, err := opts.runCLI("gh", args, env)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func publishGitLab(opts Options, summary string) error {
	if opts.GitLabToken == "" {
		return fmt.Errorf("gitlab publish requires GITLAB_TOKEN (or GLAB_TOKEN)")
	}
	repo := gitlabRepoSpec(opts)
	if repo == "" {
		return fmt.Errorf("gitlab publish requires owner/repo or GITLAB_PROJECT_ID / GITLAB_REPO")
	}
	env := append([]string{}, os.Environ()...)
	env = append(env, "GITLAB_TOKEN="+opts.GitLabToken, "GLAB_TOKEN="+opts.GitLabToken)
	if opts.GitLabHost != "" {
		host := strings.TrimPrefix(strings.TrimPrefix(opts.GitLabHost, "https://"), "http://")
		host = strings.TrimRight(host, "/")
		env = append(env, "GITLAB_HOST="+host)
	}

	body := Marker + "\n" + opts.withLinks(summary)
	logf("INFO", "========== Publishing MR summary to GitLab via glab (mode: %s) ==========", opts.Mode)

	switch opts.Mode {
	case ModeComment:
		return glabNote(opts, env, repo, body)
	case ModeDescription:
		return glabUpdateDesc(opts, env, repo, body)
	case ModeAuto:
		current, err := glabViewDesc(opts, env, repo)
		if err != nil {
			return err
		}
		if ownedByMajordomo(current) {
			logf("INFO", "claiming/updating MR description")
			return glabUpdateDesc(opts, env, repo, body)
		}
		logf("INFO", "MR description has user content — posting link note")
		artifact := opts.SummaryArtifactURL
		if artifact == "" {
			return fmt.Errorf("SUMMARY_ARTIFACT_URL required when MR description is owned by someone else")
		}
		links := opts.reviewLinks(artifact)
		msg := "🤖 **Majordomo PR Review complete** — " + strings.Join(links, " · ")
		return glabNote(opts, env, repo, msg)
	}
	return nil
}

func gitlabRepoSpec(opts Options) string {
	if id := strings.TrimSpace(opts.GitLabProjectID); id != "" {
		return id
	}
	if opts.GitHubOwner != "" && opts.GitHubRepo != "" {
		return opts.GitHubOwner + "/" + opts.GitHubRepo
	}
	return ""
}

func glabRepoArgs(repo string) []string {
	return []string{"-R", repo}
}

func glabNote(opts Options, env []string, repo, body string) error {
	args := append([]string{
		"mr", "note", "create", opts.PRNumber,
		"-m", body,
		"--resolvable=false",
	}, glabRepoArgs(repo)...)
	_, err := opts.runCLI("glab", args, env)
	return err
}

func glabUpdateDesc(opts Options, env []string, repo, body string) error {
	args := append([]string{
		"mr", "update", opts.PRNumber,
		"-d", body,
		"-y",
	}, glabRepoArgs(repo)...)
	_, err := opts.runCLI("glab", args, env)
	return err
}

func glabViewDesc(opts Options, env []string, repo string) (string, error) {
	args := append([]string{"mr", "view", opts.PRNumber, "-F", "json"}, glabRepoArgs(repo)...)
	out, err := opts.runCLI("glab", args, env)
	if err != nil {
		return "", err
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return "", fmt.Errorf("decode glab mr view json: %w", err)
	}
	if d, ok := raw["description"].(string); ok {
		return d, nil
	}
	return "", nil
}

func publishBitbucket(opts Options, summary string) error {
	if opts.BitbucketURL == "" || opts.BitbucketToken == "" || opts.BBProject == "" || opts.BBRepo == "" {
		return fmt.Errorf("bitbucket publish requires BITBUCKET_URL, BITBUCKET_TOKEN, BB_PROJECT, BB_REPO")
	}
	prURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%s",
		strings.TrimRight(opts.BitbucketURL, "/"), opts.BBProject, opts.BBRepo, opts.PRNumber)
	c := opts.client()
	logf("INFO", "========== Publishing PR summary to Bitbucket (mode: %s) ==========", opts.Mode)

	body := Marker + "\n" + opts.withLinks(summary)
	legacyBody := legacyMarker + "\n" + opts.withLinks(summary)

	putDesc := func(prMeta map[string]any, description string) error {
		payload := map[string]any{
			"version":     prMeta["version"],
			"title":       prMeta["title"],
			"description": description,
			"toRef":       prMeta["toRef"],
		}
		reviewers := []map[string]any{}
		if raw, ok := prMeta["reviewers"].([]any); ok {
			for _, r := range raw {
				if m, ok := r.(map[string]any); ok {
					if u, ok := m["user"]; ok {
						reviewers = append(reviewers, map[string]any{"user": u})
					}
				}
			}
		}
		payload["reviewers"] = reviewers
		_, _, err := httpJSON(c, "PUT", prURL, opts.BitbucketToken, payload)
		return err
	}

	postComment := func(text string) error {
		_, _, err := httpJSON(c, "POST", prURL+"/comments", opts.BitbucketToken, map[string]any{"text": text})
		return err
	}

	switch opts.Mode {
	case ModeComment:
		return postComment(opts.withLinks(summary))
	case ModeDescription:
		prMeta, _, err := httpJSON(c, "GET", prURL, opts.BitbucketToken, nil)
		if err != nil {
			return err
		}
		return putDesc(prMeta, body)
	case ModeAuto:
		prMeta, _, err := httpJSON(c, "GET", prURL, opts.BitbucketToken, nil)
		if err != nil {
			return err
		}
		current, _ := prMeta["description"].(string)
		if ownedByMajordomo(current) {
			logf("INFO", "claiming/updating PR description")
			if strings.Contains(current, legacyMarker) && !strings.Contains(current, Marker) {
				return putDesc(prMeta, legacyBody)
			}
			return putDesc(prMeta, body)
		}
		logf("INFO", "PR description has user content — posting link comment")
		if opts.SummaryArtifactURL == "" {
			return fmt.Errorf("SUMMARY_ARTIFACT_URL required when PR description is owned by someone else")
		}
		links := opts.reviewLinks(opts.SummaryArtifactURL)
		return postComment("🤖 **Majordomo PR Review complete** — " + strings.Join(links, " · "))
	}
	return nil
}

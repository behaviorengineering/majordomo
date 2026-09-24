// Package githttps builds git -c http.extraHeader args for forge HTTPS remotes.
package githttps

import (
	"encoding/base64"
	"strings"
)

// ExtraHeaderArgs returns git -c http.extraHeader=... args for forge HTTPS auth.
// An empty token yields nil (no auth config).
//
// GitHub and GitLab git smart HTTP expect HTTP Basic, not API-only headers:
// GitHub: username x-access-token (actions/checkout).
// GitLab: username oauth2 (same as clone-served-repo.sh URL embedding).
// PRIVATE-TOKEN is for the GitLab REST API only and is ignored by git HTTP.
func ExtraHeaderArgs(token, scm string) []string {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	var header string
	switch strings.ToLower(strings.TrimSpace(scm)) {
	case "gitlab":
		basic := base64.StdEncoding.EncodeToString([]byte("oauth2:" + token))
		header = "Authorization: Basic " + basic
	case "bitbucket":
		basic := base64.StdEncoding.EncodeToString([]byte("x-token-auth:" + token))
		header = "Authorization: Basic " + basic
	default:
		basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		header = "Authorization: Basic " + basic
	}
	return []string{"-c", "http.extraHeader=" + header}
}

// InferSCM guesses the forge from an HTTPS remote URL host.
func InferSCM(remoteURL string) string {
	u := strings.ToLower(remoteURL)
	switch {
	case strings.Contains(u, "gitlab"):
		return "gitlab"
	case strings.Contains(u, "bitbucket"):
		return "bitbucket"
	default:
		return "github"
	}
}

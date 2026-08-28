package contextdigest

import (
	"strings"

	"github.com/behaviorengineering/majordomo/internal/config"
)

// RepoTarget is one served repo the digest cron may run for.
type RepoTarget struct {
	RepoID   string `json:"repo_id"`
	SCM      string `json:"scm"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	CloneURL string `json:"clone_url"`
}

// ReposResult is JSON output for majordomo context repos.
type ReposResult struct {
	Repos []RepoTarget `json:"repos"`
}

// ListTargets returns non-generic served repos with a clone URL (excludes example placeholders).
func ListTargets(configDir string) (ReposResult, error) {
	configs, err := config.LoadAll(configDir)
	if err != nil {
		return ReposResult{}, err
	}
	out := ReposResult{Repos: make([]RepoTarget, 0, len(configs))}
	for _, cfg := range configs {
		scm := strings.ToLower(strings.TrimSpace(cfg.SCM))
		if scm == "" {
			scm = "github"
		}
		if scm == "generic" {
			continue
		}
		cloneURL := strings.TrimSpace(cfg.Repository.CloneURL)
		if cloneURL == "" {
			continue
		}
		repoID := cfg.Repository.ID
		if repoID == "" || isPlaceholderRepo(repoID, cfg) {
			continue
		}
		owner, name := cfg.Repository.Owner, cfg.Repository.Name
		if owner == "" || name == "" {
			owner, name = splitOwnerName(cloneURL)
		}
		if isPlaceholderOwner(owner) {
			continue
		}
		out.Repos = append(out.Repos, RepoTarget{
			RepoID:   repoID,
			SCM:      scm,
			Owner:    owner,
			Name:     name,
			CloneURL: cloneURL,
		})
	}
	return out, nil
}

func isPlaceholderRepo(repoID string, cfg config.RepoConfig) bool {
	if strings.HasPrefix(repoID, "example-") {
		return true
	}
	return strings.Contains(strings.ToUpper(cfg.Repository.CloneURL), "YOUR_")
}

func isPlaceholderOwner(owner string) bool {
	up := strings.ToUpper(strings.TrimSpace(owner))
	return strings.Contains(up, "YOUR_") || up == "YOUR_ORG"
}

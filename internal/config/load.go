package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultsFilename is the org-wide defaults file (not a served repo).
const DefaultsFilename = "_defaults.yaml"

// LoadDefaults reads _defaults.yaml if present.
func LoadDefaults(configDir string) (RepoConfig, error) {
	path := filepath.Join(configDir, DefaultsFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RepoConfig{
				Trigger: Trigger{Interval: "5m", Push: TriggerPush{Mode: PushNone}},
			}, nil
		}
		return RepoConfig{}, err
	}
	var c RepoConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return RepoConfig{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// LoadRepoFile loads one served-repo YAML and merges defaults.
func LoadRepoFile(configDir, repoID string, defaults RepoConfig) (RepoConfig, error) {
	path := filepath.Join(configDir, repoID+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return RepoConfig{}, err
	}
	var c RepoConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return RepoConfig{}, fmt.Errorf("%s: %w", path, err)
	}
	merged := mergeConfig(defaults, c)
	if merged.Repository.ID == "" {
		merged.Repository.ID = repoID
	}
	if merged.PollCache.Branch == "" {
		merged.PollCache.Branch = PollCacheBranch(merged.Repository.ID)
	}
	return merged, nil
}

// ListRepoIDs returns served repo ids from *.yaml excluding _defaults.
func ListRepoIDs(configDir string) ([]string, error) {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		if e.Name() == DefaultsFilename {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".yaml"))
	}
	return ids, nil
}

// LoadAll loads defaults + every served repo config.
func LoadAll(configDir string) ([]RepoConfig, error) {
	defaults, err := LoadDefaults(configDir)
	if err != nil {
		return nil, err
	}
	ids, err := ListRepoIDs(configDir)
	if err != nil {
		return nil, err
	}
	out := make([]RepoConfig, 0, len(ids))
	for _, id := range ids {
		c, err := LoadRepoFile(configDir, id, defaults)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func mergeConfig(base, over RepoConfig) RepoConfig {
	out := base
	if over.SCM != "" {
		out.SCM = over.SCM
	}
	if over.Repository.ID != "" || over.Repository.CloneURL != "" || over.Repository.Owner != "" || over.Repository.Name != "" {
		if over.Repository.ID != "" {
			out.Repository.ID = over.Repository.ID
		}
		if over.Repository.CloneURL != "" {
			out.Repository.CloneURL = over.Repository.CloneURL
		}
		if over.Repository.Owner != "" {
			out.Repository.Owner = over.Repository.Owner
		}
		if over.Repository.Name != "" {
			out.Repository.Name = over.Repository.Name
		}
	}
	if over.SCMAPI.BaseURL != "" {
		out.SCMAPI.BaseURL = over.SCMAPI.BaseURL
	}
	if over.SCMAPI.ProjectID != "" {
		out.SCMAPI.ProjectID = over.SCMAPI.ProjectID
	}
	if over.Trigger.Interval != "" {
		out.Trigger.Interval = over.Trigger.Interval
	}
	if over.Trigger.Push.Mode != "" {
		out.Trigger.Push.Mode = over.Trigger.Push.Mode
	}
	if over.Trigger.Poll != nil {
		out.Trigger.Poll = over.Trigger.Poll
	}
	if over.Cache.Repo != "" {
		out.Cache.Repo = over.Cache.Repo
	}
	if over.Cache.Dir != "" {
		out.Cache.Dir = over.Cache.Dir
	}
	if over.Cache.RetentionDays != 0 {
		out.Cache.RetentionDays = over.Cache.RetentionDays
	}
	out.Cache.EnableSkips = over.Cache.EnableSkips || base.Cache.EnableSkips
	if over.PollCache.Repo != "" {
		out.PollCache.Repo = over.PollCache.Repo
	}
	if over.PollCache.Branch != "" {
		out.PollCache.Branch = over.PollCache.Branch
	}
	if over.PublishMode != "" {
		out.PublishMode = over.PublishMode
	}
	return out
}

// CredentialEnvName returns MAJORDOMO_CREDENTIAL__<REPO_ID> with non-alnum → _.
func CredentialEnvName(repoID string) string {
	var b strings.Builder
	b.WriteString("MAJORDOMO_CREDENTIAL__")
	for _, r := range strings.ToUpper(repoID) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// Package config loads and merges majordomo-central-config YAML.
package config

// TriggerPushMode selects optional push accelerators on top of always-on poll.
type TriggerPushMode string

const (
	PushNone     TriggerPushMode = "none"
	PushWorkflow TriggerPushMode = "workflow"
	PushWebhook  TriggerPushMode = "webhook"
)

// Trigger is how the control tower discovers work for a served repo.
type Trigger struct {
	Poll     bool            `yaml:"poll"`
	Interval string          `yaml:"interval"`
	Push     TriggerPush     `yaml:"push"`
}

// TriggerPush is the optional fast path; poll still reconciles.
type TriggerPush struct {
	Mode TriggerPushMode `yaml:"mode"`
}

// Cache is cluster review-cache settings on the served repo.
type Cache struct {
	Repo          string `yaml:"repo"` // served
	Dir           string `yaml:"dir"`
	RetentionDays int    `yaml:"retentionDays"`
	EnableSkips   bool   `yaml:"enableSkips"`
}

// PollCache is the head_sha cursor for poll reconciliation on the served repo.
type PollCache struct {
	Repo   string `yaml:"repo"`   // served
	Branch string `yaml:"branch"` // majordomo-poll-cache/<repo-id>
}

// Repository identifies the served git remote.
type Repository struct {
	ID       string `yaml:"id"`
	CloneURL string `yaml:"cloneUrl"`
	Owner    string `yaml:"owner,omitempty"`
	Name     string `yaml:"name,omitempty"`
}

// SCMAPI holds forge API endpoints for list/publish.
type SCMAPI struct {
	BaseURL   string `yaml:"baseUrl"`
	ProjectID string `yaml:"projectId,omitempty"`
}

// RepoConfig is one file under majordomo-central-config/<repo-id>.yaml.
type RepoConfig struct {
	SCM         string     `yaml:"scm"` // github | gitlab | bitbucket | generic
	Repository  Repository `yaml:"repository"`
	SCMAPI      SCMAPI     `yaml:"scmApi"`
	Trigger     Trigger    `yaml:"trigger"`
	Cache       Cache      `yaml:"cache"`
	PollCache   PollCache  `yaml:"pollCache"`
	PublishMode string     `yaml:"publishMode,omitempty"`
}

// CacheBranch returns the review-cache git branch for projectID.
func CacheBranch(projectID string) string {
	return "majordomo-pr-reviewer-cache/" + projectID
}

// PollCacheBranch returns the poll-cursor git branch for repoID.
func PollCacheBranch(repoID string) string {
	return "majordomo-poll-cache/" + repoID
}

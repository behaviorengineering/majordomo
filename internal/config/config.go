// Package config loads and merges majordomo-central-config YAML.
package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// TriggerPushMode selects optional push accelerators on top of always-on poll.
type TriggerPushMode string

const (
	PushNone     TriggerPushMode = "none"
	PushWorkflow TriggerPushMode = "workflow"
	PushWebhook  TriggerPushMode = "webhook"
)

// Trigger is how the control tower discovers work for a served repo.
type Trigger struct {
	Poll     *bool       `yaml:"poll"`
	Interval string      `yaml:"interval"`
	Push     TriggerPush `yaml:"push"`
}

// PollEnabled returns whether poll is on (default true).
func (t Trigger) PollEnabled() bool {
	if t.Poll == nil {
		return true
	}
	return *t.Poll
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
	// DisableSkips opts out of analysis-cache skips (skips are on by default).
	DisableSkips bool `yaml:"disableSkips"`
}

// SkipsEnabled reports whether cluster analysis cache hits may skip re-analysis.
func (c Cache) SkipsEnabled() bool {
	return !c.DisableSkips
}

// PollCache is the head_sha cursor for poll reconciliation on the served repo.
type PollCache struct {
	Repo   string `yaml:"repo"`   // served
	Branch string `yaml:"branch"` // majordomo-poll-cache/<repo-id>
}

// Context is the served-repo orphan branch for human-reviewed project understanding.
type Context struct {
	Repo              string `yaml:"repo"`   // served
	Branch            string `yaml:"branch"` // majordomo-context/<repo-id>
	AutoMerge         *bool  `yaml:"autoMerge,omitempty"`
	GateCommentPrefix string `yaml:"gateCommentPrefix,omitempty"`
	Compaction        ContextCompaction    `yaml:"compaction,omitempty"`
	MaxCommitsPerRun  int                  `yaml:"maxCommitsPerRun,omitempty"`
}

const defaultMaxCommitsPerRun = 20

// MaxCommitsPerRunLimit returns the per-digest first-parent walk cap (0 = unlimited).
func (c Context) MaxCommitsPerRunLimit() int {
	if c.MaxCommitsPerRun > 0 {
		return c.MaxCommitsPerRun
	}
	return defaultMaxCommitsPerRun
}

// ContextCompaction controls teaching-document compaction during digest.
type ContextCompaction struct {
	MaxChronologyEntries int `yaml:"maxChronologyEntries,omitempty"`
	KeepRecentEntries    int `yaml:"keepRecentEntries,omitempty"`
}

// AutoMergeEnabled reports whether digest may merge context PRs when gate is done.
func (c Context) AutoMergeEnabled() bool {
	return c.AutoMerge != nil && *c.AutoMerge
}

// GatePrefix returns the @majordomo comment prefix.
func (c Context) GatePrefix() string {
	if p := strings.TrimSpace(c.GateCommentPrefix); p != "" {
		return p
	}
	return "@majordomo"
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

// Review is publish and re-run policy for a served repo.
type Review struct {
	PublishMode          string `yaml:"publishMode,omitempty"`          // auto | comment | description | check | off
	EnableContinuousRuns *bool  `yaml:"enableContinuousRuns,omitempty"` // nil/false: one review per PR; true: re-queue when head_sha changes
}

// ContinuousRunsEnabled reports whether new commits on an open PR should re-queue review.
// Default is false (conservative): review once per PR number until cursor is cleared.
func (r Review) ContinuousRunsEnabled() bool {
	return r.EnableContinuousRuns != nil && *r.EnableContinuousRuns
}

// StaticAnalysisTool is one SA tool entry under staticAnalysis:.
type StaticAnalysisTool struct {
	Tool       string `yaml:"tool,omitempty"`       // slug for .sa/<tool>.txt (default from dockerfile/image)
	Dockerfile string `yaml:"dockerfile,omitempty"` // path under majordomo repo
	Image      string `yaml:"image,omitempty"`      // full image ref (preferred at run time)
	Command    string `yaml:"command"`
	Glob       string `yaml:"glob"`
}

// PipelineRoutingEntry is one skill's globs (and optional persona) under pipelines.*.routing.
type PipelineRoutingEntry struct {
	Globs   []string
	Persona string
}

// OrderedRouting preserves YAML skill key order for first-match-wins routing.
type OrderedRouting struct {
	Keys  []string
	Rules map[string]PipelineRoutingEntry
}

// Empty reports whether no skills are configured.
func (o OrderedRouting) Empty() bool {
	return len(o.Keys) == 0
}

// UnmarshalYAML decodes a mapping of skill → glob list or {globs, persona}.
func (o *OrderedRouting) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Kind == 0 {
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("routing must be a mapping")
	}
	o.Keys = nil
	o.Rules = map[string]PipelineRoutingEntry{}
	for i := 0; i+1 < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]
		key := keyNode.Value
		var entry PipelineRoutingEntry
		switch valNode.Kind {
		case yaml.SequenceNode:
			var globs []string
			if err := valNode.Decode(&globs); err != nil {
				return fmt.Errorf("routing[%s]: %w", key, err)
			}
			entry.Globs = globs
		case yaml.MappingNode:
			var m struct {
				Globs   []string `yaml:"globs"`
				Persona string   `yaml:"persona"`
			}
			if err := valNode.Decode(&m); err != nil {
				return fmt.Errorf("routing[%s]: %w", key, err)
			}
			entry.Globs = m.Globs
			entry.Persona = m.Persona
		default:
			return fmt.Errorf("routing[%s]: expected list of globs or map with globs", key)
		}
		o.Keys = append(o.Keys, key)
		o.Rules[key] = entry
	}
	return nil
}

// Pipeline is one named pipeline under pipelines: (e.g. pr-review).
type Pipeline struct {
	Model        string             `yaml:"model,omitempty"`
	ScoreModel   string             `yaml:"scoreModel,omitempty"`
	Agent        string             `yaml:"agent,omitempty"`
	Routing      OrderedRouting     `yaml:"routing,omitempty"`
	AgentContext map[string]any     `yaml:"agentContext,omitempty"`
	Skills       map[string]*string `yaml:"skills,omitempty"` // null = submodule default
}

// RepoConfig is one file under majordomo-central-config/<repo-id>.yaml.
type RepoConfig struct {
	SCM            string               `yaml:"scm"` // github | gitlab | bitbucket | generic
	Repository     Repository           `yaml:"repository"`
	SCMAPI         SCMAPI               `yaml:"scmApi"`
	Trigger        Trigger              `yaml:"trigger"`
	Cache          Cache                `yaml:"cache"`
	PollCache      PollCache            `yaml:"pollCache"`
	Context        Context              `yaml:"context"`
	Review         Review               `yaml:"review"`
	PublishMode    string               `yaml:"publishMode,omitempty"` // legacy alias
	StaticAnalysis []StaticAnalysisTool `yaml:"staticAnalysis,omitempty"`
	Pipelines      map[string]Pipeline  `yaml:"pipelines,omitempty"`
}

// EffectivePublishMode returns review.publishMode, else top-level publishMode, else "auto".
func (c RepoConfig) EffectivePublishMode() string {
	if strings.TrimSpace(c.Review.PublishMode) != "" {
		return strings.TrimSpace(c.Review.PublishMode)
	}
	if strings.TrimSpace(c.PublishMode) != "" {
		return strings.TrimSpace(c.PublishMode)
	}
	return "auto"
}

// PipelineNamed returns pipelines[name] if present.
func (c RepoConfig) PipelineNamed(name string) (Pipeline, bool) {
	if c.Pipelines == nil {
		return Pipeline{}, false
	}
	p, ok := c.Pipelines[name]
	return p, ok
}

// CacheBranch returns the review-cache git branch for projectID.
func CacheBranch(projectID string) string {
	return "majordomo-pr-reviewer-cache/" + projectID
}

// PollCacheBranch returns the poll-cursor git branch for repoID.
func PollCacheBranch(repoID string) string {
	return "majordomo-poll-cache/" + repoID
}

// ContextBranch returns the repo-context git branch for repoID.
func ContextBranch(repoID string) string {
	return "majordomo-context/" + repoID
}

// ContextUpdateBranch returns the catch-up head branch for repoID.
// Uses a hyphen suffix because git cannot hold both refs/heads/majordomo-context/id
// and refs/heads/majordomo-context/id/update when the base branch exists.
func ContextUpdateBranch(repoID string) string {
	return ContextBranch(repoID) + "-update"
}

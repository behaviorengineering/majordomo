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
	out.Cache.DisableSkips = over.Cache.DisableSkips || base.Cache.DisableSkips
	if over.PollCache.Repo != "" {
		out.PollCache.Repo = over.PollCache.Repo
	}
	if over.PollCache.Branch != "" {
		out.PollCache.Branch = over.PollCache.Branch
	}
	if over.Review.PublishMode != "" {
		out.Review.PublishMode = over.Review.PublishMode
	}
	if over.Review.EnableContinuousRuns != nil {
		out.Review.EnableContinuousRuns = over.Review.EnableContinuousRuns
	}
	if over.PublishMode != "" {
		out.PublishMode = over.PublishMode
	}
	if len(over.StaticAnalysis) > 0 {
		out.StaticAnalysis = append([]StaticAnalysisTool(nil), over.StaticAnalysis...)
	}
	out.Pipelines = mergePipelines(base.Pipelines, over.Pipelines)
	return out
}

func mergePipelines(base, over map[string]Pipeline) map[string]Pipeline {
	if len(over) == 0 {
		return clonePipelines(base)
	}
	out := clonePipelines(base)
	if out == nil {
		out = map[string]Pipeline{}
	}
	for name, op := range over {
		bp, ok := out[name]
		if !ok {
			out[name] = clonePipeline(op)
			continue
		}
		out[name] = mergePipeline(bp, op)
	}
	return out
}

func mergePipeline(base, over Pipeline) Pipeline {
	out := clonePipeline(base)
	if over.Model != "" {
		out.Model = over.Model
	}
	if over.ScoreModel != "" {
		out.ScoreModel = over.ScoreModel
	}
	if over.Agent != "" {
		out.Agent = over.Agent
	}
	if !over.Routing.Empty() {
		out.Routing = cloneOrderedRouting(over.Routing)
	}
	if over.AgentContext != nil {
		out.AgentContext = cloneAnyMap(over.AgentContext)
	}
	if over.Skills != nil {
		out.Skills = cloneSkills(over.Skills)
	}
	return out
}

func clonePipelines(in map[string]Pipeline) map[string]Pipeline {
	if in == nil {
		return nil
	}
	out := make(map[string]Pipeline, len(in))
	for k, v := range in {
		out[k] = clonePipeline(v)
	}
	return out
}

func clonePipeline(p Pipeline) Pipeline {
	return Pipeline{
		Model:        p.Model,
		ScoreModel:   p.ScoreModel,
		Agent:        p.Agent,
		Routing:      cloneOrderedRouting(p.Routing),
		AgentContext: cloneAnyMap(p.AgentContext),
		Skills:       cloneSkills(p.Skills),
	}
}

func cloneOrderedRouting(r OrderedRouting) OrderedRouting {
	out := OrderedRouting{
		Keys:  append([]string(nil), r.Keys...),
		Rules: make(map[string]PipelineRoutingEntry, len(r.Rules)),
	}
	for k, v := range r.Rules {
		out.Rules[k] = PipelineRoutingEntry{
			Globs:   append([]string(nil), v.Globs...),
			Persona: v.Persona,
		}
	}
	return out
}

func cloneSkills(in map[string]*string) map[string]*string {
	if in == nil {
		return nil
	}
	out := make(map[string]*string, len(in))
	for k, v := range in {
		if v == nil {
			out[k] = nil
			continue
		}
		s := *v
		out[k] = &s
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	// Shallow clone is enough: materialize re-encodes to JSON; overrides replace the whole map.
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// LoadMerged loads _defaults.yaml + <repoID>.yaml.
func LoadMerged(configDir, repoID string) (RepoConfig, error) {
	defaults, err := LoadDefaults(configDir)
	if err != nil {
		return RepoConfig{}, err
	}
	return LoadRepoFile(configDir, repoID, defaults)
}

// secretKeySuffix uppercases s and maps non-alnum runes to _.
func secretKeySuffix(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// CredentialEnvName returns MAJORDOMO_CREDENTIAL_<REPO_ID> with non-alnum → _.
// Optional per-repo override when one served repo must not share the org token.
func CredentialEnvName(repoID string) string {
	return "MAJORDOMO_CREDENTIAL_" + secretKeySuffix(repoID)
}

// OrgCredentialEnvName returns the per-org/group tower secret env name.
// GitHub: GH_TOKEN_<OWNER> (not GITHUB_TOKEN_*; Actions forbids GITHUB_ secret names).
// GitLab: GITLAB_TOKEN_<OWNER> (nested owners like group/sub → GROUP_SUB).
func OrgCredentialEnvName(scm, owner string) string {
	suffix := secretKeySuffix(owner)
	switch strings.ToLower(strings.TrimSpace(scm)) {
	case "gitlab":
		return "GITLAB_TOKEN_" + suffix
	default:
		return "GH_TOKEN_" + suffix
	}
}

// ResolveCredential returns the SCM token for a served repo.
// Order: MAJORDOMO_CREDENTIAL_<repo_id>, then GH_TOKEN_<owner> / GITLAB_TOKEN_<owner>.
// Unqualified GITHUB_TOKEN / GH_TOKEN / GITLAB_TOKEN are intentionally not used.
func ResolveCredential(repoID, scm, owner string) string {
	if v := strings.TrimSpace(os.Getenv(CredentialEnvName(repoID))); v != "" {
		return v
	}
	if strings.TrimSpace(owner) == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(OrgCredentialEnvName(scm, owner)))
}

// CredentialHint names the env vars operators should set when ResolveCredential is empty.
func CredentialHint(repoID, scm, owner string) string {
	perRepo := CredentialEnvName(repoID)
	if strings.TrimSpace(owner) == "" {
		return perRepo + " (and set repository.owner for org token)"
	}
	return perRepo + " or " + OrgCredentialEnvName(scm, owner)
}

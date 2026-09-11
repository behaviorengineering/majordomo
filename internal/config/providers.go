package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	stropdspy "github.com/behaviorengineering/strop/dspy"
)

// Job keys for task/module provider selection (job_configs.<job>.modules.<task>).
const (
	JobContextDigest = "context-digest"
	JobPRReview      = "pr-review"
)

// AIProviderConfig is one named LLM backend under ai_providers.
type AIProviderConfig struct {
	APIKey           string           `yaml:"api_key"`
	Model            string           `yaml:"model"`
	BaseURL          string           `yaml:"base_url"`
	Timeout          string           `yaml:"timeout,omitempty"`
	RateLimit        int              `yaml:"rate_limit,omitempty"`
	APISchema        string           `yaml:"api_schema"`
	MaxContextTokens int              `yaml:"max_context_tokens,omitempty"`
	MaxOutputTokens  int              `yaml:"max_output_tokens,omitempty"`
	Grounding        *GroundingConfig `yaml:"grounding,omitempty"`
}

// GroundingConfig enables Google Gemini search grounding when present.
type GroundingConfig struct {
	DynamicThreshold float64 `yaml:"dynamic_threshold,omitempty"`
}

// JobConfig holds module provider references for one job.
type JobConfig struct {
	Modules map[string]ModuleTaskConfig `yaml:"modules,omitempty"`
}

// ModuleTaskConfig references a provider by name for one module/task.
type ModuleTaskConfig struct {
	Provider string `yaml:"provider"`
}

var envPlaceholderRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// JobForTask maps a generator task name to its job_configs key.
func JobForTask(task string) string {
	switch strings.TrimSpace(task) {
	case "bootstrap_story", "digest_story", "typology_inspect", "typology_objective_grounding", "typology_cluster", "typology_refine",
		"typology_human_intervention",
		"typology_intervention_journey", "typology_intervention_brief",
		"typology_intervention_weaknesses", "typology_intervention_pr_priority",
		"typology_finding_comment":
		return JobContextDigest
	case "filereview", "summary", "technical":
		return JobPRReview
	default:
		return ""
	}
}

// GetAIProvider returns a named provider after expanding ${ENV} placeholders.
func (c RepoConfig) GetAIProvider(name string) (AIProviderConfig, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return AIProviderConfig{}, fmt.Errorf("ai provider name is required")
	}
	if c.AIProviders == nil {
		return AIProviderConfig{}, fmt.Errorf("no ai_providers configured")
	}
	provider, ok := c.AIProviders[name]
	if !ok {
		return AIProviderConfig{}, fmt.Errorf("AI provider %q not found", name)
	}
	provider = expandProvider(provider)
	if err := provider.Validate(); err != nil {
		return AIProviderConfig{}, fmt.Errorf("invalid AI provider %q: %w", name, err)
	}
	return provider, nil
}

// GetModuleProvider resolves job_configs.<job>.modules.<module>.provider.
func (c RepoConfig) GetModuleProvider(job, module string) (AIProviderConfig, error) {
	job = strings.TrimSpace(job)
	module = strings.TrimSpace(module)
	if job == "" || module == "" {
		return AIProviderConfig{}, fmt.Errorf("job and module are required")
	}
	if c.JobConfigs == nil {
		return AIProviderConfig{}, fmt.Errorf("no job_configs configured")
	}
	jobCfg, ok := c.JobConfigs[job]
	if !ok {
		return AIProviderConfig{}, fmt.Errorf("job %q not found in job_configs", job)
	}
	if jobCfg.Modules == nil {
		return AIProviderConfig{}, fmt.Errorf("job %q has no modules", job)
	}
	modCfg, ok := jobCfg.Modules[module]
	if !ok || strings.TrimSpace(modCfg.Provider) == "" {
		return AIProviderConfig{}, fmt.Errorf("job %q module %q has no provider", job, module)
	}
	return c.GetAIProvider(modCfg.Provider)
}

// ResolveTaskProvider returns the configured provider for a generator task.
// ok is false when the task has no job_configs entry (caller may use gateway fallback).
func (c RepoConfig) ResolveTaskProvider(task string) (AIProviderConfig, bool, error) {
	task = strings.TrimSpace(task)
	job := JobForTask(task)
	if job == "" {
		return AIProviderConfig{}, false, fmt.Errorf("unknown generator task %q", task)
	}
	if c.JobConfigs == nil {
		return AIProviderConfig{}, false, nil
	}
	jobCfg, ok := c.JobConfigs[job]
	if !ok || jobCfg.Modules == nil {
		return AIProviderConfig{}, false, nil
	}
	modCfg, ok := jobCfg.Modules[task]
	if !ok || strings.TrimSpace(modCfg.Provider) == "" {
		return AIProviderConfig{}, false, nil
	}
	provider, err := c.GetAIProvider(modCfg.Provider)
	if err != nil {
		return AIProviderConfig{}, true, err
	}
	return provider, true, nil
}

// Validate checks required provider fields after env expansion.
func (p AIProviderConfig) Validate() error {
	if strings.TrimSpace(p.APIKey) == "" {
		return fmt.Errorf("api_key is required")
	}
	if strings.TrimSpace(p.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(p.BaseURL) == "" {
		return fmt.Errorf("base_url is required")
	}
	if strings.TrimSpace(p.APISchema) == "" {
		return fmt.Errorf("api_schema is required")
	}
	if unresolved := unresolvedPlaceholders(p.APIKey); unresolved != "" {
		return fmt.Errorf("unresolved env placeholder in api_key: %s", unresolved)
	}
	if unresolved := unresolvedPlaceholders(p.BaseURL); unresolved != "" {
		return fmt.Errorf("unresolved env placeholder in base_url: %s", unresolved)
	}
	return nil
}

// GetTimeout returns the per-attempt timeout, or moduleTimeout when unset/invalid.
func (p AIProviderConfig) GetTimeout(moduleTimeout time.Duration) time.Duration {
	if strings.TrimSpace(p.Timeout) == "" {
		return moduleTimeout
	}
	parsed, err := time.ParseDuration(p.Timeout)
	if err != nil {
		return moduleTimeout
	}
	return parsed
}

// ToStrop maps the YAML provider into the portable strop DTO.
func (p AIProviderConfig) ToStrop() stropdspy.ProviderConfig {
	out := stropdspy.ProviderConfig{
		APIKey:           strings.TrimSpace(p.APIKey),
		Model:            strings.TrimSpace(p.Model),
		BaseURL:          strings.TrimSpace(p.BaseURL),
		Timeout:          strings.TrimSpace(p.Timeout),
		RateLimit:        p.RateLimit,
		APISchema:        strings.TrimSpace(p.APISchema),
		MaxContextTokens: p.MaxContextTokens,
		MaxOutputTokens:  p.MaxOutputTokens,
	}
	if p.Grounding != nil {
		out.Grounding = &stropdspy.GroundingConfig{DynamicThreshold: p.Grounding.DynamicThreshold}
	}
	return out
}

// UsesEmbeddedGateway reports whether the provider should dial the local Bifrost loopback.
// Empty base_url is rejected by Validate; use the sentinel "gateway" or "embedded".
func (p AIProviderConfig) UsesEmbeddedGateway() bool {
	base := strings.ToLower(strings.TrimSpace(p.BaseURL))
	return base == "gateway" || base == "embedded"
}

func expandProvider(p AIProviderConfig) AIProviderConfig {
	p.APIKey = expandEnvVars(p.APIKey)
	p.Model = expandEnvVars(p.Model)
	p.BaseURL = expandEnvVars(p.BaseURL)
	p.Timeout = expandEnvVars(p.Timeout)
	p.APISchema = expandEnvVars(p.APISchema)
	return p
}

func expandEnvVars(s string) string {
	return strings.TrimSpace(envPlaceholderRE.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-1]
		return os.Getenv(name)
	}))
}

func unresolvedPlaceholders(s string) string {
	m := envPlaceholderRE.FindString(s)
	return m
}

func cloneAIProviders(in map[string]AIProviderConfig) map[string]AIProviderConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]AIProviderConfig, len(in))
	for k, v := range in {
		cp := v
		if v.Grounding != nil {
			g := *v.Grounding
			cp.Grounding = &g
		}
		out[k] = cp
	}
	return out
}

func cloneJobConfigs(in map[string]JobConfig) map[string]JobConfig {
	if in == nil {
		return nil
	}
	out := make(map[string]JobConfig, len(in))
	for k, v := range in {
		jc := JobConfig{}
		if v.Modules != nil {
			jc.Modules = make(map[string]ModuleTaskConfig, len(v.Modules))
			for mk, mv := range v.Modules {
				jc.Modules[mk] = mv
			}
		}
		out[k] = jc
	}
	return out
}

func mergeAIProviders(base, over map[string]AIProviderConfig) map[string]AIProviderConfig {
	if len(over) == 0 {
		return cloneAIProviders(base)
	}
	out := cloneAIProviders(base)
	if out == nil {
		out = map[string]AIProviderConfig{}
	}
	for name, op := range over {
		out[name] = op
	}
	return out
}

func mergeJobConfigs(base, over map[string]JobConfig) map[string]JobConfig {
	if len(over) == 0 {
		return cloneJobConfigs(base)
	}
	out := cloneJobConfigs(base)
	if out == nil {
		out = map[string]JobConfig{}
	}
	for job, oj := range over {
		bj, ok := out[job]
		if !ok {
			out[job] = cloneJobConfigs(map[string]JobConfig{job: oj})[job]
			continue
		}
		if bj.Modules == nil {
			bj.Modules = map[string]ModuleTaskConfig{}
		}
		for mod, om := range oj.Modules {
			bj.Modules[mod] = om
		}
		out[job] = bj
	}
	return out
}

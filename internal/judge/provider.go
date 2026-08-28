package judge

import (
	"fmt"
	"os"
	"strings"
	"time"

	stropdspy "github.com/behaviorengineering/strop/dspy"
)

const (
	defaultAnthropicModel = "claude-sonnet-4-20250514"
	defaultOpenAIModel    = "gpt-4.1-mini"
	defaultModuleTimeout  = 2 * time.Minute
)

// ResolveProvider picks an LLM provider from standard env keys.
func ResolveProvider() (stropdspy.ProviderConfig, error) {
	if key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); key != "" {
		model := envOr("MAJORDOMO_MODEL", defaultAnthropicModel)
		return stropdspy.ProviderConfig{
			APIKey:    key,
			Model:     model,
			BaseURL:   "https://api.anthropic.com",
			APISchema: "anthropic",
			Timeout:   "120s",
		}, nil
	}
	if key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); key != "" {
		model := envOr("MAJORDOMO_MODEL", defaultOpenAIModel)
		return stropdspy.ProviderConfig{
			APIKey:    key,
			Model:     model,
			BaseURL:   "https://api.openai.com/v1",
			APISchema: "openai",
			Timeout:   "120s",
		}, nil
	}
	return stropdspy.ProviderConfig{}, fmt.Errorf("judge: no LLM provider key (set ANTHROPIC_API_KEY or OPENAI_API_KEY)")
}

// LLMConfigured reports whether a provider key is present.
func LLMConfigured() bool {
	_, err := ResolveProvider()
	return err == nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

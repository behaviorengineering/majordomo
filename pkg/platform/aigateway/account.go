package aigateway

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

const (
	envAnthropic = "ANTHROPIC_API_KEY"
	envOpenAI    = "OPENAI_API_KEY"
	envGemini    = "GEMINI_API_KEY"
	envGoogle    = "GOOGLE_API_KEY"
	envGoogleAI  = "GOOGLE_GENERATIVE_AI_API_KEY"
)

// Account feeds Bifrost with env-backed provider keys.
type Account struct {
	providers []schemas.ModelProvider
	keys      map[schemas.ModelProvider][]schemas.Key
}

// NewAccountFromEnv builds an Account from standard provider env keys.
// Returns an error when no provider key is present.
func NewAccountFromEnv() (*Account, error) {
	a := &Account{
		keys: make(map[schemas.ModelProvider][]schemas.Key),
	}
	if v := strings.TrimSpace(os.Getenv(envAnthropic)); v != "" {
		a.add(schemas.Anthropic, v)
	}
	if v := strings.TrimSpace(os.Getenv(envOpenAI)); v != "" {
		a.add(schemas.OpenAI, v)
	}
	if v := geminiKey(); v != "" {
		a.add(schemas.Gemini, v)
	}
	if len(a.providers) == 0 {
		return nil, fmt.Errorf("aigateway: no LLM provider keys (set %s, %s, or %s)", envAnthropic, envOpenAI, envGemini)
	}
	return a, nil
}

func geminiKey() string {
	for _, k := range []string{envGemini, envGoogleAI, envGoogle} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func (a *Account) add(provider schemas.ModelProvider, key string) {
	a.providers = append(a.providers, provider)
	a.keys[provider] = []schemas.Key{{
		ID:     string(provider),
		Name:   string(provider),
		Value:  schemas.SecretVar{Val: key},
		Models: schemas.WhiteList{"*"},
		Weight: 1.0,
	}}
}

// HasProviders reports whether at least one real key is configured.
func (a *Account) HasProviders() bool {
	return a != nil && len(a.providers) > 0
}

// Providers returns configured providers in discovery order.
func (a *Account) Providers() []schemas.ModelProvider {
	if a == nil {
		return nil
	}
	out := make([]schemas.ModelProvider, len(a.providers))
	copy(out, a.providers)
	return out
}

// GetConfiguredProviders implements schemas.Account.
func (a *Account) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	if !a.HasProviders() {
		return nil, fmt.Errorf("aigateway: no providers configured")
	}
	return a.Providers(), nil
}

// GetKeysForProvider implements schemas.Account.
func (a *Account) GetKeysForProvider(_ context.Context, provider schemas.ModelProvider) ([]schemas.Key, error) {
	keys, ok := a.keys[provider]
	if !ok || len(keys) == 0 {
		return nil, fmt.Errorf("aigateway: no keys for provider %s", provider)
	}
	return keys, nil
}

// GetConfigForProvider implements schemas.Account.
func (a *Account) GetConfigForProvider(provider schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	if _, ok := a.keys[provider]; !ok {
		return nil, fmt.Errorf("aigateway: provider %s not configured", provider)
	}
	net := schemas.DefaultNetworkConfig
	net.MaxRetries = 4
	net.RetryBackoffInitial = 200 * time.Millisecond
	net.RetryBackoffMax = 8 * time.Second
	net.DefaultRequestTimeoutInSeconds = 120
	net.AllowPrivateNetwork = true // loopback/self-hosted OpenAI-compat
	return &schemas.ProviderConfig{
		NetworkConfig: net,
		ConcurrencyAndBufferSize: schemas.ConcurrencyAndBufferSize{
			Concurrency: 16,
			BufferSize:  64,
		},
	}, nil
}

var _ schemas.Account = (*Account)(nil)

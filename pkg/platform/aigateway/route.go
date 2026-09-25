package aigateway

import (
	"os"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
)

const (
	defaultAnthropicModel = "claude-sonnet-4-20250514"
	defaultOpenAIModel    = "gpt-4.1-mini"
	defaultGeminiModel    = "gemini-2.0-flash"
)

// DummyAPIKey is accepted by the loopback OpenAI surface; real keys live in Account.
const DummyAPIKey = "majordomo-gateway"

type route struct {
	Provider schemas.ModelProvider
	Model    string
}

func defaultModelFor(p schemas.ModelProvider) string {
	switch p {
	case schemas.Anthropic:
		return envOr("MAJORDOMO_ANTHROPIC_MODEL", defaultAnthropicModel)
	case schemas.OpenAI:
		return envOr("MAJORDOMO_OPENAI_MODEL", defaultOpenAIModel)
	case schemas.Gemini:
		return envOr("MAJORDOMO_GEMINI_MODEL", defaultGeminiModel)
	default:
		return ""
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// LogicalModel returns MAJORDOMO_MODEL or the primary provider default.
func LogicalModel(account *Account) string {
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_MODEL")); v != "" {
		return v
	}
	if account == nil || len(account.providers) == 0 {
		return defaultAnthropicModel
	}
	return defaultModelFor(account.providers[0])
}

func detectProvider(model string) schemas.ModelProvider {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(m, "claude"):
		return schemas.Anthropic
	case strings.HasPrefix(m, "gemini"):
		return schemas.Gemini
	case strings.HasPrefix(m, "gpt"), strings.HasPrefix(m, "o1"), strings.HasPrefix(m, "o3"), strings.HasPrefix(m, "o4"):
		return schemas.OpenAI
	default:
		return ""
	}
}

func providerPreference() []schemas.ModelProvider {
	return []schemas.ModelProvider{schemas.Anthropic, schemas.OpenAI, schemas.Gemini}
}

// resolveRoute picks primary provider/model and fallbacks from the account.
func resolveRoute(account *Account, requestedModel string) (primary route, fallbacks []schemas.Fallback) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		requestedModel = LogicalModel(account)
	}

	have := map[schemas.ModelProvider]bool{}
	for _, p := range account.Providers() {
		have[p] = true
	}

	primaryProvider := detectProvider(requestedModel)
	if primaryProvider == "" || !have[primaryProvider] {
		for _, p := range providerPreference() {
			if have[p] {
				primaryProvider = p
				break
			}
		}
	}
	if primaryProvider == "" {
		return route{}, nil
	}

	model := requestedModel
	if detectProvider(requestedModel) != primaryProvider {
		model = defaultModelFor(primaryProvider)
	}
	primary = route{Provider: primaryProvider, Model: model}

	for _, p := range providerPreference() {
		if p == primaryProvider || !have[p] {
			continue
		}
		fallbacks = append(fallbacks, schemas.Fallback{
			Provider: p,
			Model:    defaultModelFor(p),
		})
	}
	return primary, fallbacks
}

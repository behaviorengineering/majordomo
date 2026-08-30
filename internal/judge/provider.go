package judge

import (
	"os"
	"strings"
	"time"

	stropdspy "github.com/behaviorengineering/strop/dspy"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
)

const defaultModuleTimeout = 2 * time.Minute

// ResolveProvider returns an OpenAI-schema ProviderConfig aimed at the embedded
// Bifrost loopback. Real Anthropic/OpenAI/Gemini keys are owned by aigateway.
func ResolveProvider() (stropdspy.ProviderConfig, error) {
	gw, err := aigateway.Ensure()
	if err != nil {
		return stropdspy.ProviderConfig{}, err
	}
	account, _ := aigateway.NewAccountFromEnv()
	model := strings.TrimSpace(os.Getenv("MAJORDOMO_MODEL"))
	if model == "" {
		model = aigateway.LogicalModel(account)
	}
	return stropdspy.ProviderConfig{
		APIKey:    aigateway.DummyAPIKey,
		Model:     model,
		BaseURL:   gw.BaseURL(),
		APISchema: "openai",
		Timeout:   "120s",
	}, nil
}

// LLMConfigured reports whether the embedded gateway can start (at least one real key).
func LLMConfigured() bool {
	_, err := aigateway.NewAccountFromEnv()
	return err == nil
}

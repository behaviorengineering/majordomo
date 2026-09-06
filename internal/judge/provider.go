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
// Prefer ResolveGatewayProvider or NewRuntime for task-specific models.
func ResolveProvider() (stropdspy.ProviderConfig, error) {
	return ResolveGatewayProvider(strings.TrimSpace(os.Getenv("MAJORDOMO_MODEL")))
}

// LLMConfigured reports whether the embedded gateway can start (at least one real key).
func LLMConfigured() bool {
	_, err := aigateway.NewAccountFromEnv()
	return err == nil
}

package judge

import (
	"os"
	"strings"
	"time"

	stropdspy "github.com/behaviorengineering/strop/dspy"

	"github.com/behaviorengineering/majordomo/internal/aigateway"
)

// defaultModuleTimeout is the outer Process budget for reliability interceptors.
// It must cover defaultModuleRetryAttempts at the longest provider hop
// (polypus_gemma_rlm is 15m in central config).
const defaultModuleTimeout = 45 * time.Minute

// defaultModuleRetryAttempts is the MaxAttempts for dspy-go RetryModuleInterceptor
// wired through strop InterceptorSetup (provider and validation failures).
const defaultModuleRetryAttempts = 3

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

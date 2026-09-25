package staging

import (
	contextprovider "github.com/behaviorengineering/majordomo/pkg/context/provider"
)

// ResolveContextDir returns the flag value or MAJORDOMO_CONTEXT_DIR when set.
func ResolveContextDir(flag string) string {
	return contextprovider.ResolveContextDir(flag)
}

// ResolveContextSHA returns the flag value or MAJORDOMO_CONTEXT_SHA when set.
func ResolveContextSHA(flag string) string {
	return contextprovider.ResolveContextSHA(flag)
}

package staging

import (
	"os"
	"strings"
)

// ResolveContextDir returns the flag value or MAJORDOMO_CONTEXT_DIR when set.
func ResolveContextDir(flag string) string {
	if s := strings.TrimSpace(flag); s != "" {
		return s
	}
	return strings.TrimSpace(os.Getenv("MAJORDOMO_CONTEXT_DIR"))
}

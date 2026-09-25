package contextstore

import (
	"fmt"
	"regexp"
)

var contextBranchRE = regexp.MustCompile(`^majordomo-context/[a-z0-9][a-z0-9._-]*$`)

// ValidateContextBranch returns nil if branch is a canonical context branch name.
func ValidateContextBranch(branch string) error {
	if !contextBranchRE.MatchString(branch) {
		return fmt.Errorf("context branch %q must match majordomo-context/<repo-id>", branch)
	}
	return nil
}

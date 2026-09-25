package contextprovider

import "strings"

const contextBranchPrefix = "majordomo-context/"

// ContextBranchName returns the canonical orphan context branch for repoID.
func ContextBranchName(repoID string) string {
	return contextBranchPrefix + strings.TrimSpace(repoID)
}

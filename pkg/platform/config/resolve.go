package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolvePrepPaths loads central config and materializes prep JSON when configDir+repoID set.
// Explicit routingPath / agentContextPath win when non-empty.
func ResolvePrepPaths(configDir, repoID, pipelineName, materializeDir, routingPath, agentContextPath string) (routing, agentContext string, cfg RepoConfig, err error) {
	routing = routingPath
	agentContext = agentContextPath
	if strings.TrimSpace(configDir) == "" || strings.TrimSpace(repoID) == "" {
		return routing, agentContext, cfg, nil
	}
	if pipelineName == "" {
		pipelineName = "pr-review"
	}
	cfg, err = LoadMerged(configDir, repoID)
	if err != nil {
		return "", "", RepoConfig{}, err
	}
	needMat := routing == "" || agentContext == ""
	if !needMat {
		return routing, agentContext, cfg, nil
	}
	if materializeDir == "" {
		return "", "", RepoConfig{}, fmt.Errorf("materialize dir required when using --config-dir")
	}
	mat, err := MaterializePrep(cfg, pipelineName, materializeDir)
	if err != nil {
		return "", "", RepoConfig{}, err
	}
	if routing == "" {
		routing = mat.RoutingPath
	}
	if agentContext == "" {
		agentContext = mat.AgentContextPath
	}
	return routing, agentContext, cfg, nil
}

// MaterializeDirForStaging returns <stagingDir>/.majordomo-config for generated JSON.
func MaterializeDirForStaging(stagingDir string) string {
	return filepath.Join(stagingDir, ".majordomo-config")
}

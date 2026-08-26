package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaterializeResult holds paths written for prep (empty if not written).
type MaterializeResult struct {
	RoutingPath      string
	AgentContextPath string
}

// MaterializePrep writes routing.json and agent-context.json for pipelines[pipelineName].
// Missing routing leaves RoutingPath empty so prep uses built-in defaults.
func MaterializePrep(cfg RepoConfig, pipelineName, outDir string) (MaterializeResult, error) {
	var out MaterializeResult
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return out, fmt.Errorf("materialize prep dir: %w", err)
	}
	pipe, ok := cfg.PipelineNamed(pipelineName)
	if !ok {
		return out, nil
	}
	if !pipe.Routing.Empty() {
		path := filepath.Join(outDir, "routing.json")
		if err := writeRoutingJSON(path, pipe.Routing); err != nil {
			return out, err
		}
		out.RoutingPath = path
	}
	if pipe.AgentContext != nil {
		path := filepath.Join(outDir, "agent-context.json")
		data, err := json.MarshalIndent(pipe.AgentContext, "", "  ")
		if err != nil {
			return out, fmt.Errorf("marshal agentContext: %w", err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return out, fmt.Errorf("write agent-context.json: %w", err)
		}
		out.AgentContextPath = path
	}
	return out, nil
}

func writeRoutingJSON(path string, routing OrderedRouting) error {
	// Encode as ordered object so LoadRouting preserves first-match-wins order.
	var b bytes.Buffer
	b.WriteByte('{')
	for i, key := range routing.Keys {
		if i > 0 {
			b.WriteByte(',')
		}
		keyJSON, err := json.Marshal(key)
		if err != nil {
			return err
		}
		b.Write(keyJSON)
		b.WriteByte(':')
		entry := routing.Rules[key]
		var val any
		if entry.Persona != "" {
			val = map[string]any{"globs": entry.Globs, "persona": entry.Persona}
		} else {
			val = entry.Globs
		}
		valJSON, err := json.Marshal(val)
		if err != nil {
			return err
		}
		b.Write(valJSON)
	}
	b.WriteString("}\n")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write routing.json: %w", err)
	}
	return nil
}

// ApplyPipelineModelEnv sets COPILOT_MODEL / COPILOT_SCORE_MODEL from pipeline config
// when those env vars are not already set.
func ApplyPipelineModelEnv(cfg RepoConfig, pipelineName string) {
	pipe, ok := cfg.PipelineNamed(pipelineName)
	if !ok {
		return
	}
	setEnvIfEmpty("COPILOT_MODEL", pipe.Model)
	setEnvIfEmpty("OPENCODE_MODEL", pipe.Model)
	setEnvIfEmpty("COPILOT_SCORE_MODEL", pipe.ScoreModel)
	setEnvIfEmpty("OPENCODE_SCORE_MODEL", pipe.ScoreModel)
}

func setEnvIfEmpty(key, val string) {
	val = strings.TrimSpace(val)
	if val == "" {
		return
	}
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return
	}
	_ = os.Setenv(key, val)
}

// ResolveSAToolSlug returns the .sa output slug for a tool entry.
func ResolveSAToolSlug(t StaticAnalysisTool) string {
	if s := strings.TrimSpace(t.Tool); s != "" {
		return saSlug(s)
	}
	if t.Dockerfile != "" {
		base := filepath.Base(t.Dockerfile)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		base = strings.TrimPrefix(base, "Dockerfile.")
		if base != "" && !strings.EqualFold(base, "Dockerfile") {
			return saSlug(base)
		}
	}
	if t.Image != "" {
		last := t.Image
		if i := strings.LastIndex(last, "/"); i >= 0 {
			last = last[i+1:]
		}
		if i := strings.Index(last, ":"); i >= 0 {
			last = last[:i]
		}
		last = strings.TrimPrefix(last, "sa-")
		if last != "" {
			return saSlug(last)
		}
	}
	return "sa-tool"
}

func saSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == '_' || r == '.' || r == '/' {
			b.WriteByte('-')
		}
	}
	out := b.String()
	if out == "" {
		return "sa-tool"
	}
	return out
}

// ResolveSAImage returns the docker image ref for a tool.
// Prefer Image; else MAJORDOMO_SA_IMAGE_PREFIX/sa-<slug>:tag (tag from env or "local").
func ResolveSAImage(t StaticAnalysisTool, imagePrefix string) string {
	if img := strings.TrimSpace(t.Image); img != "" {
		return img
	}
	prefix := strings.TrimSpace(imagePrefix)
	if prefix == "" {
		prefix = strings.TrimSpace(os.Getenv("MAJORDOMO_SA_IMAGE_PREFIX"))
	}
	if prefix == "" {
		prefix = "majordomo"
	}
	prefix = strings.TrimRight(prefix, "/")
	slug := ResolveSAToolSlug(t)
	tag := strings.TrimSpace(os.Getenv("MAJORDOMO_SA_IMAGE_TAG"))
	if tag == "" {
		tag = "local"
	}
	return fmt.Sprintf("%s/sa-%s:%s", prefix, slug, tag)
}

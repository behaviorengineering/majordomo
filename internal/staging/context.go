package staging

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AgentContext is normalized global + scoped context.
type AgentContext struct {
	Global map[string]any
	Scoped map[string]any
}

// LoadAgentContextConfig loads agent context JSON (legacy flat or scoped).
func LoadAgentContextConfig(path string) (AgentContext, error) {
	empty := AgentContext{Global: map[string]any{}, Scoped: map[string]any{}}
	if path == "" {
		return empty, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty, nil
		}
		return empty, fatalf("Failed to load agent context config (%v)", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return empty, fatalf("Failed to load agent context config (%v)", err)
	}
	if _, hasGlobal := raw["global"]; hasGlobal {
		return loadScopedForm(raw, path)
	}
	if _, hasScoped := raw["scoped"]; hasScoped {
		return loadScopedForm(raw, path)
	}
	logf("INFO", "Loaded legacy flat agent context: %s", path)
	return AgentContext{Global: raw, Scoped: map[string]any{}}, nil
}

func loadScopedForm(raw map[string]any, path string) (AgentContext, error) {
	globalCtx, _ := raw["global"].(map[string]any)
	if globalCtx == nil {
		if raw["global"] != nil {
			return AgentContext{}, fatalf("agentContext must define object values for 'global' and 'scoped'")
		}
		globalCtx = map[string]any{}
	}
	scopedCtx, _ := raw["scoped"].(map[string]any)
	if scopedCtx == nil {
		if raw["scoped"] != nil {
			return AgentContext{}, fatalf("agentContext must define object values for 'global' and 'scoped'")
		}
		scopedCtx = map[string]any{}
	}
	logf("INFO", "Loaded scoped agent context: %s", path)
	return AgentContext{Global: globalCtx, Scoped: scopedCtx}, nil
}

func resolveRules(rawRules []any, repoRoot, sourceLabel string) ([]string, error) {
	resolved := make([]string, 0, len(rawRules))
	for idx, item := range rawRules {
		n := idx + 1
		switch v := item.(type) {
		case string:
			resolved = append(resolved, v)
		case map[string]any:
			fileVal, ok := v["file"].(string)
			if !ok {
				return nil, fatalf("%s customRules[%d] must be a string or {\"file\": \"...\"}", sourceLabel, n)
			}
			relPath := trimSpace(fileVal)
			if relPath == "" {
				return nil, fatalf("%s customRules[%d] has empty file path", sourceLabel, n)
			}
			full := filepath.Join(repoRoot, relPath)
			info, err := os.Stat(full)
			if err != nil || info.IsDir() {
				return nil, fatalf("%s customRules[%d] file not found: %s", sourceLabel, n, relPath)
			}
			data, err := os.ReadFile(full)
			if err != nil {
				return nil, fatalf("%s customRules[%d] failed reading %s: %v", sourceLabel, n, relPath, err)
			}
			text := trimSpace(string(data))
			if text == "" {
				return nil, fatalf("%s customRules[%d] file is empty: %s", sourceLabel, n, relPath)
			}
			resolved = append(resolved, text)
		default:
			return nil, fatalf("%s customRules[%d] must be a string or {\"file\": \"...\"}", sourceLabel, n)
		}
	}
	return resolved, nil
}

func resolveContextRules(context map[string]any, repoRoot, label string) (map[string]any, error) {
	resolved := copyMap(context)
	rawRules, exists := resolved["customRules"]
	if !exists || rawRules == nil {
		resolved["customRules"] = []any{}
		return resolved, nil
	}
	list, ok := rawRules.([]any)
	if !ok {
		return nil, fatalf("%s customRules must be a list", label)
	}
	rules, err := resolveRules(list, repoRoot, label)
	if err != nil {
		return nil, err
	}
	asAny := make([]any, len(rules))
	for i, r := range rules {
		asAny[i] = r
	}
	resolved["customRules"] = asAny
	return resolved, nil
}

// ContextForFile merges global + first matching scoped context.
func ContextForFile(filePath string, agentContext AgentContext, repoRoot string) (map[string]any, error) {
	globalRaw := agentContext.Global
	if globalRaw == nil {
		return nil, fatalf("agentContext.global must be an object")
	}
	globalCtx, err := resolveContextRules(globalRaw, repoRoot, "agentContext.global")
	if err != nil {
		return nil, err
	}
	scopedRaw := agentContext.Scoped
	if scopedRaw == nil {
		return nil, fatalf("agentContext.scoped must be an object")
	}

	var matchedGlob string
	var matchedCtxRaw map[string]any
	for glob, scopedCtx := range scopedRaw {
		if !MatchGlob(glob, filePath) {
			continue
		}
		m, ok := scopedCtx.(map[string]any)
		if !ok {
			return nil, fatalf("agentContext.scoped['%s'] must be an object", glob)
		}
		matchedGlob = glob
		matchedCtxRaw = m
		break
	}
	if matchedGlob == "" {
		return globalCtx, nil
	}
	scopedCtx, err := resolveContextRules(matchedCtxRaw, repoRoot, "agentContext.scoped['"+matchedGlob+"']")
	if err != nil {
		return nil, err
	}
	merged := copyMap(globalCtx)
	for key, value := range scopedCtx {
		if key == "customRules" {
			continue
		}
		merged[key] = value
	}
	gRules, _ := globalCtx["customRules"].([]any)
	sRules, _ := scopedCtx["customRules"].([]any)
	combined := append([]any{}, gRules...)
	combined = append(combined, sRules...)
	merged["customRules"] = combined
	return merged, nil
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// LoadSummaryConfig loads optional summary JSON.
func LoadSummaryConfig(path string) (map[string]any, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fatalf("--summary-config file not found: %s", path)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fatalf("invalid summary-config JSON: %v", err)
	}
	return out, nil
}

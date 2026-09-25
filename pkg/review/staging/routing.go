package staging

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// RoutingRule is one (agent, globs) entry; first match wins.
type RoutingRule struct {
	Agent string
	Globs []string
}

// DefaultRouting mirrors Python DEFAULT_ROUTING.
var DefaultRouting = []RoutingRule{
	{Agent: "pr-review-docs", Globs: []string{"**/*.md", "**/*.rst"}},
	{Agent: "pr-review-conf", Globs: []string{
		"**/*.yml", "**/*.yaml", "**/*.toml", "**/*.json", "**/*.ini",
		"**/*.cfg", "**/*.env", "**/*.xml",
	}},
	{Agent: "pr-review-code", Globs: []string{
		"**/*.py",
		"**/*.js", "**/*.jsx", "**/*.ts", "**/*.tsx", "**/*.mjs", "**/*.cjs",
		"**/*.java", "**/*.kt", "**/*.kts", "**/*.groovy", "**/*.scala",
		"**/*.c", "**/*.h", "**/*.cpp", "**/*.cc", "**/*.cxx", "**/*.hpp", "**/*.cs",
		"**/*.go", "**/*.rs", "**/*.swift",
		"**/*.rb", "**/*.php", "**/*.pl", "**/*.pm",
		"**/*.sh", "**/*.bash",
		"**/*.Jenkinsfile", "**/Jenkinsfile",
		"**/*.ps1", "**/*.psm1", "**/*.psd1",
		"**/*.html", "**/*.jinja", "**/*.jinja2", "**/*.j2",
		"**/*.css", "**/*.scss", "**/*.sass", "**/*.less",
		"**/*.tf", "**/*.hcl",
		"**/*.sql",
		"**/Dockerfile", "**/Dockerfile.*",
		"**/Makefile", "**/makefile", "**/*.mk",
		"**/*.ipynb",
		"**/*.proto", "**/*.graphql", "**/*.gql",
		"**/*.dart", "**/*.ex", "**/*.exs", "**/*.erl", "**/*.hrl",
		"**/*.hs", "**/*.lua", "**/*.r", "**/*.R", "**/*.zig", "**/*.nim",
	}},
}

// LoadRouting loads routing JSON or returns defaults. Invalid JSON → defaults.
// Invalid rule shapes → fatal error.
func LoadRouting(routingPath string) ([]RoutingRule, map[string]string, error) {
	if routingPath == "" {
		return DefaultRouting, map[string]string{}, nil
	}
	data, err := os.ReadFile(routingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultRouting, map[string]string{}, nil
		}
		logf("WARN", "Failed to load routing config (%v) — using defaults", err)
		return DefaultRouting, map[string]string{}, nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		logf("WARN", "Failed to load routing config (%v) — using defaults", err)
		return DefaultRouting, map[string]string{}, nil
	}
	rules := make([]RoutingRule, 0, len(raw))
	personaPaths := map[string]string{}
	// Preserve JSON object key order is not guaranteed; for parity with Python 3.7+
	// insertion order we re-parse with ordered decoder when needed. For typical
	// small configs, iterate via json.Decoder with UseNumber for stability.
	ordered, err := decodeJSONObjectOrder(data)
	if err != nil {
		logf("WARN", "Failed to load routing config (%v) — using defaults", err)
		return DefaultRouting, map[string]string{}, nil
	}
	for _, agent := range ordered {
		value := raw[agent]
		switch v := value.(type) {
		case []any:
			globs := make([]string, 0, len(v))
			for _, g := range v {
				s, ok := g.(string)
				if !ok {
					return nil, nil, fatalf("routing['%s'] must be a list of globs or a map with 'globs'", agent)
				}
				globs = append(globs, s)
			}
			rules = append(rules, RoutingRule{Agent: agent, Globs: globs})
		case map[string]any:
			globsRaw, ok := v["globs"]
			if !ok {
				return nil, nil, fatalf("routing['%s'] must be a list of globs or a map with 'globs'", agent)
			}
			globsList, ok := globsRaw.([]any)
			if !ok {
				return nil, nil, fatalf("routing['%s'].globs must be a list", agent)
			}
			globs := make([]string, 0, len(globsList))
			for _, g := range globsList {
				s, ok := g.(string)
				if !ok {
					return nil, nil, fatalf("routing['%s'].globs must be a list", agent)
				}
				globs = append(globs, s)
			}
			rules = append(rules, RoutingRule{Agent: agent, Globs: globs})
			if persona, exists := v["persona"]; exists && persona != nil {
				ps, ok := persona.(string)
				if !ok {
					return nil, nil, fatalf("routing['%s'].persona must be a string path", agent)
				}
				if ps != "" {
					personaPaths[agent] = ps
				}
			}
		default:
			return nil, nil, fatalf("routing['%s'] must be a list of globs or a map with 'globs'", agent)
		}
	}
	logf("INFO", "Loaded routing config: %s", routingPath)
	return rules, personaPaths, nil
}

func decodeJSONObjectOrder(data []byte) ([]string, error) {
	dec := json.NewDecoder(bytesReader(data))
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return nil, fatalf("routing root must be an object")
	}
	keys := []string{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, fatalf("invalid routing key")
		}
		keys = append(keys, key)
		var skip any
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	_, _ = dec.Token() // closing }
	return keys, nil
}

// ClassifyFile returns the agent for a path, or "" if unrouted.
func ClassifyFile(file string, routing []RoutingRule) string {
	for _, rule := range routing {
		for _, pat := range rule.Globs {
			if MatchGlob(pat, file) {
				return rule.Agent
			}
		}
	}
	return ""
}

// ResolveRoutingPersonas loads persona file contents.
func ResolveRoutingPersonas(personaPaths map[string]string, repoRoot string) (map[string]string, error) {
	resolved := map[string]string{}
	for agent, relPath := range personaPaths {
		relPath = trimSpace(relPath)
		if relPath == "" {
			return nil, fatalf("routing['%s'].persona has an empty path", agent)
		}
		full := filepath.Join(repoRoot, relPath)
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			return nil, fatalf("routing['%s'].persona file not found: %s", agent, relPath)
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, fatalf("routing['%s'].persona failed reading %s: %v", agent, relPath, err)
		}
		text := trimSpace(string(data))
		if text == "" {
			return nil, fatalf("routing['%s'].persona file is empty: %s", agent, relPath)
		}
		resolved[agent] = text
		logf("INFO", "  Loaded persona for %s: %s", agent, relPath)
	}
	return resolved, nil
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

package contextstore

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// CurrentSchemaVersion is the only meta.yaml schema_version this package accepts.
const CurrentSchemaVersion = 1

// Meta is the context-branch meta.yaml document.
type Meta struct {
	SchemaVersion int    `yaml:"schema_version"`
	RepoID        string `yaml:"repo_id"`
	LastMergedSHA string `yaml:"last_merged_sha"`
	LastDigestAt  string `yaml:"last_digest_at"`

	RewritePending    bool   `yaml:"rewrite_pending,omitempty"`
	RewriteDetectedAt string `yaml:"rewrite_detected_at,omitempty"`
	RewriteNewHead    string `yaml:"rewrite_new_head,omitempty"`
	RewriteWhy        string `yaml:"rewrite_why,omitempty"`
}

// ParseMeta reads and unmarshals meta.yaml from path.
func ParseMeta(path string) (Meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Meta{}, fmt.Errorf("read meta.yaml: %w", err)
	}
	var m Meta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Meta{}, fmt.Errorf("parse meta.yaml: %w", err)
	}
	return m, nil
}

// ValidateMeta returns nil if m is a known schema with a repo id.
func ValidateMeta(m Meta) error {
	if m.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("meta.yaml schema_version %d is not supported (want %d)", m.SchemaVersion, CurrentSchemaVersion)
	}
	if strings.TrimSpace(m.RepoID) == "" {
		return fmt.Errorf("meta.yaml repo_id is required")
	}
	if ts := strings.TrimSpace(m.LastDigestAt); ts != "" {
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			return fmt.Errorf("meta.yaml last_digest_at %q is not RFC3339: %w", m.LastDigestAt, err)
		}
	}
	return nil
}

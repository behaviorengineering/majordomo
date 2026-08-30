package contextgate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const sidecarName = "gate.json"

// Status is derived gate state for a context update PR.
type Status string

const (
	StatusOpen        Status = "open"
	StatusRejected    Status = "rejected"
	StatusDone        Status = "done"
	StatusBlockedWhy  Status = "blocked_why"
)

// Sidecar persists gate state beside the context update branch tree.
type Sidecar struct {
	Status         Status `json:"status"`
	RejectReason   string `json:"reject_reason,omitempty"`
	ConversationAt string `json:"conversation_at,omitempty"`
	PRNumber       string `json:"pr_number,omitempty"`
	UpdatedAt      string `json:"updated_at"`
}

// SidecarPath returns gate.json path under a context worktree.
func SidecarPath(dir string) string {
	return filepath.Join(dir, sidecarName)
}

// LoadSidecar reads gate.json; missing file yields open status.
func LoadSidecar(dir string) (Sidecar, error) {
	path := SidecarPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Sidecar{Status: StatusOpen, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
		}
		return Sidecar{}, err
	}
	var s Sidecar
	if err := json.Unmarshal(data, &s); err != nil {
		return Sidecar{}, fmt.Errorf("decode gate.json: %w", err)
	}
	if s.Status == "" {
		s.Status = StatusOpen
	}
	return s, nil
}

// SaveSidecar writes gate.json.
func SaveSidecar(dir string, s Sidecar) error {
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(SidecarPath(dir), data, 0o644)
}

// SyncFromComments updates sidecar from forge comments and rewrite-why requirement.
func SyncFromComments(dir, prNumber, prefix string, comments []Comment, rewritePending bool, rewriteWhy string) (Sidecar, error) {
	s, err := LoadSidecar(dir)
	if err != nil {
		return Sidecar{}, err
	}
	s.PRNumber = prNumber
	reject, done, why := ApplyComments(comments, prefix)
	if why != "" && rewritePending && strings.TrimSpace(rewriteWhy) == "" {
		s.Status = StatusBlockedWhy
	}
	if reject != "" {
		s.Status = StatusRejected
		s.RejectReason = reject
		s.ConversationAt = time.Now().UTC().Format(time.RFC3339)
	} else if done {
		s.Status = StatusDone
		s.RejectReason = ""
		s.ConversationAt = time.Now().UTC().Format(time.RFC3339)
	} else if rewritePending && strings.TrimSpace(rewriteWhy) == "" {
		s.Status = StatusBlockedWhy
	} else if s.Status == "" {
		s.Status = StatusOpen
	}
	return s, SaveSidecar(dir, s)
}

// RegenRequested reports whether the latest gate state requires story regen.
func (s Sidecar) RegenRequested() bool {
	return s.Status == StatusRejected && strings.TrimSpace(s.RejectReason) != ""
}

// ReadyToMerge reports conversation-complete gate.
func (s Sidecar) ReadyToMerge() bool {
	return s.Status == StatusDone
}

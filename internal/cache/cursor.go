package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// NormalizeClusterFiles sorts and slash-normalizes file paths.
func NormalizeClusterFiles(files []string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		f = strings.TrimSpace(strings.ReplaceAll(f, "\\", "/"))
		if f != "" {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// ClusterFilesHash is SHA-256 of normalized paths joined by newline.
func ClusterFilesHash(files []string) string {
	joined := strings.Join(NormalizeClusterFiles(files), "\n")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])
}

// ReadPollCursor loads poll-cursor.json if present.
func ReadPollCursor(path string) (*PollCursor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &PollCursor{Heads: map[string]string{}}, nil
		}
		return nil, err
	}
	var c PollCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Heads == nil {
		c.Heads = map[string]string{}
	}
	return &c, nil
}

// WritePollCursor writes poll-cursor.json atomically-ish.
func WritePollCursor(path string, c *PollCursor) error {
	if c.Heads == nil {
		c.Heads = map[string]string{}
	}
	c.Updated = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// ShouldReview reports whether PR head changed vs cursor.
func ShouldReview(c *PollCursor, prNumber, headSHA string) bool {
	if c == nil || c.Heads == nil {
		return true
	}
	prev, ok := c.Heads[prNumber]
	return !ok || prev != headSHA
}

// RecordHead updates the cursor for a PR.
func RecordHead(c *PollCursor, prNumber, headSHA string) {
	if c.Heads == nil {
		c.Heads = map[string]string{}
	}
	c.Heads[prNumber] = headSHA
}

// AnalysisCacheName builds analysis-<sha>.<ext>.
func AnalysisCacheName(clusterSHA, ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	return fmt.Sprintf("analysis-%s.%s", clusterSHA, ext)
}

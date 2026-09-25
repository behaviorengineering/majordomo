package cache

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashDigestParts returns SHA-256 hex of null-separated parts.
// Used by review clustering helpers and by the factory digest inference cache.
func HashDigestParts(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ContentSHA returns SHA-256 hex of raw bytes.
func ContentSHA(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// OwnedPathsHash hashes a stable owned-path list.
func OwnedPathsHash(paths []string) string {
	return ClusterFilesHash(paths)
}

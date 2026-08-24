package staging

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var nonWord = regexp.MustCompile(`[^\w\-]`)

// FileSlug converts a relative path to a safe filename stem.
func FileSlug(file string) string {
	return nonWord.ReplaceAllString(file, "-")
}

func truncateUTF8(text string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	encoded := []byte(text)
	if len(encoded) <= maxBytes {
		return text
	}
	encoded = encoded[:maxBytes]
	for !utf8.Valid(encoded) && len(encoded) > 0 {
		encoded = encoded[:len(encoded)-1]
	}
	return string(encoded)
}

// BuildStagingFilename builds a .txt name within MaxStageFilenameBytes.
func BuildStagingFilename(slug string, suffix string) string {
	ext := ".txt"
	baseName := slug + suffix + ext
	if len([]byte(baseName)) <= MaxStageFilenameBytes {
		return baseName
	}
	sum := sha256.Sum256([]byte(baseName))
	digest := hex.EncodeToString(sum[:])[:12]
	hashPart := "-" + digest
	reserved := len([]byte(hashPart + suffix + ext))
	slugBudget := MaxStageFilenameBytes - reserved
	if slugBudget < 1 {
		slugBudget = 1
	}
	truncated := strings.TrimRight(truncateUTF8(slug, slugBudget), "-._")
	if truncated == "" {
		truncated = "file"
	}
	candidate := truncated + hashPart + suffix + ext
	if len([]byte(candidate)) > MaxStageFilenameBytes {
		panic(fmt.Sprintf("invariant violated: %d > %d", len([]byte(candidate)), MaxStageFilenameBytes))
	}
	return candidate
}

// ParseNameStatus parses git diff -z --name-status output.
func ParseNameStatus(raw string) [][2]string {
	tokens := make([]string, 0)
	for _, t := range strings.Split(raw, "\x00") {
		if t != "" {
			tokens = append(tokens, t)
		}
	}
	result := make([][2]string, 0)
	idx := 0
	for idx < len(tokens) {
		status := tokens[idx]
		if status == "" {
			idx++
			continue
		}
		letter := status[:1]
		if letter == "R" || letter == "C" {
			if idx+2 < len(tokens) {
				result = append(result, [2]string{letter, tokens[idx+2]})
				idx += 3
			} else {
				idx++
			}
		} else if idx+1 < len(tokens) {
			result = append(result, [2]string{letter, tokens[idx+1]})
			idx += 2
		} else {
			idx++
		}
	}
	return result
}

// ChunkLines splits text into chunks of at most size lines.
func ChunkLines(text string, size int) []string {
	if size <= 0 {
		return []string{text}
	}
	lines := strings.SplitAfter(text, "\n")
	// Drop trailing empty from SplitAfter if text doesn't end with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0)
	for i := 0; i < len(lines); i += size {
		end := i + size
		if end > len(lines) {
			end = len(lines)
		}
		out = append(out, strings.Join(lines[i:end], ""))
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

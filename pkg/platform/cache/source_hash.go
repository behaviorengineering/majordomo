package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PackageSourceHash returns a stable SHA-256 of source files under analysisDir/pkgPath.
//
// Only content that should invalidate inspect/ledger reuse is hashed: tracked source
// files (.go, .py, .ts, .tsx, .js, .jsx), sorted by relative path. Ephemeral RLM
// markdown, mechanical-role lines, and cluster proposal prose MUST NOT feed this hash.
func PackageSourceHash(analysisDir, pkgPath string) (string, error) {
	root := strings.TrimSpace(analysisDir)
	rel := strings.TrimSpace(strings.ReplaceAll(pkgPath, "\\", "/"))
	rel = strings.TrimPrefix(rel, "./")
	if root == "" || rel == "" {
		return "", fmt.Errorf("package source hash: analysisDir and pkgPath required")
	}
	dir := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return HashDigestParts("missing-pkg", rel), nil
		}
		return "", fmt.Errorf("package source hash stat %s: %w", rel, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("package source hash: %s is not a directory", rel)
	}

	var files []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isSourceFileName(d.Name()) {
			return nil
		}
		relFile, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relFile))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("package source hash walk %s: %w", rel, err)
	}
	sort.Strings(files)
	h := sha256.New()
	h.Write([]byte(rel))
	h.Write([]byte{0})
	if len(files) == 0 {
		// Empty package dir still yields a stable key (path-only).
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			return "", fmt.Errorf("package source hash read %s: %w", f, err)
		}
		h.Write([]byte(f))
		h.Write([]byte{0})
		h.Write(raw)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// OwnedPackagesSourceHash hashes the stable source of each owned package path.
func OwnedPackagesSourceHash(analysisDir string, paths []string) (string, error) {
	norm := NormalizeClusterFiles(paths)
	parts := make([]string, 0, len(norm)*2)
	for _, p := range norm {
		sum, err := PackageSourceHash(analysisDir, p)
		if err != nil {
			return "", err
		}
		parts = append(parts, p, sum)
	}
	return HashDigestParts(parts...), nil
}

func isSourceFileName(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".go"):
		return true
	case strings.HasSuffix(lower, ".py"):
		return true
	case strings.HasSuffix(lower, ".ts"), strings.HasSuffix(lower, ".tsx"):
		return true
	case strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".jsx"):
		return true
	default:
		return false
	}
}

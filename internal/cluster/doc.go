package cluster

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const minTermLen = 3

var (
	inlineLinkRE = regexp.MustCompile(`(!?)\[([^\]]*)\]\(([^)]+)\)`)
	refDefRE     = regexp.MustCompile(`(?m)^\[([^\]]+)\]:\s+(\S+)`)
	refUseRE     = regexp.MustCompile(`(!?)\[([^\]]+)\]\[([^\]]*)\]`)
	h1RE         = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	h2h3RE       = regexp.MustCompile(`(?m)^#{2,3}\s+(.+)$`)
	backtickRE   = regexp.MustCompile("`([^`\\n]+)`")
	boldRE       = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)

	docScanExcludeDirs = map[string]struct{}{
		"node_modules": {}, ".git": {}, "__pycache__": {}, ".venv": {}, "venv": {},
		"dist": {}, "build": {}, ".tox": {}, ".mypy_cache": {}, ".pytest_cache": {},
		".ruff_cache": {}, ".eggs": {},
	}
)

func docPathExcluded(parts []string) bool {
	for _, part := range parts {
		if _, ok := docScanExcludeDirs[part]; ok {
			return true
		}
	}
	return false
}

func resolveMDLink(target, currentFile, repoRoot string) string {
	if idx := strings.Index(target, "#"); idx >= 0 {
		target = target[:idx]
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "ftp://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(target, "//") {
		return ""
	}

	var resolved string
	if strings.HasPrefix(target, "/") {
		resolved = filepath.Clean(filepath.Join(repoRoot, strings.TrimPrefix(target, "/")))
	} else {
		resolved = filepath.Clean(filepath.Join(filepath.Dir(currentFile), target))
	}

	rel, ok := repoRelPath(resolved, repoRoot)
	if !ok {
		return ""
	}
	return rel
}

func parseMDLinks(path, repoRoot string, targetSet map[string]struct{}) map[string]struct{} {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(content)

	refDefs := make(map[string]string)
	for _, match := range refDefRE.FindAllStringSubmatch(text, -1) {
		label := strings.ToLower(strings.TrimSpace(match[1]))
		refDefs[label] = strings.TrimSpace(match[2])
	}

	var rawTargets []string
	for _, match := range inlineLinkRE.FindAllStringSubmatch(text, -1) {
		if match[1] == "!" {
			continue
		}
		rawTargets = append(rawTargets, strings.TrimSpace(match[3]))
	}
	for _, match := range refUseRE.FindAllStringSubmatch(text, -1) {
		if match[1] == "!" {
			continue
		}
		textVal := strings.TrimSpace(match[2])
		label := strings.TrimSpace(match[3])
		lookup := label
		if lookup == "" {
			lookup = textVal
		}
		if resolvedRef, ok := refDefs[strings.ToLower(lookup)]; ok {
			rawTargets = append(rawTargets, resolvedRef)
		}
	}

	found := make(map[string]struct{})
	for _, raw := range rawTargets {
		resolved := resolveMDLink(raw, path, repoRoot)
		if resolved == "" {
			continue
		}
		if _, ok := targetSet[resolved]; ok {
			found[resolved] = struct{}{}
		}
	}
	return found
}

func extractTitle(content string) string {
	match := h1RE.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func extractHeadings(content string) []string {
	matches := h2h3RE.FindAllStringSubmatch(content, -1)
	headings := make([]string, 0, len(matches))
	for _, match := range matches {
		headings = append(headings, strings.TrimSpace(match[1]))
	}
	return headings
}

func extractKeyTerms(content string) []string {
	terms := make(map[string]struct{})
	for _, match := range backtickRE.FindAllStringSubmatch(content, -1) {
		term := strings.TrimSpace(match[1])
		if len(term) >= minTermLen {
			terms[term] = struct{}{}
		}
	}
	for _, match := range boldRE.FindAllStringSubmatch(content, -1) {
		term := strings.TrimSpace(match[1])
		if len(term) >= minTermLen {
			terms[term] = struct{}{}
		}
	}
	result := make([]string, 0, len(terms))
	for term := range terms {
		result = append(result, term)
	}
	sort.Strings(result)
	return result
}

// ClusterDocs groups changed markdown files into link-based clusters.
func ClusterDocs(changedFiles []string, repoRoot string) [][]string {
	if len(changedFiles) == 0 {
		return nil
	}

	changed := make(map[string]struct{}, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = struct{}{}
	}

	uf := NewUnionFind(changedFiles)
	for _, rel := range changedFiles {
		path := filepath.Join(repoRoot, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(path)) != ".md" {
			continue
		}

		for neighbour := range parseMDLinks(path, repoRoot, changed) {
			uf.Union(rel, neighbour)
		}
	}

	return uf.Components()
}

// DocClusterAwareBatches packs manifest tasks into batches that keep doc link clusters together.
func DocClusterAwareBatches(skillTasks []map[string]any, batchSize int, repoRoot string) [][]map[string]any {
	if len(skillTasks) == 0 {
		return nil
	}

	fileToTasks := make(map[string][]map[string]any)
	for _, task := range skillTasks {
		fileKey, _ := task["file"].(string)
		fileToTasks[fileKey] = append(fileToTasks[fileKey], task)
	}

	changedFiles := make([]string, 0, len(fileToTasks))
	for file := range fileToTasks {
		changedFiles = append(changedFiles, file)
	}

	clusters := ClusterDocs(changedFiles, repoRoot)
	sort.Slice(clusters, func(i, j int) bool {
		return len(clusters[i]) > len(clusters[j])
	})

	var batches [][]map[string]any
	var current []map[string]any

	for _, cluster := range clusters {
		var clusterTasks []map[string]any
		for _, filePath := range cluster {
			clusterTasks = append(clusterTasks, fileToTasks[filePath]...)
		}
		if len(clusterTasks) == 0 {
			continue
		}

		if len(current)+len(clusterTasks) <= batchSize {
			current = append(current, clusterTasks...)
			continue
		}

		if len(current) > 0 {
			batches = append(batches, current)
			current = nil
		}
		for _, task := range clusterTasks {
			current = append(current, task)
			if len(current) == batchSize {
				batches = append(batches, current)
				current = nil
			}
		}
	}

	if len(current) > 0 {
		batches = append(batches, current)
	}

	return batches
}

// ReverseLinks returns unchanged repo markdown files that link to any changed files.
func ReverseLinks(changedFiles []string, repoRoot string) map[string][]string {
	if len(changedFiles) == 0 {
		return map[string][]string{}
	}

	changed := make(map[string]struct{}, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = struct{}{}
	}

	result := make(map[string][]string)

	_ = filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if docPathExcluded(strings.Split(path, string(filepath.Separator))) {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		if docPathExcluded(strings.Split(path, string(filepath.Separator))) {
			return nil
		}

		rel, ok := repoRelPath(path, repoRoot)
		if !ok {
			return nil
		}
		if _, isChanged := changed[rel]; isChanged {
			return nil
		}

		for linked := range parseMDLinks(path, repoRoot, changed) {
			result[linked] = append(result[linked], rel)
		}
		return nil
	})

	for dep, linkers := range result {
		sort.Strings(linkers)
		result[dep] = linkers
	}
	return result
}

// BuildCorpusIndex extracts title, headings, key terms, and outgoing links from every markdown file.
func BuildCorpusIndex(repoRoot string) []map[string]any {
	allMD := make(map[string]string)

	_ = filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if docPathExcluded(strings.Split(path, string(filepath.Separator))) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		if docPathExcluded(strings.Split(path, string(filepath.Separator))) {
			return nil
		}
		rel, ok := repoRelPath(path, repoRoot)
		if !ok {
			return nil
		}
		allMD[rel] = path
		return nil
	})

	targetSet := make(map[string]struct{}, len(allMD))
	for rel := range allMD {
		targetSet[rel] = struct{}{}
	}

	files := make([]string, 0, len(allMD))
	for rel := range allMD {
		files = append(files, rel)
	}
	sort.Strings(files)

	entries := make([]map[string]any, 0, len(files))
	for _, rel := range files {
		path := allMD[rel]
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(content)

		links := parseMDLinks(path, repoRoot, targetSet)
		linksOut := make([]string, 0, len(links))
		for link := range links {
			linksOut = append(linksOut, link)
		}
		sort.Strings(linksOut)

		entries = append(entries, map[string]any{
			"file":      rel,
			"title":     extractTitle(text),
			"headings":  extractHeadings(text),
			"key_terms": extractKeyTerms(text),
			"links_out": linksOut,
		})
	}

	return entries
}

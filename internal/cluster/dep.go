package cluster

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	jsExts = map[string]struct{}{
		".js": {}, ".jsx": {}, ".ts": {}, ".tsx": {}, ".mjs": {}, ".cjs": {},
	}

	jsImportRE = regexp.MustCompile(`(?:import\s+[\w*{}\s,]+\s+from\s+|require\s*\(\s*)['"]([^'"]+)['"]`)

	pyImportRE = regexp.MustCompile(`(?m)^\s*import\s+(.+)$`)
	pyFromRE   = regexp.MustCompile(`(?m)^\s*from\s+(\.+[\w.]*|[\w.]+)\s+import\b`)

	scanExcludeDirs = map[string]struct{}{
		"node_modules": {}, ".git": {}, "__pycache__": {}, ".venv": {}, "venv": {},
		"dist": {}, "build": {}, ".tox": {}, ".mypy_cache": {}, ".pytest_cache": {},
		".ruff_cache": {}, ".eggs": {},
	}
)

func pathExcluded(parts []string) bool {
	for _, part := range parts {
		if _, ok := scanExcludeDirs[part]; ok {
			return true
		}
		if strings.HasSuffix(part, ".egg-info") {
			return true
		}
	}
	return false
}

func repoRelPath(absPath, repoRoot string) (string, bool) {
	rel, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func moduleToCandidates(modulePath, repoRoot string) []string {
	parts := strings.ReplaceAll(modulePath, ".", string(filepath.Separator))
	return []string{
		filepath.Join(repoRoot, parts+".py"),
		filepath.Join(repoRoot, parts, "__init__.py"),
	}
}

func resolveRelativeImport(level int, module, currentFile string) []string {
	packageDir := filepath.Dir(currentFile)
	for i := 1; i < level; i++ {
		packageDir = filepath.Dir(packageDir)
	}
	if module == "" {
		return []string{filepath.Join(packageDir, "__init__.py")}
	}
	parts := strings.ReplaceAll(module, ".", string(filepath.Separator))
	return []string{
		filepath.Join(packageDir, parts+".py"),
		filepath.Join(packageDir, parts, "__init__.py"),
	}
}

func parseImportModules(line string) []string {
	var modules []string
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if idx := strings.Index(part, " as "); idx >= 0 {
			part = strings.TrimSpace(part[:idx])
		}
		if part != "" {
			modules = append(modules, part)
		}
	}
	return modules
}

func parseFromModule(spec string) (level int, module string, relative bool) {
	if strings.HasPrefix(spec, ".") {
		i := 0
		for i < len(spec) && spec[i] == '.' {
			i++
		}
		return i, spec[i:], true
	}
	return 0, spec, false
}

func parsePythonImports(path, repoRoot string, changed map[string]struct{}) map[string]struct{} {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(content)

	found := make(map[string]struct{})
	addCandidates := func(candidates []string) {
		for _, candidate := range candidates {
			rel, ok := repoRelPath(candidate, repoRoot)
			if !ok {
				continue
			}
			if _, ok := changed[rel]; ok {
				found[rel] = struct{}{}
			}
		}
	}

	for _, match := range pyImportRE.FindAllStringSubmatch(text, -1) {
		for _, mod := range parseImportModules(match[1]) {
			addCandidates(moduleToCandidates(mod, repoRoot))
		}
	}

	for _, match := range pyFromRE.FindAllStringSubmatch(text, -1) {
		level, module, relative := parseFromModule(match[1])
		if relative {
			addCandidates(resolveRelativeImport(level, module, path))
		} else {
			addCandidates(moduleToCandidates(module, repoRoot))
		}
	}

	return found
}

func resolveJSImport(specifier, currentFile string) []string {
	if !strings.HasPrefix(specifier, ".") {
		return nil
	}
	base := filepath.Clean(filepath.Join(filepath.Dir(currentFile), specifier))
	candidates := []string{
		base,
		base + ".js",
		base + ".ts",
		base + ".tsx",
		base + ".jsx",
		filepath.Join(base, "index.js"),
		filepath.Join(base, "index.ts"),
	}
	return candidates
}

func parseJSImports(path, repoRoot string, changed map[string]struct{}) map[string]struct{} {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	found := make(map[string]struct{})
	for _, match := range jsImportRE.FindAllStringSubmatch(string(content), -1) {
		specifier := match[1]
		for _, candidate := range resolveJSImport(specifier, path) {
			rel, ok := repoRelPath(candidate, repoRoot)
			if !ok {
				continue
			}
			if _, ok := changed[rel]; ok {
				found[rel] = struct{}{}
			}
		}
	}
	return found
}

func isJSExt(suffix string) bool {
	_, ok := jsExts[strings.ToLower(suffix)]
	return ok
}

// ClusterFiles groups changed files into dependency clusters.
func ClusterFiles(changedFiles []string, repoRoot string) [][]string {
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

		suffix := strings.ToLower(filepath.Ext(path))
		var neighbours map[string]struct{}
		switch {
		case suffix == ".py":
			neighbours = parsePythonImports(path, repoRoot, changed)
		case isJSExt(suffix):
			neighbours = parseJSImports(path, repoRoot, changed)
		default:
			continue
		}

		for neighbour := range neighbours {
			uf.Union(rel, neighbour)
		}
	}

	return uf.Components()
}

// DepClusterAwareBatches packs manifest tasks into batches that keep dependency clusters together.
func DepClusterAwareBatches(skillTasks []map[string]any, batchSize int, repoRoot string) [][]map[string]any {
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

	clusters := ClusterFiles(changedFiles, repoRoot)
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

// ReverseDeps returns unchanged repo files that directly import any of the changed files.
func ReverseDeps(changedFiles []string, repoRoot string) map[string][]string {
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
			if pathExcluded(strings.Split(path, string(filepath.Separator))) {
				return filepath.SkipDir
			}
			return nil
		}

		if pathExcluded(strings.Split(path, string(filepath.Separator))) {
			return nil
		}

		suffix := strings.ToLower(filepath.Ext(path))
		if suffix != ".py" && !isJSExt(suffix) {
			return nil
		}

		rel, ok := repoRelPath(path, repoRoot)
		if !ok {
			return nil
		}
		if _, isChanged := changed[rel]; isChanged {
			return nil
		}

		var imported map[string]struct{}
		if suffix == ".py" {
			imported = parsePythonImports(path, repoRoot, changed)
		} else {
			imported = parseJSImports(path, repoRoot, changed)
		}

		for dep := range imported {
			result[dep] = append(result[dep], rel)
		}
		return nil
	})

	for dep, importers := range result {
		sort.Strings(importers)
		result[dep] = importers
	}
	return result
}

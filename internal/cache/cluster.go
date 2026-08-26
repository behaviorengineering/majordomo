package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	frontmatterDelim = "---"
	timestampFmt     = "2006-01-02T15:04:05Z"
)

var (
	validKeyRE     = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	analysisNameRE = regexp.MustCompile(`^analysis-([a-f0-9]{64})\.[A-Za-z0-9]+$`)
	clusterSHARE   = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

var requiredFields = []string{
	"cluster_sha",
	"fingerprint_version",
	"cluster_files",
	"cluster_files_hash",
	"model_id",
	"instruction_bundle_hash",
	"prompt_template_hash",
	"scoring_rubric_hash",
	"output_schema_version",
	"analysis_payload_hash",
	"created_at",
}

var storeMetadataOrder = []string{
	"cluster_sha",
	"skill_name",
	"fingerprint_version",
	"cluster_files",
	"cluster_files_hash",
	"model_id",
	"model_revision",
	"instruction_bundle_hash",
	"prompt_template_hash",
	"scoring_rubric_hash",
	"output_schema_version",
	"analysis_payload_hash",
	"markdown_artifact_file",
	"markdown_artifact_hash",
	"markdown_artifact_count",
	"created_at",
}

var indexMetaKeys = map[string]bool{
	"cluster_sha":              true,
	"skill_name":               true,
	"fingerprint_version":      true,
	"cluster_files":            true,
	"cluster_files_hash":       true,
	"model_id":                 true,
	"model_revision":           true,
	"instruction_bundle_hash":  true,
	"prompt_template_hash":     true,
	"scoring_rubric_hash":      true,
	"output_schema_version":    true,
	"analysis_payload_hash":    true,
	"markdown_artifact_file":   true,
	"markdown_artifact_hash":   true,
	"markdown_artifact_count":  true,
	"created_at":               true,
}

// Meta is frontmatter metadata: string or list of strings.
type Meta map[string]any

type cacheRecord struct {
	filePath  string
	metadata  Meta
	createdAt time.Time
}

// PrecheckOptions configures cache precheck / index build.
type PrecheckOptions struct {
	ProjectID             string
	CacheDir              string
	ProjectRetentionDays  *int
	CentralRetentionDays  *int
	GlobalRetentionDays   int
	MinRetentionDays      int
	IndexOut              string
}

// LookupOptions configures a cache hit evaluation.
type LookupOptions struct {
	IndexFile              string
	ClusterSHA             string
	SkillName              string
	FingerprintVersion     string
	ClusterFiles           []string
	ClusterFilesFile       string
	ModelID                string
	ModelRevision          string
	InstructionBundleHash  string
	PromptTemplateHash     string
	ScoringRubricHash      string
	OutputSchemaVersion    string
}

// StoreOptions configures writing a cache entry.
type StoreOptions struct {
	CacheDir               string
	SkillName              string
	ClusterSHA             string
	FingerprintVersion     string
	ClusterFiles           []string
	ClusterFilesFile       string
	ModelID                string
	ModelRevision          string
	InstructionBundleHash  string
	PromptTemplateHash     string
	ScoringRubricHash      string
	OutputSchemaVersion    string
	AnalysisFile           string
	ReportsDir             string
	ArtifactFiles          []string
}

// RestoreOptions configures restoring markdown artifacts from cache.
type RestoreOptions struct {
	CacheDir  string
	EntryFile string
	OutputDir string
}

// Precheck prunes expired entries and builds a cache index (JSON to stdout / IndexOut).
func Precheck(opts PrecheckOptions) (map[string]any, error) {
	if opts.GlobalRetentionDays == 0 {
		opts.GlobalRetentionDays = 180
	}
	if opts.MinRetentionDays == 0 {
		opts.MinRetentionDays = 30
	}
	cacheDir := filepath.Clean(opts.CacheDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	retentionDays, source, err := resolveRetentionDays(opts.ProjectRetentionDays, opts.CentralRetentionDays, opts.GlobalRetentionDays, opts.MinRetentionDays)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -retentionDays)

	expiredFiles := []string{}
	invalidFiles := map[string][]string{}
	var validRecords []cacheRecord

	for _, filePath := range collectCacheFiles(cacheDir) {
		record, errors := loadCacheRecord(filePath)
		rel := toRel(filePath, cacheDir)
		if record == nil {
			invalidFiles[rel] = errors
			continue
		}
		if record.createdAt.Before(cutoff) {
			expiredFiles = append(expiredFiles, rel)
			if err := os.Remove(filePath); err != nil {
				invalidFiles[rel] = []string{fmt.Sprintf("failed to delete expired file: %v", err)}
			}
			continue
		}
		match := analysisNameRE.FindStringSubmatch(filepath.Base(filePath))
		if match == nil {
			invalidFiles[rel] = []string{"file name must match analysis-<sha256>.<ext>"}
			continue
		}
		clusterSHA, _ := record.metadata["cluster_sha"].(string)
		if clusterSHA != "" && clusterSHA != match[1] {
			invalidFiles[rel] = []string{"cluster_sha does not match file name hash"}
			continue
		}
		validRecords = append(validRecords, *record)
	}

	indexRecords := buildIndex(validRecords)
	indexEntries := map[string]any{}
	for _, record := range indexRecords {
		metadataForIndex := Meta{}
		for k, v := range record.metadata {
			if indexMetaKeys[k] {
				metadataForIndex[k] = v
			}
		}
		metadataForIndex["file"] = toRel(record.filePath, cacheDir)
		skillName, _ := metadataForIndex["skill_name"].(string)
		clusterSHA, _ := metadataForIndex["cluster_sha"].(string)
		if strings.TrimSpace(skillName) != "" && clusterSHA != "" {
			indexEntries[skillName+":"+clusterSHA] = metadataForIndex
		} else if clusterSHA != "" {
			indexEntries[clusterSHA] = metadataForIndex
		}
	}

	result := map[string]any{
		"project_id":      opts.ProjectID,
		"cache_dir":       filepath.ToSlash(cacheDir),
		"retention_days":  retentionDays,
		"retention_source": source,
		"scanned_files":   len(collectCacheFiles(cacheDir)),
		"expired_deleted": len(expiredFiles),
		"expired_files":   expiredFiles,
		"invalid_files":   invalidFiles,
		"valid_entries":   len(indexEntries),
		"index":           indexEntries,
	}
	if opts.IndexOut != "" {
		if err := os.MkdirAll(filepath.Dir(opts.IndexOut), 0o755); err != nil {
			return nil, err
		}
		if err := writeJSONFile(opts.IndexOut, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Lookup evaluates whether a cluster cache entry is a valid hit.
func Lookup(opts LookupOptions) (map[string]any, error) {
	raw, err := os.ReadFile(opts.IndexFile)
	if err != nil {
		return nil, err
	}
	var indexData map[string]any
	if err := json.Unmarshal(raw, &indexData); err != nil {
		return nil, err
	}
	rawIndex, ok := indexData["index"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("index file missing 'index' object")
	}
	entry := resolveLookupEntry(rawIndex, opts.ClusterSHA, opts.SkillName)
	if entry == nil {
		return map[string]any{"hit": false, "reason": "cluster_sha not found"}, nil
	}

	expectedFiles, err := parseClusterFilesArgument(opts.ClusterFiles, opts.ClusterFilesFile)
	if err != nil {
		return nil, err
	}
	expectedHash := ClusterFilesHash(expectedFiles)

	checks := [][2]string{
		{"cluster_sha", opts.ClusterSHA},
		{"fingerprint_version", opts.FingerprintVersion},
		{"cluster_files_hash", expectedHash},
		{"model_id", opts.ModelID},
		{"instruction_bundle_hash", opts.InstructionBundleHash},
		{"prompt_template_hash", opts.PromptTemplateHash},
		{"scoring_rubric_hash", opts.ScoringRubricHash},
		{"output_schema_version", opts.OutputSchemaVersion},
	}
	if opts.ModelRevision != "" {
		checks = append(checks, [2]string{"model_revision", opts.ModelRevision})
	}

	mismatches := map[string]any{}
	for _, c := range checks {
		actual := entry[c[0]]
		if fmt.Sprint(actual) != c[1] {
			mismatches[c[0]] = map[string]string{
				"expected": c[1],
				"actual":   fmt.Sprint(actual),
			}
		}
	}
	actualFiles := asStringSlice(entry["cluster_files"])
	if !stringSlicesEqual(actualFiles, expectedFiles) {
		expJSON, _ := json.Marshal(expectedFiles)
		actJSON, _ := json.Marshal(actualFiles)
		mismatches["cluster_files"] = map[string]string{
			"expected": string(expJSON),
			"actual":   string(actJSON),
		}
	}
	if len(mismatches) > 0 {
		return map[string]any{
			"hit":        false,
			"reason":     "metadata mismatch",
			"mismatches": mismatches,
		}, nil
	}
	fileVal, _ := entry["file"].(string)
	return map[string]any{
		"hit":         true,
		"cluster_sha": opts.ClusterSHA,
		"file":        fileVal,
	}, nil
}

// Store writes or updates a cluster cache artifact.
func Store(opts StoreOptions) (map[string]any, error) {
	if !clusterSHARE.MatchString(opts.ClusterSHA) {
		return nil, fmt.Errorf("cluster_sha must be a lowercase 64-char hex sha256")
	}
	cacheDir := filepath.Clean(opts.CacheDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	clusterFiles, err := parseClusterFilesArgument(opts.ClusterFiles, opts.ClusterFilesFile)
	if err != nil {
		return nil, err
	}
	clusterFilesHash := ClusterFilesHash(clusterFiles)
	payloadText, err := os.ReadFile(opts.AnalysisFile)
	if err != nil {
		return nil, err
	}
	payloadHash := sha256Hex(string(payloadText))

	markdownFiles := map[string]string{}
	markdownArtifactHash := ""
	markdownArtifactCount := 0
	markdownArtifactFile := ""
	if opts.ReportsDir != "" {
		markdownFiles, markdownArtifactHash, err = collectMarkdownArtifact(opts.AnalysisFile, opts.ReportsDir, opts.ArtifactFiles)
		if err != nil {
			return nil, err
		}
		markdownArtifactCount = len(markdownFiles)
		if markdownArtifactCount > 0 {
			markdownArtifactFile = opts.SkillName + "/markdown-" + opts.ClusterSHA + ".json"
		}
	}

	createdAt := time.Now().UTC().Format(timestampFmt)
	metadata := Meta{
		"cluster_sha":             opts.ClusterSHA,
		"skill_name":              opts.SkillName,
		"fingerprint_version":     opts.FingerprintVersion,
		"cluster_files":           clusterFiles,
		"cluster_files_hash":      clusterFilesHash,
		"model_id":                opts.ModelID,
		"instruction_bundle_hash": opts.InstructionBundleHash,
		"prompt_template_hash":    opts.PromptTemplateHash,
		"scoring_rubric_hash":     opts.ScoringRubricHash,
		"output_schema_version":   opts.OutputSchemaVersion,
		"analysis_payload_hash":   payloadHash,
		"created_at":              createdAt,
	}
	if opts.ModelRevision != "" {
		metadata["model_revision"] = opts.ModelRevision
	}
	if markdownArtifactFile != "" {
		metadata["markdown_artifact_file"] = markdownArtifactFile
		metadata["markdown_artifact_hash"] = markdownArtifactHash
		metadata["markdown_artifact_count"] = strconv.Itoa(markdownArtifactCount)
	}

	skillDir := filepath.Join(cacheDir, opts.SkillName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return nil, err
	}
	outputPath := filepath.Join(skillDir, "analysis-"+opts.ClusterSHA+".json")
	if _, err := os.Stat(outputPath); err == nil {
		existing, _ := loadCacheRecord(outputPath)
		if existing != nil && existingCachePayloadUnchanged(existing, payloadHash, markdownArtifactFile, markdownArtifactHash, markdownArtifactCount) {
			return map[string]any{
				"written":     false,
				"reason":      "payload-unchanged",
				"cluster_sha": opts.ClusterSHA,
				"file":        toRel(outputPath, cacheDir),
			}, nil
		}
	}

	if markdownArtifactFile != "" {
		artifactPath := filepath.Join(cacheDir, filepath.FromSlash(markdownArtifactFile))
		if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
			return nil, err
		}
		if err := writeMarkdownArtifactFile(artifactPath, markdownFiles); err != nil {
			return nil, err
		}
	}

	frontmatter := formatFrontmatter(metadata)
	outputText := frontmatter + "\n" + string(payloadText)
	if err := os.WriteFile(outputPath, []byte(outputText), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{
		"written":                 true,
		"cluster_sha":             opts.ClusterSHA,
		"file":                    toRel(outputPath, cacheDir),
		"analysis_payload_hash":   payloadHash,
		"markdown_artifact_file":  markdownArtifactFile,
		"markdown_artifact_count": markdownArtifactCount,
	}, nil
}

// Restore writes cached markdown artifacts into OutputDir.
func Restore(opts RestoreOptions) (map[string]any, error) {
	cacheDir := filepath.Clean(opts.CacheDir)
	entryPath := filepath.Clean(filepath.Join(cacheDir, opts.EntryFile))
	if !isInside(cacheDir, entryPath) {
		return nil, fmt.Errorf("entry-file must resolve inside cache-dir")
	}
	record, errors := loadCacheRecord(entryPath)
	if record == nil {
		joined := strings.Join(errors, "; ")
		if joined == "" {
			joined = "unknown error"
		}
		return nil, fmt.Errorf("cache entry file is invalid: %s", joined)
	}
	artifactRel, _ := record.metadata["markdown_artifact_file"].(string)
	if strings.TrimSpace(artifactRel) == "" {
		return map[string]any{
			"restored":    false,
			"reason":      "no-markdown-artifact",
			"entry_file":  opts.EntryFile,
		}, nil
	}
	artifactPath := filepath.Clean(filepath.Join(cacheDir, filepath.FromSlash(artifactRel)))
	if !isInside(cacheDir, artifactPath) {
		return nil, fmt.Errorf("markdown artifact path resolves outside cache-dir")
	}
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, err
	}
	var artifactPayload struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(raw, &artifactPayload); err != nil {
		return nil, err
	}
	if artifactPayload.Files == nil {
		return nil, fmt.Errorf("markdown artifact payload missing files map")
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, err
	}
	restored := 0
	for name, content := range artifactPayload.Files {
		if !strings.HasSuffix(name, ".md") || strings.ContainsAny(name, `/\`) {
			continue
		}
		if err := os.WriteFile(filepath.Join(opts.OutputDir, name), []byte(content), 0o644); err != nil {
			return nil, err
		}
		restored++
	}
	return map[string]any{
		"restored":       restored > 0,
		"restored_count": restored,
		"entry_file":     opts.EntryFile,
		"artifact_file":  artifactRel,
	}, nil
}

func resolveRetentionDays(project, central *int, global, minDays int) (int, string, error) {
	if minDays < 0 || global < 0 {
		return 0, "", fmt.Errorf("retention days must be non-negative")
	}
	resolved := global
	source := "global"
	if central != nil {
		if *central < 0 {
			return 0, "", fmt.Errorf("central retention days must be non-negative")
		}
		resolved = *central
		source = "central"
	}
	if project != nil {
		if *project < 0 {
			return 0, "", fmt.Errorf("project retention days must be non-negative")
		}
		resolved = *project
		source = "project"
	}
	if resolved < minDays {
		resolved = minDays
	}
	return resolved, source, nil
}

func collectCacheFiles(cacheDir string) []string {
	var out []string
	_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		ok, _ := filepath.Match("analysis-*.*", info.Name())
		if ok {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func buildIndex(records []cacheRecord) map[string]cacheRecord {
	index := map[string]cacheRecord{}
	for _, record := range records {
		clusterSHA, _ := record.metadata["cluster_sha"].(string)
		if clusterSHA == "" {
			continue
		}
		skillName, _ := record.metadata["skill_name"].(string)
		key := clusterSHA
		if strings.TrimSpace(skillName) != "" {
			key = skillName + ":" + clusterSHA
		}
		existing, ok := index[key]
		if !ok || !record.createdAt.Before(existing.createdAt) {
			index[key] = record
		}
	}
	return index
}

func loadCacheRecord(filePath string) (*cacheRecord, []string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, []string{fmt.Sprintf("read failure: %v", err)}
	}
	metadata, _, err := extractFrontmatter(string(data))
	if err != nil {
		return nil, []string{err.Error()}
	}
	errors := validateMetadata(metadata)
	if len(errors) > 0 {
		return nil, errors
	}
	createdAtStr, _ := metadata["created_at"].(string)
	createdAt, err := time.Parse(timestampFmt, createdAtStr)
	if err != nil {
		return nil, []string{"created_at must use UTC format YYYY-MM-DDTHH:MM:SSZ"}
	}
	return &cacheRecord{filePath: filePath, metadata: metadata, createdAt: createdAt.UTC()}, nil
}

func extractFrontmatter(content string) (Meta, string, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != frontmatterDelim {
		return nil, "", fmt.Errorf("Missing frontmatter start delimiter")
	}
	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == frontmatterDelim {
			endIndex = i
			break
		}
	}
	if endIndex < 0 {
		return nil, "", fmt.Errorf("Missing frontmatter end delimiter")
	}
	block := strings.Join(lines[1:endIndex], "\n")
	body := strings.Join(lines[endIndex+1:], "\n")
	meta, err := parseFrontmatterBlock(block)
	if err != nil {
		return nil, "", err
	}
	return meta, body, nil
}

func parseFrontmatterBlock(blockText string) (Meta, error) {
	metadata := Meta{}
	currentListKey := ""
	for _, rawLine := range strings.Split(blockText, "\n") {
		line := strings.TrimRight(rawLine, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := regexp.MustCompile(`^\s*-\s+(.*)$`).FindStringSubmatch(line); m != nil {
			if currentListKey == "" {
				return nil, fmt.Errorf("List item appears before a list key")
			}
			cur, ok := metadata[currentListKey].([]string)
			if !ok {
				return nil, fmt.Errorf("Key '%s' is not a list", currentListKey)
			}
			metadata[currentListKey] = append(cur, stripWrappedQuotes(m[1]))
			continue
		}
		currentListKey = ""
		keyMatch := regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(.*)$`).FindStringSubmatch(line)
		if keyMatch == nil {
			return nil, fmt.Errorf("Unsupported frontmatter line: %q", line)
		}
		keyName := keyMatch[1]
		if !validKeyRE.MatchString(keyName) {
			return nil, fmt.Errorf("Invalid metadata key: %q", keyName)
		}
		valueRaw := keyMatch[2]
		if valueRaw == "" {
			metadata[keyName] = []string{}
			currentListKey = keyName
			continue
		}
		metadata[keyName] = stripWrappedQuotes(valueRaw)
	}
	return metadata, nil
}

func stripWrappedQuotes(raw string) string {
	stripped := strings.TrimSpace(raw)
	if len(stripped) >= 2 {
		q := stripped[0]
		if (q == '"' || q == '\'') && stripped[len(stripped)-1] == q {
			return stripped[1 : len(stripped)-1]
		}
	}
	return stripped
}

func validateMetadata(metadata Meta) []string {
	var errors []string
	for _, field := range requiredFields {
		if _, ok := metadata[field]; !ok {
			errors = append(errors, fmt.Sprintf("missing field '%s'", field))
		}
	}
	errors = append(errors, validateClusterSHAField(metadata)...)
	errors = append(errors, validateClusterFilesFields(metadata)...)
	errors = append(errors, validateCreatedAtField(metadata)...)
	return errors
}

func validateClusterSHAField(metadata Meta) []string {
	clusterSHA := metadata["cluster_sha"]
	if s, ok := clusterSHA.(string); ok {
		if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(s) {
			return []string{"cluster_sha must be a lowercase 64-char hex sha256"}
		}
		return nil
	}
	if clusterSHA != nil {
		return []string{"cluster_sha must be a string"}
	}
	return nil
}

func validateClusterFilesFields(metadata Meta) []string {
	clusterFilesRaw := metadata["cluster_files"]
	files := asStringSlice(clusterFilesRaw)
	if files == nil {
		if clusterFilesRaw != nil {
			return []string{"cluster_files must be a list"}
		}
		return nil
	}
	normalized := NormalizeClusterFiles(files)
	if len(normalized) == 0 {
		return []string{"cluster_files must contain at least one path"}
	}
	expectedHash := ClusterFilesHash(normalized)
	h, ok := metadata["cluster_files_hash"].(string)
	if !ok {
		return []string{"cluster_files_hash must be a string"}
	}
	if h != expectedHash {
		return []string{"cluster_files_hash does not match cluster_files"}
	}
	return nil
}

func validateCreatedAtField(metadata Meta) []string {
	createdAt := metadata["created_at"]
	if s, ok := createdAt.(string); ok {
		if _, err := time.Parse(timestampFmt, s); err != nil {
			return []string{"created_at must use UTC format YYYY-MM-DDTHH:MM:SSZ"}
		}
		return nil
	}
	if createdAt != nil {
		return []string{"created_at must be a string"}
	}
	return nil
}

func parseClusterFilesArgument(files []string, filesPath string) ([]string, error) {
	merged := []string{}
	for _, f := range files {
		if strings.TrimSpace(f) != "" {
			merged = append(merged, f)
		}
	}
	if filesPath != "" {
		raw, err := os.ReadFile(filesPath)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				merged = append(merged, line)
			}
		}
	}
	normalized := NormalizeClusterFiles(merged)
	if len(normalized) == 0 {
		return nil, fmt.Errorf("cluster file list is empty")
	}
	return normalized, nil
}

func formatFrontmatter(metadata Meta) string {
	lines := []string{frontmatterDelim}
	for _, key := range storeMetadataOrder {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		if list, ok := value.([]string); ok {
			lines = append(lines, key+":")
			for _, item := range list {
				encoded, _ := json.Marshal(item)
				lines = append(lines, "  - "+string(encoded))
			}
			continue
		}
		var encoded []byte
		switch v := value.(type) {
		case string:
			encoded, _ = json.Marshal(v)
		default:
			encoded, _ = json.Marshal(fmt.Sprint(v))
		}
		lines = append(lines, key+": "+string(encoded))
	}
	lines = append(lines, frontmatterDelim)
	return strings.Join(lines, "\n")
}

func collectMarkdownArtifact(analysisFile, reportsDir string, artifactFiles []string) (map[string]string, string, error) {
	slugs, err := loadManifestSlugs(analysisFile)
	if err != nil {
		return nil, "", err
	}
	requested := map[string]bool{}
	for _, slug := range slugs {
		requested[slug+".md"] = true
	}
	for _, name := range artifactFiles {
		name = strings.TrimSpace(name)
		if name != "" {
			requested[name] = true
		}
	}
	markdownFiles := map[string]string{}
	names := make([]string, 0, len(requested))
	for n := range requested {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, reportName := range names {
		if strings.ContainsAny(reportName, `/\`) || !strings.HasSuffix(reportName, ".md") {
			continue
		}
		p := filepath.Join(reportsDir, reportName)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		markdownFiles[reportName] = string(data)
	}
	artifactHash := ""
	if len(markdownFiles) > 0 {
		canonical, _ := json.Marshal(markdownFiles) // sorted keys
		artifactHash = sha256Hex(string(canonical))
	}
	return markdownFiles, artifactHash, nil
}

func loadManifestSlugs(analysisFile string) ([]string, error) {
	raw, err := os.ReadFile(analysisFile)
	if err != nil {
		return nil, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	reviewable, ok := parsed["reviewable"].([]any)
	if !ok {
		return nil, fmt.Errorf("analysis file reviewable field must be a list")
	}
	slugSet := map[string]bool{}
	for _, task := range reviewable {
		m, ok := task.(map[string]any)
		if !ok {
			continue
		}
		slug := strings.TrimSpace(fmt.Sprint(m["slug"]))
		if slug != "" && slug != "<nil>" {
			slugSet[slug] = true
		}
	}
	out := make([]string, 0, len(slugSet))
	for s := range slugSet {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

func writeMarkdownArtifactFile(path string, files map[string]string) error {
	payload := map[string]any{"files": files}
	return writeJSONFile(path, payload)
}

func existingCachePayloadUnchanged(existing *cacheRecord, payloadHash, markdownArtifactFile, markdownArtifactHash string, markdownArtifactCount int) bool {
	existingPayload, _ := existing.metadata["analysis_payload_hash"].(string)
	existingArtifactHash, _ := existing.metadata["markdown_artifact_hash"].(string)
	existingArtifactCount := existing.metadata["markdown_artifact_count"]
	artifactUnchanged := false
	if markdownArtifactFile != "" {
		count, err := strconv.Atoi(fmt.Sprint(existingArtifactCount))
		artifactUnchanged = err == nil && existingArtifactHash == markdownArtifactHash && count == markdownArtifactCount
	} else {
		artifactUnchanged = existingArtifactHash == ""
	}
	return existingPayload == payloadHash && artifactUnchanged
}

func resolveLookupEntry(rawIndex map[string]any, clusterSHA, skillName string) map[string]any {
	keys := []string{}
	if skillName != "" {
		keys = append(keys, skillName+":"+clusterSHA)
	}
	keys = append(keys, clusterSHA)
	for _, key := range keys {
		if candidate, ok := rawIndex[key].(map[string]any); ok {
			return candidate
		}
		// Meta may have been stored as map[string]any via JSON round-trip already.
	}
	if skillName == "" {
		return nil
	}
	for _, candidate := range rawIndex {
		m, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(m["cluster_sha"]) == clusterSHA && fmt.Sprint(m["skill_name"]) == skillName {
			return m
		}
	}
	return nil
}

func toRel(filePath, rootDir string) string {
	rel, err := filepath.Rel(rootDir, filePath)
	if err != nil {
		return filepath.ToSlash(filePath)
	}
	return filepath.ToSlash(rel)
}

func isInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func asStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// PrintJSON writes compact-ish sorted JSON like the Python helpers.
func PrintJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(data))
	return err
}

// PrintJSONPretty writes indented sorted-key JSON.
func PrintJSONPretty(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(data))
	return err
}

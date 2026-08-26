package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	doesRE = regexp.MustCompile(`(?m)^\*\*Does:\*\*\s+\x60([^\x60]+\.py[^\x60]*)\x60`)
	h3RE   = regexp.MustCompile(`(?m)^### .+`)
	hrRE   = regexp.MustCompile(`(?m)^---\s*$`)
	slugRE = regexp.MustCompile(`[^a-zA-Z0-9]`)
)

// TechDeepOptions configures the technical deep second pass.
type TechDeepOptions struct {
	PRNumber       string
	TechReviewPath string
	WorkspaceRoot  string
	StagingBase    string
	OutputDir      string
	ScriptsDir     string
	ChunkLines     int
	Concurrency    int
	Dispatch       func(DispatchOptions) error
}

// ParseRisksByFile maps cited .py paths to risk blocks from tech-review.md.
// Order is first-seen citation order (stable for aggregation).
func ParseRisksByFile(techReview string) (map[string][]string, []string) {
	sections := hrRE.Split(techReview, -1)
	risks := map[string][]string{}
	var order []string
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if !h3RE.MatchString(section) {
			continue
		}
		m := doesRE.FindStringSubmatch(section)
		if m == nil {
			continue
		}
		filePath := strings.TrimSpace(m[1])
		if _, ok := risks[filePath]; !ok {
			order = append(order, filePath)
		}
		risks[filePath] = append(risks[filePath], section)
	}
	return risks, order
}

func fileSlug(filePath string) string {
	return strings.Trim(slugRE.ReplaceAllString(filePath, "-"), "-")
}

// RunTechDeep ports pipelines/scripts/tech-review-deep.py.
func RunTechDeep(opts TechDeepOptions) error {
	if opts.ChunkLines <= 0 {
		opts.ChunkLines = envInt("TECH_DEEP_CHUNK_LINES", 400)
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = envInt("TECH_DEEP_CONCURRENCY", 6)
	}
	dispatch := opts.Dispatch
	if dispatch == nil {
		dispatch = Dispatch
	}

	Logf("INFO", "========== Tech-review deep pass: PR #%s ==========", opts.PRNumber)

	raw, err := os.ReadFile(opts.TechReviewPath)
	if err != nil {
		return fmt.Errorf("tech-review.md not found: %w", err)
	}
	risksByFile, citedOrder := ParseRisksByFile(string(raw))
	if len(risksByFile) == 0 {
		Logf("INFO", "[tech-review-deep] No file citations found in tech-review.md — nothing to do.")
		if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
			return err
		}
		body := fmt.Sprintf("# PR #%s — Technical Deep Review\n\n_No correctness risks were cited in the technical review._\n", opts.PRNumber)
		return os.WriteFile(filepath.Join(opts.OutputDir, "tech-review-deep.md"), []byte(body), 0o644)
	}

	Logf("INFO", "[tech-review-deep] Cited files: %v", citedOrder)

	skillDir, err := resolveTechnicalDeepSkill(opts.ScriptsDir)
	if err != nil {
		return err
	}
	stagingDir := filepath.Join(opts.StagingBase, "pr-review-technical-deep")
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return err
	}
	contextByFile := loadTechnicalAgentContexts(opts.StagingBase)

	var allBatchDirs []string
	for _, filePath := range citedOrder {
		dirs, err := stageDeepFile(stageDeepArgs{
			FilePath:      filePath,
			Risks:         risksByFile[filePath],
			WorkspaceRoot: opts.WorkspaceRoot,
			StagingDir:    stagingDir,
			ChunkLines:    opts.ChunkLines,
			PRNumber:      opts.PRNumber,
			SkillDir:      skillDir,
			AgentContext:  contextByFile[filePath],
		})
		if err != nil {
			return err
		}
		allBatchDirs = append(allBatchDirs, dirs...)
	}
	if len(allBatchDirs) == 0 {
		Logf("WARN", "[tech-review-deep] No stageable files found (all cited files missing from workspace).")
		return nil
	}

	Logf("INFO", "[tech-review-deep] Dispatching %d batch(es) with concurrency=%d", len(allBatchDirs), opts.Concurrency)

	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []string
	for _, bd := range allBatchDirs {
		wg.Add(1)
		go func(batchDir string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			err := dispatch(DispatchOptions{
				PRNumber:   opts.PRNumber,
				StagingDir: batchDir,
				OutputDir:  opts.OutputDir,
				Mode:       ModeTechnicalDeep,
				ScriptsDir: opts.ScriptsDir,
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				Logf("ERROR", "[tech-review-deep] Batch failed: %s: %v", batchDir, err)
				failures = append(failures, batchDir)
			} else {
				Logf("INFO", "[tech-review-deep] Batch done: %s", filepath.Base(batchDir))
			}
		}(bd)
	}
	wg.Wait()

	if err := aggregateDeepOutputs(citedOrder, opts.OutputDir, opts.PRNumber); err != nil {
		return err
	}
	if len(failures) > 0 {
		Logf("ERROR", "[tech-review-deep] %d batch(es) failed — partial output written.", len(failures))
		return fmt.Errorf("tech-review-deep: %d batch(es) failed", len(failures))
	}
	return nil
}

func resolveTechnicalDeepSkill(scriptsDir string) (string, error) {
	scripts, err := ResolveScriptsDir(scriptsDir)
	if err != nil {
		return "", err
	}
	skill := filepath.Clean(filepath.Join(scripts, "..", "..", "agents", "skills", "pr-review-technical-deep"))
	if _, err := os.Stat(skill); err != nil {
		return "", fmt.Errorf("skill directory not found: %s", skill)
	}
	return skill, nil
}

func loadTechnicalAgentContexts(stagingBase string) map[string]map[string]any {
	manifest := filepath.Join(stagingBase, "pr-review-technical", "batch_000", "manifest.json")
	raw, err := os.ReadFile(manifest)
	if err != nil {
		return map[string]map[string]any{}
	}
	var data struct {
		Reviewable []struct {
			File         string         `json:"file"`
			AgentContext map[string]any `json:"agent_context"`
		} `json:"reviewable"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return map[string]map[string]any{}
	}
	out := map[string]map[string]any{}
	for _, e := range data.Reviewable {
		if e.File != "" && len(e.AgentContext) > 0 {
			out[e.File] = e.AgentContext
		}
	}
	return out
}

type stageDeepArgs struct {
	FilePath      string
	Risks         []string
	WorkspaceRoot string
	StagingDir    string
	ChunkLines    int
	PRNumber      string
	SkillDir      string
	AgentContext  map[string]any
}

func stageDeepFile(a stageDeepArgs) ([]string, error) {
	fullPath := filepath.Join(a.WorkspaceRoot, a.FilePath)
	if _, err := os.Stat(fullPath); err != nil {
		Logf("WARN", "[tech-review-deep] Cited file not found in workspace: %s — skipping", a.FilePath)
		return nil, nil
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	fileLines := splitLinesKeepContent(string(data))
	chunks := chunkLines(fileLines, a.ChunkLines)
	slug := fileSlug(a.FilePath)
	risksBlock := strings.Join(a.Risks, "\n\n---\n\n")

	var batchDirs []string
	total := len(chunks)
	for idx, chunk := range chunks {
		n := idx + 1
		chunkLabel := fmt.Sprintf("=== FULL FILE: %s ===", a.FilePath)
		if total > 1 {
			chunkLabel = fmt.Sprintf("=== FULL FILE CHUNK %d of %d: %s ===", n, total, a.FilePath)
		}
		contentParts := []string{
			"=== RISKS FROM TECH-REVIEW ===",
			"",
			risksBlock,
			"",
			chunkLabel,
			"",
		}
		contentParts = append(contentParts, chunk...)
		content := strings.Join(contentParts, "\n")

		chunkSuffix := ""
		if total > 1 {
			chunkSuffix = fmt.Sprintf("-chunk%03d", n)
		}
		inputFilename := slug + chunkSuffix + ".txt"
		batchDir := filepath.Join(a.StagingDir, "batch_"+slug+chunkSuffix)
		if err := os.MkdirAll(batchDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(batchDir, inputFilename), []byte(content), 0o644); err != nil {
			return nil, err
		}

		mode := "full_and_diff"
		var chunkVal any
		var totalChunks any
		if total > 1 {
			mode = "diff_chunk"
			chunkVal = n
			totalChunks = total
		}
		entry := map[string]any{
			"file":       a.FilePath,
			"slug":       slug,
			"mode":       mode,
			"chunk":      chunkVal,
			"total_chunks": totalChunks,
			"input_file": inputFilename,
			"agent":      "pr-review-technical-deep",
		}
		if len(a.AgentContext) > 0 {
			entry["agent_context"] = a.AgentContext
		}
		manifest := map[string]any{
			"base_branch": "",
			"refspec":     "",
			"skill_dir":   "pr-review-technical-deep",
			"review_agents": map[string]any{
				"pr-review-technical-deep": []string{a.FilePath},
			},
			"reviewable": []any{entry},
			"excluded":   []any{},
		}
		manBytes, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(batchDir, "manifest.json"), manBytes, 0o644); err != nil {
			return nil, err
		}
		if err := copyFile(filepath.Join(a.SkillDir, "SKILL.md"), filepath.Join(batchDir, "SKILL.md")); err != nil {
			return nil, err
		}
		templatesSrc := filepath.Join(a.SkillDir, "templates")
		if st, err := os.Stat(templatesSrc); err == nil && st.IsDir() {
			if err := copyTree(templatesSrc, filepath.Join(batchDir, "templates")); err != nil {
				return nil, err
			}
		}
		ts := sydneyTimestamp()
		if err := os.WriteFile(filepath.Join(batchDir, "review_timestamp.txt"), []byte(ts), 0o644); err != nil {
			return nil, err
		}
		batchDirs = append(batchDirs, batchDir)
	}
	return batchDirs, nil
}

func aggregateDeepOutputs(citedOrder []string, outputDir, prNumber string) error {
	lines := []string{
		fmt.Sprintf("# PR #%s — Technical Deep Review", prNumber),
		"",
		fmt.Sprintf("_Generated: %s_", time.Now().UTC().Format("2006-01-02T15:04:05Z")),
		"",
		"---",
		"",
	}
	for _, filePath := range citedOrder {
		slug := fileSlug(filePath)
		reportPath := filepath.Join(outputDir, slug+".md")
		if data, err := os.ReadFile(reportPath); err == nil {
			lines = append(lines, strings.TrimSpace(string(data)))
		} else {
			lines = append(lines, fmt.Sprintf("## %s\n\n_No deep review output produced for this file._", filePath))
		}
		lines = append(lines, "\n---\n")
	}
	out := filepath.Join(outputDir, "tech-review-deep.md")
	if err := os.WriteFile(out, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	Logf("INFO", "[tech-review-deep] Wrote %s", out)
	return nil
}

func chunkLines(lines []string, size int) [][]string {
	if size <= 0 {
		size = 400
	}
	var out [][]string
	for i := 0; i < len(lines); i += size {
		end := i + size
		if end > len(lines) {
			end = len(lines)
		}
		out = append(out, lines[i:end])
	}
	if len(out) == 0 {
		out = append(out, []string{})
	}
	return out
}

func splitLinesKeepContent(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text == "" {
		return []string{}
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func sydneyTimestamp() string {
	cmd := exec.Command("date", "+%Y-%m-%dT%H:%M:%S%:z")
	cmd.Env = append(os.Environ(), "TZ=Australia/Sydney")
	out, err := cmd.Output()
	if err != nil {
		return time.Now().Format(time.RFC3339)
	}
	return strings.TrimSpace(string(out))
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

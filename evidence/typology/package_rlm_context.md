# Package RLM context index

AST-derived package context for recursive exploration.
Classify from symbols, imports, and bodies. Directory basename is not evidence.

## ./cmd/majordomo
- package: `main`
- packageDoc: Command majordomo is the control-plane CLI for repository operations.
- hasMain: true
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- deliveryHint: cli
- mechanicalRole: entrypoint
- mechanicalConfidence: 0.90
- mechanicalEvidence: has_main
- exportedDecls: (none)
- exportedFuncs: (none)
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: main
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/cmd/majordomo/main.go

## ./internal/agent
- package: `agent`
- packageDoc: Package agent owns review dispatch helpers.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: DispatchOptions, Mode, ModeFiles, ModeFinalize, ModeProse, ModeScore, ModeSummary, ModeTechScore, ModeTechnical, ModeTechnicalDeep, SummaryLoopOptions, TechDeepOptions, TechLoopOptions
- exportedFuncs: Dispatch, FindScript, GroundingPaths, GroundingSkillDir, Logf, OpenCodeEnv, ParseRisksByFile, ParseScore, ResolveScriptsDir, RunOpenCode, RunSummaryLoop, RunTechDeep, RunTechLoop
- exportedMethods: (none)
- unexportedDecls: dispatchScriptLegacy, dispatchScriptPrimary, doesRE, groundingPack, h3RE, hrRE, slugRE, stageDeepArgs
- unexportedFuncs: aggregateDeepOutputs, chunkLines, copyFile, copyIfExists, copyTree, defaultScriptRunner, dispatchScriptIn, envInt, fileSlug, loadGroundingPacks, loadTechnicalAgentContexts, manifestSkill, resolveTechnicalDeepSkill, splitLinesKeepContent, stageDeepFile, sydneyTimestamp
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agent/dispatch.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agent/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agent/grounding.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agent/harness.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agent/loops.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agent/techdeep.go

### Exported bodies

#### Mode (type)

```go
type Mode string
```

#### DispatchOptions (type)

```go
type DispatchOptions struct {
	Context    context.Context
	PRNumber   string
	StagingDir string
	OutputDir  string
	Mode       Mode
	// ScriptsDir is retained for ResolveScriptsDir callers (tech-deep helpers); unused by strop Judge.
	ScriptsDir string
	// Env is the parent environ for RunOpenCode (nil → os.Environ). Unused by Dispatch.
	Env []string
	// Timeout kills the OpenCode script process when > 0. Unused by in-process Judge.
	Timeout time.Duration
	// Runner overrides script exec for RunOpenCode tests. Unused by Dispatch.
	Runner func(name string, args []string, env []string, dir string) error
}
```

#### ResolveScriptsDir (func)

```go
func ResolveScriptsDir(explicit string) (string, error) {
	if explicit != "" {
		if _, ok := dispatchScriptIn(explicit); ok {
			return filepath.Clean(explicit), nil
		}
		// Allow directory even without dispatch script (scripts still hold helpers).
		if info, err := os.Stat(explicit); err == nil && info.IsDir() {
			return filepath.Clean(explicit), nil
		}
		return "", fmt.Errorf("scripts dir not found: %s", explicit)
	}
	if v := os.Getenv("MAJORDOMO_SCRIPTS"); v != "" {
		if info, err := os.Stat(v); err == nil && info.IsDir() {
			return filepath.Clean(v), nil
		}
	}
	candidates := []string{
		"pipelines/scripts",
		".majordomo/pipelines/scripts",
	}
	wd, _ := os.Getwd()
	dir := wd
	for i := 0; i < 8 && dir != ""; i++ {
		for _, rel := range candidates {
			cand := filepath.Join(dir, rel)
			if info, err := os.Stat(cand); err == nil && info.IsDir() {
				return cand, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("pipelines/scripts not found (set --scripts-dir or MAJORDOMO_SCRIPTS)")
}
```

#### Dispatch (func)

```go
func Dispatch(opts DispatchOptions) error {
	if opts.PRNumber == "" || opts.StagingDir == "" || opts.OutputDir == "" {
		return fmt.Errorf("dispatch requires pr, staging-dir, and output-dir")
	}
	return judge.Dispatch(judge.DispatchOptions{
		Context:    opts.Context,
		PRNumber:   opts.PRNumber,
		StagingDir: opts.StagingDir,
		OutputDir:  opts.OutputDir,
		Mode:       judge.DispatchMode(opts.Mode),
	})
}
```

#### FindScript (func)

```go
func FindScript(scriptsDir, name string) (string, error) {
	dir, err := ResolveScriptsDir(scriptsDir)
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%s not found in %s", name, dir)
	}
	return p, nil
}
```

#### Logf (func)

```go
func Logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

#### ParseScore (func)

```go
func ParseScore(text string) (int, bool) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SCORE:") {
			var n int
			_, err := fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "SCORE:")), "%d", &n)
			if err == nil {
				return n, true
			}
		}
	}
	return 0, false
}
```

#### GroundingPaths (func)

```go
func GroundingPaths(batchDir, skillDir string) ([]string, error) {
	batchDir = filepath.Clean(batchDir)
	packs, err := loadGroundingPacks(batchDir)
	if err != nil {
		return nil, err
	}
	if len(packs) == 0 {
		return nil, nil
	}
	skillDir = filepath.Clean(skillDir)
	var paths []string
	for _, p := range packs {
		if strings.TrimSpace(p.File) == "" {
			continue
		}
		rel := filepath.FromSlash(strings.TrimPrefix(p.File, "./"))
		candidates := []string{
			filepath.Join(skillDir, rel),
			filepath.Join(batchDir, rel),
		}
		var found string
		for _, c := range candidates {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				found = c
				break
			}
		}
		if found == "" {
			return nil, fmt.Errorf("grounding pack %q: file not found (%s)", p.ID, p.File)
		}
		paths = append(paths, found)
	}
	return paths, nil
}
```

#### GroundingSkillDir (func)

```go
func GroundingSkillDir(stagingDir string, mode Mode) (string, error) {
	stagingDir = filepath.Clean(stagingDir)
	switch mode {
	case ModeProse, ModeFinalize, ModeScore, ModeTechScore:
		return "", nil
	case ModeTechnicalDeep:
		return stagingDir, nil
	}
	skill, err := manifestSkill(stagingDir)
	if err != nil {
		return "", err
	}
	if skill == "" {
		return "", nil
	}
	return filepath.Join(stagingDir, skill), nil
}
```

#### OpenCodeEnv (func)

```go
func OpenCodeEnv(parent []string) ([]string, error) {
	if parent == nil {
		parent = os.Environ()
	}
	return aigateway.PrepareChildEnv(parent)
}
```

#### RunOpenCode (func)

```go
func RunOpenCode(opts DispatchOptions) error {
	if opts.PRNumber == "" || opts.StagingDir == "" || opts.OutputDir == "" {
		return fmt.Errorf("opencode harness requires pr, staging-dir, and output-dir")
	}
	scriptsDir, err := ResolveScriptsDir(opts.ScriptsDir)
	if err != nil {
		return err
	}
	script, ok := dispatchScriptIn(scriptsDir)
	if !ok {
		return fmt.Errorf("opencode harness: %s not found in %s", dispatchScriptPrimary, scriptsDir)
	}

	parent := opts.Env
	if parent == nil {
		parent = os.Environ()
	}
	env, err := OpenCodeEnv(parent)
	if err != nil {
		return fmt.Errorf("opencode harness env: %w", err)
	}

	args := []string{opts.PRNumber, opts.StagingDir, opts.OutputDir}
	if opts.Mode != "" && opts.Mode != ModeFiles {
		args = append(args, string(opts.Mode))
	}

	runner := opts.Runner
	if runner == nil {
		runner = defaultScriptRunner(opts.Timeout)
	}
	return runner(script, args, env, "")
}
```

#### SummaryLoopOptions (type)

```go
type SummaryLoopOptions struct {
	Context    context.Context
	PRNumber   string
	StagingDir string
	OutputDir  string // skill output: .../pr-review-summary
	PassScore  int
	MaxIter    int
	ScriptsDir string
	Dispatch   func(DispatchOptions) error
}
```

#### RunSummaryLoop (func)

```go
func RunSummaryLoop(opts SummaryLoopOptions) error {
	if opts.PassScore <= 0 {
		opts.PassScore = envInt("SUMMARY_PASS_SCORE", 15)
	}
	if opts.MaxIter <= 0 {
		opts.MaxIter = envInt("SUMMARY_MAX_ITERATIONS", 5)
	}
	dispatch := opts.Dispatch
	if dispatch == nil {
		dispatch = Dispatch
	}
	pipelineOut := filepath.Dir(opts.OutputDir)
	scoreFile := filepath.Join(pipelineOut, "score.md")
	logsDir := filepath.Join(opts.OutputDir, "logs")
	_ = os.MkdirAll(logsDir, 0o755)

	Logf("INFO", "========== Summary loop: PR #%s (pass>=%d, max=%d) ==========",
		opts.PRNumber, opts.PassScore, opts.MaxIter)
	defer func() { _ = os.Unsetenv("SUMMARY_ITER") }()

	for iteration := 1; iteration <= opts.MaxIter; iteration++ {
		Logf("INFO", "[summary-loop] Iteration %d/%d", iteration, opts.MaxIter)
		if err := os.Setenv("SUMMARY_ITER", strconv.Itoa(iteration)); err != nil {
			return fmt.Errorf("summary-loop: set SUMMARY_ITER: %w", err)
		}

		if err := dispatch(DispatchOptions{
			Context:  opts.Context,
			PRNumber: opts.PRNumber, StagingDir: opts.StagingDir,
			OutputDir: opts.OutputDir, Mode: ModeSummary, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("summary-loop: --summary failed: %w", err)
		}
		if err := dispatch(DispatchOptions{
			Context:  opts.Context,
			PRNumber: opts.PRNumber, StagingDir: opts.StagingDir,
			OutputDir: opts.OutputDir, Mode: ModeScore, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("summary-loop: --score failed: %w", err)
		}

		data, err := os.ReadFile(scoreFile)
		if err != nil {
			return fmt.Errorf("summary-loop: score.md not found: %w", err)
		}
		score, ok := ParseScore(string(data))
		if !ok {
			return fmt.Errorf("summary-loop: could not parse SCORE from score.md")
		}
		Logf("INFO", "[summary-loop] Score: %d (threshold %d)", score, opts.PassScore)

		// Archive iteration artefacts
		summarySrc := filepath.Join(pipelineOut, "summary.md")
		_ = copyIfExists(summarySrc, filepath.Join(logsDir, fmt.Sprintf("summary_iter_%d.md", iteration)))
		_ = copyIfExists(scoreFile, filepath.Join(logsDir, fmt.Sprintf("score_iter_%d.md", iteration)))

		if score >= opts.PassScore {
			Logf("INFO", "[summary-loop] Accepted score %d", score)
			return nil
		}
		feedback := filepath.Join(opts.StagingDir, "score_feedback.md")
		if err := copyFile(scoreFile, feedback); err != nil {
			return err
		}
	}
	Logf("INFO", "[summary-loop] Reached max iterations; keeping last summary")
	return nil
}
```

#### TechLoopOptions (type)

```go
type TechLoopOptions struct {
	Context    context.Context
	PRNumber   string
	StagingDir string
	OutputDir  string
	PassScore  int
	MaxIter    int
	ScriptsDir string
	Dispatch   func(DispatchOptions) error
}
```

#### RunTechLoop (func)

```go
func RunTechLoop(opts TechLoopOptions) error {
	if opts.PassScore <= 0 {
		opts.PassScore = envInt("TECH_PASS_SCORE", 11)
	}
	if opts.MaxIter <= 0 {
		opts.MaxIter = envInt("TECH_MAX_ITERATIONS", 3)
	}
	dispatch := opts.Dispatch
	if dispatch == nil {
		dispatch = Dispatch
	}
	pipelineOut := filepath.Dir(opts.OutputDir)
	scoreFile := filepath.Join(pipelineOut, "tech-score.md")
	logsDir := filepath.Join(opts.OutputDir, "logs")
	_ = os.MkdirAll(logsDir, 0o755)

	Logf("INFO", "========== Tech loop: PR #%s (pass>=%d, max=%d) ==========",
		opts.PRNumber, opts.PassScore, opts.MaxIter)
	defer func() { _ = os.Unsetenv("TECH_ITER") }()

	for iteration := 1; iteration <= opts.MaxIter; iteration++ {
		Logf("INFO", "[tech-loop] Iteration %d/%d", iteration, opts.MaxIter)
		if err := os.Setenv("TECH_ITER", strconv.Itoa(iteration)); err != nil {
			return fmt.Errorf("tech-loop: set TECH_ITER: %w", err)
		}

		if err := dispatch(DispatchOptions{
			Context:  opts.Context,
			PRNumber: opts.PRNumber, StagingDir: opts.StagingDir,
			OutputDir: opts.OutputDir, Mode: ModeTechnical, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("tech-loop: --technical failed: %w", err)
		}
		if err := dispatch(DispatchOptions{
			Context:  opts.Context,
			PRNumber: opts.PRNumber, StagingDir: opts.StagingDir,
			OutputDir: opts.OutputDir, Mode: ModeTechScore, ScriptsDir: opts.ScriptsDir,
		}); err != nil {
			return fmt.Errorf("tech-loop: --tech-score failed: %w", err)
		}

		data, err := os.ReadFile(scoreFile)
		if err != nil {
			return fmt.Errorf("tech-loop: tech-score.md not found: %w", err)
		}
		score, ok := ParseScore(string(data))
		if !ok {
			return fmt.Errorf("tech-loop: could not parse SCORE from tech-score.md")
		}
		Logf("INFO", "[tech-loop] Score: %d (threshold %d)", score, opts.PassScore)

		_ = copyIfExists(filepath.Join(pipelineOut, "tech-review.md"),
			filepath.Join(logsDir, fmt.Sprintf("tech_review_iter_%d.md", iteration)))
		_ = copyIfExists(scoreFile, filepath.Join(logsDir, fmt.Sprintf("tech_score_iter_%d.md", iteration)))

		if score >= opts.PassScore {
			Logf("INFO", "[tech-loop] Accepted score %d", score)
			return nil
		}
		feedback := filepath.Join(opts.StagingDir, "tech_feedback.md")
		if err := copyFile(scoreFile, feedback); err != nil {
			return err
		}
	}
	Logf("INFO", "[tech-loop] Reached max iterations; keeping last tech-review")
	return nil
}
```

#### TechDeepOptions (type)

```go
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
```

#### ParseRisksByFile (func)

```go
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
```

#### RunTechDeep (func)

```go
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
```

### Private one-hop bodies

#### aggregateDeepOutputs (func)

```go
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
```

#### copyFile (func)

```go
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
```

#### copyIfExists (func)

```go
func copyIfExists(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return copyFile(src, dst)
}
```

#### defaultScriptRunner (func)

```go
func defaultScriptRunner(timeout time.Duration) func(name string, args []string, env []string, dir string) error {
	return func(name string, args []string, env []string, dir string) error {
		cmd := exec.Command(name, args...)
		cmd.Env = env
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if timeout > 0 {
			timer := time.AfterFunc(timeout, func() {
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
			})
			defer timer.Stop()
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("opencode harness %s: %w", filepath.Base(name), err)
		}
		return nil
	}
}
```

#### dispatchScriptIn (func)

```go
func dispatchScriptIn(dir string) (string, bool) {
	for _, name := range []string{dispatchScriptPrimary, dispatchScriptLegacy} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}
```

#### envInt (func)

```go
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
```

#### loadGroundingPacks (func)

```go
func loadGroundingPacks(batchDir string) ([]groundingPack, error) {
	data, err := os.ReadFile(filepath.Join(batchDir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var raw struct {
		GroundingPacks []groundingPack `json:"grounding_packs"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode manifest grounding_packs: %w", err)
	}
	return raw.GroundingPacks, nil
}
```

#### loadTechnicalAgentContexts (func)

```go
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
```

#### manifestSkill (func)

```go
func manifestSkill(batchDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(batchDir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read manifest: %w", err)
	}
	var raw struct {
		ReviewAgents map[string][]string `json:"review_agents"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("decode manifest review_agents: %w", err)
	}
	for skill := range raw.ReviewAgents {
		if strings.TrimSpace(skill) != "" {
			return skill, nil
		}
	}
	return "", nil
}
```

#### resolveTechnicalDeepSkill (func)

```go
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
```

#### stageDeepArgs (type)

```go
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
```

#### stageDeepFile (func)

```go
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
			"file":         a.FilePath,
			"slug":         slug,
			"mode":         mode,
			"chunk":        chunkVal,
			"total_chunks": totalChunks,
			"input_file":   inputFilename,
			"agent":        "pr-review-technical-deep",
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
```


## ./internal/agenting
- package: `agenting`
- packageDoc: Package agenting loads and selects context-branch grounding packs for review prep.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: GroundingName, Index, IndexRelPath, ModeDigest, ModeFiles, ModeSummary, ModeTechnical, Pack, StagedPack
- exportedFuncs: LoadIndex, MatchGlob, ModeForSkill, Select, Stage, ValidateIndex
- exportedMethods: Index.PackIDs
- unexportedDecls: stagingDirName
- unexportedFuncs: anyGlobMatch, filepathToSlash, matchStarSlash, modeAllowed, packOrder, validMode
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agenting/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agenting/index.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agenting/match.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agenting/select.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/agenting/stage.go

### Exported bodies

#### Index (type)

```go
type Index struct {
	Packs map[string]Pack `yaml:"packs"`
	order []string
}
```

#### Pack (type)

```go
type Pack struct {
	Globs []string `yaml:"globs,omitempty"`
	Modes []string `yaml:"modes"`
}
```

#### LoadIndex (func)

```go
func LoadIndex(contextDir string) (Index, error) {
	path := filepath.Join(contextDir, IndexRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return Index{}, fmt.Errorf("read %s: %w", IndexRelPath, err)
	}
	var doc struct {
		Packs map[string]Pack `yaml:"packs"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Index{}, fmt.Errorf("parse %s: %w", IndexRelPath, err)
	}
	if len(doc.Packs) == 0 {
		return Index{}, fmt.Errorf("%s: packs is required", IndexRelPath)
	}
	order := packOrder(data)
	if len(order) == 0 {
		for id := range doc.Packs {
			order = append(order, id)
		}
	}
	idx := Index{Packs: doc.Packs, order: order}
	if err := ValidateIndex(idx); err != nil {
		return Index{}, err
	}
	return idx, nil
}
```

#### ValidateIndex (func)

```go
func ValidateIndex(idx Index) error {
	if len(idx.Packs) == 0 {
		return fmt.Errorf("agenting index: no packs defined")
	}
	for id, pack := range idx.Packs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("agenting index: empty pack id")
		}
		if len(pack.Modes) == 0 {
			return fmt.Errorf("agenting index: pack %q requires modes", id)
		}
		for _, m := range pack.Modes {
			if !validMode(m) {
				return fmt.Errorf("agenting index: pack %q has unknown mode %q", id, m)
			}
		}
	}
	return nil
}
```

#### Index.PackIDs (method)

```go
func (idx Index) PackIDs() []string {
	out := make([]string, len(idx.order))
	copy(out, idx.order)
	return out
}
```

#### MatchGlob (func)

```go
func MatchGlob(pattern, name string) bool {
	pattern = strings.ReplaceAll(pattern, "**", "*")
	return matchStarSlash(pattern, name)
}
```

#### Select (func)

```go
func Select(idx Index, mode string, changedFiles []string) []string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	var out []string
	seen := map[string]struct{}{}
	order := idx.PackIDs()
	for _, id := range order {
		pack, ok := idx.Packs[id]
		if !ok {
			continue
		}
		if !modeAllowed(pack, mode) {
			continue
		}
		if len(pack.Globs) == 0 || anyGlobMatch(pack.Globs, changedFiles) {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}
```

#### ModeForSkill (func)

```go
func ModeForSkill(skill string) string {
	switch strings.TrimSpace(skill) {
	case "pr-review-summary", "pr-review-blast-radius":
		return ModeSummary
	case "pr-review-technical":
		return ModeTechnical
	default:
		return ModeFiles
	}
}
```

#### StagedPack (type)

```go
type StagedPack struct {
	ID       string `json:"id"`
	Filename string `json:"file"`
}
```

#### Stage (func)

```go
func Stage(contextDir, batchDir string, packIDs []string) ([]StagedPack, error) {
	if len(packIDs) == 0 {
		return nil, nil
	}
	outDir := filepath.Join(batchDir, stagingDirName)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	var staged []StagedPack
	for _, id := range packIDs {
		src := filepath.Join(contextDir, "agenting", id, GroundingName)
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("read grounding pack %q: %w", id, err)
		}
		name := id + ".md"
		dst := filepath.Join(outDir, name)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return nil, fmt.Errorf("stage grounding pack %q: %w", id, err)
		}
		staged = append(staged, StagedPack{ID: id, Filename: filepath.Join(stagingDirName, name)})
	}
	return staged, nil
}
```

### Private one-hop bodies

#### anyGlobMatch (func)

```go
func anyGlobMatch(globs, files []string) bool {
	for _, file := range files {
		for _, g := range globs {
			if MatchGlob(g, filepathToSlash(file)) {
				return true
			}
		}
	}
	return false
}
```

#### matchStarSlash (func)

```go
func matchStarSlash(pattern, name string) bool {
	var match func(p, n int) bool
	match = func(p, n int) bool {
		for p < len(pattern) {
			if pattern[p] == '*' {
				for ; p < len(pattern) && pattern[p] == '*'; p++ {
				}
				if p == len(pattern) {
					return true
				}
				for i := n; i <= len(name); i++ {
					if match(p, i) {
						return true
					}
				}
				return false
			}
			if n >= len(name) || pattern[p] != name[n] {
				if pattern[p] == '?' && n < len(name) {
					p++
					n++
					continue
				}
				return false
			}
			p++
			n++
		}
		return n == len(name)
	}
	return match(0, 0)
}
```

#### modeAllowed (func)

```go
func modeAllowed(pack Pack, mode string) bool {
	for _, m := range pack.Modes {
		if strings.ToLower(strings.TrimSpace(m)) == mode {
			return true
		}
	}
	return false
}
```

#### packOrder (func)

```go
func packOrder(raw []byte) []string {
	var root yaml.Node
	if yaml.Unmarshal(raw, &root) != nil || len(root.Content) == 0 {
		return nil
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == "packs" && doc.Content[i+1].Kind == yaml.MappingNode {
			var ids []string
			for j := 0; j+1 < len(doc.Content[i+1].Content); j += 2 {
				ids = append(ids, doc.Content[i+1].Content[j].Value)
			}
			return ids
		}
	}
	return nil
}
```

#### validMode (func)

```go
func validMode(m string) bool {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case ModeFiles, ModeSummary, ModeTechnical, ModeDigest:
		return true
	default:
		return false
	}
}
```


## ./internal/aigateway
- package: `aigateway`
- packageDoc: Package aigateway embeds Bifrost as an in-process LLM gateway.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: true
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- deliveryHint: server-http
- mechanicalRole: server
- mechanicalConfidence: 0.90
- mechanicalEvidence: delivery:http, imports_net_http, http_route_register
- exportedDecls: Account, DummyAPIKey, Gateway
- exportedFuncs: Ensure, LogicalModel, NewAccountFromEnv, PrepareChildEnv, ResetForTests, ShutdownGlobal, Start
- exportedMethods: Account.GetConfigForProvider, Account.GetConfiguredProviders, Account.GetKeysForProvider, Account.HasProviders, Account.Providers, Gateway.BaseURL, Gateway.ChildEnv, Gateway.Origin, Gateway.Shutdown
- unexportedDecls: _, defaultAnthropicModel, defaultGeminiModel, defaultOpenAIModel, envAnthropic, envGemini, envGoogle, envGoogleAI, envOpenAI, globalGW, globalMu, openAIChatRequest, openAIMessage, route
- unexportedFuncs: defaultModelFor, detectProvider, envOr, geminiKey, messageContentString, providerPreference, resolveRoute, toBifrostMessages, writeOpenAIChatResponse, writeOpenAIError
- unexportedMethods: Account.add, Gateway.handleChatCompletions
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/aigateway/account.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/aigateway/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/aigateway/gateway.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/aigateway/route.go

### Exported bodies

#### Account (type)

```go
type Account struct {
	providers []schemas.ModelProvider
	keys      map[schemas.ModelProvider][]schemas.Key
}
```

#### NewAccountFromEnv (func)

```go
func NewAccountFromEnv() (*Account, error) {
	a := &Account{
		keys: make(map[schemas.ModelProvider][]schemas.Key),
	}
	if v := strings.TrimSpace(os.Getenv(envAnthropic)); v != "" {
		a.add(schemas.Anthropic, v)
	}
	if v := strings.TrimSpace(os.Getenv(envOpenAI)); v != "" {
		a.add(schemas.OpenAI, v)
	}
	if v := geminiKey(); v != "" {
		a.add(schemas.Gemini, v)
	}
	if len(a.providers) == 0 {
		return nil, fmt.Errorf("aigateway: no LLM provider keys (set %s, %s, or %s)", envAnthropic, envOpenAI, envGemini)
	}
	return a, nil
}
```

#### Account.HasProviders (method)

```go
func (a *Account) HasProviders() bool {
	return a != nil && len(a.providers) > 0
}
```

#### Account.Providers (method)

```go
func (a *Account) Providers() []schemas.ModelProvider {
	if a == nil {
		return nil
	}
	out := make([]schemas.ModelProvider, len(a.providers))
	copy(out, a.providers)
	return out
}
```

#### Account.GetConfiguredProviders (method)

```go
func (a *Account) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	if !a.HasProviders() {
		return nil, fmt.Errorf("aigateway: no providers configured")
	}
	return a.Providers(), nil
}
```

#### Account.GetKeysForProvider (method)

```go
func (a *Account) GetKeysForProvider(_ context.Context, provider schemas.ModelProvider) ([]schemas.Key, error) {
	keys, ok := a.keys[provider]
	if !ok || len(keys) == 0 {
		return nil, fmt.Errorf("aigateway: no keys for provider %s", provider)
	}
	return keys, nil
}
```

#### Account.GetConfigForProvider (method)

```go
func (a *Account) GetConfigForProvider(provider schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	if _, ok := a.keys[provider]; !ok {
		return nil, fmt.Errorf("aigateway: provider %s not configured", provider)
	}
	net := schemas.DefaultNetworkConfig
	net.MaxRetries = 4
	net.RetryBackoffInitial = 200 * time.Millisecond
	net.RetryBackoffMax = 8 * time.Second
	net.DefaultRequestTimeoutInSeconds = 120
	net.AllowPrivateNetwork = true // loopback/self-hosted OpenAI-compat
	return &schemas.ProviderConfig{
		NetworkConfig: net,
		ConcurrencyAndBufferSize: schemas.ConcurrencyAndBufferSize{
			Concurrency: 16,
			BufferSize:  64,
		},
	}, nil
}
```

#### Gateway (type)

```go
type Gateway struct {
	account *Account
	client  *bifrost.Bifrost
	server  *http.Server
	ln      net.Listener
	baseURL string // e.g. http://127.0.0.1:port/v1

	mu      sync.Mutex
	started bool
}
```

#### Ensure (func)

```go
func Ensure() (*Gateway, error) {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalGW != nil && globalGW.started {
		return globalGW, nil
	}
	account, err := NewAccountFromEnv()
	if err != nil {
		return nil, err
	}
	gw, err := Start(context.Background(), account)
	if err != nil {
		return nil, err
	}
	globalGW = gw
	return gw, nil
}
```

#### ShutdownGlobal (func)

```go
func ShutdownGlobal() {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalGW != nil {
		globalGW.Shutdown()
		globalGW = nil
	}
}
```

#### ResetForTests (func)

```go
func ResetForTests() {
	ShutdownGlobal()
}
```

#### Start (func)

```go
func Start(ctx context.Context, account *Account) (*Gateway, error) {
	if ctx == nil {
		return nil, fmt.Errorf("aigateway: context is required")
	}
	if account == nil || !account.HasProviders() {
		return nil, fmt.Errorf("aigateway: account required")
	}
	client, err := bifrost.Init(ctx, schemas.BifrostConfig{
		Account:         account,
		InitialPoolSize: 64,
	})
	if err != nil {
		return nil, fmt.Errorf("aigateway: bifrost init: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		client.Shutdown()
		return nil, fmt.Errorf("aigateway: listen: %w", err)
	}
	gw := &Gateway{
		account: account,
		client:  client,
		ln:      ln,
		baseURL: fmt.Sprintf("http://%s/v1", ln.Addr().String()),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", gw.handleChatCompletions)
	mux.HandleFunc("/openai/v1/chat/completions", gw.handleChatCompletions)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	gw.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = gw.server.Serve(ln) }()
	gw.started = true
	return gw, nil
}
```

#### Gateway.BaseURL (method)

```go
func (g *Gateway) BaseURL() string {
	if g == nil {
		return ""
	}
	return g.baseURL
}
```

#### Gateway.Origin (method)

```go
func (g *Gateway) Origin() string {
	if g == nil {
		return ""
	}
	return strings.TrimSuffix(g.baseURL, "/v1")
}
```

#### Gateway.Shutdown (method)

```go
func (g *Gateway) Shutdown() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.started {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if g.server != nil {
		_ = g.server.Shutdown(ctx)
	}
	if g.client != nil {
		g.client.Shutdown()
	}
	g.started = false
}
```

#### PrepareChildEnv (func)

```go
func PrepareChildEnv(parent []string) ([]string, error) {
	gw, err := Ensure()
	if err != nil {
		return nil, err
	}
	return gw.ChildEnv(parent), nil
}
```

#### Gateway.ChildEnv (method)

```go
func (g *Gateway) ChildEnv(parent []string) []string {
	if g == nil {
		return parent
	}
	strip := map[string]struct{}{
		envAnthropic: {},
		envOpenAI:    {},
		envGemini:    {},
		envGoogle:    {},
		envGoogleAI:  {},
	}
	out := make([]string, 0, len(parent)+8)
	hasConfig := false
	hasConfigContent := false
	hasProvider := false
	for _, e := range parent {
		key, _, _ := strings.Cut(e, "=")
		if _, bad := strip[key]; bad {
			continue
		}
		switch key {
		case "OPENCODE_CONFIG":
			hasConfig = true
		case "OPENCODE_CONFIG_CONTENT":
			hasConfigContent = true
		case "OPENCODE_PROVIDER":
			hasProvider = true
		}
		out = append(out, e)
	}
	out = append(out,
		"OPENAI_API_KEY="+DummyAPIKey,
		"OPENAI_BASE_URL="+g.baseURL,
		"OPENCODE_PROVIDER_API_KEY="+DummyAPIKey,
	)
	if !hasProvider {
		out = append(out, "OPENCODE_PROVIDER=openai")
	}
	if !hasConfig && !hasConfigContent {
		// OpenCode needs provider.options.baseURL; OPENAI_BASE_URL alone is not enough.
		cfg := fmt.Sprintf(
			`{"provider":{"openai":{"options":{"baseURL":%q,"apiKey":"{env:OPENAI_API_KEY}"}}}}`,
			g.baseURL,
		)
		out = append(out, "OPENCODE_CONFIG_CONTENT="+cfg)
	}
	return out
}
```

#### LogicalModel (func)

```go
func LogicalModel(account *Account) string {
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_MODEL")); v != "" {
		return v
	}
	if account == nil || len(account.providers) == 0 {
		return defaultAnthropicModel
	}
	return defaultModelFor(account.providers[0])
}
```

### Private one-hop bodies

#### Account.add (method)

```go
func (a *Account) add(provider schemas.ModelProvider, key string) {
	a.providers = append(a.providers, provider)
	a.keys[provider] = []schemas.Key{{
		ID:     string(provider),
		Name:   string(provider),
		Value:  schemas.SecretVar{Val: key},
		Models: schemas.WhiteList{"*"},
		Weight: 1.0,
	}}
}
```

#### Gateway.handleChatCompletions (method)

```go
func (g *Gateway) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	var req openAIChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Stream {
		writeOpenAIError(w, http.StatusBadRequest, "streaming not supported on embedded gateway")
		return
	}

	primary, fallbacks := resolveRoute(g.account, req.Model)
	if primary.Provider == "" {
		writeOpenAIError(w, http.StatusBadRequest, "no providers configured")
		return
	}

	messages, err := toBifrostMessages(req.Messages)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, err.Error())
		return
	}

	bfReq := &schemas.BifrostChatRequest{
		Provider:  primary.Provider,
		Model:     primary.Model,
		Input:     messages,
		Fallbacks: fallbacks,
	}
	if req.Temperature != nil || req.MaxTokens != nil {
		bfReq.Params = &schemas.ChatParameters{}
		if req.Temperature != nil {
			bfReq.Params.Temperature = req.Temperature
		}
		if req.MaxTokens != nil {
			bfReq.Params.MaxCompletionTokens = req.MaxTokens
		}
	}

	bfCtx := schemas.NewBifrostContext(r.Context(), schemas.NoDeadline)
	resp, bfErr := g.client.ChatCompletionRequest(bfCtx, bfReq)
	if bfErr != nil {
		msg := "provider error"
		status := http.StatusBadGateway
		if bfErr.Error != nil && bfErr.Error.Message != "" {
			msg = bfErr.Error.Message
		}
		if bfErr.StatusCode != nil {
			status = *bfErr.StatusCode
		}
		writeOpenAIError(w, status, msg)
		return
	}
	writeOpenAIChatResponse(w, resp)
}
```

#### defaultModelFor (func)

```go
func defaultModelFor(p schemas.ModelProvider) string {
	switch p {
	case schemas.Anthropic:
		return envOr("MAJORDOMO_ANTHROPIC_MODEL", defaultAnthropicModel)
	case schemas.OpenAI:
		return envOr("MAJORDOMO_OPENAI_MODEL", defaultOpenAIModel)
	case schemas.Gemini:
		return envOr("MAJORDOMO_GEMINI_MODEL", defaultGeminiModel)
	default:
		return ""
	}
}
```

#### geminiKey (func)

```go
func geminiKey() string {
	for _, k := range []string{envGemini, envGoogleAI, envGoogle} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}
```


## ./internal/cache
- package: `cache`
- packageDoc: Package cache reads and writes review-cache, poll-cursor, and digest-inference data on the served repo (branches majordomo-inference-cache/<id> with path prefixes review/ and digest/, plus majordomo-poll-cache/<id>).
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: ClusterAuditCached, ClusterAuditCachedMerge, ClusterAuditFingerprint, ClusterCoTCached, ClusterCoTFingerprint, DigestCachePrefix, DigestClusterAuditPromptV1, DigestClusterAuditSchemaV1, DigestClusterCoTPromptV1, DigestClusterCoTSchemaV1, DigestClusterCoTSchemaV2, DigestClusterCoTSchemaV3, DigestInspectPromptV1, DigestInspectSchemaV1, DigestInspectSchemaV2, DigestInterventionPromptV1, DigestInterventionSchemaV1, DigestInterventionSchemaV2, DigestLedgerPromptV1, DigestLedgerSchemaV1, DigestLedgerSchemaV2, DigestPushOptions, DigestRefinePromptV1, DigestRefinePromptV2, DigestRefineSchemaV1, DigestRefineSchemaV2, DigestRefineSchemaV3, DigestRefineSchemaV4, DigestRefineSchemaV5, DigestRunStats, DigestStore, DigestStoryPromptV1, DigestStoryPromptV2, DigestStorySchemaV1, DigestStorySchemaV2, DigestStorySchemaV3, ExitPatternViolation, InspectCachedRole, InspectFingerprint, InterventionCached, InterventionFingerprint, LedgerCachedEntry, LedgerFingerprint, LookupOptions, Meta, PollCursor, PrecheckOptions, PushOptions, RefineCached, RefineFingerprint, RestoreOptions, ReviewCachePrefix, StoreOptions, StoryCached, StoryFingerprint
- exportedFuncs: AnalysisCacheName, ClusterFilesHash, ContentSHA, CursorPath, FormatStatsLine, HashDigestParts, Lookup, NormalizeClusterFiles, OwnedPackagesSourceHash, OwnedPathsHash, PackageSourceHash, Precheck, PrintJSON, PrintJSONPretty, Push, PushDigest, ReadPollCursor, RecordHead, Restore, ShouldReview, Store, ValidateDigestCacheBranch, ValidateInferenceCacheBranch, ValidatePollCacheBranch, ValidateReviewCacheBranch, WritePollCursor
- exportedMethods: DigestStore.ConfigurePush, DigestStore.Flush, DigestStore.LookupClusterAudit, DigestStore.LookupClusterCoT, DigestStore.LookupInspect, DigestStore.LookupIntervention, DigestStore.LookupLedger, DigestStore.LookupRefine, DigestStore.LookupStory, DigestStore.RecordClusterAuditHit, DigestStore.RecordClusterAuditMiss, DigestStore.RecordClusterCoTHit, DigestStore.RecordClusterCoTMiss, DigestStore.RecordInspectHit, DigestStore.RecordInspectMiss, DigestStore.RecordInterventionHit, DigestStore.RecordInterventionMiss, DigestStore.RecordLedgerHit, DigestStore.RecordLedgerMiss, DigestStore.RecordRefineHit, DigestStore.RecordRefineMiss, DigestStore.RecordStoryHit, DigestStore.RecordStoryMiss, DigestStore.Stats, DigestStore.StoreClusterAudit, DigestStore.StoreClusterCoT, DigestStore.StoreInspect, DigestStore.StoreIntervention, DigestStore.StoreLedger, DigestStore.StoreRefine, DigestStore.StoreStory
- unexportedDecls: analysisNameRE, cacheRecord, clusterSHARE, digestRecord, frontmatterDelim, indexMetaKeys, inferenceCacheBranchRE, pollBranchRE, requiredFields, storeMetadataOrder, timestampFmt, validKeyRE
- unexportedFuncs: asStringSlice, buildIndex, collectCacheFiles, collectMarkdownArtifact, existingCachePayloadUnchanged, extractFrontmatter, first, formatFrontmatter, isInside, isSourceFileName, loadCacheRecord, loadManifestSlugs, parseClusterFilesArgument, parseFrontmatterBlock, readDigestRecord, resolveLookupEntry, resolveRetentionDays, sha256Hex, stringSlicesEqual, stripWrappedQuotes, toRel, validateClusterFilesFields, validateClusterSHAField, validateCreatedAtField, validateMetadata, writeDigestRecord, writeJSONFile, writeMarkdownArtifactFile
- unexportedMethods: ClusterAuditFingerprint.key, ClusterCoTFingerprint.key, DigestStore.avgLocked, DigestStore.clusterAuditPath, DigestStore.clusterCoTPath, DigestStore.flushAfterStore, DigestStore.inspectPath, DigestStore.interventionPath, DigestStore.ledgerPath, DigestStore.noteClusterAuditStoredUsage, DigestStore.noteClusterCoTStoredUsage, DigestStore.noteInspectStoredUsage, DigestStore.noteInterventionStoredUsage, DigestStore.noteLedgerStoredUsage, DigestStore.noteRefineStoredUsage, DigestStore.noteStoryStoredUsage, DigestStore.recordTokenHit, DigestStore.refinePath, DigestStore.storyPath, InspectFingerprint.key, InterventionFingerprint.key, LedgerFingerprint.key, RefineFingerprint.key, StoryFingerprint.key
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cache/cache.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cache/cluster.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cache/cursor.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cache/digest.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cache/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cache/source_hash.go

### Exported bodies

#### ValidateInferenceCacheBranch (func)

```go
func ValidateInferenceCacheBranch(branch string) error {
	if !inferenceCacheBranchRE.MatchString(branch) {
		return fmt.Errorf("cache branch %q does not match majordomo-inference-cache/<repo-id>", branch)
	}
	return nil
}
```

#### ValidateReviewCacheBranch (func)

```go
func ValidateReviewCacheBranch(branch string) error {
	return ValidateInferenceCacheBranch(branch)
}
```

#### ValidatePollCacheBranch (func)

```go
func ValidatePollCacheBranch(branch string) error {
	if !pollBranchRE.MatchString(branch) {
		return fmt.Errorf("poll cache branch %q must match majordomo-poll-cache/<repo-id>", branch)
	}
	return nil
}
```

#### PushOptions (type)

```go
type PushOptions struct {
	Remote   string
	Branch   string
	Worktree string
	Token    string // BITBUCKET_TOKEN or GITHUB_TOKEN
}
```

#### Push (func)

```go
func Push(opts PushOptions) error {
	if err := ValidateInferenceCacheBranch(opts.Branch); err != nil {
		return err
	}
	if opts.Remote == "" || opts.Worktree == "" {
		return fmt.Errorf("--remote and --worktree required")
	}
	token := opts.Token
	if token == "" {
		token = first(os.Getenv("BITBUCKET_TOKEN"), os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))
	}
	if token == "" {
		return fmt.Errorf("token required (BITBUCKET_TOKEN or GITHUB_TOKEN)")
	}
	if !strings.HasPrefix(opts.Remote, "https://") {
		return fmt.Errorf("remote URL must be https")
	}
	info, err := os.Stat(opts.Worktree)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("worktree not found: %s", opts.Worktree)
	}
	authArgs := githttps.ExtraHeaderArgs(token, githttps.InferSCM(opts.Remote))
	run := func(args ...string) error {
		cmdArgs := append([]string{"-C", opts.Worktree}, authArgs...)
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command("git", cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// Best-effort fetch: branch may not exist yet on first push.
	if err := run("fetch", opts.Remote, opts.Branch+":"+opts.Branch); err != nil {
		_ = run("fetch", opts.Remote, opts.Branch)
	}
	err = run("push", opts.Remote, "HEAD:"+opts.Branch)
	if err != nil {
		if ferr := run("fetch", opts.Remote, opts.Branch); ferr != nil {
			return fmt.Errorf("cache push failed (%v); refetch also failed: %w", err, ferr)
		}
		err = run("push", opts.Remote, "HEAD:"+opts.Branch)
		if err != nil {
			return fmt.Errorf("cache push failed after refetch: %w", err)
		}
	}
	return nil
}
```

#### PollCursor (type)

```go
type PollCursor struct {
	RepoID  string            `json:"repo_id"`
	Heads   map[string]string `json:"heads"` // pr_number -> head_sha
	Updated string            `json:"updated,omitempty"`
}
```

#### CursorPath (func)

```go
func CursorPath(worktree string) string {
	return filepath.Join(worktree, "poll-cursor.json")
}
```

#### Meta (type)

```go
type Meta map[string]any
```

#### PrecheckOptions (type)

```go
type PrecheckOptions struct {
	ProjectID            string
	CacheDir             string
	ProjectRetentionDays *int
	CentralRetentionDays *int
	GlobalRetentionDays  int
	MinRetentionDays     int
	IndexOut             string
}
```

#### LookupOptions (type)

```go
type LookupOptions struct {
	IndexFile             string
	ClusterSHA            string
	SkillName             string
	FingerprintVersion    string
	ClusterFiles          []string
	ClusterFilesFile      string
	ModelID               string
	ModelRevision         string
	InstructionBundleHash string
	PromptTemplateHash    string
	ScoringRubricHash     string
	OutputSchemaVersion   string
}
```

#### StoreOptions (type)

```go
type StoreOptions struct {
	CacheDir              string
	SkillName             string
	ClusterSHA            string
	FingerprintVersion    string
	ClusterFiles          []string
	ClusterFilesFile      string
	ModelID               string
	ModelRevision         string
	InstructionBundleHash string
	PromptTemplateHash    string
	ScoringRubricHash     string
	OutputSchemaVersion   string
	AnalysisFile          string
	ReportsDir            string
	ArtifactFiles         []string
}
```

#### RestoreOptions (type)

```go
type RestoreOptions struct {
	CacheDir  string
	EntryFile string
	OutputDir string
}
```

#### Precheck (func)

```go
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
		clusterSHA, ok := record.metadata["cluster_sha"].(string)
		if ok && clusterSHA != "" && clusterSHA != match[1] {
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
		skillName, skillNameOK := metadataForIndex["skill_name"].(string)
		clusterSHA, clusterSHAOK := metadataForIndex["cluster_sha"].(string)
		if skillNameOK && strings.TrimSpace(skillName) != "" && clusterSHAOK && clusterSHA != "" {
			indexEntries[skillName+":"+clusterSHA] = metadataForIndex
		} else if clusterSHAOK && clusterSHA != "" {
			indexEntries[clusterSHA] = metadataForIndex
		}
	}

	result := map[string]any{
		"project_id":       opts.ProjectID,
		"cache_dir":        filepath.ToSlash(cacheDir),
		"retention_days":   retentionDays,
		"retention_source": source,
		"scanned_files":    len(collectCacheFiles(cacheDir)),
		"expired_deleted":  len(expiredFiles),
		"expired_files":    expiredFiles,
		"invalid_files":    invalidFiles,
		"valid_entries":    len(indexEntries),
		"index":            indexEntries,
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
```

#### Lookup (func)

```go
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
	fileVal, ok := entry["file"].(string)
	if !ok || strings.TrimSpace(fileVal) == "" {
		return nil, fmt.Errorf("index entry missing string file")
	}
	return map[string]any{
		"hit":         true,
		"cluster_sha": opts.ClusterSHA,
		"file":        fileVal,
	}, nil
}
```

#### Store (func)

```go
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
			markdownArtifactFile = ReviewCachePrefix + "/" + opts.SkillName + "/markdown-" + opts.ClusterSHA + ".json"
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

	skillDir := filepath.Join(cacheDir, ReviewCachePrefix, opts.SkillName)
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
```

#### Restore (func)

```go
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
	artifactRel, ok := record.metadata["markdown_artifact_file"].(string)
	if !ok {
		return nil, fmt.Errorf("cache entry markdown_artifact_file must be a string")
	}
	if strings.TrimSpace(artifactRel) == "" {
		return map[string]any{
			"restored":   false,
			"reason":     "no-markdown-artifact",
			"entry_file": opts.EntryFile,
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
```

#### PrintJSON (func)

```go
func PrintJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(data))
	return err
}
```

#### PrintJSONPretty (func)

```go
func PrintJSONPretty(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(data))
	return err
}
```

#### NormalizeClusterFiles (func)

```go
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
```

#### ClusterFilesHash (func)

```go
func ClusterFilesHash(files []string) string {
	joined := strings.Join(NormalizeClusterFiles(files), "\n")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])
}
```

#### ReadPollCursor (func)

```go
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
```

#### WritePollCursor (func)

```go
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
```

#### ShouldReview (func)

```go
func ShouldReview(c *PollCursor, prNumber, headSHA string, continuous bool) bool {
	if c == nil || c.Heads == nil {
		return true
	}
	prev, ok := c.Heads[prNumber]
	if !ok {
		return true
	}
	if !continuous {
		return false
	}
	return prev != headSHA
}
```

#### RecordHead (func)

```go
func RecordHead(c *PollCursor, prNumber, headSHA string) {
	if c.Heads == nil {
		c.Heads = map[string]string{}
	}
	c.Heads[prNumber] = headSHA
}
```

#### AnalysisCacheName (func)

```go
func AnalysisCacheName(clusterSHA, ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	return fmt.Sprintf("analysis-%s.%s", clusterSHA, ext)
}
```

#### ValidateDigestCacheBranch (func)

```go
func ValidateDigestCacheBranch(branch string) error {
	return ValidateInferenceCacheBranch(branch)
}
```

#### DigestRunStats (type)

```go
type DigestRunStats struct {
	InspectHits              int
	InspectMisses            int
	LedgerHits               int
	LedgerMisses             int
	ClusterAuditHits         int
	ClusterAuditMisses       int
	ClusterCoTHits           int
	ClusterCoTMisses         int
	RefineHits               int
	RefineMisses             int
	InterventionHits         int
	InterventionMisses       int
	StoryHits                int
	StoryMisses              int
	TokensSavedPrompt        int
	TokensSavedCompletion    int
	TokensSavedTotal         int
	inspectStoredTotalSum    int
	inspectStoredTotalN      int
	ledgerStoredTotalSum     int
	ledgerStoredTotalN       int
	clusterStoredTotalSum    int
	clusterStoredTotalN      int
	clusterCoTStoredTotalSum int
	clusterCoTStoredTotalN   int
	refineStoredTotalSum     int
	refineStoredTotalN       int
	interventionStoredSum    int
	interventionStoredN      int
	storyStoredTotalSum      int
	storyStoredTotalN        int
}
```

#### DigestStore (type)

```go
type DigestStore struct {
	Dir string

	pushMu       sync.Mutex
	pushOpts     *DigestPushOptions
	PushFn       func() error // optional test seam; overrides PushDigest when set
	OnFlushError func(error)

	statsMu sync.Mutex
	stats   DigestRunStats
}
```

#### DigestStore.ConfigurePush (method)

```go
func (s *DigestStore) ConfigurePush(opts DigestPushOptions) {
	if s == nil {
		return
	}
	cp := opts
	s.pushOpts = &cp
}
```

#### DigestStore.Flush (method)

```go
func (s *DigestStore) Flush() error {
	if s == nil {
		return nil
	}
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	var err error
	switch {
	case s.PushFn != nil:
		err = s.PushFn()
	case s.pushOpts != nil:
		err = PushDigest(*s.pushOpts)
	default:
		return nil
	}
	if err != nil && s.OnFlushError != nil {
		s.OnFlushError(err)
	}
	return err
}
```

#### DigestStore.Stats (method)

```go
func (s *DigestStore) Stats() DigestRunStats {
	if s == nil {
		return DigestRunStats{}
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return s.stats
}
```

#### DigestStore.RecordInspectHit (method)

```go
func (s *DigestStore) RecordInspectHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.InspectHits++
	if total <= 0 {
		total = s.avgLocked(s.stats.inspectStoredTotalSum, s.stats.inspectStoredTotalN)
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	s.stats.TokensSavedPrompt += prompt
	s.stats.TokensSavedCompletion += completion
	s.stats.TokensSavedTotal += total
}
```

#### DigestStore.RecordInspectMiss (method)

```go
func (s *DigestStore) RecordInspectMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.InspectMisses++
}
```

#### DigestStore.RecordLedgerHit (method)

```go
func (s *DigestStore) RecordLedgerHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.LedgerHits++
	if total <= 0 {
		total = s.avgLocked(s.stats.ledgerStoredTotalSum, s.stats.ledgerStoredTotalN)
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	s.stats.TokensSavedPrompt += prompt
	s.stats.TokensSavedCompletion += completion
	s.stats.TokensSavedTotal += total
}
```

#### DigestStore.RecordLedgerMiss (method)

```go
func (s *DigestStore) RecordLedgerMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.LedgerMisses++
}
```

#### DigestStore.RecordClusterAuditHit (method)

```go
func (s *DigestStore) RecordClusterAuditHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.ClusterAuditHits++
	if total <= 0 {
		total = s.avgLocked(s.stats.clusterStoredTotalSum, s.stats.clusterStoredTotalN)
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	s.stats.TokensSavedPrompt += prompt
	s.stats.TokensSavedCompletion += completion
	s.stats.TokensSavedTotal += total
}
```

#### DigestStore.RecordClusterAuditMiss (method)

```go
func (s *DigestStore) RecordClusterAuditMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.ClusterAuditMisses++
}
```

#### DigestStore.RecordClusterCoTHit (method)

```go
func (s *DigestStore) RecordClusterCoTHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.recordTokenHit(prompt, completion, total, func(st *DigestRunStats) { st.ClusterCoTHits++ }, &s.stats.clusterCoTStoredTotalSum, &s.stats.clusterCoTStoredTotalN)
}
```

#### DigestStore.RecordClusterCoTMiss (method)

```go
func (s *DigestStore) RecordClusterCoTMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.ClusterCoTMisses++
}
```

#### DigestStore.RecordRefineHit (method)

```go
func (s *DigestStore) RecordRefineHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.recordTokenHit(prompt, completion, total, func(st *DigestRunStats) { st.RefineHits++ }, &s.stats.refineStoredTotalSum, &s.stats.refineStoredTotalN)
}
```

#### DigestStore.RecordRefineMiss (method)

```go
func (s *DigestStore) RecordRefineMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.RefineMisses++
}
```

#### DigestStore.RecordInterventionHit (method)

```go
func (s *DigestStore) RecordInterventionHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.recordTokenHit(prompt, completion, total, func(st *DigestRunStats) { st.InterventionHits++ }, &s.stats.interventionStoredSum, &s.stats.interventionStoredN)
}
```

#### DigestStore.RecordInterventionMiss (method)

```go
func (s *DigestStore) RecordInterventionMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.InterventionMisses++
}
```

#### DigestStore.RecordStoryHit (method)

```go
func (s *DigestStore) RecordStoryHit(prompt, completion, total int) {
	if s == nil {
		return
	}
	s.recordTokenHit(prompt, completion, total, func(st *DigestRunStats) { st.StoryHits++ }, &s.stats.storyStoredTotalSum, &s.stats.storyStoredTotalN)
}
```

#### DigestStore.RecordStoryMiss (method)

```go
func (s *DigestStore) RecordStoryMiss() {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.StoryMisses++
}
```

#### FormatStatsLine (func)

```go
func FormatStatsLine(st DigestRunStats) string {
	return fmt.Sprintf(
		"digest cache summary inspect_hits=%d inspect_misses=%d ledger_hits=%d ledger_misses=%d cluster_audit_hits=%d cluster_audit_misses=%d cluster_cot_hits=%d cluster_cot_misses=%d refine_hits=%d refine_misses=%d intervention_hits=%d intervention_misses=%d story_hits=%d story_misses=%d estimated_tokens_saved=%d (prompt=%d completion=%d)",
		st.InspectHits, st.InspectMisses, st.LedgerHits, st.LedgerMisses,
		st.ClusterAuditHits, st.ClusterAuditMisses,
		st.ClusterCoTHits, st.ClusterCoTMisses,
		st.RefineHits, st.RefineMisses,
		st.InterventionHits, st.InterventionMisses,
		st.StoryHits, st.StoryMisses,
		st.TokensSavedTotal, st.TokensSavedPrompt, st.TokensSavedCompletion,
	)
}
```

#### InspectFingerprint (type)

```go
type InspectFingerprint struct {
	PackagePath   string
	ContextSHA    string
	ModelID       string
	PromptVersion string
	SchemaVersion string
}
```

#### LedgerFingerprint (type)

```go
type LedgerFingerprint struct {
	SliceID         string
	OwnedPathsHash  string
	ContextSHA      string
	ConstraintsHash string
	ClusterHash     string
	ModelID         string
	PromptVersion   string
	SchemaVersion   string
}
```

#### InspectCachedRole (type)

```go
type InspectCachedRole struct {
	Path             string   `json:"path"`
	Role             string   `json:"role"`
	Confidence       float64  `json:"confidence"`
	Evidence         []string `json:"evidence"`
	InspectedStage   int      `json:"inspected_stage"`
	Language         string   `json:"language,omitempty"`
	CandidateRole    string   `json:"candidate_role,omitempty"`
	MechanicalRole   string   `json:"mechanical_role,omitempty"`
	LLMRole          string   `json:"llm_role,omitempty"`
	Agreement        string   `json:"agreement,omitempty"`
	RLMIterations    int      `json:"rlm_iterations,omitempty"`
	PromptTokens     int      `json:"prompt_tokens,omitempty"`
	CompletionTokens int      `json:"completion_tokens,omitempty"`
	TotalTokens      int      `json:"total_tokens,omitempty"`
}
```

#### LedgerCachedEntry (type)

```go
type LedgerCachedEntry struct {
	ID               string   `json:"id"`
	OwnedPaths       []string `json:"owned_paths,omitempty"`
	Evidence         []string `json:"evidence"`
	Claims           []string `json:"claims"`
	Objective        string   `json:"objective"`
	Verdict          string   `json:"verdict"`
	Source           string   `json:"source,omitempty"`
	PromptTokens     int      `json:"prompt_tokens,omitempty"`
	CompletionTokens int      `json:"completion_tokens,omitempty"`
	TotalTokens      int      `json:"total_tokens,omitempty"`
}
```

#### ClusterAuditFingerprint (type)

```go
type ClusterAuditFingerprint struct {
	MergesHash      string
	RolesHash       string
	ConstraintsHash string
	MechanicalHash  string
	ModelID         string
	PromptVersion   string
	SchemaVersion   string
}
```

#### ClusterAuditCachedMerge (type)

```go
type ClusterAuditCachedMerge struct {
	ID       string   `json:"id"`
	Packages []string `json:"packages"`
	Verdict  string   `json:"verdict"`
	Reason   string   `json:"reason,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}
```

#### ClusterAuditCached (type)

```go
type ClusterAuditCached struct {
	Merges           []ClusterAuditCachedMerge `json:"merges"`
	RLMIterations    int                       `json:"rlm_iterations,omitempty"`
	PromptTokens     int                       `json:"prompt_tokens,omitempty"`
	CompletionTokens int                       `json:"completion_tokens,omitempty"`
	TotalTokens      int                       `json:"total_tokens,omitempty"`
}
```

#### ClusterCoTFingerprint (type)

```go
type ClusterCoTFingerprint struct {
	DraftHash       string
	RolesHash       string
	ConstraintsHash string
	MechanicalHash  string
	ModelID         string
	PromptVersion   string
	SchemaVersion   string
}
```

#### ClusterCoTCached (type)

```go
type ClusterCoTCached struct {
	MergeIDs         string `json:"merge_ids"`
	MergePackages    string `json:"merge_packages"`
	MergeIntents     string `json:"merge_intents"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
}
```

#### RefineFingerprint (type)

```go
type RefineFingerprint struct {
	DraftHash       string
	RolesHash       string
	ConstraintsHash string
	LedgerHash      string
	VerdictsHash    string
	MechanicalHash  string
	ModelID         string
	PromptVersion   string
	SchemaVersion   string
}
```

#### RefineCached (type)

```go
type RefineCached struct {
	RefinedCatalogYAML string `json:"refined_catalog_yaml"`
	PromptTokens       int    `json:"prompt_tokens,omitempty"`
	CompletionTokens   int    `json:"completion_tokens,omitempty"`
	TotalTokens        int    `json:"total_tokens,omitempty"`
}
```

#### InterventionFingerprint (type)

```go
type InterventionFingerprint struct {
	TaskID           string
	ArchitectureHash string
	RefinedHash      string
	VerdictsHash     string
	FindingsHash     string
	FindingHash      string // per finding-comment; empty for journey/brief/weaknesses/priority
	ModelID          string
	PromptVersion    string
	SchemaVersion    string
}
```

#### InterventionCached (type)

```go
type InterventionCached struct {
	Markdown         string `json:"markdown"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
}
```

#### StoryFingerprint (type)

```go
type StoryFingerprint struct {
	SectionID        string
	RefinedHash      string
	LedgerHash       string
	ArchitectureHash string
	ReadmeHash       string
	ModelID          string
	PromptVersion    string
	SchemaVersion    string
}
```

#### StoryCached (type)

```go
type StoryCached struct {
	Markdown         string `json:"markdown"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
}
```

#### HashDigestParts (func)

```go
func HashDigestParts(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
```

#### ContentSHA (func)

```go
func ContentSHA(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
```

#### OwnedPathsHash (func)

```go
func OwnedPathsHash(paths []string) string {
	return ClusterFilesHash(paths)
}
```

#### DigestStore.LookupInspect (method)

```go
func (s *DigestStore) LookupInspect(fp InspectFingerprint) (InspectCachedRole, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return InspectCachedRole{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.inspectPath(key), key)
	if err != nil || !ok {
		return InspectCachedRole{}, false, err
	}
	var out InspectCachedRole
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return InspectCachedRole{}, false, fmt.Errorf("digest inspect payload: %w", err)
	}
	return out, true, nil
}
```

#### DigestStore.StoreInspect (method)

```go
func (s *DigestStore) StoreInspect(fp InspectFingerprint, role InspectCachedRole) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	key := fp.key()
	payload, err := json.Marshal(role)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.inspectPath(key), digestRecord{
		Kind:        "inspect",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteInspectStoredUsage(role.TotalTokens)
	s.flushAfterStore()
	return nil
}
```

#### DigestStore.LookupLedger (method)

```go
func (s *DigestStore) LookupLedger(fp LedgerFingerprint) (LedgerCachedEntry, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return LedgerCachedEntry{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.ledgerPath(key), key)
	if err != nil || !ok {
		return LedgerCachedEntry{}, false, err
	}
	var out LedgerCachedEntry
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return LedgerCachedEntry{}, false, fmt.Errorf("digest ledger payload: %w", err)
	}
	return out, true, nil
}
```

#### DigestStore.StoreLedger (method)

```go
func (s *DigestStore) StoreLedger(fp LedgerFingerprint, entry LedgerCachedEntry) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.Verdict) != "grounded" {
		return fmt.Errorf("digest ledger store refuses verdict %q", entry.Verdict)
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.ledgerPath(key), digestRecord{
		Kind:        "ledger",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteLedgerStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}
```

#### DigestStore.LookupClusterAudit (method)

```go
func (s *DigestStore) LookupClusterAudit(fp ClusterAuditFingerprint) (ClusterAuditCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return ClusterAuditCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.clusterAuditPath(key), key)
	if err != nil || !ok {
		return ClusterAuditCached{}, false, err
	}
	var out ClusterAuditCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return ClusterAuditCached{}, false, fmt.Errorf("digest cluster_audit payload: %w", err)
	}
	return out, true, nil
}
```

#### DigestStore.StoreClusterAudit (method)

```go
func (s *DigestStore) StoreClusterAudit(fp ClusterAuditFingerprint, entry ClusterAuditCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.clusterAuditPath(key), digestRecord{
		Kind:        "cluster_audit",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteClusterAuditStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}
```

#### DigestStore.LookupClusterCoT (method)

```go
func (s *DigestStore) LookupClusterCoT(fp ClusterCoTFingerprint) (ClusterCoTCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return ClusterCoTCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.clusterCoTPath(key), key)
	if err != nil || !ok {
		return ClusterCoTCached{}, false, err
	}
	var out ClusterCoTCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return ClusterCoTCached{}, false, fmt.Errorf("digest cluster_cot payload: %w", err)
	}
	return out, true, nil
}
```

#### DigestStore.StoreClusterCoT (method)

```go
func (s *DigestStore) StoreClusterCoT(fp ClusterCoTFingerprint, entry ClusterCoTCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.MergeIDs) == "" || strings.TrimSpace(entry.MergePackages) == "" || strings.TrimSpace(entry.MergeIntents) == "" {
		return fmt.Errorf("digest cluster_cot store refuses empty merge fields")
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.clusterCoTPath(key), digestRecord{
		Kind:        "cluster_cot",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteClusterCoTStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}
```

#### DigestStore.LookupRefine (method)

```go
func (s *DigestStore) LookupRefine(fp RefineFingerprint) (RefineCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return RefineCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.refinePath(key), key)
	if err != nil || !ok {
		return RefineCached{}, false, err
	}
	var out RefineCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return RefineCached{}, false, fmt.Errorf("digest refine payload: %w", err)
	}
	return out, true, nil
}
```

#### DigestStore.StoreRefine (method)

```go
func (s *DigestStore) StoreRefine(fp RefineFingerprint, entry RefineCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.RefinedCatalogYAML) == "" {
		return fmt.Errorf("digest refine store refuses empty catalog")
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.refinePath(key), digestRecord{
		Kind:        "refine",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteRefineStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}
```

#### DigestStore.LookupIntervention (method)

```go
func (s *DigestStore) LookupIntervention(fp InterventionFingerprint) (InterventionCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return InterventionCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.interventionPath(key), key)
	if err != nil || !ok {
		return InterventionCached{}, false, err
	}
	var out InterventionCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return InterventionCached{}, false, fmt.Errorf("digest intervention payload: %w", err)
	}
	return out, true, nil
}
```

#### DigestStore.StoreIntervention (method)

```go
func (s *DigestStore) StoreIntervention(fp InterventionFingerprint, entry InterventionCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.Markdown) == "" {
		return fmt.Errorf("digest intervention store refuses empty markdown")
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.interventionPath(key), digestRecord{
		Kind:        "intervention",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteInterventionStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}
```

#### DigestStore.LookupStory (method)

```go
func (s *DigestStore) LookupStory(fp StoryFingerprint) (StoryCached, bool, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return StoryCached{}, false, nil
	}
	key := fp.key()
	rec, ok, err := readDigestRecord(s.storyPath(key), key)
	if err != nil || !ok {
		return StoryCached{}, false, err
	}
	var out StoryCached
	if err := json.Unmarshal(rec.Payload, &out); err != nil {
		return StoryCached{}, false, fmt.Errorf("digest story payload: %w", err)
	}
	return out, true, nil
}
```

#### DigestStore.StoreStory (method)

```go
func (s *DigestStore) StoreStory(fp StoryFingerprint, entry StoryCached) error {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return nil
	}
	if strings.TrimSpace(entry.Markdown) == "" {
		return fmt.Errorf("digest story store refuses empty markdown")
	}
	key := fp.key()
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := writeDigestRecord(s.storyPath(key), digestRecord{
		Kind:        "story",
		Fingerprint: key,
		CreatedAt:   time.Now().UTC().Format(timestampFmt),
		Payload:     payload,
	}); err != nil {
		return err
	}
	s.noteStoryStoredUsage(entry.TotalTokens)
	s.flushAfterStore()
	return nil
}
```

#### DigestPushOptions (type)

```go
type DigestPushOptions struct {
	Remote   string
	Branch   string
	Worktree string
	Token    string
	SCM      string
}
```

#### PushDigest (func)

```go
func PushDigest(opts DigestPushOptions) error {
	if err := ValidateInferenceCacheBranch(opts.Branch); err != nil {
		return err
	}
	if opts.Remote == "" || opts.Worktree == "" {
		return fmt.Errorf("--remote and --worktree required")
	}
	token := opts.Token
	if token == "" {
		token = first(os.Getenv("BITBUCKET_TOKEN"), os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))
	}
	if token == "" {
		return fmt.Errorf("token required (BITBUCKET_TOKEN or GITHUB_TOKEN)")
	}
	info, err := os.Stat(opts.Worktree)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("worktree not found: %s", opts.Worktree)
	}
	scm := opts.SCM
	if scm == "" {
		scm = githttps.InferSCM(opts.Remote)
	}
	authArgs := githttps.ExtraHeaderArgs(token, scm)
	run := func(args ...string) error {
		cmdArgs := append([]string{"-C", opts.Worktree}, authArgs...)
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command("git", cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	_ = run("add", "-A")
	// Commit only when there is something to commit.
	if err := run("diff", "--cached", "--quiet"); err != nil {
		if err := run("commit", "-m", "digest inference cache"); err != nil {
			return fmt.Errorf("digest cache commit: %w", err)
		}
	}
	_ = run("fetch", opts.Remote, opts.Branch+":"+opts.Branch)
	if err := run("push", opts.Remote, "HEAD:"+opts.Branch); err != nil {
		if ferr := run("fetch", opts.Remote, opts.Branch); ferr != nil {
			return fmt.Errorf("digest cache push failed (%v); refetch also failed: %w", err, ferr)
		}
		if err := run("push", opts.Remote, "HEAD:"+opts.Branch); err != nil {
			return fmt.Errorf("digest cache push failed after refetch: %w", err)
		}
	}
	return nil
}
```

#### PackageSourceHash (func)

```go
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
```

#### OwnedPackagesSourceHash (func)

```go
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
```

### Private one-hop bodies

#### ClusterAuditFingerprint.key (method)

```go
func (fp ClusterAuditFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestClusterAuditPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestClusterAuditSchemaV1
	}
	return HashDigestParts(
		"cluster_audit", fp.MergesHash, fp.RolesHash, fp.ConstraintsHash, fp.MechanicalHash,
		fp.ModelID, prompt, schema,
	)
}
```

#### ClusterCoTFingerprint.key (method)

```go
func (fp ClusterCoTFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestClusterCoTPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestClusterCoTSchemaV3
	}
	return HashDigestParts(
		"cluster_cot", fp.DraftHash, fp.RolesHash, fp.ConstraintsHash, fp.MechanicalHash,
		fp.ModelID, prompt, schema,
	)
}
```

#### DigestStore.avgLocked (method)

```go
func (s *DigestStore) avgLocked(sum, n int) int {
	if n <= 0 {
		return 0
	}
	return sum / n
}
```

#### DigestStore.clusterAuditPath (method)

```go
func (s *DigestStore) clusterAuditPath(key string) string {
	return filepath.Join(s.Dir, "cluster_audit", key+".json")
}
```

#### DigestStore.clusterCoTPath (method)

```go
func (s *DigestStore) clusterCoTPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "cluster", key+".json")
}
```

#### DigestStore.flushAfterStore (method)

```go
func (s *DigestStore) flushAfterStore() {
	if s == nil || (s.PushFn == nil && s.pushOpts == nil) {
		return
	}
	_ = s.Flush()
}
```

#### DigestStore.inspectPath (method)

```go
func (s *DigestStore) inspectPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "inspect", key+".json")
}
```

#### DigestStore.interventionPath (method)

```go
func (s *DigestStore) interventionPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "intervention", key+".json")
}
```

#### DigestStore.ledgerPath (method)

```go
func (s *DigestStore) ledgerPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "ledger", key+".json")
}
```

#### DigestStore.noteClusterAuditStoredUsage (method)

```go
func (s *DigestStore) noteClusterAuditStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.clusterStoredTotalSum += total
	s.stats.clusterStoredTotalN++
}
```

#### DigestStore.noteClusterCoTStoredUsage (method)

```go
func (s *DigestStore) noteClusterCoTStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.clusterCoTStoredTotalSum += total
	s.stats.clusterCoTStoredTotalN++
}
```

#### DigestStore.noteInspectStoredUsage (method)

```go
func (s *DigestStore) noteInspectStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.inspectStoredTotalSum += total
	s.stats.inspectStoredTotalN++
}
```

#### DigestStore.noteInterventionStoredUsage (method)

```go
func (s *DigestStore) noteInterventionStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.interventionStoredSum += total
	s.stats.interventionStoredN++
}
```

#### DigestStore.noteLedgerStoredUsage (method)

```go
func (s *DigestStore) noteLedgerStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.ledgerStoredTotalSum += total
	s.stats.ledgerStoredTotalN++
}
```

#### DigestStore.noteRefineStoredUsage (method)

```go
func (s *DigestStore) noteRefineStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.refineStoredTotalSum += total
	s.stats.refineStoredTotalN++
}
```

#### DigestStore.noteStoryStoredUsage (method)

```go
func (s *DigestStore) noteStoryStoredUsage(total int) {
	if s == nil || total <= 0 {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.stats.storyStoredTotalSum += total
	s.stats.storyStoredTotalN++
}
```

#### DigestStore.recordTokenHit (method)

```go
func (s *DigestStore) recordTokenHit(prompt, completion, total int, bumpHits func(*DigestRunStats), avgSum, avgN *int) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	bumpHits(&s.stats)
	if total <= 0 {
		total = s.avgLocked(*avgSum, *avgN)
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	s.stats.TokensSavedPrompt += prompt
	s.stats.TokensSavedCompletion += completion
	s.stats.TokensSavedTotal += total
}
```

#### DigestStore.refinePath (method)

```go
func (s *DigestStore) refinePath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "refine", key+".json")
}
```

#### DigestStore.storyPath (method)

```go
func (s *DigestStore) storyPath(key string) string {
	return filepath.Join(s.Dir, DigestCachePrefix, "story", key+".json")
}
```

#### InspectFingerprint.key (method)

```go
func (fp InspectFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestInspectPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestInspectSchemaV2
	}
	return HashDigestParts("inspect", fp.PackagePath, fp.ContextSHA, fp.ModelID, prompt, schema)
}
```

#### InterventionFingerprint.key (method)

```go
func (fp InterventionFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestInterventionPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestInterventionSchemaV2
	}
	return HashDigestParts(
		"intervention", fp.TaskID, fp.ArchitectureHash, fp.RefinedHash, fp.VerdictsHash,
		fp.FindingsHash, fp.FindingHash, fp.ModelID, prompt, schema,
	)
}
```

#### LedgerFingerprint.key (method)

```go
func (fp LedgerFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestLedgerPromptV1
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestLedgerSchemaV2
	}
	if schema == DigestLedgerSchemaV1 {
		return HashDigestParts(
			"ledger", fp.SliceID, fp.OwnedPathsHash, fp.ContextSHA, fp.ConstraintsHash,
			fp.ClusterHash, fp.ModelID, prompt, schema,
		)
	}
	// v2+: cluster proposal prose is not an invalidation input.
	return HashDigestParts(
		"ledger", fp.SliceID, fp.OwnedPathsHash, fp.ContextSHA, fp.ConstraintsHash,
		fp.ModelID, prompt, schema,
	)
}
```

#### RefineFingerprint.key (method)

```go
func (fp RefineFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestRefinePromptV2
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestRefineSchemaV5
	}
	return HashDigestParts(
		"refine", fp.DraftHash, fp.RolesHash, fp.ConstraintsHash, fp.LedgerHash, fp.VerdictsHash,
		fp.MechanicalHash, fp.ModelID, prompt, schema,
	)
}
```

#### StoryFingerprint.key (method)

```go
func (fp StoryFingerprint) key() string {
	prompt := fp.PromptVersion
	if prompt == "" {
		prompt = DigestStoryPromptV2
	}
	schema := fp.SchemaVersion
	if schema == "" {
		schema = DigestStorySchemaV3
	}
	return HashDigestParts(
		"story", fp.SectionID, fp.RefinedHash, fp.LedgerHash, fp.ArchitectureHash, fp.ReadmeHash,
		fp.ModelID, prompt, schema,
	)
}
```

#### asStringSlice (func)

```go
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
```

#### buildIndex (func)

```go
func buildIndex(records []cacheRecord) map[string]cacheRecord {
	index := map[string]cacheRecord{}
	for _, record := range records {
		clusterSHA, clusterSHAOK := record.metadata["cluster_sha"].(string)
		if !clusterSHAOK || clusterSHA == "" {
			continue
		}
		skillName, skillNameOK := record.metadata["skill_name"].(string)
		key := clusterSHA
		if skillNameOK && strings.TrimSpace(skillName) != "" {
			key = skillName + ":" + clusterSHA
		}
		existing, ok := index[key]
		if !ok || !record.createdAt.Before(existing.createdAt) {
			index[key] = record
		}
	}
	return index
}
```

#### cacheRecord (type)

```go
type cacheRecord struct {
	filePath  string
	metadata  Meta
	createdAt time.Time
}
```

#### collectCacheFiles (func)

```go
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
```

#### collectMarkdownArtifact (func)

```go
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
```

#### digestRecord (type)

```go
type digestRecord struct {
	Kind        string          `json:"kind"`
	Fingerprint string          `json:"fingerprint"`
	CreatedAt   string          `json:"created_at"`
	Payload     json.RawMessage `json:"payload"`
}
```

#### existingCachePayloadUnchanged (func)

```go
func existingCachePayloadUnchanged(existing *cacheRecord, payloadHash, markdownArtifactFile, markdownArtifactHash string, markdownArtifactCount int) bool {
	existingPayload, payloadOK := existing.metadata["analysis_payload_hash"].(string)
	existingArtifactHash, artifactHashOK := existing.metadata["markdown_artifact_hash"].(string)
	if !payloadOK || !artifactHashOK {
		return false
	}
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
```

#### first (func)

```go
func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
```

#### formatFrontmatter (func)

```go
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
```

#### isInside (func)

```go
func isInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
```

#### isSourceFileName (func)

```go
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
```

#### loadCacheRecord (func)

```go
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
	createdAtStr, ok := metadata["created_at"].(string)
	if !ok {
		return nil, []string{"created_at must be a string"}
	}
	createdAt, err := time.Parse(timestampFmt, createdAtStr)
	if err != nil {
		return nil, []string{"created_at must use UTC format YYYY-MM-DDTHH:MM:SSZ"}
	}
	return &cacheRecord{filePath: filePath, metadata: metadata, createdAt: createdAt.UTC()}, nil
}
```

#### parseClusterFilesArgument (func)

```go
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
```

#### readDigestRecord (func)

```go
func readDigestRecord(path, wantKey string) (digestRecord, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return digestRecord{}, false, nil
		}
		return digestRecord{}, false, err
	}
	var rec digestRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return digestRecord{}, false, err
	}
	if rec.Fingerprint != wantKey {
		return digestRecord{}, false, nil
	}
	return rec, true, nil
}
```

#### resolveLookupEntry (func)

```go
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
```

#### resolveRetentionDays (func)

```go
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
```

#### sha256Hex (func)

```go
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
```

#### stringSlicesEqual (func)

```go
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
```

#### toRel (func)

```go
func toRel(filePath, rootDir string) string {
	rel, err := filepath.Rel(rootDir, filePath)
	if err != nil {
		return filepath.ToSlash(filePath)
	}
	return filepath.ToSlash(rel)
}
```

#### writeDigestRecord (func)

```go
func writeDigestRecord(path string, rec digestRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeJSONFile(path, rec)
}
```

#### writeJSONFile (func)

```go
func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
```

#### writeMarkdownArtifactFile (func)

```go
func writeMarkdownArtifactFile(path string, files map[string]string) error {
	payload := map[string]any{"files": files}
	return writeJSONFile(path, payload)
}
```


## ./internal/cli
- package: `cli`
- packageDoc: Package cli wires majordomo subcommands.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: cli
- mechanicalConfidence: 0.80
- mechanicalEvidence: cli_flag_parse, cli_subcommand, imports_cli_framework
- exportedDecls: Version
- exportedFuncs: NewRoot
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: mustMarkFlagRequired, newBuildSAToolsCmd, newCacheCmd, newContextCmd, newDispatchCmd, newOrchestrateCmd, newPollCmd, newPrepCmd, newPublishCmd, newReportCmd, newRunCmd, newRunReviewCmd, newSACmd, newStatusCmd, newSubmoduleCmd, newVersionCmd, resolveOTELConfig, stub
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cli/root.go

### Exported bodies

#### NewRoot (func)

```go
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "majordomo",
		Short: "Repository operations for evolving software",
		Long: `Majordomo — repository operations for evolving software.

Control-plane CLI for PR/MR review: poll, prep, orchestrate, publish, and cache.
See docs/PLAN-control-tower-github-go.md.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newPollCmd())
	root.AddCommand(newPrepCmd())
	root.AddCommand(newDispatchCmd())
	root.AddCommand(newOrchestrateCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newPublishCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newCacheCmd())
	root.AddCommand(newContextCmd())
	root.AddCommand(newReportCmd())
	root.AddCommand(newBuildSAToolsCmd())
	root.AddCommand(newSACmd())
	root.AddCommand(newSubmoduleCmd())

	return root
}
```

### Private one-hop bodies

#### newBuildSAToolsCmd (func)

```go
func newBuildSAToolsCmd() *cobra.Command {
	var dryRun, verbose, corp bool
	cmd := &cobra.Command{
		Use:   "build-sa-tools",
		Short: "Build local SA tool Docker images to validate Dockerfiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return satools.Run(satools.Options{DryRun: dryRun, Verbose: verbose, Corp: corp})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list tools without building")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print full build output on success")
	cmd.Flags().BoolVar(&corp, "corp", false, "corporate registry mode (PACKAGE_REGISTRY_* + credentials)")
	return cmd
}
```

#### newCacheCmd (func)

```go
func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Review, poll, and digest inference cache on served repo",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "validate-branch <branch>",
		Short: "Validate majordomo-pr-reviewer-cache branch name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cache.ValidateReviewCacheBranch(args[0])
		},
	})
	var remote, branch, worktree string
	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push review-cache branch with constrained auth",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cache.Push(cache.PushOptions{Remote: remote, Branch: branch, Worktree: worktree})
		},
	}
	pushCmd.Flags().StringVar(&remote, "remote", "", "https remote URL")
	pushCmd.Flags().StringVar(&branch, "branch", "", "cache branch name")
	pushCmd.Flags().StringVar(&worktree, "worktree", "", "cache worktree path")
	mustMarkFlagRequired(pushCmd, "remote")
	mustMarkFlagRequired(pushCmd, "branch")
	mustMarkFlagRequired(pushCmd, "worktree")
	cmd.AddCommand(pushCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "poll-get <cursor-file>",
		Short: "Print poll-cursor.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cache.ReadPollCursor(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
			return nil
		},
	})
	var pr, sha string
	setCmd := &cobra.Command{
		Use:   "poll-set <cursor-file>",
		Short: "Record PR head SHA in poll-cursor.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cache.ReadPollCursor(args[0])
			if err != nil {
				return err
			}
			cache.RecordHead(c, pr, sha)
			return cache.WritePollCursor(args[0], c)
		},
	}
	setCmd.Flags().StringVar(&pr, "pr", "", "PR number")
	setCmd.Flags().StringVar(&sha, "sha", "", "head commit SHA")
	mustMarkFlagRequired(setCmd, "pr")
	mustMarkFlagRequired(setCmd, "sha")
	cmd.AddCommand(setCmd)

	var (
		projectID            string
		cacheDir             string
		projectRetentionDays int
		centralRetentionDays int
		globalRetentionDays  int
		minRetentionDays     int
		indexOut             string
		haveProjectRet       bool
		haveCentralRet       bool
	)
	precheckCmd := &cobra.Command{
		Use:   "precheck",
		Short: "Prune expired cluster cache entries and build index",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := cache.PrecheckOptions{
				ProjectID:           projectID,
				CacheDir:            cacheDir,
				GlobalRetentionDays: globalRetentionDays,
				MinRetentionDays:    minRetentionDays,
				IndexOut:            indexOut,
			}
			if haveProjectRet {
				v := projectRetentionDays
				opts.ProjectRetentionDays = &v
			}
			if haveCentralRet {
				v := centralRetentionDays
				opts.CentralRetentionDays = &v
			}
			result, err := cache.Precheck(opts)
			if err != nil {
				return err
			}
			return cache.PrintJSONPretty(result)
		},
	}
	precheckCmd.Flags().StringVar(&projectID, "project-id", "", "project id")
	precheckCmd.Flags().StringVar(&cacheDir, "cache-dir", "", "cache directory")
	precheckCmd.Flags().IntVar(&projectRetentionDays, "project-retention-days", 0, "project retention days")
	precheckCmd.Flags().IntVar(&centralRetentionDays, "central-retention-days", 0, "central retention days")
	precheckCmd.Flags().IntVar(&globalRetentionDays, "global-retention-days", 180, "global retention days")
	precheckCmd.Flags().IntVar(&minRetentionDays, "min-retention-days", 30, "minimum retention days")
	precheckCmd.Flags().StringVar(&indexOut, "index-out", "", "optional path to write index JSON")
	mustMarkFlagRequired(precheckCmd, "project-id")
	mustMarkFlagRequired(precheckCmd, "cache-dir")
	precheckCmd.PreRun = func(cmd *cobra.Command, args []string) {
		haveProjectRet = cmd.Flags().Changed("project-retention-days")
		haveCentralRet = cmd.Flags().Changed("central-retention-days")
	}
	cmd.AddCommand(precheckCmd
// ... truncated
```

#### newContextCmd (func)

```go
func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Served-repo context branch (validate, digest, list targets)",
	}
	var dir string
	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate a context-branch worktree (meta.yaml, chronology, required files)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return contextstore.ValidateTree(dir)
		},
	}
	validate.Flags().StringVar(&dir, "dir", "", "context worktree directory")
	mustMarkFlagRequired(validate, "dir")
	var digestConfigDir, digestRepoID, digestWorkDir, digestOut, digestWorkStoryDir string
	var digestTypologyBinary, digestModuleScope, digestBootstrapPolicy string
	var digestFromStage, digestLocalSeedDir string
	var digestResumePR int
	var skipStory, skipCompact, forceCompact, digestAllowSourceMove bool
	digest := &cobra.Command{
		Use:   "digest",
		Short: "Catch up the served-repo context branch when the cursor is behind default HEAD",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			otelCfg := resolveOTELConfig("", digestConfigDir, digestRepoID)
			if _, otelErr := observability.Init(otelCfg); otelErr != nil {
				fmt.Fprintf(os.Stderr, "otel init: %v\n", otelErr)
			}
			defer func() {
				flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = observability.Flush(flushCtx)
				_ = observability.Shutdown(flushCtx)
			}()
			ctx, span := observability.StartChainSpan(cmd.Context(), otelCfg.ServiceName, "majordomo.context.digest")
			defer observability.EndSpanWithStatus(span, &err)

			res, err := contextdigest.Run(contextdigest.Options{
				ConfigDir:             digestConfigDir,
				RepoID:                digestRepoID,
				WorkDir:               digestWorkDir,
				TypologyBinary:        digestTypologyBinary,
				ModuleScope:           digestModuleScope,
				BootstrapSurveyPolicy: digestBootstrapPolicy,
				SkipStory:             skipStory,
				SkipCompact:           skipCompact,
				ForceCompact:          forceCompact,
				WorkStoryDir:          digestWorkStoryDir,
				Context:               ctx,
				ResumePR:              digestResumePR,
				FromStage:             digestFromStage,
				LocalSeedDir:          digestLocalSeedDir,
				AllowSourceMove:       digestAllowSourceMove,
			})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if digestOut != "" && digestOut != "-" {
				f, err := os.Create(digestOut)
				if err != nil {
					return fmt.Errorf("create --out %s: %w", digestOut, err)
				}
				defer f.Close()
				out = f
			}
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		},
	}
	digest.Flags().StringVar(&digestConfigDir, "config-dir", "majordomo-central-config", "path to majordomo-central-config")
	digest.Flags().StringVar(&digestRepoID, "repo-id", "", "served repo id")
	digest.Flags().StringVar(&digestWorkDir, "workdir", "", "served-repo clone with origin remote")
	digest.Flags().StringVar(&digestOut, "out", "-", "write result JSON (default stdout; logs stay on stdout)")
	digest.Flags().StringVar(&digestWorkStoryDir, "work-story-dir", "", "durable local dump for RLM + CoT module traces + runreports (AI testing); default tmp/digest-runs/<repo>-<ts> or MAJORDOMO_DIGEST_WORK_STORY_DIR/<repo>-<ts>; with --local-seed-dir defaults to <seed>/work-story")
	digest.Flags().StringVar(&digestTypologyBinary, "typology-binary", os.Getenv("MAJORDOMO_TYPOLOGY_BINARY"), "Typology executable path")
	digest.Flags().StringVar(&digestModuleScope, "module-scope", os.Getenv("MAJORDOMO_TYPOLOGY_MODULE_SCOPE"), "Typology module scope within the served repo")
	digest.Flags().StringVar(&digestBootstrapPolicy, "bootstrap-survey-policy", "auto", "bootstrap survey policy: auto|always|never")
	digest.Flags().IntVar(&digestResumePR, "resume-pr", 0, "PR-seeded stage replay: load this context PR head as local evidence (requires --from-stage; never pushes context/PR)")
	digest.Flags().StringVar(&digestFromStage, "
// ... truncated
```

#### newDispatchCmd (func)

```go
func newDispatchCmd() *cobra.Command {
	var (
		scriptsDir  string
		finalize    bool
		summary     bool
		score       bool
		technical   bool
		techScore   bool
		prose       bool
		techDeep    bool
		useOpenCode bool
	)
	cmd := &cobra.Command{
		Use:   "dispatch <pr-number> <staging-dir> <output-dir>",
		Short: "Run one Judge batch (in-process strop; --opencode for harness)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := agent.ModeFiles
			switch {
			case finalize:
				mode = agent.ModeFinalize
			case summary:
				mode = agent.ModeSummary
			case score:
				mode = agent.ModeScore
			case technical:
				mode = agent.ModeTechnical
			case techScore:
				mode = agent.ModeTechScore
			case prose:
				mode = agent.ModeProse
			case techDeep:
				mode = agent.ModeTechnicalDeep
			}
			opts := agent.DispatchOptions{
				Context:    cmd.Context(),
				PRNumber:   args[0],
				StagingDir: args[1],
				OutputDir:  args[2],
				Mode:       mode,
				ScriptsDir: scriptsDir,
			}
			if useOpenCode {
				return agent.RunOpenCode(opts)
			}
			return agent.Dispatch(opts)
		},
	}
	cmd.Flags().StringVar(&scriptsDir, "scripts-dir", "", "pipelines/scripts directory")
	cmd.Flags().BoolVar(&finalize, "finalize", false, "finalize mode")
	cmd.Flags().BoolVar(&summary, "summary", false, "summary mode")
	cmd.Flags().BoolVar(&score, "score", false, "score mode")
	cmd.Flags().BoolVar(&technical, "technical", false, "technical mode")
	cmd.Flags().BoolVar(&techScore, "tech-score", false, "tech-score mode")
	cmd.Flags().BoolVar(&prose, "prose", false, "prose mode")
	cmd.Flags().BoolVar(&techDeep, "technical-deep", false, "technical-deep mode")
	cmd.Flags().BoolVar(&useOpenCode, "opencode", false, "run agent-dispatch.sh via Bifrost ChildEnv (legacy harness)")
	return cmd
}
```

#### newOrchestrateCmd (func)

```go
func newOrchestrateCmd() *cobra.Command {
	var (
		pr, stagingDir, outputDir, baseBranch, pipeline, scriptsDir, repoRoot string
		routing, agentContext, summaryConfig, configDir, repoID, contextDir   string
		concurrency                                                           int
		skipPrep, skipDeep, skipReport                                        bool
		until                                                                 string
		timeoutMin                                                            int
	)
	cmd := &cobra.Command{
		Use:   "orchestrate",
		Short: "Run review waves, checkpoints, finalize, and synthesis loops",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if concurrency <= 0 {
				if v := os.Getenv("COPILOT_CONCURRENCY"); v != "" {
					if n, err := strconv.Atoi(v); err == nil {
						concurrency = n
					}
				}
			}
			var timeout time.Duration
			if timeoutMin > 0 {
				timeout = time.Duration(timeoutMin) * time.Minute
			}
			otelCfg := resolveOTELConfig(outputDir, configDir, repoID)
			if _, otelErr := observability.Init(otelCfg); otelErr != nil {
				fmt.Fprintf(os.Stderr, "otel init: %v\n", otelErr)
			}
			ctx, span := observability.StartChainSpan(cmd.Context(), otelCfg.ServiceName, "majordomo.orchestrate")
			defer observability.EndSpanWithStatus(span, &err)
			return orchestrate.Run(orchestrate.Options{
				Context:           ctx,
				PRNumber:          pr,
				BaseBranch:        baseBranch,
				StagingDir:        stagingDir,
				OutputDir:         outputDir,
				Pipeline:          pipeline,
				Concurrency:       concurrency,
				ScriptsDir:        scriptsDir,
				SkipPrep:          skipPrep,
				SkipDeep:          skipDeep,
				SkipReport:        skipReport,
				Until:             until,
				RepoRoot:          repoRoot,
				RoutingPath:       routing,
				AgentContextPath:  agentContext,
				SummaryConfigPath: summaryConfig,
				ConfigDir:         configDir,
				RepoID:            repoID,
				ContextDir:        staging.ResolveContextDir(contextDir),
				BatchTimeout:      timeout,
			})
		},
	}
	cmd.Flags().StringVar(&pr, "pr", "", "PR/MR number (required)")
	cmd.Flags().StringVar(&stagingDir, "staging-dir", "", "prep staging directory (required)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "pipeline output directory (required)")
	cmd.Flags().StringVar(&baseBranch, "base-branch", "", "base branch for prep (required unless --skip-prep)")
	cmd.Flags().StringVar(&pipeline, "pipeline", "pr-review", "pipeline name label")
	cmd.Flags().StringVar(&scriptsDir, "scripts-dir", "", "pipelines/scripts directory")
	cmd.Flags().StringVar(&repoRoot, "repo-root", "", "app repo root for prep/deep (default: cwd)")
	cmd.Flags().StringVar(&routing, "routing", "", "routing JSON for prep")
	cmd.Flags().StringVar(&agentContext, "agent-context", "", "agent context JSON for prep")
	cmd.Flags().StringVar(&summaryConfig, "summary-config", "", "summary config JSON for prep")
	cmd.Flags().StringVar(&configDir, "config-dir", "", "majordomo-central-config dir")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "served repo id under config-dir")
	cmd.Flags().StringVar(&contextDir, "context-dir", "", "merged context-branch checkout (agenting packs; or MAJORDOMO_CONTEXT_DIR)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "max parallel batches (default COPILOT_CONCURRENCY or 6)")
	cmd.Flags().IntVar(&timeoutMin, "batch-timeout-minutes", 0, "per-batch timeout (default 8)")
	cmd.Flags().BoolVar(&skipPrep, "skip-prep", false, "assume staging already prepared")
	cmd.Flags().BoolVar(&skipDeep, "skip-deep", false, "skip technical deep review pass")
	cmd.Flags().BoolVar(&skipReport, "skip-report", false, "skip JUnit conversion")
	cmd.Flags().StringVar(&until, "until", "", "stop after stage: prep|waves|finalize|prose|synth|report")
	mustMarkFlagRequired(cmd, "pr")
	mustMarkFlagRequired(cmd, "staging-dir")
	mustMarkFlagRequired(cmd, "output-dir")
	return cmd
}
```

#### newPollCmd (func)

```go
func newPollCmd() *cobra.Command {
	var configDir, cursorDir, outPath string
	cmd := &cobra.Command{
		Use:   "poll",
		Short: "Poll SCM APIs for open PRs/MRs that need review (reconciliation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := outPath
			if out == "-" {
				out = ""
			}
			return poll.Run(poll.Options{
				ConfigDir: configDir,
				CursorDir: cursorDir,
				OutPath:   out,
			})
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "majordomo-central-config", "path to majordomo-central-config")
	cmd.Flags().StringVar(&cursorDir, "cursor-dir", ".poll-cache", "local poll-cursor store (use Actions cache)")
	cmd.Flags().StringVar(&outPath, "out", "pending-reviews.json", "write pending reviews JSON (\"-\" for stdout)")
	return cmd
}
```

#### newPrepCmd (func)

```go
func newPrepCmd() *cobra.Command {
	var routing, agentContext, summaryConfig, configDir, repoID, pipeline, contextDir string
	cmd := &cobra.Command{
		Use:   "prep <base-branch> <staging-dir>",
		Short: "Classify diffs, cluster files, write staging manifest",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			matDir := config.MaterializeDirForStaging(args[1])
			routingPath, agentContextPath, _, err := config.ResolvePrepPaths(
				configDir, repoID, pipeline, matDir, routing, agentContext,
			)
			if err != nil {
				return err
			}
			opts := staging.Options{
				BaseBranch:        args[0],
				StagingDir:        args[1],
				RoutingPath:       routingPath,
				AgentContextPath:  agentContextPath,
				SummaryConfigPath: summaryConfig,
				ContextDir:        staging.ResolveContextDir(contextDir),
			}
			return staging.Run(opts)
		},
	}
	cmd.Flags().StringVar(&routing, "routing", "", "path to routing JSON")
	cmd.Flags().StringVar(&agentContext, "agent-context", "", "path to agent context JSON")
	cmd.Flags().StringVar(&summaryConfig, "summary-config", "", "path to summary config JSON")
	cmd.Flags().StringVar(&configDir, "config-dir", "", "majordomo-central-config dir (materialize routing/agentContext)")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "served repo id under config-dir")
	cmd.Flags().StringVar(&pipeline, "pipeline", "pr-review", "pipelines.<name> key when using --config-dir")
	cmd.Flags().StringVar(&contextDir, "context-dir", "", "merged context-branch checkout (agenting packs; or MAJORDOMO_CONTEXT_DIR)")
	return cmd
}
```

#### newPublishCmd (func)

```go
func newPublishCmd() *cobra.Command {
	var scm, owner, repo, repoID, gitlabHost, gitlabProjectID string
	cmd := &cobra.Command{
		Use:   "publish <pr-number> <summary-file> <mode>",
		Short: "Publish summary to PR/MR (github|gitlab via gh/glab; bitbucket HTTP)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return publish.Run(publish.Options{
				SCM:             scm,
				PRNumber:        args[0],
				SummaryFile:     args[1],
				Mode:            publish.Mode(args[2]),
				RepoID:          repoID,
				GitHubOwner:     owner,
				GitHubRepo:      repo,
				GitLabHost:      gitlabHost,
				GitLabProjectID: gitlabProjectID,
			})
		},
	}
	cmd.Flags().StringVar(&scm, "scm", "github", "scm forge: github|gitlab|bitbucket")
	cmd.Flags().StringVar(&owner, "owner", "", "GitHub/GitLab owner or group path")
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub/GitLab project name")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "central-config id (MAJORDOMO_CREDENTIAL_ override; or MAJORDOMO_REPO_ID)")
	cmd.Flags().StringVar(&gitlabHost, "gitlab-host", "", "GitLab host (default gitlab.com; or GITLAB_HOST)")
	cmd.Flags().StringVar(&gitlabProjectID, "gitlab-project-id", "", "GitLab numeric project id (or GITLAB_PROJECT_ID)")
	return cmd
}
```

#### newReportCmd (func)

```go
func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Convert review reports (junit, html, all-diffs)",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "junit <review-output-dir> <junit-output-dir>",
		Short: "Convert findings to JUnit XML",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return report.ConvertToJUnit(args[0], args[1])
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "html <input.md> <output.html>",
		Short: "Convert markdown reports to HTML",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return report.ConvertMarkdownToHTMLCLI(args[0], args[1])
		},
	})
	var capLines int
	allDiffsCmd := &cobra.Command{
		Use:   "all-diffs <manifest.json> <output-file>",
		Short: "Concatenate per-file staging diffs into all-diffs.txt",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := diffpkg.BuildAllOptions{Manifest: args[0], Output: args[1]}
			if cmd.Flags().Changed("cap") {
				c := capLines
				opts.Cap = &c
			}
			return diffpkg.BuildAll(opts)
		},
	}
	allDiffsCmd.Flags().IntVar(&capLines, "cap", 0, "truncate each file diff to N lines")
	cmd.AddCommand(allDiffsCmd)
	return cmd
}
```

#### newRunCmd (func)

```go
func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a job locally or in CI (same command)",
	}
	cmd.AddCommand(newRunReviewCmd())
	return cmd
}
```

#### newSACmd (func)

```go
func newSACmd() *cobra.Command {
	var configDir, repoID, repoRoot, baseBranch, scriptsDir, imagePrefix string
	cmd := &cobra.Command{
		Use:   "sa",
		Short: "Run staticAnalysis tools from central config into .sa/",
		RunE: func(cmd *cobra.Command, args []string) error {
			return sa.Run(sa.Options{
				ConfigDir:   configDir,
				RepoID:      repoID,
				RepoRoot:    repoRoot,
				BaseBranch:  baseBranch,
				ScriptsDir:  scriptsDir,
				ImagePrefix: imagePrefix,
			})
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "majordomo-central-config", "path to majordomo-central-config")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "served repo id (required)")
	cmd.Flags().StringVar(&repoRoot, "repo-root", "", "served repo checkout (default: cwd)")
	cmd.Flags().StringVar(&baseBranch, "base-branch", "", "base branch for changed-file list (required)")
	cmd.Flags().StringVar(&scriptsDir, "scripts-dir", "", "pipelines/scripts with run-sa-tool.sh")
	cmd.Flags().StringVar(&imagePrefix, "image-prefix", "", "registry prefix when tool has no image (or MAJORDOMO_SA_IMAGE_PREFIX)")
	mustMarkFlagRequired(cmd, "repo-id")
	mustMarkFlagRequired(cmd, "base-branch")
	return cmd
}
```

#### newStatusCmd (func)

```go
func newStatusCmd() *cobra.Command {
	var scm, contextName string
	cmd := &cobra.Command{
		Use:   "status <commit-sha> <state>",
		Short: "Post commit/check status (INPROGRESS|SUCCESSFUL|FAILED)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return status.Run(status.Options{
				SCM:       scm,
				CommitSHA: args[0],
				State:     status.State(args[1]),
				Context:   contextName,
			})
		},
	}
	cmd.Flags().StringVar(&scm, "scm", "github", "scm forge: github|bitbucket")
	cmd.Flags().StringVar(&contextName, "context", "", "GitHub status context (default majordomo)")
	return cmd
}
```

#### newSubmoduleCmd (func)

```go
func newSubmoduleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "submodule",
		Short: "Interactive manager for a vendored .majordomo submodule",
		RunE: func(cmd *cobra.Command, args []string) error {
			return submodule.Run(submodule.Options{})
		},
	}
}
```

#### newVersionCmd (func)

```go
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print majordomo version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
		},
	}
}
```


## ./internal/cluster
- package: `cluster`
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: UnionFind
- exportedFuncs: BuildCorpusIndex, ClusterDocs, ClusterFiles, DepClusterAwareBatches, DocClusterAwareBatches, NewUnionFind, ReverseDeps, ReverseLinks
- exportedMethods: UnionFind.Components, UnionFind.Find, UnionFind.Union
- unexportedDecls: backtickRE, boldRE, docScanExcludeDirs, h1RE, h2h3RE, inlineLinkRE, jsExts, jsImportRE, minTermLen, pyFromRE, pyImportRE, refDefRE, refUseRE, scanExcludeDirs
- unexportedFuncs: docPathExcluded, extractHeadings, extractKeyTerms, extractTitle, isJSExt, moduleToCandidates, parseFromModule, parseImportModules, parseJSImports, parseMDLinks, parsePythonImports, pathExcluded, repoRelPath, resolveJSImport, resolveMDLink, resolveRelativeImport
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cluster/dep.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cluster/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/cluster/unionfind.go

### Exported bodies

#### ClusterFiles (func)

```go
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
```

#### DepClusterAwareBatches (func)

```go
func DepClusterAwareBatches(skillTasks []map[string]any, batchSize int, repoRoot string) ([][]map[string]any, error) {
	if len(skillTasks) == 0 {
		return nil, nil
	}

	fileToTasks := make(map[string][]map[string]any)
	for _, task := range skillTasks {
		fileKey, ok := task["file"].(string)
		if !ok || strings.TrimSpace(fileKey) == "" {
			return nil, fmt.Errorf("dependency cluster task requires a non-empty string file")
		}
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

	return batches, nil
}
```

#### ReverseDeps (func)

```go
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
```

#### ClusterDocs (func)

```go
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
```

#### DocClusterAwareBatches (func)

```go
func DocClusterAwareBatches(skillTasks []map[string]any, batchSize int, repoRoot string) ([][]map[string]any, error) {
	if len(skillTasks) == 0 {
		return nil, nil
	}

	fileToTasks := make(map[string][]map[string]any)
	for _, task := range skillTasks {
		fileKey, ok := task["file"].(string)
		if !ok || strings.TrimSpace(fileKey) == "" {
			return nil, fmt.Errorf("document cluster task requires a non-empty string file")
		}
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

	return batches, nil
}
```

#### ReverseLinks (func)

```go
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
```

#### BuildCorpusIndex (func)

```go
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
```

#### UnionFind (type)

```go
type UnionFind struct {
	parent map[string]string
	rank   map[string]int
	order  []string
}
```

#### NewUnionFind (func)

```go
func NewUnionFind(items []string) *UnionFind {
	uf := &UnionFind{
		parent: make(map[string]string, len(items)),
		rank:   make(map[string]int, len(items)),
		order:  append([]string(nil), items...),
	}
	for _, item := range items {
		uf.parent[item] = item
	}
	return uf
}
```

#### UnionFind.Find (method)

```go
func (uf *UnionFind) Find(item string) string {
	if uf.parent[item] != item {
		uf.parent[item] = uf.Find(uf.parent[item])
	}
	return uf.parent[item]
}
```

#### UnionFind.Union (method)

```go
func (uf *UnionFind) Union(a, b string) {
	rootA, rootB := uf.Find(a), uf.Find(b)
	if rootA == rootB {
		return
	}
	if uf.rank[rootA] < uf.rank[rootB] {
		rootA, rootB = rootB, rootA
	}
	uf.parent[rootB] = rootA
	if uf.rank[rootA] == uf.rank[rootB] {
		uf.rank[rootA]++
	}
}
```

#### UnionFind.Components (method)

```go
func (uf *UnionFind) Components() [][]string {
	groups := make(map[string][]string)
	for _, item := range uf.order {
		root := uf.Find(item)
		groups[root] = append(groups[root], item)
	}
	result := make([][]string, 0, len(groups))
	for _, item := range uf.order {
		root := uf.Find(item)
		if len(groups[root]) == 0 {
			continue
		}
		result = append(result, groups[root])
		delete(groups, root)
	}
	return result
}
```

### Private one-hop bodies

#### docPathExcluded (func)

```go
func docPathExcluded(parts []string) bool {
	for _, part := range parts {
		if _, ok := docScanExcludeDirs[part]; ok {
			return true
		}
	}
	return false
}
```

#### extractHeadings (func)

```go
func extractHeadings(content string) []string {
	matches := h2h3RE.FindAllStringSubmatch(content, -1)
	headings := make([]string, 0, len(matches))
	for _, match := range matches {
		headings = append(headings, strings.TrimSpace(match[1]))
	}
	return headings
}
```

#### extractKeyTerms (func)

```go
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
```

#### extractTitle (func)

```go
func extractTitle(content string) string {
	match := h1RE.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
```

#### isJSExt (func)

```go
func isJSExt(suffix string) bool {
	_, ok := jsExts[strings.ToLower(suffix)]
	return ok
}
```

#### parseJSImports (func)

```go
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
```

#### parseMDLinks (func)

```go
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
```

#### parsePythonImports (func)

```go
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
```

#### pathExcluded (func)

```go
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
```

#### repoRelPath (func)

```go
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
```


## ./internal/config
- package: `config`
- packageDoc: Package config loads and merges majordomo-central-config YAML.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: AIProviderConfig, Cache, Context, ContextCompaction, DefaultsFilename, GroundingConfig, JobConfig, JobContextDigest, JobPRReview, MaterializeResult, ModuleTaskConfig, Observability, OrderedRouting, Pipeline, PipelineRoutingEntry, PollCache, PushNone, PushWebhook, PushWorkflow, RepoConfig, Repository, Review, SCMAPI, StaticAnalysisTool, Trigger, TriggerPush, TriggerPushMode
- exportedFuncs: ApplyPipelineModelEnv, CacheBranch, ContextBranch, ContextUpdateBranch, CredentialEnvName, CredentialHint, DigestCacheBranch, InferenceCacheBranch, JobForTask, ListRepoIDs, LoadAll, LoadDefaults, LoadMerged, LoadRepoFile, MaterializeDirForStaging, MaterializePrep, OrgCredentialEnvName, PollCacheBranch, ResolveCredential, ResolvePrepPaths, ResolveSAImage, ResolveSAToolSlug, TypologyPromoteBranch, TypologyPromoteUpdateBranch
- exportedMethods: AIProviderConfig.GetTimeout, AIProviderConfig.ToStrop, AIProviderConfig.UsesEmbeddedGateway, AIProviderConfig.Validate, Cache.SkipsEnabled, Context.AutoMergeEnabled, Context.GatePrefix, Context.MaxCommitsPerRunLimit, Observability.Expand, OrderedRouting.Empty, OrderedRouting.UnmarshalYAML, RepoConfig.EffectivePublishMode, RepoConfig.GetAIProvider, RepoConfig.GetModuleProvider, RepoConfig.PipelineNamed, RepoConfig.ResolveTaskProvider, Review.ContinuousRunsEnabled, Trigger.PollEnabled
- unexportedDecls: defaultMaxCommitsPerRun, envPlaceholderRE
- unexportedFuncs: cloneAIProviders, cloneAnyMap, cloneJobConfigs, cloneOrderedRouting, clonePipeline, clonePipelines, cloneSkills, expandEnvVars, expandProvider, mergeAIProviders, mergeConfig, mergeJobConfigs, mergeObservability, mergePipeline, mergePipelines, saSlug, secretKeySuffix, setEnvIfEmpty, unresolvedPlaceholders, writeRoutingJSON
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/config/config.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/config/load.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/config/materialize.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/config/providers.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/config/resolve.go

### Exported bodies

#### TriggerPushMode (type)

```go
type TriggerPushMode string
```

#### Trigger (type)

```go
type Trigger struct {
	Poll     *bool       `yaml:"poll"`
	Interval string      `yaml:"interval"`
	Push     TriggerPush `yaml:"push"`
}
```

#### Trigger.PollEnabled (method)

```go
func (t Trigger) PollEnabled() bool {
	if t.Poll == nil {
		return true
	}
	return *t.Poll
}
```

#### TriggerPush (type)

```go
type TriggerPush struct {
	Mode TriggerPushMode `yaml:"mode"`
}
```

#### Cache (type)

```go
type Cache struct {
	Repo          string `yaml:"repo"` // served
	Dir           string `yaml:"dir"`
	RetentionDays int    `yaml:"retentionDays"`
	// DisableSkips opts out of analysis-cache skips (skips are on by default).
	DisableSkips bool `yaml:"disableSkips"`
}
```

#### Cache.SkipsEnabled (method)

```go
func (c Cache) SkipsEnabled() bool {
	return !c.DisableSkips
}
```

#### PollCache (type)

```go
type PollCache struct {
	Repo   string `yaml:"repo"`   // served
	Branch string `yaml:"branch"` // majordomo-poll-cache/<repo-id>
}
```

#### Context (type)

```go
type Context struct {
	Repo              string            `yaml:"repo"`   // served
	Branch            string            `yaml:"branch"` // majordomo-context/<repo-id>
	AutoMerge         *bool             `yaml:"autoMerge,omitempty"`
	GateCommentPrefix string            `yaml:"gateCommentPrefix,omitempty"`
	Compaction        ContextCompaction `yaml:"compaction,omitempty"`
	MaxCommitsPerRun  int               `yaml:"maxCommitsPerRun,omitempty"`
}
```

#### Context.MaxCommitsPerRunLimit (method)

```go
func (c Context) MaxCommitsPerRunLimit() int {
	if c.MaxCommitsPerRun > 0 {
		return c.MaxCommitsPerRun
	}
	return defaultMaxCommitsPerRun
}
```

#### ContextCompaction (type)

```go
type ContextCompaction struct {
	MaxChronologyEntries int `yaml:"maxChronologyEntries,omitempty"`
	KeepRecentEntries    int `yaml:"keepRecentEntries,omitempty"`
}
```

#### Context.AutoMergeEnabled (method)

```go
func (c Context) AutoMergeEnabled() bool {
	return c.AutoMerge != nil && *c.AutoMerge
}
```

#### Context.GatePrefix (method)

```go
func (c Context) GatePrefix() string {
	if p := strings.TrimSpace(c.GateCommentPrefix); p != "" {
		return p
	}
	return "@majordomo"
}
```

#### Repository (type)

```go
type Repository struct {
	ID       string `yaml:"id"`
	CloneURL string `yaml:"cloneUrl"`
	Owner    string `yaml:"owner,omitempty"`
	Name     string `yaml:"name,omitempty"`
}
```

#### SCMAPI (type)

```go
type SCMAPI struct {
	BaseURL   string `yaml:"baseUrl"`
	ProjectID string `yaml:"projectId,omitempty"`
}
```

#### Review (type)

```go
type Review struct {
	PublishMode          string `yaml:"publishMode,omitempty"`          // auto | comment | description | check | off
	EnableContinuousRuns *bool  `yaml:"enableContinuousRuns,omitempty"` // nil/false: one review per PR; true: re-queue when head_sha changes
}
```

#### Review.ContinuousRunsEnabled (method)

```go
func (r Review) ContinuousRunsEnabled() bool {
	return r.EnableContinuousRuns != nil && *r.EnableContinuousRuns
}
```

#### StaticAnalysisTool (type)

```go
type StaticAnalysisTool struct {
	Tool       string `yaml:"tool,omitempty"`       // slug for .sa/<tool>.txt (default from dockerfile/image)
	Dockerfile string `yaml:"dockerfile,omitempty"` // path under majordomo repo
	Image      string `yaml:"image,omitempty"`      // full image ref (preferred at run time)
	Command    string `yaml:"command"`
	Glob       string `yaml:"glob"`
}
```

#### PipelineRoutingEntry (type)

```go
type PipelineRoutingEntry struct {
	Globs   []string
	Persona string
}
```

#### OrderedRouting (type)

```go
type OrderedRouting struct {
	Keys  []string
	Rules map[string]PipelineRoutingEntry
}
```

#### OrderedRouting.Empty (method)

```go
func (o OrderedRouting) Empty() bool {
	return len(o.Keys) == 0
}
```

#### OrderedRouting.UnmarshalYAML (method)

```go
func (o *OrderedRouting) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || value.Kind == 0 {
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("routing must be a mapping")
	}
	o.Keys = nil
	o.Rules = map[string]PipelineRoutingEntry{}
	for i := 0; i+1 < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]
		key := keyNode.Value
		var entry PipelineRoutingEntry
		switch valNode.Kind {
		case yaml.SequenceNode:
			var globs []string
			if err := valNode.Decode(&globs); err != nil {
				return fmt.Errorf("routing[%s]: %w", key, err)
			}
			entry.Globs = globs
		case yaml.MappingNode:
			var m struct {
				Globs   []string `yaml:"globs"`
				Persona string   `yaml:"persona"`
			}
			if err := valNode.Decode(&m); err != nil {
				return fmt.Errorf("routing[%s]: %w", key, err)
			}
			entry.Globs = m.Globs
			entry.Persona = m.Persona
		default:
			return fmt.Errorf("routing[%s]: expected list of globs or map with globs", key)
		}
		o.Keys = append(o.Keys, key)
		o.Rules[key] = entry
	}
	return nil
}
```

#### Pipeline (type)

```go
type Pipeline struct {
	Model        string             `yaml:"model,omitempty"`
	ScoreModel   string             `yaml:"scoreModel,omitempty"`
	Agent        string             `yaml:"agent,omitempty"`
	Routing      OrderedRouting     `yaml:"routing,omitempty"`
	AgentContext map[string]any     `yaml:"agentContext,omitempty"`
	Skills       map[string]*string `yaml:"skills,omitempty"` // null = submodule default
}
```

#### Observability (type)

```go
type Observability struct {
	// Enabled defaults true when nil. Set false to disable tracing.
	Enabled *bool `yaml:"enabled,omitempty"`
	// Endpoint is OTLP gRPC host:port (example: localhost:4317). Empty skips OTLP export.
	Endpoint string `yaml:"endpoint,omitempty"`
	// APIKey is a Bearer token (${PHOENIX_API_KEY}). Optional for local Phoenix without auth.
	APIKey string `yaml:"api_key,omitempty"`
	// ServiceName overrides the OpenTelemetry service.name resource attribute.
	ServiceName string `yaml:"service_name,omitempty"`
	// Insecure forces plaintext gRPC. Nil means auto: true for localhost/127.0.0.1.
	Insecure *bool `yaml:"insecure,omitempty"`
}
```

#### RepoConfig (type)

```go
type RepoConfig struct {
	SCM            string                      `yaml:"scm"` // github | gitlab | bitbucket | generic
	Repository     Repository                  `yaml:"repository"`
	SCMAPI         SCMAPI                      `yaml:"scmApi"`
	Trigger        Trigger                     `yaml:"trigger"`
	Cache          Cache                       `yaml:"cache"`
	PollCache      PollCache                   `yaml:"pollCache"`
	Context        Context                     `yaml:"context"`
	Review         Review                      `yaml:"review"`
	PublishMode    string                      `yaml:"publishMode,omitempty"` // legacy alias
	StaticAnalysis []StaticAnalysisTool        `yaml:"staticAnalysis,omitempty"`
	Pipelines      map[string]Pipeline         `yaml:"pipelines,omitempty"`
	AIProviders    map[string]AIProviderConfig `yaml:"ai_providers,omitempty"`
	JobConfigs     map[string]JobConfig        `yaml:"job_configs,omitempty"`
	Observability  Observability               `yaml:"observability,omitempty"`
}
```

#### RepoConfig.EffectivePublishMode (method)

```go
func (c RepoConfig) EffectivePublishMode() string {
	if strings.TrimSpace(c.Review.PublishMode) != "" {
		return strings.TrimSpace(c.Review.PublishMode)
	}
	if strings.TrimSpace(c.PublishMode) != "" {
		return strings.TrimSpace(c.PublishMode)
	}
	return "auto"
}
```

#### RepoConfig.PipelineNamed (method)

```go
func (c RepoConfig) PipelineNamed(name string) (Pipeline, bool) {
	if c.Pipelines == nil {
		return Pipeline{}, false
	}
	p, ok := c.Pipelines[name]
	return p, ok
}
```

#### InferenceCacheBranch (func)

```go
func InferenceCacheBranch(repoID string) string {
	return "majordomo-inference-cache/" + repoID
}
```

#### CacheBranch (func)

```go
func CacheBranch(projectID string) string {
	return InferenceCacheBranch(projectID)
}
```

#### PollCacheBranch (func)

```go
func PollCacheBranch(repoID string) string {
	return "majordomo-poll-cache/" + repoID
}
```

#### ContextBranch (func)

```go
func ContextBranch(repoID string) string {
	return "majordomo-context/" + repoID
}
```

#### DigestCacheBranch (func)

```go
func DigestCacheBranch(repoID string) string {
	return InferenceCacheBranch(repoID)
}
```

#### ContextUpdateBranch (func)

```go
func ContextUpdateBranch(repoID string) string {
	return ContextBranch(repoID) + "-update"
}
```

#### TypologyPromoteBranch (func)

```go
func TypologyPromoteBranch(repoID string) string {
	return "majordomo-typology/" + repoID
}
```

#### TypologyPromoteUpdateBranch (func)

```go
func TypologyPromoteUpdateBranch(repoID string) string {
	return TypologyPromoteBranch(repoID) + "-update"
}
```

#### LoadDefaults (func)

```go
func LoadDefaults(configDir string) (RepoConfig, error) {
	path := filepath.Join(configDir, DefaultsFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RepoConfig{
				Trigger: Trigger{Interval: "5m", Push: TriggerPush{Mode: PushNone}},
			}, nil
		}
		return RepoConfig{}, err
	}
	var c RepoConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return RepoConfig{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}
```

#### LoadRepoFile (func)

```go
func LoadRepoFile(configDir, repoID string, defaults RepoConfig) (RepoConfig, error) {
	path := filepath.Join(configDir, repoID+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return RepoConfig{}, err
	}
	var c RepoConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return RepoConfig{}, fmt.Errorf("%s: %w", path, err)
	}
	merged := mergeConfig(defaults, c)
	if merged.Repository.ID == "" {
		merged.Repository.ID = repoID
	}
	if merged.PollCache.Branch == "" {
		merged.PollCache.Branch = PollCacheBranch(merged.Repository.ID)
	}
	if merged.Context.Branch == "" {
		merged.Context.Branch = ContextBranch(merged.Repository.ID)
	}
	return merged, nil
}
```

#### ListRepoIDs (func)

```go
func ListRepoIDs(configDir string) ([]string, error) {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		if e.Name() == DefaultsFilename {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".yaml"))
	}
	return ids, nil
}
```

#### LoadAll (func)

```go
func LoadAll(configDir string) ([]RepoConfig, error) {
	defaults, err := LoadDefaults(configDir)
	if err != nil {
		return nil, err
	}
	ids, err := ListRepoIDs(configDir)
	if err != nil {
		return nil, err
	}
	out := make([]RepoConfig, 0, len(ids))
	for _, id := range ids {
		c, err := LoadRepoFile(configDir, id, defaults)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
```

#### LoadMerged (func)

```go
func LoadMerged(configDir, repoID string) (RepoConfig, error) {
	defaults, err := LoadDefaults(configDir)
	if err != nil {
		return RepoConfig{}, err
	}
	return LoadRepoFile(configDir, repoID, defaults)
}
```

#### CredentialEnvName (func)

```go
func CredentialEnvName(repoID string) string {
	return "MAJORDOMO_CREDENTIAL_" + secretKeySuffix(repoID)
}
```

#### OrgCredentialEnvName (func)

```go
func OrgCredentialEnvName(scm, owner string) string {
	suffix := secretKeySuffix(owner)
	switch strings.ToLower(strings.TrimSpace(scm)) {
	case "gitlab":
		return "GITLAB_TOKEN_" + suffix
	default:
		return "GH_TOKEN_" + suffix
	}
}
```

#### ResolveCredential (func)

```go
func ResolveCredential(repoID, scm, owner string) string {
	if v := strings.TrimSpace(os.Getenv(CredentialEnvName(repoID))); v != "" {
		return v
	}
	if strings.TrimSpace(owner) == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(OrgCredentialEnvName(scm, owner)))
}
```

#### CredentialHint (func)

```go
func CredentialHint(repoID, scm, owner string) string {
	perRepo := CredentialEnvName(repoID)
	if strings.TrimSpace(owner) == "" {
		return perRepo + " (and set repository.owner for org token)"
	}
	return perRepo + " or " + OrgCredentialEnvName(scm, owner)
}
```

#### MaterializeResult (type)

```go
type MaterializeResult struct {
	RoutingPath      string
	AgentContextPath string
}
```

#### MaterializePrep (func)

```go
func MaterializePrep(cfg RepoConfig, pipelineName, outDir string) (MaterializeResult, error) {
	var out MaterializeResult
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return out, fmt.Errorf("materialize prep dir: %w", err)
	}
	pipe, ok := cfg.PipelineNamed(pipelineName)
	if !ok {
		return out, nil
	}
	if !pipe.Routing.Empty() {
		path := filepath.Join(outDir, "routing.json")
		if err := writeRoutingJSON(path, pipe.Routing); err != nil {
			return out, err
		}
		out.RoutingPath = path
	}
	if pipe.AgentContext != nil {
		path := filepath.Join(outDir, "agent-context.json")
		data, err := json.MarshalIndent(pipe.AgentContext, "", "  ")
		if err != nil {
			return out, fmt.Errorf("marshal agentContext: %w", err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return out, fmt.Errorf("write agent-context.json: %w", err)
		}
		out.AgentContextPath = path
	}
	return out, nil
}
```

#### ApplyPipelineModelEnv (func)

```go
func ApplyPipelineModelEnv(cfg RepoConfig, pipelineName string) error {
	pipe, ok := cfg.PipelineNamed(pipelineName)
	if !ok {
		return nil
	}
	for key, value := range map[string]string{
		"COPILOT_MODEL":        pipe.Model,
		"OPENCODE_MODEL":       pipe.Model,
		"COPILOT_SCORE_MODEL":  pipe.ScoreModel,
		"OPENCODE_SCORE_MODEL": pipe.ScoreModel,
	} {
		if err := setEnvIfEmpty(key, value); err != nil {
			return err
		}
	}
	return nil
}
```

#### ResolveSAToolSlug (func)

```go
func ResolveSAToolSlug(t StaticAnalysisTool) string {
	if s := strings.TrimSpace(t.Tool); s != "" {
		return saSlug(s)
	}
	if t.Dockerfile != "" {
		base := filepath.Base(t.Dockerfile)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		base = strings.TrimPrefix(base, "Dockerfile.")
		if base != "" && !strings.EqualFold(base, "Dockerfile") {
			return saSlug(base)
		}
	}
	if t.Image != "" {
		last := t.Image
		if i := strings.LastIndex(last, "/"); i >= 0 {
			last = last[i+1:]
		}
		if i := strings.Index(last, ":"); i >= 0 {
			last = last[:i]
		}
		last = strings.TrimPrefix(last, "sa-")
		if last != "" {
			return saSlug(last)
		}
	}
	return "sa-tool"
}
```

#### ResolveSAImage (func)

```go
func ResolveSAImage(t StaticAnalysisTool, imagePrefix string) string {
	if img := strings.TrimSpace(t.Image); img != "" {
		return img
	}
	prefix := strings.TrimSpace(imagePrefix)
	if prefix == "" {
		prefix = strings.TrimSpace(os.Getenv("MAJORDOMO_SA_IMAGE_PREFIX"))
	}
	if prefix == "" {
		prefix = "majordomo"
	}
	prefix = strings.TrimRight(prefix, "/")
	slug := ResolveSAToolSlug(t)
	tag := strings.TrimSpace(os.Getenv("MAJORDOMO_SA_IMAGE_TAG"))
	if tag == "" {
		tag = "local"
	}
	return fmt.Sprintf("%s/sa-%s:%s", prefix, slug, tag)
}
```

#### AIProviderConfig (type)

```go
type AIProviderConfig struct {
	APIKey           string           `yaml:"api_key"`
	Model            string           `yaml:"model"`
	BaseURL          string           `yaml:"base_url"`
	Timeout          string           `yaml:"timeout,omitempty"`
	RateLimit        int              `yaml:"rate_limit,omitempty"`
	APISchema        string           `yaml:"api_schema"`
	MaxContextTokens int              `yaml:"max_context_tokens,omitempty"`
	MaxOutputTokens  int              `yaml:"max_output_tokens,omitempty"`
	Grounding        *GroundingConfig `yaml:"grounding,omitempty"`
}
```

#### GroundingConfig (type)

```go
type GroundingConfig struct {
	DynamicThreshold float64 `yaml:"dynamic_threshold,omitempty"`
}
```

#### JobConfig (type)

```go
type JobConfig struct {
	Modules map[string]ModuleTaskConfig `yaml:"modules,omitempty"`
}
```

#### ModuleTaskConfig (type)

```go
type ModuleTaskConfig struct {
	Provider string `yaml:"provider"`
}
```

#### JobForTask (func)

```go
func JobForTask(task string) string {
	switch strings.TrimSpace(task) {
	case "bootstrap_story", "digest_story", "typology_inspect",
		"typology_slice_meaning", "typology_slice_grouping_audit", "typology_slice_grouping", "typology_slice_catalog",
		"typology_human_intervention",
		"typology_intervention_journey", "typology_intervention_brief",
		"typology_intervention_weaknesses", "typology_intervention_pr_priority",
		"typology_finding_comment":
		return JobContextDigest
	case "filereview", "summary", "technical":
		return JobPRReview
	default:
		return ""
	}
}
```

#### RepoConfig.GetAIProvider (method)

```go
func (c RepoConfig) GetAIProvider(name string) (AIProviderConfig, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return AIProviderConfig{}, fmt.Errorf("ai provider name is required")
	}
	if c.AIProviders == nil {
		return AIProviderConfig{}, fmt.Errorf("no ai_providers configured")
	}
	provider, ok := c.AIProviders[name]
	if !ok {
		return AIProviderConfig{}, fmt.Errorf("AI provider %q not found", name)
	}
	provider = expandProvider(provider)
	if err := provider.Validate(); err != nil {
		return AIProviderConfig{}, fmt.Errorf("invalid AI provider %q: %w", name, err)
	}
	return provider, nil
}
```

#### RepoConfig.GetModuleProvider (method)

```go
func (c RepoConfig) GetModuleProvider(job, module string) (AIProviderConfig, error) {
	job = strings.TrimSpace(job)
	module = strings.TrimSpace(module)
	if job == "" || module == "" {
		return AIProviderConfig{}, fmt.Errorf("job and module are required")
	}
	if c.JobConfigs == nil {
		return AIProviderConfig{}, fmt.Errorf("no job_configs configured")
	}
	jobCfg, ok := c.JobConfigs[job]
	if !ok {
		return AIProviderConfig{}, fmt.Errorf("job %q not found in job_configs", job)
	}
	if jobCfg.Modules == nil {
		return AIProviderConfig{}, fmt.Errorf("job %q has no modules", job)
	}
	modCfg, ok := jobCfg.Modules[module]
	if !ok || strings.TrimSpace(modCfg.Provider) == "" {
		return AIProviderConfig{}, fmt.Errorf("job %q module %q has no provider", job, module)
	}
	return c.GetAIProvider(modCfg.Provider)
}
```

#### RepoConfig.ResolveTaskProvider (method)

```go
func (c RepoConfig) ResolveTaskProvider(task string) (AIProviderConfig, bool, error) {
	task = strings.TrimSpace(task)
	job := JobForTask(task)
	if job == "" {
		return AIProviderConfig{}, false, fmt.Errorf("unknown generator task %q", task)
	}
	if c.JobConfigs == nil {
		return AIProviderConfig{}, false, nil
	}
	jobCfg, ok := c.JobConfigs[job]
	if !ok || jobCfg.Modules == nil {
		return AIProviderConfig{}, false, nil
	}
	modCfg, ok := jobCfg.Modules[task]
	if !ok || strings.TrimSpace(modCfg.Provider) == "" {
		return AIProviderConfig{}, false, nil
	}
	provider, err := c.GetAIProvider(modCfg.Provider)
	if err != nil {
		return AIProviderConfig{}, true, err
	}
	return provider, true, nil
}
```

#### AIProviderConfig.Validate (method)

```go
func (p AIProviderConfig) Validate() error {
	if strings.TrimSpace(p.APIKey) == "" {
		return fmt.Errorf("api_key is required")
	}
	if strings.TrimSpace(p.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if strings.TrimSpace(p.BaseURL) == "" {
		return fmt.Errorf("base_url is required")
	}
	if strings.TrimSpace(p.APISchema) == "" {
		return fmt.Errorf("api_schema is required")
	}
	if unresolved := unresolvedPlaceholders(p.APIKey); unresolved != "" {
		return fmt.Errorf("unresolved env placeholder in api_key: %s", unresolved)
	}
	if unresolved := unresolvedPlaceholders(p.BaseURL); unresolved != "" {
		return fmt.Errorf("unresolved env placeholder in base_url: %s", unresolved)
	}
	return nil
}
```

#### AIProviderConfig.GetTimeout (method)

```go
func (p AIProviderConfig) GetTimeout(moduleTimeout time.Duration) time.Duration {
	if strings.TrimSpace(p.Timeout) == "" {
		return moduleTimeout
	}
	parsed, err := time.ParseDuration(p.Timeout)
	if err != nil {
		return moduleTimeout
	}
	return parsed
}
```

#### AIProviderConfig.ToStrop (method)

```go
func (p AIProviderConfig) ToStrop() stropdspy.ProviderConfig {
	out := stropdspy.ProviderConfig{
		APIKey:           strings.TrimSpace(p.APIKey),
		Model:            strings.TrimSpace(p.Model),
		BaseURL:          strings.TrimSpace(p.BaseURL),
		Timeout:          strings.TrimSpace(p.Timeout),
		RateLimit:        p.RateLimit,
		APISchema:        strings.TrimSpace(p.APISchema),
		MaxContextTokens: p.MaxContextTokens,
		MaxOutputTokens:  p.MaxOutputTokens,
	}
	if p.Grounding != nil {
		out.Grounding = &stropdspy.GroundingConfig{DynamicThreshold: p.Grounding.DynamicThreshold}
	}
	return out
}
```

#### AIProviderConfig.UsesEmbeddedGateway (method)

```go
func (p AIProviderConfig) UsesEmbeddedGateway() bool {
	base := strings.ToLower(strings.TrimSpace(p.BaseURL))
	return base == "gateway" || base == "embedded"
}
```

#### Observability.Expand (method)

```go
func (o Observability) Expand() Observability {
	o.Endpoint = expandEnvVars(o.Endpoint)
	o.APIKey = expandEnvVars(o.APIKey)
	o.ServiceName = expandEnvVars(o.ServiceName)
	return o
}
```

#### ResolvePrepPaths (func)

```go
func ResolvePrepPaths(configDir, repoID, pipelineName, materializeDir, routingPath, agentContextPath string) (routing, agentContext string, cfg RepoConfig, err error) {
	routing = routingPath
	agentContext = agentContextPath
	if strings.TrimSpace(configDir) == "" || strings.TrimSpace(repoID) == "" {
		return routing, agentContext, cfg, nil
	}
	if pipelineName == "" {
		pipelineName = "pr-review"
	}
	cfg, err = LoadMerged(configDir, repoID)
	if err != nil {
		return "", "", RepoConfig{}, err
	}
	needMat := routing == "" || agentContext == ""
	if !needMat {
		return routing, agentContext, cfg, nil
	}
	if materializeDir == "" {
		return "", "", RepoConfig{}, fmt.Errorf("materialize dir required when using --config-dir")
	}
	mat, err := MaterializePrep(cfg, pipelineName, materializeDir)
	if err != nil {
		return "", "", RepoConfig{}, err
	}
	if routing == "" {
		routing = mat.RoutingPath
	}
	if agentContext == "" {
		agentContext = mat.AgentContextPath
	}
	return routing, agentContext, cfg, nil
}
```

#### MaterializeDirForStaging (func)

```go
func MaterializeDirForStaging(stagingDir string) string {
	return filepath.Join(stagingDir, ".majordomo-config")
}
```

### Private one-hop bodies

#### expandEnvVars (func)

```go
func expandEnvVars(s string) string {
	return strings.TrimSpace(envPlaceholderRE.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-1]
		return os.Getenv(name)
	}))
}
```

#### expandProvider (func)

```go
func expandProvider(p AIProviderConfig) AIProviderConfig {
	p.APIKey = expandEnvVars(p.APIKey)
	p.Model = expandEnvVars(p.Model)
	p.BaseURL = expandEnvVars(p.BaseURL)
	p.Timeout = expandEnvVars(p.Timeout)
	p.APISchema = expandEnvVars(p.APISchema)
	return p
}
```

#### mergeConfig (func)

```go
func mergeConfig(base, over RepoConfig) RepoConfig {
	out := base
	if over.SCM != "" {
		out.SCM = over.SCM
	}
	if over.Repository.ID != "" || over.Repository.CloneURL != "" || over.Repository.Owner != "" || over.Repository.Name != "" {
		if over.Repository.ID != "" {
			out.Repository.ID = over.Repository.ID
		}
		if over.Repository.CloneURL != "" {
			out.Repository.CloneURL = over.Repository.CloneURL
		}
		if over.Repository.Owner != "" {
			out.Repository.Owner = over.Repository.Owner
		}
		if over.Repository.Name != "" {
			out.Repository.Name = over.Repository.Name
		}
	}
	if over.SCMAPI.BaseURL != "" {
		out.SCMAPI.BaseURL = over.SCMAPI.BaseURL
	}
	if over.SCMAPI.ProjectID != "" {
		out.SCMAPI.ProjectID = over.SCMAPI.ProjectID
	}
	if over.Trigger.Interval != "" {
		out.Trigger.Interval = over.Trigger.Interval
	}
	if over.Trigger.Push.Mode != "" {
		out.Trigger.Push.Mode = over.Trigger.Push.Mode
	}
	if over.Trigger.Poll != nil {
		out.Trigger.Poll = over.Trigger.Poll
	}
	if over.Cache.Repo != "" {
		out.Cache.Repo = over.Cache.Repo
	}
	if over.Cache.Dir != "" {
		out.Cache.Dir = over.Cache.Dir
	}
	if over.Cache.RetentionDays != 0 {
		out.Cache.RetentionDays = over.Cache.RetentionDays
	}
	out.Cache.DisableSkips = over.Cache.DisableSkips || base.Cache.DisableSkips
	if over.PollCache.Repo != "" {
		out.PollCache.Repo = over.PollCache.Repo
	}
	if over.PollCache.Branch != "" {
		out.PollCache.Branch = over.PollCache.Branch
	}
	if over.Context.Repo != "" {
		out.Context.Repo = over.Context.Repo
	}
	if over.Context.Branch != "" {
		out.Context.Branch = over.Context.Branch
	}
	if over.Context.AutoMerge != nil {
		out.Context.AutoMerge = over.Context.AutoMerge
	}
	if over.Context.GateCommentPrefix != "" {
		out.Context.GateCommentPrefix = over.Context.GateCommentPrefix
	}
	if over.Context.Compaction.MaxChronologyEntries != 0 {
		out.Context.Compaction.MaxChronologyEntries = over.Context.Compaction.MaxChronologyEntries
	}
	if over.Context.Compaction.KeepRecentEntries != 0 {
		out.Context.Compaction.KeepRecentEntries = over.Context.Compaction.KeepRecentEntries
	}
	if over.Review.PublishMode != "" {
		out.Review.PublishMode = over.Review.PublishMode
	}
	if over.Review.EnableContinuousRuns != nil {
		out.Review.EnableContinuousRuns = over.Review.EnableContinuousRuns
	}
	if over.PublishMode != "" {
		out.PublishMode = over.PublishMode
	}
	if len(over.StaticAnalysis) > 0 {
		out.StaticAnalysis = append([]StaticAnalysisTool(nil), over.StaticAnalysis...)
	}
	out.Pipelines = mergePipelines(base.Pipelines, over.Pipelines)
	out.AIProviders = mergeAIProviders(base.AIProviders, over.AIProviders)
	out.JobConfigs = mergeJobConfigs(base.JobConfigs, over.JobConfigs)
	out.Observability = mergeObservability(base.Observability, over.Observability)
	return out
}
```

#### saSlug (func)

```go
func saSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == '_' || r == '.' || r == '/' {
			b.WriteByte('-')
		}
	}
	out := b.String()
	if out == "" {
		return "sa-tool"
	}
	return out
}
```

#### secretKeySuffix (func)

```go
func secretKeySuffix(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
```

#### setEnvIfEmpty (func)

```go
func setEnvIfEmpty(key, val string) error {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return nil
	}
	return os.Setenv(key, val)
}
```

#### unresolvedPlaceholders (func)

```go
func unresolvedPlaceholders(s string) string {
	m := envPlaceholderRE.FindString(s)
	return m
}
```

#### writeRoutingJSON (func)

```go
func writeRoutingJSON(path string, routing OrderedRouting) error {
	// Encode as ordered object so LoadRouting preserves first-match-wins order.
	var b bytes.Buffer
	b.WriteByte('{')
	for i, key := range routing.Keys {
		if i > 0 {
			b.WriteByte(',')
		}
		keyJSON, err := json.Marshal(key)
		if err != nil {
			return err
		}
		b.Write(keyJSON)
		b.WriteByte(':')
		entry := routing.Rules[key]
		var val any
		if entry.Persona != "" {
			val = map[string]any{"globs": entry.Globs, "persona": entry.Persona}
		} else {
			val = entry.Globs
		}
		valJSON, err := json.Marshal(val)
		if err != nil {
			return err
		}
		b.Write(valJSON)
	}
	b.WriteString("}\n")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write routing.json: %w", err)
	}
	return nil
}
```


## ./internal/contextdigest
- package: `contextdigest`
- packageDoc: Package contextdigest runs the served-repo context catch-up job.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: true
- importsOsExec: true
- importsGrpc: false
- importsOtel: true
- importsPrometheus: false
- mechanicalRole: observability
- mechanicalConfidence: 0.90
- mechanicalEvidence: imports_otel
- exportedDecls: BootstrapStoryGenerator, BootstrapStoryInput, BootstrapStoryOutput, BootstrapSurveyInput, BootstrapSurveyRunner, CommitContext, CompactOptions, FindingCommentBodiesFile, FindingCommentBody, FindingCommentsSidecar, Forge, Git, HumanInterventionGenerator, HumanInterventionInput, HumanInterventionOutput, JudgeBootstrapStoryGenerator, JudgeHumanInterventionGenerator, JudgeTypologySlicePipeline, LocalBootstrapSurveyRunner, LocalSeedManifest, LocalSeedWorkspace, LocalStageCatalog, LocalStageIntervention, LocalStageStory, LocalStageSurvey, Options, PRCommentAPI, PRCommentWithID, PRHead, RepoTarget, ReposResult, Result, ResumeProvenance, ResumeStageCatalog, ResumeStageIntervention, ResumeStageStory, RewriteInfo, TypologySlicePipeline, TypologySlicePipelineInput, TypologySlicePipelineOutput
- exportedFuncs: ApplyRewriteWhy, BeginRewrite, CheckoutBranch, CheckoutOrCreate, CheckoutUpdateBranch, CollectChangedFiles, CommitAll, CompactChronology, CompleteRewrite, DeepenDefaultBranch, DefaultCompactOptions, DetectRewrite, EnsureAncestor, FetchOrigin, FirstParentCommits, InitOrphan, IsAncestor, IsBehind, ListTargets, LoadCommitContext, LocalBranchExists, MaterializeAgenting, NeedsMetaUpdate, OpenLocalSeedWorkspace, ProcessCommit, Push, PushForce, ReadCursor, RemoteBranchExists, ReshapeStory, ResolveDefaultBranch, Run, SyncFindingPRComments, UpdateMeta, WalkCommits
- exportedMethods: Forge.ListPRComments, Forge.ListPRCommentsWithIDs, Forge.MergeUpdatePR, Forge.OpenUpdatePR, Forge.PostPRComment, Forge.ResolvePRHead, Forge.UpdatePRComment, Forge.WriteTipBranch, JudgeBootstrapStoryGenerator.Generate, JudgeHumanInterventionGenerator.Generate, JudgeTypologySlicePipeline.Assemble, LocalBootstrapSurveyRunner.Survey, LocalSeedWorkspace.AnalysisDir, LocalSeedWorkspace.BeginStage, LocalSeedWorkspace.CacheDir, LocalSeedWorkspace.ContextDir, LocalSeedWorkspace.DiffPath, LocalSeedWorkspace.IsNew, LocalSeedWorkspace.PersistAnalysisDrafts, LocalSeedWorkspace.RecordFailure, LocalSeedWorkspace.Release, LocalSeedWorkspace.RestoreAnalysisDrafts, LocalSeedWorkspace.SaveCheckpoint, LocalSeedWorkspace.WorkStoryDir, LocalSeedWorkspace.WriteLocalDiff, digestSectionRunner.Run, ledgerGroundingIssue.Error, ledgerStepRunner.RunStep, packageJudgeGenerator.Evaluate, packageJudgeGenerator.Generate, packageJudgeGenerator.Ready, packageJudgeGenerator.TaskModel, rlmBootstrapStoryGenerator.Generate, rlmCompleteAdapter.Complete, stropBootstrapStoryRLM.Complete, stropClusterMergeAuditor.Audit, stropClusterMergeAuditor.Complete, stropClusterMergeAuditor.TraceDir, stropPackageRoleRLM.Validate, stropSliceCatalogRLM.AssembleSlices, stropSliceCatalogRLM.Complete, stropSliceObjectiveLedgerRLM.BuildSliceLedger, stropSliceObjectiveLedgerRLM.Complete, stubSliceLedgerCaller.Complete
- unexportedDecls: agreementAbstain, agreementDisagree, agreementMatch, agreementUnvalidated, analysisDraftArchRel, analysisDraftCatalogRel, bootstrapStoryCaller, bootstrapStorySection, capAdaptExternal, capAggregateViews, capConfig, capDataShape, capExecProcess, capFillDTO, capMergeAdapters, capObservability, capOrchestrate, capOwnDomainRules, capRunCLI, capServeHTTP, capSynchronizeState, capWireHandlers, capabilityConstraintsSection, catalogAssembleTarget, catalogRLMWorkers, catchUpSliceDelta, cloneAnalysisRepoFn, clusterAuditCaller, clusterAuditIDRE, clusterAuditRequest, clusterAuditResult, clusterAuditVerdictRE, clusterMergeAuditor, clusterMergeProposalRel, clusterMergeVerdict, clusterMergeVerdictsDoc, clusterMergeVerdictsRel, confidenceAgreeMatch, confidenceConflictBar, confirmedCatalogRel, contextPRMarker, digestPRBodyData, digestPRBodyTemplate, digestPRBodyTmpl, digestSectionRunner, edgeFillsDTO, edgeServesServer, edgeUsesRunner, evidenceReq, findingCommentBodiesRel, findingCommentBodyTmpl, findingCommentsRel, findingMarkerPrefix, findingMarkerRE, findingMarkerSuffix, finishParams, groundedObjectivesAppendTemplate, groundedObjectivesAppendTmpl, groundedSliceObjective, hollowObjectiveRe, humanInterventionRel, interventionCacheOpts, journeyStatusOpenPendingMerges, ledgerClaimsRE, ledgerEvidenceRE, ledgerEvidenceStepPref, ledgerEvidenceTimeout, ledgerGroundingIssue, ledgerMaxContextChars, ledgerObjectiveRE, ledgerPlanRelDir, ledgerRLMTimeout, ledgerRLMWorkers, ledgerSliceTarget, ledgerSourceRLM, ledgerSourceUnclaimed, ledgerStepRunner, ledgerStepplanRelDir, ledgerSynthesisStepID, ledgerSynthesisTimeout, ledgerUnclaimedObjective, ledgerVerdictGrounded, ledgerVerdictOverclaim, llmInspectConfidence, localAnalysisRel, localCacheRel, localContextRel, localDiffRel, localSeedLock, localSeedSchemaVersion, localWorkStoryRel, localWorkspaceLock, localWorkspaceManifest, majordomoReadingBlockRE, majordomoReadingMarkerRE, materializePRHeadForResume, maxBootstrapStoryAttempts, maxClusterAuditMerges, maxDiffLines, maxHumanInterventionAttempts, maxLedgerSlices, maxRLMValidatePackages, maxReadmeSnapshotRunes, maxTypologyRefineAttempts, mechanicalGroupingRel, mergeIntentNickname, mergeIntentSlice, missingCatalogComponent, missingSliceBindingRE, objectiveVerdictRE, ownerSet, ownerSetIndex, packageCapabilityConstraint, packageCapabilityConstraintsDoc, packageCapabilityConstraintsRel, packageEvidenceNote, packageJudgeGenerator, packageRoleEdge, packageRoleNode, packageRoleRLMValidator, packageRolesDoc, packageRolesRel, prPriorityRel, promoteConfirmedTypologyResult, proposedMerge, refinedSnapshotRel, rlmBootstrapStoryGenerator, rlmCompleteAdapter, rlmValidateWorkers, roleAdapter, roleAggregator, roleCapabilityDefaults, roleConfig, roleDTO, roleEntrypoint, roleExecRunner, roleHTTPSurface, roleObservability, roleTokenRE, roleUnknown, sliceCatalogAssembleRequest, sliceCatalogAssembleResult, sliceCatalogAssembler, sliceCatalogRLMCaller, sliceLedgerBuildRequest, sliceLedgerRLMCaller, sliceLedgerStepPlan, sliceObjectiveClaim, sliceObjectiveClaimsDoc, sliceObjectiveClaimsRel, sliceObjectiveLedgerBuilder, sliceObjectiveLedgerDoc, sliceObjectiveLedgerEntry, sliceObjectiveLedgerRel, stickyVerdictMap, storyArchitectureBanner, storyArchitectureRoleMarker, storySection, storySections, stropBootstrapStoryRLM, stropClusterMergeAuditor, stropPackageRoleRLM, stropSliceCatalogRLM, stropSliceObjectiveLedgerRLM, stubSliceLedgerCaller, surveyRoots, typologyArchitectureBanner, typologyArchitectureRoleMarker, typologyEvidenceDir, typologyManifestRel, typologyPromoteDecision, unmappedPackageFindingRE, verdictAccept, verdictOverlay, verdictReject
- unexportedFuncs: acceptedPackageSets, alignClusterAuditRows, alignLedgerToRefinedCatalog, annotateCatalogYAMLError, anyPathMatches, appendConstraintClaimIssues, appendDuplicatePackageOwnerIssues, appendEvidenceGroundingIssues, appendHTTPSurfaceComponents, appendHollowSliceOwnershipIssues, appendUnique, applyAcceptedMerges, applyCatchUpSliceDelta, applyEvidencedLibraryBindings, applyRLMAgreement, applyStoryLLM, architectureHasFindings, architectureIdentitySHA, assembleSliceCatalogFragments, assertAcceptedMembership, assertArchitectureKeepsGroundedObjectives, assertEvidenceGroundingBeforeStory, assertEvidenceGroundingYAML, assertStoryResumeReady, attachMissingComponents, attachModuleTrace, auditBlockFieldStart, bindingSortKey, bootstrapContextBranch, bootstrapStorySections, buildBootstrapStoryContext, buildCapabilityConstraints, buildClusterAuditContext, buildLedgerStepplanSteps, buildSliceCatalogContext, buildSliceLedgerStepPlan, buildSliceObjectiveLedger, cachedFromPackageRole, capReadmeSnapshot, catalogAssembleTargets, catalogsNormalizedEqual, chooseLocalWorkStory, claimEntailed, claimPolicyPromptRules, claimsBySliceID, claimsDocFromLedger, claimsImplyRuntimeWork, claimsNotAllowedByOwnedIs, classifyLedgerStepError, cloneAnalysisRepo, cloneTypology, clusterProposalHasCapabilityConstraints, clusterVerdictsIdentitySHA, coerceMergeIntent, collapseHollowPackageSlices, collectCatalogPathList, collectCatalogPaths, collectEvidenceFromBlock, collectFlatOwnsItems, collectFlatSurfaceItems, collectSliceEntrypointPaths, collectTopLevelEntries, completeEvidencedLibraryBindings, componentPaths, configureCommitIdentity, constrainedSlicesWithObjectives, constraintsByPath, containsNormalizedPath, containsString, copyFile, copyStringMap, copyTree, decideTypologyPromote, decodeCommentIDLines, defaultRunner, demoteRejectedMergesList, digestModelID, digestPRBody, discoverGoModules, discoverPythonRoot, discoverSurveyRoots, draftCatalogIdentitySHA, draftPackageOwner, dropAllowedCapabilityMustNot, dropBindingsToMissingSlices, dropLibraryPackagesAbsent, dropSlicePackagesAbsent, dumpRefinedCatalogFailure, emptyHumanInterventionNote, enforceAcceptEvidenceCoverage, ensureArchitectureMarkdownKeepsGroundedObjectives, ensureDigestJudge, ensureRemote, ensureRoleBanner, ensureRolesNonEmpty, ensureStoryArchitectureBanner, ensureTypologyArchitectureBanner, ensureWorkStoryDir, evaluateTypologyBoundaries, evaluateTypologyCatalogBoundaries, evidenceForChronology, evidenceHasAny, evidenceHasAnyPrefix, extractArchitectureFindings, extractSliceCatalogYAMLDocument, fallbackArchitecture, fieldLine, fileExists, fillsDTOFromSet, filterClaimsToOwnedIs, filterIssuesForSlice, filterOwnsToAcceptedOrDraft, finalizeLedgerAnswer, finalizeLedgerEntryFromAnswer, findSliceIndexByID, findingCommentMarker, findingCoverageNeedles, findingFingerprint, findingMatchNeedle, finishDigestRun, firstLine, firstMarkdownParagraph, firstNonEmpty, flagHumanIntervention, flattenMergesForCache, formatBootstrapStorySectionQuery, formatCapabilityConstraintsMarkdown, formatClusterAuditQuery, formatClusterAuditRejectFeedback, formatConstraintRowsForPaths, formatDistilledPackageEvidence, formatFindingCommentBody, formatFindingsList, formatMissingGroundedObjectivesFeedback, formatPackageEvidenceQuery, formatProposedMergesForAudit, formatRolesYAMLAsMarkdown, formatSliceCatalogQuery, formatSliceObjectiveLedgerQuery, fromCachedVerdicts, gatherSlicePackageEvidence, generateInterventionStep, glabRepoArgs, groundOneSliceCatalog, groundOneSliceLedger, handleRewrite, hasOpenClusterRejects, httpImportedOnlyBySliceEntrypoints, inferInteractionKind, inferenceWorkRoot, intersectStrings, interventionCacheOptsFrom, interventionEvidenceReqs, isArchitectureFindingsHeading, isExecAdapterPath, isExecRunnerRole, isHollowObjective, isInteractionRole, isLocalSeedMode, isMergeNoneSentinel, isPRResumeMode, isPlaceholderOwner, isPlaceholderRepo, isResumeMode, isSliceSectionBoundary, isStage1DeliveryRole, joinSliceCatalog, journeyDebtStillSaysMerge, journeyHasDebtTable, journeyStatusClaimsComplete, knownCapabilityCodes, ledgerEvidenceStepID, ledgerPlanID, libraryPackagePaths, loadArchitectureFindingsFromContext, loadBootstrapStoryInput, loadCapabilityConstraints, loadDraftCatalog, loadFindingCommentBodies, loadFindingCommentsSidecar, loadHumanInterventionMarkdown, loadPRPriorityMarkdown, loadPackageRoles, loadPackageRolesFromEvidenceDir, loadRefinedCatalogIfPresent, loadStoryDraft, loadTypologyFromYAML, loadTypologyManifestFromContext, localStageReady, logf, looksLikeClusterAuditYAML, looksLikeInteractionPath, looksLikeStoryMarkdownEnvelope, markUnvalidated, marshalClaims, marshalClusterMergeProposal, marshalClusterMergeVerdicts, marshalConstraints, marshalLedger, marshalRoles, matchSimpleGlob, materializeContextWorktree, materializeDigestCacheWorktree, materializePRHeadTree, mechanicalIdentitySHA, mechanicalPackageEvidenceNote, mechanicalPreClusterYAML, mergeChangedLedger, mergesFromClusterOut, missingGroundedObjectives, moduleScopeExists, moduleTraceDir, multiPackageOwnerSetIndex, multiPackageOwnerSets, mustParseRoles, newBootstrapStoryRLMFromOpts, newCatalogAssemblerFromOpts, newClusterAuditorFromOpts, newLedgerBuilderFromOpts, newObjectiveLedgerPredictModule, newRLMValidatorFromOpts, newStropBootstrapStoryRLM, newStropClusterMergeAuditor, newStropPackageRoleRLM, newStropSliceCatalogRLM, newStropSliceObjectiveLedgerRLM, nextLocalStage, normalizeCatalogPath, normalizeCatalogSurfaceKind, normalizeDraftCatalogID, normalizeEphemeralCatalogDoc, normalizeEvidenceList, normalizeObjectiveText, normalizeObservedRole, normalizePackageList, normalizeResumeStage, normalizeRolePath, openJourneyNoFindings, ownedEvidenceHasAny, ownedEvidenceHasPrefix, ownedIsCodes, ownedIsSet, packageBaseOwners, packageEvidenceInputHash, packageInventoryFromGraph, packageInventoryLooksLikeModuleScope, packageRLMContextSnippet, packageRoleFromCached, packageSetKey, packageUnconstrained, parseAndValidateSliceCatalogFragment, parseBootstrapStoryMarkdownAnswer, parseCapabilityConstraintsYAML, parseClusterAuditAnswer, parseClusterAuditAnswerLines, parseClusterAuditAnswerYAML, parseFindingFingerprint, parseGraphImporters, parseMissingSliceBinding, parseObjectiveClaimsYAML, parseObjectiveLedgerYAML, parsePackageEvidenceAnswer, parseProposedMergesYAML, parseRLMRoleAnswer, parseRLMRoleAnswerYAML, parseRolesYAML, parseSliceObjectiveLedgerAnswer, parseSliceObjectiveLedgerAnswerLines, parseSliceObjectiveLedgerAnswerYAML, pathSetKey, persistBootstrapStory, persistResumeLocalOutputs, persistSliceLedgerStepPlan, placePackageOnSlice, polishTypologyArchitectureBrief, polishTypologyArchitectureBriefText, prepareWorkStory, prependObservedRolesBrief, processAlive, producedBootstrapOutput, promoteConfirmedTypology, pruneDraftPackagesAbsentFromRoles, pushDigestCacheWorktree, readEffectiveCursor, readEvidenceRel, readResponseBody, readRewriteMeta, readText, reconcileJourneyStatusWithDebt, refineEvidenceReqs, refineTypologyEvidence, refinedSnapshotPath, refreshTypologyOnCatchUp, rejectInspectRoleContradiction, rejectInventedCatalogPaths, rejectMajordomoAsProduct, rejectMissingDraftPackages, rejectUnentailedClaims, remapInventedCatalogPaths, repairFlatSliceCatalogYAML, replaceSlices, requireBootstrapStoryEvidence, requireEvidenceFiles, resolveLedgerEntryForTarget, resolveToken, resolveWorkStoryDir, restoreMissingDraftPackages, restoreMissingRolePackages, rewriteChangedSlices, rewriteChronology, rewriteJourneyStatusBody, rewriteModuleScopePackageInventory, rlmTraceDir, roleByPath, roleCapabilityTable, rolesIdentitySHA, runBootstrapFromStage, runClusterMergeAudit, runLocalSeed, runLocalStages, runResumeFromPR, runSliceLedgerViaStepPlan, runTypology, runTypologyCapture, runreportDir, sanitizeRefinedCatalog, saveFindingCommentBodies, saveFindingCommentsSidecar, saveRefinedCatalog, scrubForbiddenHTTPEntrypointMergeRows, scrubForbiddenHTTPEntrypointMerges, sectionSignals, seedOrphan, separateHTTPSurfacesFromEntrypoint, shortSHA, sliceAllCapabilityCodesMustNot, sliceAppearsInBindings, sliceIDFromPath, sliceIsUnion, sliceMustNotUnion, sliceOwningNeighborhood, slicePackagePaths, slicesByID, splitAuditBlocks, splitCommaPackages, splitCommaTokens, splitEvidenceList, splitIllegalMembershipToDraftOwners, splitLedgerList, splitOwnerName, splitSemicolonGroups, splitYAMLSimpleKV, stageAnalysisDraftsFromEvidence, stageOrder, stampChangedSliceObjectives, stampLedgerObjectivesOntoCatalog, statusTextClaimsComplete, storyEvidenceReqs, stringField, stringListField, stripCodeFence, stripGeneratedAtLines, stripLeakedStoryMarkdownEnvelope, stripMajordomoReadingMarkers, surveyWithTypologyPythonOnly, toCachedVerdicts, touchStorySections, treeHasChanges, treeHasRequiredContext, truncateErr, truncateToLedgerBudget, typologyPromotePRBody, typologyVersion, uniqueCatchUpSliceID, uniqueNearestDraftPath, uniqueStrings, validateArchitectureGroundedObjectives, validateBootstrapStoryEvidenceMap, validateBootstrapStoryOutput, validateBootstrapStorySection, validateDigestModeOptions, validateHumanInterventionOutputs, validateJourneyFindings, validateLedgerAgainstConstraints, validateLocalSeedOptions, validateNamedFindingCoverage, validateObjectiveLedgerDoc, validatePackageRolesRLM, validateRefinedCatalogYAML, validateRefinedCatalogYAMLWithGraph, validateResumeEvidence, validateResumeOptions, walkCommitContexts, writeBootstrapStory, writeCapabilityConstraints, writeFallbackSurvey, writeFileAtomic, writeMeta, writeObjectiveClaims, writeObjectiveLedger, writePackageRoles, writeRequiredFile, writeStoryDraft, writeTempBody, writeTypologyManifest, yamlErrorLine, yamlLineSnippet, yamlMarshalLedgerPlan
- unexportedMethods: Forge.client, Forge.findBitbucketOpen, Forge.findGitHubOpen, Forge.findGitLabOpen, Forge.findOpenPRNumber, Forge.ghEnv, Forge.glabEnv, Forge.listBitbucketCommentsWithIDs, Forge.listGitHubCommentsWithIDs, Forge.listGitLabCommentsWithIDs, Forge.openBitbucket, Forge.openGitHub, Forge.openGitLab, Forge.postBitbucketComment, Forge.postGitHubComment, Forge.postGitLabComment, Forge.repoSlug, Forge.resolveBitbucketPRHead, Forge.resolveGitHubPRHead, Forge.resolveGitLabPRHead, Forge.runCLI, Forge.runCLIWithStdin, Forge.updateBitbucketComment, Forge.updateBitbucketPR, Forge.updateGitHubComment, Forge.updateGitHubPR, Forge.updateGitLabComment, Forge.updateGitLabMR, Git.authConfigArgs, Git.run, Git.runAllowFail, Git.trim, LocalSeedWorkspace.acquireLock, LocalSeedWorkspace.ensureLayout, LocalSeedWorkspace.loadManifest, LocalSeedWorkspace.validateIdentity, LocalSeedWorkspace.writeManifest, ledgerStepRunner.loadEvidenceNotes, ledgerStepRunner.runEvidence, ledgerStepRunner.runSynthesis, ownerSetIndex.contains, stickyVerdictMap.applySticky, stickyVerdictMap.get, stickyVerdictMap.put
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/agenting.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/architecture_findings.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/architecture_polish.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/architecture_roles.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/bootstrap_context.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/bootstrap_human_intervention.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/bootstrap_story.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/bootstrap_story_rlm.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/bootstrap_survey.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/bootstrap_typology_refine.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/capability_constraints.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/catchup_slice_delta.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/claim_entailment.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/cluster_audit_rlm.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/cluster_membership_gate.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/cluster_merge_verdicts.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/compact.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/digest_cache.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/digest_fingerprint.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/evidence_grounding.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/evidence_path.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/finding_comments.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/forge.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/forge_comments.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/git.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/helpers.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/list.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/local_seed_run.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/local_seed_workspace.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/mechanical_grouping.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/meta.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/objective_grounding_rlm.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/objective_grounding_stepplan.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/package_roles.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/resume.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/resume_evidence.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/resume_pr_head.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/rewrite.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/rlm_trace.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/role_rlm_validate.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/run.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/slice_catalog_fold.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/slice_catalog_rlm.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/slice_objective_ledger.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/story.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/story_llm.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextdigest/typology_promote.go

### Exported bodies

#### MaterializeAgenting (func)

```go
func MaterializeAgenting(ctxDir string, changedFiles []string) error {
	mission, err := os.ReadFile(filepath.Join(ctxDir, "mission.md"))
	if err != nil {
		return err
	}
	arch, err := os.ReadFile(filepath.Join(ctxDir, "architecture.md"))
	if err != nil {
		return err
	}
	overview := fmt.Sprintf("# Overview\n\n%s\n\n## Architecture\n\n%s\n",
		strings.TrimSpace(string(mission)), strings.TrimSpace(string(arch)))
	overviewDir := filepath.Join(ctxDir, "agenting", "overview")
	if err := os.MkdirAll(overviewDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(overviewDir, agenting.GroundingName), []byte(overview), 0o644); err != nil {
		return err
	}

	if _, statErr := os.Stat(filepath.Join(ctxDir, agenting.IndexRelPath)); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil
		}
		return statErr
	}
	idx, err := agenting.LoadIndex(ctxDir)
	if err != nil {
		return err
	}
	for _, id := range idx.PackIDs() {
		if id == "overview" {
			continue
		}
		pack, ok := idx.Packs[id]
		if !ok || len(pack.Globs) == 0 {
			continue
		}
		if !anyPathMatches(changedFiles, pack.Globs) {
			continue
		}
		packDir := filepath.Join(ctxDir, "agenting", id)
		if err := os.MkdirAll(packDir, 0o755); err != nil {
			return err
		}
		body := fmt.Sprintf("# %s\n\nGrounding for `%s` from digest (files touched: %s).\n",
			id, id, strings.Join(changedFiles, ", "))
		if err := os.WriteFile(filepath.Join(packDir, agenting.GroundingName), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return contextstore.ValidateTree(ctxDir)
}
```

#### CollectChangedFiles (func)

```go
func CollectChangedFiles(commits []CommitContext) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, c := range commits {
		for _, f := range c.Files {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	return out
}
```

#### HumanInterventionInput (type)

```go
type HumanInterventionInput struct {
	RepoID                   string
	ArchitectureMD           string
	RefinedCatalogYAML       string
	JourneyMD                string
	ClusterMergeProposalYAML string
	ClusterMergeVerdictsYAML string
	FindingsList             string
	ValidationFeedback       string
	DigestCache              *cache.DigestStore
	DigestSkips              bool
	DigestModelID            string
}
```

#### FindingCommentBody (type)

```go
type FindingCommentBody struct {
	Finding     string `json:"finding"`
	Fingerprint string `json:"fingerprint"`
	Body        string `json:"body"`
}
```

#### HumanInterventionOutput (type)

```go
type HumanInterventionOutput struct {
	JourneyMD           string
	HumanInterventionMD string
	WeaknessesSeedMD    string
	PRPriorityMD        string
	FindingComments     []FindingCommentBody
}
```

#### HumanInterventionGenerator (type)

```go
type HumanInterventionGenerator interface {
	Generate(ctx context.Context, input HumanInterventionInput) (HumanInterventionOutput, error)
}
```

#### JudgeHumanInterventionGenerator (type)

```go
type JudgeHumanInterventionGenerator struct {
	Gen judge.Generator
}
```

#### JudgeHumanInterventionGenerator.Generate (method)

```go
func (g JudgeHumanInterventionGenerator) Generate(ctx context.Context, input HumanInterventionInput) (HumanInterventionOutput, error) {
	findings := extractArchitectureFindings(input.ArchitectureMD)
	if len(findings) == 0 {
		return HumanInterventionOutput{
			JourneyMD:           openJourneyNoFindings(input.JourneyMD),
			HumanInterventionMD: emptyHumanInterventionNote(),
			WeaknessesSeedMD:    "# Weaknesses\n\nNo open Typology architecture findings after refine.\n",
			PRPriorityMD:        "",
		}, nil
	}
	gen := g.Gen
	if gen == nil {
		if !judge.StoryLLMAvailable() {
			return HumanInterventionOutput{}, fmt.Errorf("LLM human intervention unavailable")
		}
		gen = packageJudgeGenerator{}
	}
	if strings.TrimSpace(input.FindingsList) == "" {
		input.FindingsList = formatFindingsList(findings)
	}

	baseFields := map[string]interface{}{
		"repo_id":                     input.RepoID,
		"architecture_md":             input.ArchitectureMD,
		"refined_catalog_yaml":        input.RefinedCatalogYAML,
		"journey_md":                  input.JourneyMD,
		"slice_grouping_proposal_yaml": input.ClusterMergeProposalYAML,
		"slice_grouping_verdicts_yaml": input.ClusterMergeVerdictsYAML,
		"findings_list":               input.FindingsList,
	}

	journey, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyInterventionJourney, baseFields, "journey_md",
		func(out map[string]interface{}) error {
			return validateJourneyFindings(findings, stringField(out, "journey_md"))
		}, interventionCacheOptsFrom(input, ""))
	if err != nil {
		return HumanInterventionOutput{}, err
	}
	baseFields["journey_md"] = journey

	brief, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyInterventionBrief, baseFields, "human_intervention_md",
		func(out map[string]interface{}) error {
			return validateNamedFindingCoverage(findings, "human_intervention_md", stringField(out, "human_intervention_md"))
		}, interventionCacheOptsFrom(input, ""))
	if err != nil {
		return HumanInterventionOutput{}, err
	}
	baseFields["human_intervention_md"] = brief

	weakFields := copyStringMap(baseFields)
	weaknesses, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyInterventionWeaknesses, weakFields, "weaknesses_seed_md",
		func(out map[string]interface{}) error {
			return validateNamedFindingCoverage(findings, "weaknesses_seed_md", stringField(out, "weaknesses_seed_md"))
		}, interventionCacheOptsFrom(input, ""))
	if err != nil {
		return HumanInterventionOutput{}, err
	}

	prFields := copyStringMap(baseFields)
	prPriority, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyInterventionPRPriority, prFields, "pr_priority_md",
		func(out map[string]interface{}) error {
			return validateNamedFindingCoverage(findings, "pr_priority_md", stringField(out, "pr_priority_md"))
		}, interventionCacheOptsFrom(input, ""))
	if err != nil {
		return HumanInterventionOutput{}, err
	}

	comments := make([]FindingCommentBody, 0, len(findings))
	for _, finding := range findings {
		commentFields := map[string]interface{}{
			"repo_id":               input.RepoID,
			"architecture_md":       input.ArchitectureMD,
			"refined_catalog_yaml":  input.RefinedCatalogYAML,
			"journey_md":            journey,
			"human_intervention_md": brief,
			"finding":               finding,
		}
		body, err := generateInterventionStep(ctx, gen, jmodules.TaskTypologyFindingComment, commentFields, "comment_md",
			func(out map[string]interface{}) error {
				return validateNamedFindingCoverage([]string{finding}, "comment_md", stringField(out, "comment_md"))
			}, interventionCacheOptsFrom(input, finding))
		if err != nil {
			return HumanInterventionOutput{}, fmt.Errorf("finding comment %q: %w", findingMatchNeedle(finding), err)
		}
		comments = append(comments, FindingCommentBody{
			Finding:     finding,
			Fingerprint: findingFingerprint(finding),
			Body:        body,
		})
	}

	return HumanInterventionOutput{
		JourneyMD:           journey,
		HumanInterventionMD: brief,

// ... truncated
```

#### BootstrapStoryInput (type)

```go
type BootstrapStoryInput struct {
	RepoID                               string
	SourceSHA                            string
	GeneratedAt                          time.Time
	EvidenceMode                         string
	ModuleScope                          string
	ReadmeSnapshot                       string
	TypologyManifest                     string
	TypologyArchitecture                 string
	TypologyRefinedCatalog               string
	TypologyJourney                      string
	TypologySliceObjectiveLedger         string
	TypologyPackageCapabilityConstraints string
	RepoLayout                           string
	CurrentReadme                        string
	CurrentMission                       string
	CurrentArchitecture                  string
	CurrentConventions                   string
	CurrentWeaknesses                    string
	CurrentChronology                    string
	CurrentGrounding                     string
	ValidationFeedback                   string
	DigestCache                          *cache.DigestStore
	DigestSkips                          bool
	DigestModelID                        string
}
```

#### BootstrapStoryOutput (type)

```go
type BootstrapStoryOutput struct {
	ReadmeMD       string
	MissionMD      string
	ArchitectureMD string
	ConventionsMD  string
	WeaknessesMD   string
	ChronologyMD   string
	GroundingMD    string
}
```

#### BootstrapStoryGenerator (type)

```go
type BootstrapStoryGenerator interface {
	Generate(ctx context.Context, input BootstrapStoryInput) (BootstrapStoryOutput, error)
}
```

#### JudgeBootstrapStoryGenerator (type)

```go
type JudgeBootstrapStoryGenerator struct {
	Gen judge.Generator
}
```

#### JudgeBootstrapStoryGenerator.Generate (method)

```go
func (g JudgeBootstrapStoryGenerator) Generate(ctx context.Context, input BootstrapStoryInput) (BootstrapStoryOutput, error) {
	gen := g.Gen
	if gen == nil {
		if !judge.StoryLLMAvailable() {
			return BootstrapStoryOutput{}, fmt.Errorf("LLM bootstrap story unavailable")
		}
		gen = packageJudgeGenerator{}
	}
	feedback := strings.TrimSpace(input.ValidationFeedback)
	var lastErr error
	for attempt := 1; attempt <= maxBootstrapStoryAttempts; attempt++ {
		fields := map[string]interface{}{
			"repo_id":                  input.RepoID,
			"source_sha":               input.SourceSHA,
			"generated_at":             input.GeneratedAt.UTC().Format(time.RFC3339),
			"evidence_mode":            input.EvidenceMode,
			"module_scope":             input.ModuleScope,
			"readme_snapshot":          input.ReadmeSnapshot,
			"typology_manifest":        input.TypologyManifest,
			"typology_architecture":    input.TypologyArchitecture,
			"typology_refined_catalog": input.TypologyRefinedCatalog,
			"slice_meaning_ledger":   input.TypologySliceObjectiveLedger,
			"typology_journey":         input.TypologyJourney,
			"repo_layout":              input.RepoLayout,
			"current_readme":           input.CurrentReadme,
			"current_mission":          input.CurrentMission,
			"current_architecture":     input.CurrentArchitecture,
			"current_conventions":      input.CurrentConventions,
			"current_weaknesses":       input.CurrentWeaknesses,
			"current_chronology":       input.CurrentChronology,
			"current_grounding":        input.CurrentGrounding,
			"validation_feedback":      feedback,
		}
		out, err := gen.Generate(ctx, jmodules.TaskBootstrapStory, fields, attempt)
		if err != nil {
			return BootstrapStoryOutput{}, err
		}
		res := BootstrapStoryOutput{
			ReadmeMD:       stringField(out, "readme_md"),
			MissionMD:      stringField(out, "mission_md"),
			ArchitectureMD: stringField(out, "architecture_md"),
			ConventionsMD:  stringField(out, "conventions_md"),
			WeaknessesMD:   stringField(out, "weaknesses_md"),
			ChronologyMD:   stringField(out, "chronology_md"),
			GroundingMD:    stringField(out, "grounding_md"),
		}
		if err := validateBootstrapStoryOutput(res); err != nil {
			lastErr = err
			if attempt == maxBootstrapStoryAttempts {
				return BootstrapStoryOutput{}, err
			}
			feedback = err.Error()
			continue
		}
		// Intentionally no LLM Evaluate: production path is rlmBootstrapStoryGenerator.
		return res, nil
	}
	if lastErr != nil {
		return BootstrapStoryOutput{}, lastErr
	}
	return BootstrapStoryOutput{}, fmt.Errorf("bootstrap story failed after %d attempts", maxBootstrapStoryAttempts)
}
```

#### packageJudgeGenerator.Generate (method)

```go
func (packageJudgeGenerator) Generate(ctx context.Context, task string, fields map[string]interface{}, version int) (map[string]interface{}, error) {
	return judge.Generate(ctx, task, fields, version)
}
```

#### packageJudgeGenerator.Evaluate (method)

```go
func (packageJudgeGenerator) Evaluate(
	ctx context.Context,
	task string,
	inputFields, outputFields map[string]interface{},
	version int,
) (*evaluation.AggregatedEvaluation, error) {
	return judge.Evaluate(ctx, task, inputFields, outputFields, version)
}
```

#### packageJudgeGenerator.Ready (method)

```go
func (packageJudgeGenerator) Ready() bool { return judge.StoryLLMAvailable() }
```

#### packageJudgeGenerator.TaskModel (method)

```go
func (packageJudgeGenerator) TaskModel(string) string { return "" }
```

#### stropBootstrapStoryRLM.Complete (method)

```go
func (v stropBootstrapStoryRLM) Complete(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	return v.module.Complete(ctx, contextPayload, query)
}
```

#### rlmBootstrapStoryGenerator.Generate (method)

```go
func (g rlmBootstrapStoryGenerator) Generate(ctx context.Context, input BootstrapStoryInput) (BootstrapStoryOutput, error) {
	if g.caller == nil {
		return BootstrapStoryOutput{}, fmt.Errorf("LLM bootstrap story RLM unavailable")
	}
	if err := validateBootstrapStoryEvidenceMap(input); err != nil {
		return BootstrapStoryOutput{}, err
	}
	contextMD := buildBootstrapStoryContext(input)
	sections := bootstrapStorySections(input)
	var out BootstrapStoryOutput
	feedback := strings.TrimSpace(input.ValidationFeedback)
	for _, sec := range sections {
		var sectionErr error
		secCtx, span := observability.StartChainSpan(ctx, "majordomo.context.digest", "bootstrap_story."+sec.ID)
		span.SetAttributes(attribute.String("bootstrap.section_id", sec.ID))
		var markdown string
		var lastErr error
		storyFP := cache.StoryFingerprint{
			SectionID:        sec.ID,
			RefinedHash:      draftCatalogIdentitySHA(input.TypologyRefinedCatalog),
			LedgerHash:       cache.ContentSHA(input.TypologySliceObjectiveLedger),
			ArchitectureHash: architectureIdentitySHA(input.TypologyArchitecture),
			ReadmeHash:       cache.ContentSHA(input.ReadmeSnapshot),
			ModelID:          input.DigestModelID,
			PromptVersion:    cache.DigestStoryPromptV2,
			SchemaVersion:    cache.DigestStorySchemaV3,
		}
		for attempt := 1; attempt <= maxBootstrapStoryAttempts; attempt++ {
			if attempt == 1 && strings.TrimSpace(feedback) == "" &&
				input.DigestSkips && input.DigestCache != nil {
				if hit, ok, err := input.DigestCache.LookupStory(storyFP); err == nil && ok && strings.TrimSpace(hit.Markdown) != "" {
					cachedMD := hit.Markdown
					if sec.ID == "architecture" {
						ensured, ensureErr := ensureArchitectureMarkdownKeepsGroundedObjectives(
							cachedMD, input.TypologyRefinedCatalog, input.TypologyPackageCapabilityConstraints,
						)
						if ensureErr == nil {
							cachedMD = ensured
						}
					}
					if err := validateBootstrapStorySection(input, sec.ID, cachedMD); err == nil {
						input.DigestCache.RecordStoryHit(hit.PromptTokens, hit.CompletionTokens, hit.TotalTokens)
						logf("INFO", "digest cache hit story section=%s", sec.ID)
						sec.Setter(&out, cachedMD)
						feedback = ""
						lastErr = nil
						break
					}
				}
			}
			if input.DigestCache != nil && attempt == 1 {
				input.DigestCache.RecordStoryMiss()
			}
			query := formatBootstrapStorySectionQuery(input.RepoID, sec, feedback)
			answer, _, promptTok, completionTok, totalTok, err := g.caller.Complete(secCtx, contextMD, query)
			if err != nil {
				lastErr = err
				if attempt == maxBootstrapStoryAttempts {
					sectionErr = fmt.Errorf("bootstrap story section %s: %w", sec.ID, err)
					observability.EndSpanWithStatus(span, &sectionErr)
					return BootstrapStoryOutput{}, sectionErr
				}
				feedback = err.Error()
				continue
			}
			markdown, err = parseBootstrapStoryMarkdownAnswer(answer)
			if err != nil {
				lastErr = err
				if attempt == maxBootstrapStoryAttempts {
					sectionErr = fmt.Errorf("bootstrap story section %s: %w", sec.ID, err)
					observability.EndSpanWithStatus(span, &sectionErr)
					return BootstrapStoryOutput{}, sectionErr
				}
				feedback = err.Error()
				continue
			}
			sec.Setter(&out, markdown)
			if err := validateBootstrapStorySection(input, sec.ID, markdown); err != nil {
				lastErr = err
				if attempt < maxBootstrapStoryAttempts {
					feedback = err.Error()
					continue
				}
				// Last attempt: for architecture, Go appends missing grounded objectives.
				if sec.ID != "architecture" {
					sectionErr = err
					observability.EndSpanWithStatus(span, &sectionErr)
					return BootstrapStoryOutput{}, sectionErr
				}
			}
			if sec.ID == "architecture" {
				ensured, ensureErr := ensureArchitectureMarkdownKeepsGroundedObjectives(
					markdown, input.TypologyRefinedCatalog, input.TypologyPackageCapabilityConstraints,
				)
				if ensureErr != nil {
					lastErr = ensureErr
					if attempt < maxBootstrapStoryAttempts {
						feedback = ensu
// ... truncated
```

#### BootstrapSurveyInput (type)

```go
type BootstrapSurveyInput struct {
	AnalysisDir    string
	EvidenceDir    string
	SourceSHA      string
	RepoID         string
	TypologyBinary string
	ModuleScope    string
	GeneratedAt    time.Time
}
```

#### BootstrapSurveyRunner (type)

```go
type BootstrapSurveyRunner interface {
	Survey(ctx context.Context, input BootstrapSurveyInput) error
}
```

#### LocalBootstrapSurveyRunner (type)

```go
type LocalBootstrapSurveyRunner struct{}
```

#### LocalBootstrapSurveyRunner.Survey (method)

```go
func (LocalBootstrapSurveyRunner) Survey(ctx context.Context, input BootstrapSurveyInput) error {
	if strings.TrimSpace(input.AnalysisDir) == "" {
		return fmt.Errorf("bootstrap survey: analysis_dir is required")
	}
	if strings.TrimSpace(input.EvidenceDir) == "" {
		return fmt.Errorf("bootstrap survey: evidence_dir is required")
	}
	if strings.TrimSpace(input.SourceSHA) == "" {
		return fmt.Errorf("bootstrap survey: source_sha is required")
	}
	if strings.TrimSpace(input.RepoID) == "" {
		return fmt.Errorf("bootstrap survey: repo_id is required")
	}
	if err := os.MkdirAll(input.EvidenceDir, 0o755); err != nil {
		return err
	}

	roots, err := discoverSurveyRoots(input.AnalysisDir)
	if err != nil {
		return err
	}
	if !roots.HasGo && !roots.HasPython {
		return writeFallbackSurvey(input)
	}
	if strings.TrimSpace(input.TypologyBinary) == "" {
		return fmt.Errorf("bootstrap survey: Typology binary is required for Go or Python repositories")
	}
	if !roots.HasGo {
		return surveyWithTypologyPythonOnly(ctx, input)
	}

	modules := roots.GoModules
	moduleScope := strings.TrimSpace(input.ModuleScope)
	if moduleScope == "" {
		if len(modules) != 1 {
			return fmt.Errorf("bootstrap survey: multiple go modules found and no module scope supplied")
		}
		moduleScope = modules[0]
	}
	if !moduleScopeExists(moduleScope, modules) {
		return fmt.Errorf("bootstrap survey: module scope %q is not present in discovered modules %v", moduleScope, modules)
	}

	version, err := typologyVersion(ctx, input.TypologyBinary, input.AnalysisDir)
	if err != nil {
		return err
	}

	mode := contextstore.TypologyModeReuse
	draftLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "typology.yaml")
	confirmed := filepath.Join(input.AnalysisDir, ".typology", "typology.yaml")
	if _, err := os.Stat(confirmed); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("bootstrap survey: stat typology catalog: %w", err)
		}
		mode = contextstore.TypologyModeDiscover
		if err := runTypology(ctx, input.TypologyBinary, input.AnalysisDir, "discover", moduleScope); err != nil {
			return err
		}
	} else {
		if err := copyFile(confirmed, draftLocal); err != nil {
			return fmt.Errorf("bootstrap survey: stage confirmed catalog as draft: %w", err)
		}
	}

	graphOut, err := runTypologyCapture(ctx, input.TypologyBinary, input.AnalysisDir,
		"show", "graph", "--module", moduleScope, "--catalog", draftLocal)
	if err != nil {
		return err
	}
	graphPath := filepath.Join(input.EvidenceDir, "graph.txt")
	if err := os.WriteFile(graphPath, []byte(graphOut), 0o644); err != nil {
		return fmt.Errorf("bootstrap survey: write graph: %w", err)
	}

	contractsLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "package_contracts.md")
	if err := runTypology(ctx, input.TypologyBinary, input.AnalysisDir, "contracts", moduleScope,
		"--out", contractsLocal); err != nil {
		return err
	}
	contractsPath := filepath.Join(input.EvidenceDir, "package_contracts.md")
	if err := copyFile(contractsLocal, contractsPath); err != nil {
		return fmt.Errorf("bootstrap survey: copy package contracts: %w", err)
	}

	rolesLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", packageRolesRel)
	if _, err := os.Stat(rolesLocal); err != nil {
		// contracts writes roles beside the default path under tmp/typology.
		rolesLocal = filepath.Join(input.AnalysisDir, "tmp", "typology", packageRolesRel)
		if _, err := os.Stat(rolesLocal); err != nil {
			return fmt.Errorf("bootstrap survey: package roles missing after contracts: %w", err)
		}
	}
	rolesPath := filepath.Join(input.EvidenceDir, packageRolesRel)
	if err := copyFile(rolesLocal, rolesPath); err != nil {
		return fmt.Errorf("bootstrap survey: copy package roles: %w", err)
	}

	rlmLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "package_rlm_context.md")
	rlmPath := filepath.Join(input.EvidenceDir, "package_rlm_context.md")
	if _, err := os.Stat(rlmLocal); err == nil {
		if err := copyFile(rlmLocal, rlmPath); err != nil {
			retur
// ... truncated
```

#### TypologySlicePipeline (type)

```go
type TypologySlicePipeline interface {
	Assemble(ctx context.Context, input TypologySlicePipelineInput) (TypologySlicePipelineOutput, error)
}
```

#### TypologySlicePipelineInput (type)

```go
type TypologySlicePipelineInput struct {
	RepoID                string
	ModuleScope           string
	DraftCatalogYAML      string
	GraphText             string
	PackageContracts      string
	PackageRoles          string
	CapabilityConstraints string
	ArchitectureDraft     string
	RepoLayout            string
	ReadmeSnapshot        string
	ValidationFeedback    string
	AnalysisDir           string
	EvidenceDir           string
	LedgerBuilder         sliceObjectiveLedgerBuilder
	ClusterAuditor        clusterMergeAuditor
	CatalogAssembler      sliceCatalogAssembler
	DigestCache           *cache.DigestStore
	DigestSkips           bool
	DigestModelID         string
}
```

#### TypologySlicePipelineOutput (type)

```go
type TypologySlicePipelineOutput struct {
	MechanicalGroupingYAML   string
	ClusterMergeProposalYAML string
	ClusterMergeVerdictsYAML string
	RefinedCatalogYAML       string
	ObjectiveLedgerYAML      string
	ObjectiveClaimsYAML      string
}
```

#### JudgeTypologySlicePipeline (type)

```go
type JudgeTypologySlicePipeline struct {
	Gen judge.Generator
}
```

#### JudgeTypologySlicePipeline.Assemble (method)

```go
func (g JudgeTypologySlicePipeline) Assemble(ctx context.Context, input TypologySlicePipelineInput) (TypologySlicePipelineOutput, error) {
	gen := g.Gen
	if gen == nil {
		if !judge.StoryLLMAvailable() {
			return TypologySlicePipelineOutput{}, fmt.Errorf("LLM typology slice catalog unavailable")
		}
		gen = packageJudgeGenerator{}
	}
	rolesDoc := mustParseRoles(input.PackageRoles)
	mechanicalGroupingYAML, err := mechanicalPreClusterYAML(rolesDoc)
	if err != nil {
		return TypologySlicePipelineOutput{}, err
	}
	constraintsDoc, constraintsErr := parseCapabilityConstraintsYAML(input.CapabilityConstraints)
	if constraintsErr != nil && strings.TrimSpace(input.CapabilityConstraints) != "" {
		return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine capability constraints: %w", constraintsErr)
	}
	if constraintsErr != nil {
		constraintsDoc = buildCapabilityConstraints(rolesDoc)
	}

	clusterFields := map[string]interface{}{
		"repo_id":                        input.RepoID,
		"module_scope":                   input.ModuleScope,
		"draft_catalog_yaml":             input.DraftCatalogYAML,
		"graph_text":                     input.GraphText,
		"package_contracts":              input.PackageContracts,
		"package_roles":                  input.PackageRoles,
		"package_capability_constraints": input.CapabilityConstraints,
		"mechanical_grouping_yaml":       mechanicalGroupingYAML,
		"architecture_draft":             input.ArchitectureDraft,
		"repo_layout":                    input.RepoLayout,
		"readme_snapshot":                input.ReadmeSnapshot,
		"validation_feedback":            input.ValidationFeedback,
	}
	if strings.TrimSpace(input.CapabilityConstraints) == "" {
		encoded, err := marshalConstraints(constraintsDoc)
		if err != nil {
			return TypologySlicePipelineOutput{}, err
		}
		clusterFields["package_capability_constraints"] = encoded
	}
	var proposedMerges []proposedMerge
	clusterFeedback := input.ValidationFeedback
	sticky := stickyVerdictMap{}
	var mergeVerdicts []clusterMergeVerdict
	var auditMeta clusterAuditResult
	auditor := input.ClusterAuditor
	constraintsForCluster := stringField(clusterFields, "package_capability_constraints")
	mechIdentityHash, mechErr := mechanicalIdentitySHA(input.PackageRoles)
	if mechErr != nil {
		mechIdentityHash = cache.ContentSHA(mechanicalGroupingYAML)
	}
	clusterFP := cache.ClusterCoTFingerprint{
		DraftHash:       draftCatalogIdentitySHA(input.DraftCatalogYAML),
		RolesHash:       rolesIdentitySHA(input.PackageRoles),
		ConstraintsHash: cache.ContentSHA(constraintsForCluster),
		MechanicalHash:  mechIdentityHash,
		ModelID:         input.DigestModelID,
		PromptVersion:   cache.DigestClusterCoTPromptV1,
		SchemaVersion:   cache.DigestClusterCoTSchemaV3,
	}
	for attempt := 1; attempt <= maxTypologyRefineAttempts; attempt++ {
		if attempt > 1 {
			clusterFields["validation_feedback"] = clusterFeedback
		}
		var clusterOut map[string]interface{}
		usedClusterCache := false
		if attempt == 1 && strings.TrimSpace(clusterFeedback) == "" &&
			input.DigestSkips && input.DigestCache != nil {
			if hit, ok, err := input.DigestCache.LookupClusterCoT(clusterFP); err == nil && ok {
				input.DigestCache.RecordClusterCoTHit(hit.PromptTokens, hit.CompletionTokens, hit.TotalTokens)
				logf("INFO", "digest cache hit cluster_cot")
				clusterOut = map[string]interface{}{
					"merge_ids":      hit.MergeIDs,
					"merge_packages": hit.MergePackages,
					"merge_intents":  hit.MergeIntents,
				}
				usedClusterCache = true
			}
		}
		if clusterOut == nil {
			if input.DigestCache != nil {
				input.DigestCache.RecordClusterCoTMiss()
			}
			var err error
			clusterOut, err = gen.Generate(ctx, jmodules.TaskTypologySliceGrouping, clusterFields, attempt)
			if err != nil {
				return TypologySlicePipelineOutput{}, fmt.Errorf("typology cluster: %w", err)
			}
		}
		merges, parseErr := mergesFromClusterOut(clusterOut)
		if parseErr != nil {
			if attempt == maxTypologyRefineAttempts {
				return Typo
// ... truncated
```

#### stropClusterMergeAuditor.Audit (method)

```go
func (v stropClusterMergeAuditor) Audit(ctx context.Context, req clusterAuditRequest) (clusterAuditResult, error) {
	return runClusterMergeAudit(ctx, v, req)
}
```

#### stropClusterMergeAuditor.Complete (method)

```go
func (v stropClusterMergeAuditor) Complete(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	return v.module.Complete(ctx, contextPayload, query)
}
```

#### stropClusterMergeAuditor.TraceDir (method)

```go
func (v stropClusterMergeAuditor) TraceDir() string { return v.traceDir }
```

#### CompactOptions (type)

```go
type CompactOptions struct {
	MaxEntries   int
	KeepRecent   int
	ForceCompact bool
}
```

#### DefaultCompactOptions (func)

```go
func DefaultCompactOptions(cfgMax int) CompactOptions {
	max := cfgMax
	if max <= 0 {
		max = 40
	}
	return CompactOptions{MaxEntries: max, KeepRecent: 10}
}
```

#### CompactChronology (func)

```go
func CompactChronology(ctxDir string, opts CompactOptions) (bool, error) {
	path := filepath.Join(ctxDir, "chronology.md")
	events, err := contextstore.ParseChronologyFile(path)
	if err != nil {
		return false, err
	}
	if !opts.ForceCompact && len(events) <= opts.MaxEntries {
		return false, nil
	}
	if len(events) <= opts.KeepRecent {
		return false, nil
	}
	keep := events[:opts.KeepRecent]
	older := events[opts.KeepRecent:]
	if len(older) == 0 {
		return false, nil
	}
	summary := contextstore.ChronologyEvent{
		Actor:     "majordomo",
		Source:    "compaction",
		Did:       fmt.Sprintf("compacted %d older chronology entries", len(older)),
		Because:   "teaching document readability threshold exceeded",
		InOrderTo: "keep chronology scannable for newcomers",
		Evidence:  fmt.Sprintf("entries %d–%d merged without inventing new claims", opts.KeepRecent+1, len(events)),
	}
	if err := rewriteChronology(path, append([]contextstore.ChronologyEvent{summary}, keep...)); err != nil {
		return false, err
	}
	_, err = contextstore.ParseChronologyFile(path)
	return true, err
}
```

#### FindingCommentsSidecar (type)

```go
type FindingCommentsSidecar struct {
	ByFingerprint map[string]string `json:"by_fingerprint"`
}
```

#### FindingCommentBodiesFile (type)

```go
type FindingCommentBodiesFile struct {
	Comments []FindingCommentBody `json:"comments"`
}
```

#### PRCommentAPI (type)

```go
type PRCommentAPI interface {
	ListPRCommentsWithIDs(prNumber string) ([]PRCommentWithID, error)
	PostPRComment(prNumber, body string) (string, error)
	UpdatePRComment(prNumber, commentID, body string) error
}
```

#### SyncFindingPRComments (func)

```go
func SyncFindingPRComments(api PRCommentAPI, prNumber, ctxDir string) error {
	if api == nil || strings.TrimSpace(prNumber) == "" {
		return nil
	}
	bodies, err := loadFindingCommentBodies(ctxDir)
	if err != nil {
		return err
	}
	sidecar, err := loadFindingCommentsSidecar(ctxDir)
	if err != nil {
		return err
	}
	listed, err := api.ListPRCommentsWithIDs(prNumber)
	if err != nil {
		return fmt.Errorf("list finding comments: %w", err)
	}
	for _, c := range listed {
		fp := parseFindingFingerprint(c.Body)
		if fp == "" {
			continue
		}
		if _, ok := sidecar.ByFingerprint[fp]; !ok {
			sidecar.ByFingerprint[fp] = c.ID
		}
	}

	active := map[string]struct{}{}
	for _, body := range bodies {
		fp := strings.TrimSpace(body.Fingerprint)
		if fp == "" {
			fp = findingFingerprint(body.Finding)
		}
		active[fp] = struct{}{}
		text := formatFindingCommentBody(fp, body.Body)
		id := strings.TrimSpace(sidecar.ByFingerprint[fp])
		if id == "" {
			newID, err := api.PostPRComment(prNumber, text)
			if err != nil {
				return fmt.Errorf("post finding comment %s: %w", fp, err)
			}
			sidecar.ByFingerprint[fp] = newID
			continue
		}
		if err := api.UpdatePRComment(prNumber, id, text); err != nil {
			newID, postErr := api.PostPRComment(prNumber, text)
			if postErr != nil {
				return fmt.Errorf("update finding comment %s: %v; recreate: %w", fp, err, postErr)
			}
			sidecar.ByFingerprint[fp] = newID
		}
	}

	for fp, id := range sidecar.ByFingerprint {
		if _, ok := active[fp]; ok {
			continue
		}
		cleared := formatFindingCommentBody(fp, "Cleared by digest: this architecture finding is no longer open.")
		if strings.TrimSpace(id) == "" {
			continue
		}
		if err := api.UpdatePRComment(prNumber, id, cleared); err != nil {
			logf("WARN", "clear finding comment %s: %v", fp, err)
		}
	}
	return saveFindingCommentsSidecar(ctxDir, sidecar)
}
```

#### Forge (type)

```go
type Forge struct {
	SCM     string
	RepoID  string
	Owner   string
	Name    string
	Token   string
	BaseURL string
	Runner  publish.CLIRunner
	Client  *http.Client
}
```

#### Forge.OpenUpdatePR (method)

```go
func (f *Forge) OpenUpdatePR(baseBranch, headBranch, title, body string) (string, error) {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.openGitHub(baseBranch, headBranch, title, body)
	case "gitlab":
		return f.openGitLab(baseBranch, headBranch, title, body)
	case "bitbucket":
		return f.openBitbucket(baseBranch, headBranch, title, body)
	default:
		return "", fmt.Errorf("unsupported scm %q for context PR", scm)
	}
}
```

#### Forge.WriteTipBranch (method)

```go
func (f *Forge) WriteTipBranch(baseBranch, headBranch string) (string, bool, error) {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		n, err := f.findGitHubOpen(baseBranch, headBranch)
		return headBranch, n != "", err
	case "gitlab":
		env := f.glabEnv()
		repoArgs := glabRepoArgs(f.Owner, f.Name)
		n, err := f.findGitLabOpen(baseBranch, headBranch, env, repoArgs)
		return headBranch, n != "", err
	case "bitbucket":
		n, err := f.findBitbucketOpen(baseBranch, headBranch)
		return headBranch, n != "", err
	default:
		return "", false, fmt.Errorf("unsupported scm %q", scm)
	}
}
```

#### Forge.ListPRComments (method)

```go
func (f *Forge) ListPRComments(prNumber string) ([]contextgate.Comment, error) {
	withIDs, err := f.ListPRCommentsWithIDs(prNumber)
	if err != nil {
		return nil, err
	}
	out := make([]contextgate.Comment, 0, len(withIDs))
	for _, c := range withIDs {
		out = append(out, contextgate.Comment{Body: c.Body, Author: c.Author, PostedAt: c.PostedAt})
	}
	return out, nil
}
```

#### Forge.MergeUpdatePR (method)

```go
func (f *Forge) MergeUpdatePR(prNumber string) error {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		env := f.ghEnv()
		_, err := f.runCLI("gh", []string{"pr", "merge", prNumber, "--merge", "-R", f.repoSlug()}, env)
		return err
	case "gitlab":
		env := f.glabEnv()
		args := append([]string{"mr", "merge", prNumber}, glabRepoArgs(f.Owner, f.Name)...)
		_, err := f.runCLI("glab", args, env)
		return err
	default:
		return fmt.Errorf("autoMerge not supported for scm %q", scm)
	}
}
```

#### PRCommentWithID (type)

```go
type PRCommentWithID struct {
	ID       string
	Body     string
	Author   string
	PostedAt string
}
```

#### Forge.ListPRCommentsWithIDs (method)

```go
func (f *Forge) ListPRCommentsWithIDs(prNumber string) ([]PRCommentWithID, error) {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.listGitHubCommentsWithIDs(prNumber)
	case "gitlab":
		return f.listGitLabCommentsWithIDs(prNumber)
	case "bitbucket":
		return f.listBitbucketCommentsWithIDs(prNumber)
	default:
		return nil, fmt.Errorf("unsupported scm %q for comments", scm)
	}
}
```

#### Forge.PostPRComment (method)

```go
func (f *Forge) PostPRComment(prNumber, body string) (string, error) {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.postGitHubComment(prNumber, body)
	case "gitlab":
		return f.postGitLabComment(prNumber, body)
	case "bitbucket":
		return f.postBitbucketComment(prNumber, body)
	default:
		return "", fmt.Errorf("unsupported scm %q for post comment", scm)
	}
}
```

#### Forge.UpdatePRComment (method)

```go
func (f *Forge) UpdatePRComment(prNumber, commentID, body string) error {
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.updateGitHubComment(commentID, body)
	case "gitlab":
		return f.updateGitLabComment(prNumber, commentID, body)
	case "bitbucket":
		return f.updateBitbucketComment(prNumber, commentID, body)
	default:
		return fmt.Errorf("unsupported scm %q for update comment", scm)
	}
}
```

#### Git (type)

```go
type Git struct {
	Dir   string
	Token string
	SCM   string // github|gitlab|bitbucket
}
```

#### ResolveDefaultBranch (func)

```go
func ResolveDefaultBranch(g *Git) (string, error) {
	out, code := g.runAllowFail("symbolic-ref", "refs/remotes/origin/HEAD")
	if code == 0 {
		ref := strings.TrimSpace(out)
		const prefix = "refs/remotes/origin/"
		if strings.HasPrefix(ref, prefix) {
			return strings.TrimPrefix(ref, prefix), nil
		}
	}
	for _, name := range []string{"main", "master"} {
		_, code := g.runAllowFail("rev-parse", "--verify", "origin/"+name)
		if code == 0 {
			return name, nil
		}
	}
	return "", fmt.Errorf("cannot resolve default branch (set origin/HEAD or main/master)")
}
```

#### RemoteBranchExists (func)

```go
func RemoteBranchExists(g *Git, branch string) (bool, error) {
	out, err := g.trim("ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}
```

#### LocalBranchExists (func)

```go
func LocalBranchExists(g *Git, branch string) bool {
	_, code := g.runAllowFail("show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return code == 0
}
```

#### FetchOrigin (func)

```go
func FetchOrigin(g *Git, refspecs ...string) error {
	args := append([]string{"fetch", "origin"}, refspecs...)
	_, err := g.run(args...)
	return err
}
```

#### DeepenDefaultBranch (func)

```go
func DeepenDefaultBranch(g *Git, defaultBranch string, depth int) error {
	if depth <= 0 {
		depth = 500
	}
	_, err := g.run("fetch", "--deepen="+fmt.Sprint(depth), "origin", defaultBranch)
	return err
}
```

#### IsAncestor (func)

```go
func IsAncestor(g *Git, ancestor, desc string) (bool, error) {
	if strings.TrimSpace(ancestor) == "" {
		return false, nil
	}
	_, code := g.runAllowFail("merge-base", "--is-ancestor", ancestor, desc)
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("merge-base --is-ancestor %s %s failed", ancestor, desc)
	}
}
```

#### EnsureAncestor (func)

```go
func EnsureAncestor(g *Git, ancestor, desc, defaultBranch string) error {
	ok, err := IsAncestor(g, ancestor, desc)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if err := DeepenDefaultBranch(g, defaultBranch, 500); err != nil {
		return fmt.Errorf("cursor %s not ancestor of %s (deepen: %w)", ancestor, desc, err)
	}
	ok, err = IsAncestor(g, ancestor, desc)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("cursor %s is not an ancestor of %s", ancestor, desc)
	}
	return nil
}
```

#### FirstParentCommits (func)

```go
func FirstParentCommits(g *Git, fromExclusive, toInclusive, defaultBranch string) ([]string, error) {
	if fromExclusive == toInclusive {
		return nil, nil
	}
	if err := EnsureAncestor(g, fromExclusive, toInclusive, defaultBranch); err != nil {
		return nil, err
	}
	rangeSpec := fromExclusive + ".." + toInclusive
	out, err := g.trim("rev-list", "--first-parent", "--reverse", rangeSpec)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Fields(out), nil
}
```

#### CheckoutBranch (func)

```go
func CheckoutBranch(g *Git, branch string) error {
	_, err := g.run("checkout", branch)
	return err
}
```

#### CheckoutOrCreate (func)

```go
func CheckoutOrCreate(g *Git, branch, base string) error {
	if LocalBranchExists(g, branch) {
		return CheckoutBranch(g, branch)
	}
	_, err := g.run("checkout", "-B", branch, base)
	return err
}
```

#### CheckoutUpdateBranch (func)

```go
func CheckoutUpdateBranch(g *Git, baseBranch, updateBranch string) error {
	if LocalBranchExists(g, updateBranch) {
		return CheckoutBranch(g, updateBranch)
	}
	exists, err := RemoteBranchExists(g, updateBranch)
	if err != nil {
		return err
	}
	if exists {
		if err := FetchOrigin(g, updateBranch+":"+updateBranch); err != nil {
			return fmt.Errorf("fetch update branch %s: %w", updateBranch, err)
		}
		return CheckoutBranch(g, updateBranch)
	}
	if err := CheckoutBranch(g, baseBranch); err != nil {
		return err
	}
	return CheckoutOrCreate(g, updateBranch, baseBranch)
}
```

#### CommitAll (func)

```go
func CommitAll(g *Git, message string) (bool, error) {
	if _, err := g.run("add", "-A"); err != nil {
		return false, err
	}
	status, err := g.trim("status", "--porcelain")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(status) == "" {
		return false, nil
	}
	_, err = g.run("commit", "-m", message)
	return true, err
}
```

#### Push (func)

```go
func Push(g *Git, branch string) error {
	ref := "refs/heads/" + branch
	_, err := g.run("push", "origin", ref+":"+ref)
	return err
}
```

#### PushForce (func)

```go
func PushForce(g *Git, branch string) error {
	ref := "refs/heads/" + branch
	_, err := g.run("push", "--force", "origin", ref+":"+ref)
	return err
}
```

#### InitOrphan (func)

```go
func InitOrphan(dir, branch string) (*Git, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	g := &Git{Dir: dir}
	if _, err := g.run("init"); err != nil {
		return nil, err
	}
	if _, err := g.run("checkout", "--orphan", branch); err != nil {
		return nil, err
	}
	return g, nil
}
```

#### RepoTarget (type)

```go
type RepoTarget struct {
	RepoID   string `json:"repo_id"`
	SCM      string `json:"scm"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	CloneURL string `json:"clone_url"`
}
```

#### ReposResult (type)

```go
type ReposResult struct {
	Repos []RepoTarget `json:"repos"`
}
```

#### ListTargets (func)

```go
func ListTargets(configDir string) (ReposResult, error) {
	configs, err := config.LoadAll(configDir)
	if err != nil {
		return ReposResult{}, err
	}
	out := ReposResult{Repos: make([]RepoTarget, 0, len(configs))}
	for _, cfg := range configs {
		scm := strings.ToLower(strings.TrimSpace(cfg.SCM))
		if scm == "" {
			scm = "github"
		}
		if scm == "generic" {
			continue
		}
		cloneURL := strings.TrimSpace(cfg.Repository.CloneURL)
		if cloneURL == "" {
			continue
		}
		repoID := cfg.Repository.ID
		if repoID == "" || isPlaceholderRepo(repoID, cfg) {
			continue
		}
		owner, name := cfg.Repository.Owner, cfg.Repository.Name
		if owner == "" || name == "" {
			owner, name = splitOwnerName(cloneURL)
		}
		if isPlaceholderOwner(owner) {
			continue
		}
		out.Repos = append(out.Repos, RepoTarget{
			RepoID:   repoID,
			SCM:      scm,
			Owner:    owner,
			Name:     name,
			CloneURL: cloneURL,
		})
	}
	return out, nil
}
```

#### LocalSeedManifest (type)

```go
type LocalSeedManifest struct {
	SchemaVersion   int    `yaml:"schema_version"`
	RepoID          string `yaml:"repo_id"`
	SourceSHA       string `yaml:"source_sha"`
	ModuleScope     string `yaml:"module_scope,omitempty"`
	TypologyVersion string `yaml:"typology_version,omitempty"`
	CompletedStage  string `yaml:"completed_stage,omitempty"`
	CurrentStage    string `yaml:"current_stage,omitempty"`
	CreatedAt       string `yaml:"created_at"`
	UpdatedAt       string `yaml:"updated_at"`
	LastError       string `yaml:"last_error,omitempty"`
}
```

#### LocalSeedWorkspace (type)

```go
type LocalSeedWorkspace struct {
	Root     string
	Manifest LocalSeedManifest
	lockPath string
	locked   bool
}
```

#### OpenLocalSeedWorkspace (func)

```go
func OpenLocalSeedWorkspace(root, repoID, sourceSHA, moduleScope string, allowSourceMove bool, now time.Time) (*LocalSeedWorkspace, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("local seed dir is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("local seed mkdir: %w", err)
	}
	ws := &LocalSeedWorkspace{Root: root, lockPath: filepath.Join(root, localWorkspaceLock)}
	if err := ws.acquireLock(now); err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(root, localWorkspaceManifest)
	_, err := os.Stat(manifestPath)
	switch {
	case os.IsNotExist(err):
		ws.Manifest = LocalSeedManifest{
			SchemaVersion: localSeedSchemaVersion,
			RepoID:        repoID,
			SourceSHA:     sourceSHA,
			ModuleScope:   moduleScope,
			CreatedAt:     now.UTC().Format(time.RFC3339),
			UpdatedAt:     now.UTC().Format(time.RFC3339),
		}
		if err := ws.ensureLayout(); err != nil {
			_ = ws.Release()
			return nil, err
		}
		if err := ws.writeManifest(); err != nil {
			_ = ws.Release()
			return nil, err
		}
		return ws, nil
	case err != nil:
		_ = ws.Release()
		return nil, fmt.Errorf("stat workspace.yaml: %w", err)
	}
	if err := ws.loadManifest(); err != nil {
		_ = ws.Release()
		return nil, err
	}
	if err := ws.validateIdentity(repoID, sourceSHA, moduleScope, allowSourceMove); err != nil {
		_ = ws.Release()
		return nil, err
	}
	if err := ws.ensureLayout(); err != nil {
		_ = ws.Release()
		return nil, err
	}
	return ws, nil
}
```

#### LocalSeedWorkspace.ContextDir (method)

```go
func (ws *LocalSeedWorkspace) ContextDir() string {
	return filepath.Join(ws.Root, localContextRel)
}
```

#### LocalSeedWorkspace.AnalysisDir (method)

```go
func (ws *LocalSeedWorkspace) AnalysisDir() string {
	return filepath.Join(ws.Root, localAnalysisRel)
}
```

#### LocalSeedWorkspace.CacheDir (method)

```go
func (ws *LocalSeedWorkspace) CacheDir() string {
	return filepath.Join(ws.Root, localCacheRel)
}
```

#### LocalSeedWorkspace.WorkStoryDir (method)

```go
func (ws *LocalSeedWorkspace) WorkStoryDir() string {
	return filepath.Join(ws.Root, localWorkStoryRel)
}
```

#### LocalSeedWorkspace.DiffPath (method)

```go
func (ws *LocalSeedWorkspace) DiffPath() string {
	return filepath.Join(ws.Root, localDiffRel)
}
```

#### LocalSeedWorkspace.IsNew (method)

```go
func (ws *LocalSeedWorkspace) IsNew() bool {
	return strings.TrimSpace(ws.Manifest.CompletedStage) == ""
}
```

#### LocalSeedWorkspace.BeginStage (method)

```go
func (ws *LocalSeedWorkspace) BeginStage(stage string, now time.Time) error {
	ws.Manifest.CurrentStage = stage
	ws.Manifest.LastError = ""
	ws.Manifest.UpdatedAt = now.UTC().Format(time.RFC3339)
	return ws.writeManifest()
}
```

#### LocalSeedWorkspace.SaveCheckpoint (method)

```go
func (ws *LocalSeedWorkspace) SaveCheckpoint(stage string, now time.Time) error {
	ws.Manifest.CompletedStage = stage
	ws.Manifest.CurrentStage = ""
	ws.Manifest.LastError = ""
	ws.Manifest.UpdatedAt = now.UTC().Format(time.RFC3339)
	return ws.writeManifest()
}
```

#### LocalSeedWorkspace.RecordFailure (method)

```go
func (ws *LocalSeedWorkspace) RecordFailure(stage string, stageErr error, now time.Time) error {
	ws.Manifest.CurrentStage = stage
	ws.Manifest.LastError = stageErr.Error()
	ws.Manifest.UpdatedAt = now.UTC().Format(time.RFC3339)
	return ws.writeManifest()
}
```

#### LocalSeedWorkspace.PersistAnalysisDrafts (method)

```go
func (ws *LocalSeedWorkspace) PersistAnalysisDrafts(analysisDir string) error {
	dstRoot := ws.AnalysisDir()
	if err := os.MkdirAll(filepath.Join(dstRoot, "tmp", "typology"), 0o755); err != nil {
		return err
	}
	for _, rel := range []string{analysisDraftCatalogRel, analysisDraftArchRel} {
		src := filepath.Join(analysisDir, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(dstRoot, rel)); err != nil {
			return fmt.Errorf("persist analysis %s: %w", rel, err)
		}
	}
	readme := filepath.Join(analysisDir, "README.md")
	if _, err := os.Stat(readme); err == nil {
		if err := copyFile(readme, filepath.Join(dstRoot, "README.md")); err != nil {
			return fmt.Errorf("persist analysis README: %w", err)
		}
	}
	return nil
}
```

#### LocalSeedWorkspace.RestoreAnalysisDrafts (method)

```go
func (ws *LocalSeedWorkspace) RestoreAnalysisDrafts(analysisDir string) error {
	srcRoot := ws.AnalysisDir()
	for _, rel := range []string{analysisDraftCatalogRel, analysisDraftArchRel, "README.md"} {
		src := filepath.Join(srcRoot, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(analysisDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("restore analysis %s: %w", rel, err)
		}
	}
	return nil
}
```

#### LocalSeedWorkspace.WriteLocalDiff (method)

```go
func (ws *LocalSeedWorkspace) WriteLocalDiff(content string) error {
	return writeFileAtomic(ws.DiffPath(), []byte(content))
}
```

#### LocalSeedWorkspace.Release (method)

```go
func (ws *LocalSeedWorkspace) Release() error {
	if ws == nil || !ws.locked {
		return nil
	}
	ws.locked = false
	return os.Remove(ws.lockPath)
}
```

#### UpdateMeta (func)

```go
func UpdateMeta(dir, cursorSHA string, at time.Time) error {
	path := filepath.Join(dir, "meta.yaml")
	meta, err := contextstore.ParseMeta(path)
	if err != nil {
		return err
	}
	meta.LastMergedSHA = strings.TrimSpace(cursorSHA)
	meta.LastDigestAt = at.UTC().Format(time.RFC3339)
	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta.yaml: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write meta.yaml: %w", err)
	}
	return contextstore.ValidateTree(dir)
}
```

#### ReadCursor (func)

```go
func ReadCursor(dir string) (string, error) {
	meta, err := contextstore.ParseMeta(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(meta.LastMergedSHA), nil
}
```

#### NeedsMetaUpdate (func)

```go
func NeedsMetaUpdate(dir, cursorSHA string) (bool, error) {
	cur, err := ReadCursor(dir)
	if err != nil {
		return true, err
	}
	return cur != strings.TrimSpace(cursorSHA), nil
}
```

#### IsBehind (func)

```go
func IsBehind(g *Git, cursor, head, defaultBranch string) (bool, error) {
	cursor = strings.TrimSpace(cursor)
	head = strings.TrimSpace(head)
	if cursor == head {
		return false, nil
	}
	if cursor == "" {
		return head != "", nil
	}
	if err := EnsureAncestor(g, cursor, head, defaultBranch); err != nil {
		return false, err
	}
	return true, nil
}
```

#### stropSliceObjectiveLedgerRLM.BuildSliceLedger (method)

```go
func (v stropSliceObjectiveLedgerRLM) BuildSliceLedger(ctx context.Context, req sliceLedgerBuildRequest) (sliceObjectiveLedgerDoc, []string, error) {
	return buildSliceObjectiveLedger(ctx, v, req)
}
```

#### stropSliceObjectiveLedgerRLM.Complete (method)

```go
func (v stropSliceObjectiveLedgerRLM) Complete(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	return v.module.Complete(ctx, contextPayload, query)
}
```

#### stubSliceLedgerCaller.Complete (method)

```go
func (s stubSliceLedgerCaller) Complete(_ context.Context, _ any, query string) (string, int, int, int, int, error) {
	if strings.Contains(query, "extract grounding evidence for one package") {
		return "evidence:\n  - StubSymbol\nnotes: stub package notes\n", 1, 0, 0, 0, s.err
	}
	return s.answer, 1, 0, 0, 0, s.err
}
```

#### ledgerGroundingIssue.Error (method)

```go
func (e ledgerGroundingIssue) Error() string { return e.msg }
```

#### ledgerStepRunner.RunStep (method)

```go
func (r *ledgerStepRunner) RunStep(ctx context.Context, plan *stepplan.Plan, step stepplan.Step) (*orchestration.StepRunResult, error) {
	if r == nil || r.caller == nil {
		return nil, fmt.Errorf("ledger step runner is nil")
	}
	if step.ID == ledgerSynthesisStepID {
		return r.runSynthesis(ctx, plan, step)
	}
	return r.runEvidence(ctx, step)
}
```

#### ResumeProvenance (type)

```go
type ResumeProvenance struct {
	ResumePR  int    `json:"resume_pr"`
	HeadSHA   string `json:"head_sha"`
	FromStage string `json:"from_stage"`
	RepoID    string `json:"repo_id"`
	At        string `json:"at"`
}
```

#### PRHead (type)

```go
type PRHead struct {
	SHA     string // commit SHA
	RefName string // head branch name when known (bitbucket/gitlab fetch)
}
```

#### Forge.ResolvePRHead (method)

```go
func (f *Forge) ResolvePRHead(prNumber string) (PRHead, error) {
	prNumber = strings.TrimSpace(prNumber)
	if prNumber == "" {
		return PRHead{}, fmt.Errorf("PR number is required")
	}
	scm := strings.ToLower(strings.TrimSpace(f.SCM))
	switch scm {
	case "github":
		return f.resolveGitHubPRHead(prNumber)
	case "gitlab":
		return f.resolveGitLabPRHead(prNumber)
	case "bitbucket":
		return f.resolveBitbucketPRHead(prNumber)
	default:
		return PRHead{}, fmt.Errorf("unsupported scm %q for resume PR head", scm)
	}
}
```

#### RewriteInfo (type)

```go
type RewriteInfo struct {
	CursorBefore string
	NewHead      string
	Why          string
	Pending      bool
}
```

#### DetectRewrite (func)

```go
func DetectRewrite(g *Git, cursor, head, defaultBranch string) (bool, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" || cursor == strings.TrimSpace(head) {
		return false, nil
	}
	ok, err := IsAncestor(g, cursor, head)
	if err != nil {
		return false, err
	}
	return !ok, nil
}
```

#### BeginRewrite (func)

```go
func BeginRewrite(ctxDir string, newHead string, at time.Time, actor, evidence string) (RewriteInfo, error) {
	meta, err := readRewriteMeta(ctxDir)
	if err != nil {
		return RewriteInfo{}, err
	}
	meta.RewritePending = true
	meta.RewriteDetectedAt = at.UTC().Format(time.RFC3339)
	meta.RewriteNewHead = strings.TrimSpace(newHead)
	if strings.TrimSpace(evidence) == "" {
		evidence = "default branch history no longer contains cursor " + meta.LastMergedSHA
	}
	ev := contextstore.ChronologyEvent{
		Date:      at,
		Actor:     actor,
		Source:    "history-rewrite",
		Did:       "detected default-branch history rewrite",
		Because:   "last_merged_sha is not an ancestor of current default HEAD",
		InOrderTo: "reshape the teaching story for the new tape",
		Evidence:  evidence,
	}
	if err := contextstore.AppendChronologyEvent(ctxDir, ev); err != nil {
		return RewriteInfo{}, err
	}
	if err := writeMeta(ctxDir, meta); err != nil {
		return RewriteInfo{}, err
	}
	return RewriteInfo{
		CursorBefore: meta.LastMergedSHA,
		NewHead:      newHead,
		Pending:      true,
	}, nil
}
```

#### ApplyRewriteWhy (func)

```go
func ApplyRewriteWhy(ctxDir, why string) error {
	meta, err := readRewriteMeta(ctxDir)
	if err != nil {
		return err
	}
	meta.RewriteWhy = strings.TrimSpace(why)
	if meta.RewriteWhy != "" {
		meta.RewritePending = false
	}
	return writeMeta(ctxDir, meta)
}
```

#### CompleteRewrite (func)

```go
func CompleteRewrite(ctxDir, newHead string, at time.Time) error {
	meta, err := readRewriteMeta(ctxDir)
	if err != nil {
		return err
	}
	if meta.RewritePending && strings.TrimSpace(meta.RewriteWhy) == "" {
		return fmt.Errorf("rewrite blocked: why is required (@majordomo why … on context PR)")
	}
	meta.LastMergedSHA = strings.TrimSpace(newHead)
	meta.LastDigestAt = at.UTC().Format(time.RFC3339)
	meta.RewritePending = false
	meta.RewriteDetectedAt = ""
	meta.RewriteNewHead = ""
	meta.RewriteWhy = ""
	return writeMeta(ctxDir, meta)
}
```

#### rlmCompleteAdapter.Complete (method)

```go
func (a rlmCompleteAdapter) Complete(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	return a.complete(ctx, contextPayload, query)
}
```

#### stropPackageRoleRLM.Validate (method)

```go
func (v stropPackageRoleRLM) Validate(ctx context.Context, pkgPath, contextMD, mechanicalRole, mechanicalEvidence string) (string, string, int, int, int, int, error) {
	query := fmt.Sprintf(`Classify this Go package into exactly one role.
Allowed roles: entrypoint, server, dto, exec_runner, aggregator, adapter, config, observability, unknown.
Mechanical prior: role=%s evidence=%s
MUST NOT use the directory basename as evidence.
Explore exports and bodies in the context; dig into private helpers only if needed.
Final answer MUST be a small YAML object (no markdown fences):
role: <one allowed role>
evidence: <short symbol quotes>
Package path (not evidence): %s`, mechanicalRole, mechanicalEvidence, pkgPath)
	answer, iters, prompt, completion, total, err := v.module.Complete(ctx, contextMD, query)
	if err != nil {
		return "", "", iters, prompt, completion, total, err
	}
	role, evidence := parseRLMRoleAnswer(answer)
	return role, evidence, iters, prompt, completion, total, nil
}
```

#### Result (type)

```go
type Result struct {
	Action        string            `json:"action"` // noop | skipped | seed | catchup | rewrite | rewrite_blocked | gate_regen | resume | local
	DefaultBranch string            `json:"default_branch,omitempty"`
	DefaultHEAD   string            `json:"default_head,omitempty"`
	CursorBefore  string            `json:"cursor_before,omitempty"`
	CursorAfter   string            `json:"cursor_after,omitempty"`
	CommitsWalked int               `json:"commits_walked,omitempty"`
	ContextPR     string            `json:"context_pr,omitempty"`
	TypologyPR    string            `json:"typology_pr,omitempty"` // product PR promoting refined catalog into .typology/
	GateStatus    string            `json:"gate_status,omitempty"`
	Message       string            `json:"message,omitempty"`
	WorkStoryDir  string            `json:"work_story_dir,omitempty"` // durable RLM + runreport dump (local AI testing)
	TraceID       string            `json:"trace_id,omitempty"`
	LLMUsage      *llmusage.Summary `json:"llm_usage,omitempty"`
	// Resume provenance (PR-seeded stage replay; teaching outputs local-only).
	ResumePR    int    `json:"resume_pr,omitempty"`
	ResumeHead  string `json:"resume_head,omitempty"`
	FromStage   string `json:"from_stage,omitempty"`
	LocalOutDir string `json:"local_out_dir,omitempty"` // work-story local-context dump or local seed context/
	// Local seed workspace provenance (filesystem-only; no forge push).
	LocalSeedDir   string `json:"local_seed_dir,omitempty"`
	CacheMode      string `json:"cache_mode,omitempty"` // remote | local
	SourceSHA      string `json:"source_sha,omitempty"`
	CompletedStage string `json:"completed_stage,omitempty"`
}
```

#### Options (type)

```go
type Options struct {
	ConfigDir                  string
	RepoID                     string
	WorkDir                    string // served-repo clone with origin remote
	Now                        time.Time
	Forge                      *Forge // optional inject for tests
	TypologyBinary             string
	ModuleScope                string
	BootstrapSurveyRunner      BootstrapSurveyRunner
	BootstrapStoryGenerator    BootstrapStoryGenerator
	TypologySlicePipeline      TypologySlicePipeline
	HumanInterventionGenerator HumanInterventionGenerator
	BootstrapSurveyPolicy      string
	Judge                      judge.Generator // optional; built from config when nil
	SkipStory                  bool
	SkipCompact                bool
	ForceCompact               bool
	DigestCache                *cache.DigestStore // optional; inspect/ledger fingerprint skips
	DigestSkips                bool               // when true with DigestCache, skip LLM on hit
	DigestModelID              string             // model id for fingerprints
	// WorkStoryDir is a durable local dump for RLM JSONL, module TraceSession JSONL, and runreports (AI testing).
	// When empty, Run creates tmp/digest-runs/<repo-id>-<timestamp> (or MAJORDOMO_DIGEST_WORK_STORY_DIR).
	// Not the teaching context branch; not deleted with the analysis clone.
	WorkStoryDir string
	// Context nests OTEL chain spans, module TraceSession, and runreport into digest Judge/RLM work.
	// When nil, Background is used.
	Context context.Context
	// ResumePR + FromStage enable PR-seeded stage replay: load that context PR head into a
	// temp ctx dir, re-run only the requested stages, keep outputs local (no context push/PR).
	ResumePR  int    // served-repo context PR/MR number; 0 = disabled
	FromStage string // catalog|intervention|story (also survey for local seed)
	// LocalSeedDir enables filesystem-only seeding under this directory (no forge token / push).
	LocalSeedDir string
	// AllowSourceMove retargets an existing local workspace when workdir HEAD moved.
	AllowSourceMove bool
}
```

#### Run (func)

```go
func Run(opts Options) (res Result, err error) {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	opts.Context = ctx
	if tid := observability.TraceIDFromContext(ctx); tid != "" {
		logf("INFO", "repo=%s otel trace_id=%s", opts.RepoID, tid)
	}
	usage := llmusage.New()
	llmusage.Push(usage)
	defer func() {
		if tid := observability.TraceIDFromContext(ctx); tid != "" {
			res.TraceID = tid
		}
		snap := usage.Snapshot()
		res.LLMUsage = &snap
		logf("INFO", "repo=%s LLM usage summary", opts.RepoID)
		for _, line := range strings.Split(llmusage.Format(snap), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			logf("INFO", "%s", line)
		}
		llmusage.Pop()
	}()

	if opts.ConfigDir == "" || opts.RepoID == "" {
		return Result{}, fmt.Errorf("--config-dir and --repo-id required")
	}
	if opts.WorkDir == "" {
		return Result{}, fmt.Errorf("--workdir required (served-repo clone with origin)")
	}
	if err := validateDigestModeOptions(opts); err != nil {
		return Result{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	defaults, err := config.LoadDefaults(opts.ConfigDir)
	if err != nil {
		return Result{}, err
	}
	cfg, err := config.LoadRepoFile(opts.ConfigDir, opts.RepoID, defaults)
	if err != nil {
		return Result{}, err
	}

	if isLocalSeedMode(opts) {
		return runLocalSeed(opts, cfg, now)
	}

	scm := strings.ToLower(strings.TrimSpace(cfg.SCM))
	if scm == "" {
		scm = "github"
	}
	if scm == "generic" {
		logf("INFO", "scm=generic: skipping context digest for %s", opts.RepoID)
		return Result{Action: "skipped", Message: "generic scm skips context PR"}, nil
	}

	owner, name := cfg.Repository.Owner, cfg.Repository.Name
	if owner == "" || name == "" {
		owner, name = splitOwnerName(cfg.Repository.CloneURL)
	}
	token := resolveToken(cfg, scm, owner)
	if token == "" {
		return Result{}, fmt.Errorf("forge token required (%s)", config.CredentialHint(cfg.Repository.ID, scm, owner))
	}

	baseBranch := cfg.Context.Branch
	if baseBranch == "" {
		baseBranch = config.ContextBranch(cfg.Repository.ID)
	}
	updateBranch := config.ContextUpdateBranch(cfg.Repository.ID)
	if err := contextstore.ValidateContextBranch(baseBranch); err != nil {
		return Result{}, err
	}

	servedGit := &Git{Dir: opts.WorkDir, Token: token, SCM: scm}
	if err := FetchOrigin(servedGit); err != nil {
		return Result{}, fmt.Errorf("fetch served repo: %w", err)
	}

	digestBranch := config.DigestCacheBranch(cfg.Repository.ID)
	digestDir, err := os.MkdirTemp("", "majordomo-digest-cache-*")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(digestDir) }()
	if err := materializeDigestCacheWorktree(digestDir, servedGit, digestBranch, token, scm); err != nil {
		logf("WARN", "digest inference cache unavailable: %v", err)
	} else {
		store := &cache.DigestStore{Dir: digestDir}
		if remote, rerr := servedGit.trim("remote", "get-url", "origin"); rerr != nil {
			logf("WARN", "digest inference cache push disabled: remote URL: %v", rerr)
		} else {
			store.ConfigurePush(cache.DigestPushOptions{
				Remote:   remote,
				Branch:   digestBranch,
				Worktree: digestDir,
				Token:    token,
				SCM:      scm,
			})
			store.OnFlushError = func(err error) {
				logf("WARN", "digest inference cache push: %v", err)
			}
		}
		opts.DigestCache = store
		opts.DigestSkips = cfg.Cache.SkipsEnabled()
		opts.DigestModelID = digestModelID(cfg)
		logf("INFO", "digest inference cache ready branch=%s skips=%v model=%s", digestBranch, opts.DigestSkips, opts.DigestModelID)
		defer func() {
			if ferr := store.Flush(); ferr != nil {
				logf("WARN", "digest inference cache final flush: %v", ferr)
			}
			logf("INFO", "%s", cache.FormatStatsLine(store.Stats()))
		}()
	}

	defaultBranch, err := ResolveDefaultBranch(servedGit)
	if err != nil {
		return Result{}, err
	}
	defaultHEAD, err := servedGit.trim("rev-parse", "origin/"+defaultBranch)
	if err != nil {
		return Result{}, fmt.Errorf("resolve default HEAD: %w", err)
	}

	forg
// ... truncated
```

#### stropSliceCatalogRLM.AssembleSlices (method)

```go
func (v stropSliceCatalogRLM) AssembleSlices(ctx context.Context, req sliceCatalogAssembleRequest) (sliceCatalogAssembleResult, error) {
	return assembleSliceCatalogFragments(ctx, v, req)
}
```

#### stropSliceCatalogRLM.Complete (method)

```go
func (v stropSliceCatalogRLM) Complete(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	return v.module.Complete(ctx, contextPayload, query)
}
```

#### CommitContext (type)

```go
type CommitContext struct {
	SHA     string
	Subject string
	Body    string
	Diff    string
	Files   []string
}
```

#### LoadCommitContext (func)

```go
func LoadCommitContext(g *Git, sha string) (CommitContext, error) {
	subject, err := g.trim("show", "-s", "--format=%s", sha)
	if err != nil {
		return CommitContext{}, err
	}
	body, err := g.trim("show", "-s", "--format=%b", sha)
	if err != nil {
		return CommitContext{}, err
	}
	diff, err := g.trim("show", "--format=", "--no-color", "-U3", sha)
	if err != nil {
		return CommitContext{}, err
	}
	lines := strings.Split(diff, "\n")
	if len(lines) > maxDiffLines {
		diff = strings.Join(lines[:maxDiffLines], "\n") + fmt.Sprintf("\n[... %d lines omitted — digest diff cap]\n", len(lines)-maxDiffLines)
	}
	namesOut, err := g.trim("show", "--name-only", "--format=", sha)
	if err != nil {
		return CommitContext{}, err
	}
	var files []string
	for _, f := range strings.Split(namesOut, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, f)
		}
	}
	return CommitContext{SHA: sha, Subject: subject, Body: body, Diff: diff, Files: files}, nil
}
```

#### ProcessCommit (func)

```go
func ProcessCommit(ctxDir string, cc CommitContext, at time.Time) error {
	if !evidenceForChronology(cc) {
		return nil
	}
	ev := contextstore.ChronologyEvent{
		Date:      at,
		Actor:     "majordomo",
		Source:    shortSHA(cc.SHA),
		Did:       cc.Subject,
		Because:   firstLine(cc.Body),
		InOrderTo: "advance context cursor on default first-parent tape",
		Evidence:  "commit " + cc.SHA + "; files: " + strings.Join(cc.Files, ", "),
	}
	if strings.TrimSpace(ev.Because) == "" {
		ev.Because = "shown in commit diff on default branch"
	}
	return contextstore.AppendChronologyEvent(ctxDir, ev)
}
```

#### WalkCommits (func)

```go
func WalkCommits(ctxDir string, g *Git, commits []string, at time.Time, regenFeedback string) error {
	_ = regenFeedback
	for _, sha := range commits {
		cc, err := LoadCommitContext(g, sha)
		if err != nil {
			return fmt.Errorf("commit %s: %w", sha, err)
		}
		if err := ProcessCommit(ctxDir, cc, at); err != nil {
			return err
		}
		if err := touchStorySections(ctxDir, cc); err != nil {
			return err
		}
	}
	return nil
}
```

#### ReshapeStory (func)

```go
func ReshapeStory(ctxDir, newHead, why string) error {
	note := fmt.Sprintf("# Architecture\n\nReshaped after history rewrite (HEAD `%s`).\n\n**Why:** %s\n\nRe-read the codebase on default; prior first-parent tape is obsolete.\n",
		shortSHA(newHead), strings.TrimSpace(why))
	if err := os.WriteFile(filepath.Join(ctxDir, "architecture.md"), []byte(ensureStoryArchitectureBanner(note)), 0o644); err != nil {
		return err
	}
	return contextstore.ApplyReadingPath(ctxDir)
}
```

#### digestSectionRunner.Run (method)

```go
func (r *digestSectionRunner) Run(ctx context.Context, req orchestration.SectionFieldRequest, _ streaming.EventChannel) (*orchestration.SectionFieldResponse, error) {
	gen := r.gen
	if gen == nil {
		gen = packageJudgeGenerator{}
	}
	fields := map[string]interface{}{
		"section_id":     req.SectionID,
		"current_text":   req.SourceText,
		"commit_subject": r.cc.Subject,
		"commit_diff":    r.cc.Diff,
		"changed_files":  r.changedFiles,
		"regen_feedback": r.regenFeedback,
	}
	out, err := gen.Generate(ctx, jmodules.TaskDigestStory, fields, req.Version)
	if err != nil {
		return nil, err
	}
	text, ok := out["updated_text"].(string)
	if !ok {
		return nil, fmt.Errorf("digest story generator: output missing string field updated_text")
	}
	if strings.TrimSpace(text) == "" {
		text = req.SourceText
	}
	agg, err := gen.Evaluate(ctx, jmodules.TaskDigestStory, fields, map[string]interface{}{
		"updated_text": text,
	}, req.Version)
	if err != nil {
		return nil, fmt.Errorf("digest story evaluation: %w", err)
	}
	return &orchestration.SectionFieldResponse{
		OutputText: text,
		Rationale:  "digest story generator",
		Eval:       agg,
	}, nil
}
```

### Private one-hop bodies

#### Forge.findBitbucketOpen (method)

```go
func (f *Forge) findBitbucketOpen(baseBranch, headBranch string) (string, error) {
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests?state=OPEN"
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Accept", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read bitbucket PR list response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("bitbucket list PR HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Values []struct {
			ID      int `json:"id"`
			FromRef struct {
				ID string `json:"id"`
			} `json:"fromRef"`
			ToRef struct {
				ID string `json:"id"`
			} `json:"toRef"`
		} `json:"values"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	wantFrom := "refs/heads/" + headBranch
	wantTo := "refs/heads/" + baseBranch
	for _, pr := range out.Values {
		if pr.FromRef.ID == wantFrom && pr.ToRef.ID == wantTo {
			return fmt.Sprint(pr.ID), nil
		}
	}
	return "", nil
}
```

#### Forge.findGitHubOpen (method)

```go
func (f *Forge) findGitHubOpen(baseBranch, headBranch string) (string, error) {
	repo := f.repoSlug()
	env := f.ghEnv()
	head := f.Owner + ":" + headBranch
	args := []string{
		"pr", "list",
		"--state", "open",
		"--base", baseBranch,
		"--head", head,
		"--json", "number",
		"-q", ".[0].number",
		"-R", repo,
	}
	out, err := f.runCLI("gh", args, env)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
```

#### Forge.findGitLabOpen (method)

```go
func (f *Forge) findGitLabOpen(baseBranch, headBranch string, env, repoArgs []string) (string, error) {
	// glab mr list defaults to open MRs; it has no --state flag (unlike gh pr list).
	args := append([]string{
		"mr", "list",
		"--target-branch", baseBranch,
		"--source-branch", headBranch,
		"-F", "json",
	}, repoArgs...)
	out, err := f.runCLI("glab", args, env)
	if err != nil {
		return "", err
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return "", fmt.Errorf("decode glab mr list: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	if iid, ok := rows[0]["iid"]; ok {
		return fmt.Sprint(iid), nil
	}
	return "", nil
}
```

#### Forge.ghEnv (method)

```go
func (f *Forge) ghEnv() []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GH_TOKEN="+f.Token, "GITHUB_TOKEN="+f.Token)
	return env
}
```

#### Forge.glabEnv (method)

```go
func (f *Forge) glabEnv() []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GITLAB_TOKEN="+f.Token, "GLAB_TOKEN="+f.Token)
	return env
}
```

#### Forge.listBitbucketCommentsWithIDs (method)

```go
func (f *Forge) listBitbucketCommentsWithIDs(prNumber string) ([]PRCommentWithID, error) {
	base := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests/" + url.PathEscape(prNumber) + "/activities"
	var comments []PRCommentWithID
	start := 0
	const pageSize = 50
	for {
		api := fmt.Sprintf("%s?start=%d&limit=%d", base, start, pageSize)
		req, err := http.NewRequest(http.MethodGet, api, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+f.Token)
		req.Header.Set("Accept", "application/json")
		resp, err := f.client().Do(req)
		if err != nil {
			return nil, err
		}
		raw, err := readResponseBody(resp)
		if err != nil {
			return nil, fmt.Errorf("read bitbucket activities response: %w", err)
		}
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("bitbucket list activities HTTP %d: %s", resp.StatusCode, string(raw))
		}
		var out struct {
			Values []struct {
				Action      string `json:"action"`
				CreatedDate int64  `json:"createdDate"`
				User        struct {
					Name string `json:"name"`
				} `json:"user"`
				Comment struct {
					ID   int    `json:"id"`
					Text string `json:"text"`
				} `json:"comment"`
			} `json:"values"`
			IsLastPage    bool `json:"isLastPage"`
			NextPageStart *int `json:"nextPageStart"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("decode bitbucket activities: %w", err)
		}
		for _, row := range out.Values {
			if row.Action != "COMMENTED" || strings.TrimSpace(row.Comment.Text) == "" {
				continue
			}
			comments = append(comments, PRCommentWithID{
				ID:       strconv.Itoa(row.Comment.ID),
				Body:     row.Comment.Text,
				Author:   row.User.Name,
				PostedAt: fmt.Sprint(row.CreatedDate),
			})
		}
		if out.IsLastPage || out.NextPageStart == nil {
			break
		}
		start = *out.NextPageStart
	}
	return comments, nil
}
```

#### Forge.listGitHubCommentsWithIDs (method)

```go
func (f *Forge) listGitHubCommentsWithIDs(prNumber string) ([]PRCommentWithID, error) {
	repo := f.repoSlug()
	env := f.ghEnv()
	args := []string{
		"api", "repos/" + repo + "/issues/" + prNumber + "/comments",
		"--jq", ".[] | {id: .id, body: .body, user: .user.login, created_at: .created_at}",
	}
	out, err := f.runCLI("gh", args, env)
	if err != nil {
		return nil, err
	}
	return decodeCommentIDLines(out)
}
```

#### Forge.listGitLabCommentsWithIDs (method)

```go
func (f *Forge) listGitLabCommentsWithIDs(iid string) ([]PRCommentWithID, error) {
	env := f.glabEnv()
	repoArgs := glabRepoArgs(f.Owner, f.Name)
	args := append([]string{
		"api", "projects/" + url.PathEscape(f.Owner+"/"+f.Name) + "/merge_requests/" + iid + "/notes",
	}, repoArgs...)
	out, err := f.runCLI("glab", args, env)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Username string `json:"username"`
		} `json:"author"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, fmt.Errorf("decode gitlab notes: %w", err)
	}
	var comments []PRCommentWithID
	for _, r := range rows {
		comments = append(comments, PRCommentWithID{
			ID: strconv.Itoa(r.ID), Body: r.Body, Author: r.User.Username, PostedAt: r.CreatedAt,
		})
	}
	return comments, nil
}
```

#### Forge.openBitbucket (method)

```go
func (f *Forge) openBitbucket(baseBranch, headBranch, title, body string) (string, error) {
	if f.Token == "" || f.BaseURL == "" || f.Owner == "" || f.Name == "" {
		return "", fmt.Errorf("bitbucket context PR requires BITBUCKET_URL, token, project, repo")
	}
	if existing, err := f.findBitbucketOpen(baseBranch, headBranch); err != nil {
		return "", err
	} else if existing != "" {
		if err := f.updateBitbucketPR(existing, title, body); err != nil {
			return existing, err
		}
		return existing, nil
	}
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests"
	payload := map[string]any{
		"title":       title,
		"description": body,
		"state":       "OPEN",
		"open":        true,
		"closed":      false,
		"fromRef": map[string]any{
			"id": "refs/heads/" + headBranch,
			"repository": map[string]any{
				"slug":    f.Name,
				"project": map[string]string{"key": f.Owner},
			},
		},
		"toRef": map[string]any{
			"id": "refs/heads/" + baseBranch,
			"repository": map[string]any{
				"slug":    f.Name,
				"project": map[string]string{"key": f.Owner},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, api, strings.NewReader(string(raw)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read bitbucket create PR response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("bitbucket create PR HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	if id, ok := out["id"]; ok {
		return fmt.Sprint(id), nil
	}
	return "", nil
}
```

#### Forge.openGitHub (method)

```go
func (f *Forge) openGitHub(baseBranch, headBranch, title, body string) (string, error) {
	if f.Token == "" || f.Owner == "" || f.Name == "" {
		return "", fmt.Errorf("github context PR requires token and owner/name (%s)",
			config.CredentialHint(f.RepoID, "github", f.Owner))
	}
	repo := f.repoSlug()
	env := f.ghEnv()
	if existing, err := f.findGitHubOpen(baseBranch, headBranch); err != nil {
		return "", err
	} else if existing != "" {
		if err := f.updateGitHubPR(existing, title, body); err != nil {
			return existing, err
		}
		return existing, nil
	}
	path, cleanup, err := writeTempBody(body)
	if err != nil {
		return "", err
	}
	defer cleanup()
	args := []string{
		"pr", "create",
		"--base", baseBranch,
		"--head", headBranch,
		"--title", title,
		"--body-file", path,
		"-R", repo,
	}
	out, err := f.runCLI("gh", args, env)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
```

#### Forge.openGitLab (method)

```go
func (f *Forge) openGitLab(baseBranch, headBranch, title, body string) (string, error) {
	if f.Token == "" {
		return "", fmt.Errorf("gitlab context MR requires token (%s)",
			config.CredentialHint(f.RepoID, "gitlab", f.Owner))
	}
	env := f.glabEnv()
	repoArgs := glabRepoArgs(f.Owner, f.Name)
	if existing, err := f.findGitLabOpen(baseBranch, headBranch, env, repoArgs); err != nil {
		return "", err
	} else if existing != "" {
		if err := f.updateGitLabMR(existing, title, body, env, repoArgs); err != nil {
			return existing, err
		}
		return existing, nil
	}
	// glab accepts --description (inline), not --description-file (unlike gh --body-file).
	args := append([]string{
		"mr", "create",
		"--target-branch", baseBranch,
		"--source-branch", headBranch,
		"--title", title,
		"--description", body,
		"-y",
	}, repoArgs...)
	out, err := f.runCLI("glab", args, env)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
```

#### Forge.postBitbucketComment (method)

```go
func (f *Forge) postBitbucketComment(prNumber, body string) (string, error) {
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests/" + url.PathEscape(prNumber) + "/comments"
	payload, err := json.Marshal(map[string]string{"text": body})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, api, strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return "", err
	}
	raw, err := readResponseBody(resp)
	if err != nil {
		return "", fmt.Errorf("read bitbucket comment response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("bitbucket post comment HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var row struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return "", fmt.Errorf("decode bitbucket comment: %w", err)
	}
	if row.ID == 0 {
		return "", fmt.Errorf("bitbucket post comment: empty id")
	}
	return strconv.Itoa(row.ID), nil
}
```

#### Forge.postGitHubComment (method)

```go
func (f *Forge) postGitHubComment(prNumber, body string) (string, error) {
	repo := f.repoSlug()
	env := f.ghEnv()
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return "", err
	}
	args := []string{
		"api", "-X", "POST", "repos/" + repo + "/issues/" + prNumber + "/comments",
		"--input", "-",
		"--jq", ".id",
	}
	out, err := f.runCLIWithStdin("gh", args, env, string(payload))
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(out)
	if id == "" {
		return "", fmt.Errorf("github post comment: empty id")
	}
	return id, nil
}
```

#### Forge.postGitLabComment (method)

```go
func (f *Forge) postGitLabComment(iid, body string) (string, error) {
	env := f.glabEnv()
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return "", err
	}
	path := "projects/" + url.PathEscape(f.Owner+"/"+f.Name) + "/merge_requests/" + iid + "/notes"
	args := []string{"api", "-X", "POST", path, "--input", "-"}
	out, err := f.runCLIWithStdin("glab", args, env, string(payload))
	if err != nil {
		return "", err
	}
	var row struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		return "", fmt.Errorf("decode gitlab note create: %w", err)
	}
	if row.ID == 0 {
		return "", fmt.Errorf("gitlab post comment: empty id")
	}
	return strconv.Itoa(row.ID), nil
}
```

#### Forge.repoSlug (method)

```go
func (f *Forge) repoSlug() string {
	return f.Owner + "/" + f.Name
}
```

#### Forge.resolveBitbucketPRHead (method)

```go
func (f *Forge) resolveBitbucketPRHead(prNumber string) (PRHead, error) {
	if f.Token == "" || f.BaseURL == "" || f.Owner == "" || f.Name == "" {
		return PRHead{}, fmt.Errorf("bitbucket resume PR requires BITBUCKET_URL, token, project, repo")
	}
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests/" + url.PathEscape(prNumber)
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return PRHead{}, err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Accept", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return PRHead{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return PRHead{}, fmt.Errorf("read bitbucket PR response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return PRHead{}, fmt.Errorf("bitbucket get PR HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		FromRef struct {
			ID           string `json:"id"`
			LatestCommit string `json:"latestCommit"`
			DisplayID    string `json:"displayId"`
		} `json:"fromRef"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return PRHead{}, err
	}
	sha := strings.TrimSpace(out.FromRef.LatestCommit)
	if sha == "" {
		return PRHead{}, fmt.Errorf("bitbucket PR %s has empty head SHA", prNumber)
	}
	ref := strings.TrimSpace(out.FromRef.DisplayID)
	if ref == "" {
		ref = strings.TrimPrefix(out.FromRef.ID, "refs/heads/")
	}
	return PRHead{SHA: sha, RefName: ref}, nil
}
```

#### Forge.resolveGitHubPRHead (method)

```go
func (f *Forge) resolveGitHubPRHead(prNumber string) (PRHead, error) {
	if f.Token == "" || f.Owner == "" || f.Name == "" {
		return PRHead{}, fmt.Errorf("github resume PR requires token and owner/name (%s)",
			config.CredentialHint(f.RepoID, "github", f.Owner))
	}
	args := []string{
		"pr", "view", prNumber,
		"--json", "headRefOid,headRefName",
		"-R", f.repoSlug(),
	}
	out, err := f.runCLI("gh", args, f.ghEnv())
	if err != nil {
		return PRHead{}, err
	}
	var row struct {
		HeadRefOid  string `json:"headRefOid"`
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		return PRHead{}, fmt.Errorf("decode gh pr view: %w", err)
	}
	if strings.TrimSpace(row.HeadRefOid) == "" {
		return PRHead{}, fmt.Errorf("github PR %s has empty head SHA", prNumber)
	}
	return PRHead{SHA: row.HeadRefOid, RefName: row.HeadRefName}, nil
}
```

#### Forge.resolveGitLabPRHead (method)

```go
func (f *Forge) resolveGitLabPRHead(prNumber string) (PRHead, error) {
	if f.Token == "" {
		return PRHead{}, fmt.Errorf("gitlab resume MR requires token (%s)",
			config.CredentialHint(f.RepoID, "gitlab", f.Owner))
	}
	env := f.glabEnv()
	args := append([]string{"mr", "view", prNumber, "-F", "json"}, glabRepoArgs(f.Owner, f.Name)...)
	out, err := f.runCLI("glab", args, env)
	if err != nil {
		return PRHead{}, err
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		return PRHead{}, fmt.Errorf("decode glab mr view: %w", err)
	}
	sha := ""
	if v, ok := row["sha"]; ok {
		sha = fmt.Sprint(v)
	}
	if sha == "" {
		if diff, ok := row["diff_refs"].(map[string]any); ok {
			sha = fmt.Sprint(diff["head_sha"])
		}
	}
	ref := ""
	if v, ok := row["source_branch"]; ok {
		ref = fmt.Sprint(v)
	}
	if strings.TrimSpace(sha) == "" {
		return PRHead{}, fmt.Errorf("gitlab MR %s has empty head SHA", prNumber)
	}
	return PRHead{SHA: sha, RefName: ref}, nil
}
```

#### Forge.runCLI (method)

```go
func (f *Forge) runCLI(name string, args []string, env []string) (string, error) {
	runner := f.Runner
	if runner == nil {
		runner = defaultRunner
	}
	return runner(name, args, env)
}
```

#### Forge.updateBitbucketComment (method)

```go
func (f *Forge) updateBitbucketComment(prNumber, commentID, body string) error {
	api := strings.TrimRight(f.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(f.Owner) +
		"/repos/" + url.PathEscape(f.Name) + "/pull-requests/" + url.PathEscape(prNumber) +
		"/comments/" + url.PathEscape(commentID)
	payload, err := json.Marshal(map[string]string{"text": body})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, api, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+f.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return err
	}
	raw, err := readResponseBody(resp)
	if err != nil {
		return fmt.Errorf("read bitbucket comment update response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("bitbucket update comment HTTP %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}
```

#### Forge.updateGitHubComment (method)

```go
func (f *Forge) updateGitHubComment(commentID, body string) error {
	repo := f.repoSlug()
	env := f.ghEnv()
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	args := []string{
		"api", "-X", "PATCH", "repos/" + repo + "/issues/comments/" + commentID,
		"--input", "-",
	}
	_, err = f.runCLIWithStdin("gh", args, env, string(payload))
	return err
}
```

#### Forge.updateGitLabComment (method)

```go
func (f *Forge) updateGitLabComment(iid, noteID, body string) error {
	env := f.glabEnv()
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	path := "projects/" + url.PathEscape(f.Owner+"/"+f.Name) + "/merge_requests/" + iid + "/notes/" + noteID
	args := []string{"api", "-X", "PUT", path, "--input", "-"}
	_, err = f.runCLIWithStdin("glab", args, env, string(payload))
	return err
}
```

#### Git.run (method)

```go
func (g *Git) run(args ...string) (string, error) {
	cmdArgs := append([]string{"-C", g.Dir}, args...)
	cmdArgs = append(g.authConfigArgs(), cmdArgs...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}
```

#### Git.runAllowFail (method)

```go
func (g *Git) runAllowFail(args ...string) (string, int) {
	cmdArgs := append([]string{"-C", g.Dir}, args...)
	cmdArgs = append(g.authConfigArgs(), cmdArgs...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return stdout.String(), ee.ExitCode()
	}
	return stdout.String(), 1
}
```

#### Git.trim (method)

```go
func (g *Git) trim(args ...string) (string, error) {
	out, err := g.run(args...)
	return strings.TrimSpace(out), err
}
```

#### LocalSeedWorkspace.acquireLock (method)

```go
func (ws *LocalSeedWorkspace) acquireLock(now time.Time) error {
	payload, err := json.Marshal(localSeedLock{PID: os.Getpid(), At: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(ws.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		_, werr := f.Write(payload)
		_ = f.Close()
		if werr != nil {
			_ = os.Remove(ws.lockPath)
			return werr
		}
		ws.locked = true
		return nil
	}
	if !os.IsExist(err) {
		return fmt.Errorf("local seed lock: %w", err)
	}
	raw, rerr := os.ReadFile(ws.lockPath)
	if rerr != nil {
		return fmt.Errorf("local seed lock busy and unreadable: %w", rerr)
	}
	var existing localSeedLock
	_ = json.Unmarshal(raw, &existing)
	if existing.PID > 0 && processAlive(existing.PID) {
		return fmt.Errorf("local seed workspace locked by pid %d (started %s)", existing.PID, existing.At)
	}
	// Stale lock: remove and retry once.
	_ = os.Remove(ws.lockPath)
	f, err = os.OpenFile(ws.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("local seed lock retry: %w", err)
	}
	_, werr := f.Write(payload)
	_ = f.Close()
	if werr != nil {
		_ = os.Remove(ws.lockPath)
		return werr
	}
	ws.locked = true
	return nil
}
```

#### LocalSeedWorkspace.ensureLayout (method)

```go
func (ws *LocalSeedWorkspace) ensureLayout() error {
	for _, rel := range []string{localContextRel, localAnalysisRel, localCacheRel, localWorkStoryRel} {
		if err := os.MkdirAll(filepath.Join(ws.Root, rel), 0o755); err != nil {
			return fmt.Errorf("local seed layout %s: %w", rel, err)
		}
	}
	return nil
}
```

#### LocalSeedWorkspace.loadManifest (method)

```go
func (ws *LocalSeedWorkspace) loadManifest() error {
	raw, err := os.ReadFile(filepath.Join(ws.Root, localWorkspaceManifest))
	if err != nil {
		return fmt.Errorf("read workspace.yaml: %w (repair or delete the workspace and start fresh)", err)
	}
	var m LocalSeedManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("parse workspace.yaml: %w (corrupt manifest; repair or delete the workspace)", err)
	}
	if m.SchemaVersion != localSeedSchemaVersion {
		return fmt.Errorf("workspace.yaml schema_version %d unsupported (want %d)", m.SchemaVersion, localSeedSchemaVersion)
	}
	if strings.TrimSpace(m.RepoID) == "" || strings.TrimSpace(m.SourceSHA) == "" {
		return fmt.Errorf("workspace.yaml missing repo_id or source_sha (corrupt; repair or delete)")
	}
	ws.Manifest = m
	return nil
}
```

#### LocalSeedWorkspace.validateIdentity (method)

```go
func (ws *LocalSeedWorkspace) validateIdentity(repoID, sourceSHA, moduleScope string, allowSourceMove bool) error {
	if ws.Manifest.RepoID != repoID {
		return fmt.Errorf("local seed workspace repo_id=%q does not match --repo-id %q", ws.Manifest.RepoID, repoID)
	}
	wantScope := strings.TrimSpace(moduleScope)
	haveScope := strings.TrimSpace(ws.Manifest.ModuleScope)
	if haveScope != "" && wantScope != "" && haveScope != wantScope {
		return fmt.Errorf("local seed workspace module_scope=%q does not match --module-scope %q", haveScope, wantScope)
	}
	if ws.Manifest.SourceSHA != sourceSHA {
		if !allowSourceMove {
			return fmt.Errorf("local seed workspace source_sha=%s does not match workdir HEAD %s (pass --allow-source-move to retarget, or use a new --local-seed-dir)",
				shortSHA(ws.Manifest.SourceSHA), shortSHA(sourceSHA))
		}
		ws.Manifest.SourceSHA = sourceSHA
		ws.Manifest.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := ws.writeManifest(); err != nil {
			return err
		}
		logf("WARN", "local seed source_sha retargeted to %s (--allow-source-move)", shortSHA(sourceSHA))
	}
	if haveScope == "" && wantScope != "" {
		ws.Manifest.ModuleScope = wantScope
	}
	return nil
}
```

#### LocalSeedWorkspace.writeManifest (method)

```go
func (ws *LocalSeedWorkspace) writeManifest() error {
	ws.Manifest.SchemaVersion = localSeedSchemaVersion
	raw, err := yaml.Marshal(&ws.Manifest)
	if err != nil {
		return fmt.Errorf("encode workspace.yaml: %w", err)
	}
	return writeFileAtomic(filepath.Join(ws.Root, localWorkspaceManifest), raw)
}
```

#### anyPathMatches (func)

```go
func anyPathMatches(files, globs []string) bool {
	for _, f := range files {
		for _, g := range globs {
			if matchSimpleGlob(g, f) {
				return true
			}
		}
	}
	return false
}
```

#### architectureIdentitySHA (func)

```go
func architectureIdentitySHA(architectureMD string) string {
	return cache.ContentSHA(stripGeneratedAtLines(architectureMD))
}
```

#### assembleSliceCatalogFragments (func)

```go
func assembleSliceCatalogFragments(ctx context.Context, caller sliceCatalogRLMCaller, req sliceCatalogAssembleRequest) (sliceCatalogAssembleResult, error) {
	out := sliceCatalogAssembleResult{}
	start := time.Now()
	defer func() { out.Duration = time.Since(start) }()

	targets := catalogAssembleTargets(req.FoldedTypo)
	if len(targets) == 0 {
		return out, nil
	}

	wholeContext := ""
	if req.EvidenceDir != "" {
		raw, err := os.ReadFile(filepath.Join(req.EvidenceDir, "package_rlm_context.md"))
		if err != nil && !os.IsNotExist(err) {
			return out, fmt.Errorf("slice catalog RLM read package_rlm_context: %w", err)
		}
		wholeContext = string(raw)
	}

	ledgerByID := map[string]sliceObjectiveLedgerEntry{}
	for _, e := range req.LedgerDoc.Slices {
		if id := strings.TrimSpace(e.ID); id != "" {
			ledgerByID[id] = e
		}
	}

	kept := map[string]catalog.Slice{}
	for id, s := range req.Kept {
		if strings.TrimSpace(id) != "" {
			kept[id] = s
		}
	}

	var lastIssues []string
	for attempt := 1; attempt <= maxTypologyRefineAttempts; attempt++ {
		var pending []catalogAssembleTarget
		for _, t := range targets {
			if _, ok := kept[t.id]; ok {
				continue
			}
			pending = append(pending, t)
		}
		if len(pending) == 0 {
			break
		}

		type result struct {
			frag  catalog.Slice
			issue string
			err   error
			pt    int
			ct    int
			tt    int
			iters int
		}
		results := make([]result, len(pending))
		sem := make(chan struct{}, catalogRLMWorkers)
		var wg sync.WaitGroup
		for i, t := range pending {
			wg.Add(1)
			go func(i int, t catalogAssembleTarget) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				ledger := resolveLedgerEntryForTarget(t, req.LedgerDoc, ledgerByID)
				frag, issue, pt, ct, tt, iters, err := groundOneSliceCatalog(
					ctx, caller, req, t, wholeContext, ledger, filterIssuesForSlice(lastIssues, t.id),
				)
				results[i] = result{frag: frag, issue: issue, err: err, pt: pt, ct: ct, tt: tt, iters: iters}
			}(i, t)
		}
		wg.Wait()

		var issues []string
		for _, r := range results {
			out.PromptTokens += r.pt
			out.CompletionTok += r.ct
			out.TotalTokens += r.tt
			out.RLMIterations += r.iters
			if r.err != nil {
				return out, fmt.Errorf("typology_slice_catalog RLM assemble: %w", r.err)
			}
			if strings.TrimSpace(r.issue) != "" {
				issues = append(issues, r.issue)
				continue
			}
			if id := strings.TrimSpace(r.frag.ID); id != "" {
				kept[id] = r.frag
			}
		}
		lastIssues = issues
		if len(issues) == 0 {
			break
		}
		if attempt == maxTypologyRefineAttempts {
			out.Issues = issues
			break
		}
	}

	ids := make([]string, 0, len(kept))
	for id := range kept {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		out.Fragments = append(out.Fragments, kept[id])
	}
	return out, nil
}
```

#### bootstrapStorySection (type)

```go
type bootstrapStorySection struct {
	ID          string
	Current     string
	Setter      func(*BootstrapStoryOutput, string)
	Getter      func(BootstrapStoryOutput) string
	Instruction string
}
```

#### bootstrapStorySections (func)

```go
func bootstrapStorySections(input BootstrapStoryInput) []bootstrapStorySection {
	return []bootstrapStorySection{
		{ID: "readme", Current: input.CurrentReadme, Instruction: "Context-branch README: seed origin and reading order for the served repo. Keep ## Reading order and majordomo reading markers when present. README may mention the context branch; do not rewrite the product mission as Majordomo.", Setter: func(o *BootstrapStoryOutput, s string) { o.ReadmeMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.ReadmeMD }},
		{ID: "mission", Current: input.CurrentMission, Instruction: "Mission markdown: present-tense purpose of the served product from README + typology + slice objective ledger. MUST NOT invent a Majordomo/control-plane mission.", Setter: func(o *BootstrapStoryOutput, s string) { o.MissionMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.MissionMD }},
		{ID: "architecture", Current: input.CurrentArchitecture, Instruction: "Teaching architecture for the served product. Use refined catalog slice ids and ledger objectives by name. Mention CLI/server entrypoints only when evidence labels them; MUST NOT invent a Jobs, Doors, or product-pillar taxonomy. MUST NOT invent a glamorous core-value slice; MUST NOT claim confirmed .typology/.", Setter: func(o *BootstrapStoryOutput, s string) { o.ArchitectureMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.ArchitectureMD }},
		{ID: "conventions", Current: input.CurrentConventions, Instruction: "Conventions markdown grounded in evidence for the served product.", Setter: func(o *BootstrapStoryOutput, s string) { o.ConventionsMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.ConventionsMD }},
		{ID: "weaknesses", Current: input.CurrentWeaknesses, Instruction: "Weaknesses markdown from typology debt and findings; no invented history.", Setter: func(o *BootstrapStoryOutput, s string) { o.WeaknessesMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.WeaknessesMD }},
		{ID: "chronology", Current: input.CurrentChronology, Instruction: "Honest chronology with at most one explicit seed marker; do not reconstruct past decisions.", Setter: func(o *BootstrapStoryOutput, s string) { o.ChronologyMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.ChronologyMD }},
		{ID: "grounding", Current: input.CurrentGrounding, Instruction: "Agenting GROUNDING.md: cold-reader map of the served product for agents. Prefer ledger objectives. MUST NOT read as Majordomo agent-bootstrap meta or invent agent taxonomies.", Setter: func(o *BootstrapStoryOutput, s string) { o.GroundingMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.GroundingMD }},
	}
}
```

#### buildBootstrapStoryContext (func)

```go
func buildBootstrapStoryContext(input BootstrapStoryInput) string {
	var b strings.Builder
	b.WriteString("# Bootstrap story evidence\n\n")
	fmt.Fprintf(&b, "repo_id: %s\nsource_sha: %s\ngenerated_at: %s\nevidence_mode: %s\nmodule_scope: %s\n\n",
		input.RepoID, input.SourceSHA, input.GeneratedAt.UTC().Format(time.RFC3339), input.EvidenceMode, input.ModuleScope)
	b.WriteString("## README snapshot\n\n")
	b.WriteString(strings.TrimSpace(input.ReadmeSnapshot))
	b.WriteString("\n\n## Typology manifest\n\n")
	b.WriteString(strings.TrimSpace(input.TypologyManifest))
	b.WriteString("\n\n## Typology architecture brief\n\n")
	b.WriteString(strings.TrimSpace(input.TypologyArchitecture))
	b.WriteString("\n\n## Refined catalog\n\n")
	b.WriteString(strings.TrimSpace(input.TypologyRefinedCatalog))
	b.WriteString("\n\n## Slice objective ledger\n\n")
	b.WriteString(strings.TrimSpace(input.TypologySliceObjectiveLedger))
	b.WriteString("\n\n## Journey\n\n")
	b.WriteString(strings.TrimSpace(input.TypologyJourney))
	b.WriteString("\n\n## Repo layout\n\n")
	b.WriteString(strings.TrimSpace(input.RepoLayout))
	return b.String()
}
```

#### buildCapabilityConstraints (func)

```go
func buildCapabilityConstraints(doc packageRolesDoc) packageCapabilityConstraintsDoc {
	table := roleCapabilityTable()
	filledBy := map[string][]string{}
	fillsDTOFrom := map[string]struct{}{}
	for _, e := range doc.Edges {
		kind := strings.ToLower(strings.TrimSpace(e.Kind))
		from := normalizeRolePath(e.From)
		to := normalizeRolePath(e.To)
		if from == "" || to == "" {
			continue
		}
		switch kind {
		case edgeFillsDTO:
			filledBy[to] = append(filledBy[to], from)
			fillsDTOFrom[from] = struct{}{}
		}
	}

	out := packageCapabilityConstraintsDoc{}
	for _, n := range doc.Packages {
		path := normalizeRolePath(n.Path)
		if path == "" {
			continue
		}
		role := strings.TrimSpace(n.Role)
		if role == "" {
			role = roleUnknown
		}
		defs, ok := table[role]
		if !ok {
			defs = table[roleUnknown]
		}
		c := packageCapabilityConstraint{
			Path:    path,
			Is:      append([]string(nil), defs.Is...),
			MustNot: append([]string(nil), defs.MustNot...),
			Source:  "role_table",
			Role:    role,
		}
		// Fail-closed post-pass is the authority for unknown/custom roles.
		if role != roleEntrypoint {
			c.MustNot = uniqueStrings(append(c.MustNot, capOrchestrate))
		}
		if role != roleExecRunner && !evidenceHasAny(n.Evidence, "imports_os_exec") {
			c.MustNot = uniqueStrings(append(c.MustNot, capExecProcess))
		}
		if role != roleAdapter {
			if _, ok := fillsDTOFrom[path]; !ok {
				c.MustNot = uniqueStrings(append(c.MustNot, capFillDTO))
			}
		}
		// own_domain_rules: aggregator-only (entrypoint/http already forbid in table).
		if role != roleAggregator {
			c.MustNot = uniqueStrings(append(c.MustNot, capOwnDomainRules))
		}
		if role != roleAdapter {
			c.MustNot = uniqueStrings(append(c.MustNot, capAdaptExternal))
		}
		if role != roleAggregator {
			c.MustNot = uniqueStrings(append(c.MustNot, capAggregateViews))
		}
		if role != roleDTO {
			c.MustNot = uniqueStrings(append(c.MustNot, capDataShape))
		}
		if role != roleObservability &&
			!evidenceHasAny(n.Evidence, "imports_otel", "imports_prometheus") {
			c.MustNot = uniqueStrings(append(c.MustNot, capObservability))
		}
		allowHTTP := role == roleHTTPSurface ||
			evidenceHasAny(n.Evidence, "delivery:http", "delivery:grpc") ||
			evidenceHasAnyPrefix(n.Evidence, "imports_net_http", "imports_grpc")
		if !allowHTTP {
			c.MustNot = uniqueStrings(append(c.MustNot, capServeHTTP, capWireHandlers))
		}
		if role != roleEntrypoint && !evidenceHasAny(n.Evidence, "has_main") {
			c.MustNot = uniqueStrings(append(c.MustNot, capRunCLI))
		}
		allowConfig := role == roleConfig
		if !allowConfig &&
			!evidenceHasAny(n.Evidence, "imports_otel", "imports_prometheus") &&
			evidenceHasAny(n.Evidence, "config_keys", "env_config") {
			allowConfig = true
		}
		if !allowConfig {
			c.MustNot = uniqueStrings(append(c.MustNot, capConfig))
		}
		// No role puts these in is; entailment always rejects.
		c.MustNot = uniqueStrings(append(c.MustNot, capSynchronizeState, capMergeAdapters))
		if fillers := uniqueStrings(filledBy[path]); len(fillers) > 0 {
			c.FilledBy = fillers
		}
		// Evidence/role exceptions must clear table defaults, not only skip appends.
		c.MustNot = dropAllowedCapabilityMustNot(c.MustNot, role, n.Evidence, path, fillsDTOFrom)
		if n.Agreement == agreementMatch || n.Agreement == agreementDisagree {
			c.Source = "role_rlm"
		}
		out.Packages = append(out.Packages, c)
	}
	sort.Slice(out.Packages, func(i, j int) bool {
		return out.Packages[i].Path < out.Packages[j].Path
	})
	return out
}
```

#### buildSliceObjectiveLedger (func)

```go
func buildSliceObjectiveLedger(
	ctx context.Context,
	caller sliceLedgerRLMCaller,
	req sliceLedgerBuildRequest,
) (sliceObjectiveLedgerDoc, []string, error) {
	if caller == nil {
		return sliceObjectiveLedgerDoc{}, []string{fmt.Sprintf(
			"%s: slice_objective_ledger RLM is required for owned packages",
			typologypack.CriterionIDRoleGrounding,
		)}, nil
	}
	byPath := constraintsByPath(req.Constraints)
	var allTargets []ledgerSliceTarget
	for _, s := range req.DraftTypo.Slices {
		id := strings.TrimSpace(s.ID)
		paths := slicePackagePaths(s)
		if id == "" || len(paths) == 0 {
			continue
		}
		allTargets = append(allTargets, ledgerSliceTarget{id: id, paths: paths})
	}
	sort.Slice(allTargets, func(i, j int) bool { return allTargets[i].id < allTargets[j].id })
	if len(allTargets) == 0 {
		return sliceObjectiveLedgerDoc{}, nil, nil
	}
	if len(allTargets) > maxLedgerSlices {
		allTargets = allTargets[:maxLedgerSlices]
	}

	rolesDoc, err := loadPackageRolesFromEvidenceDir(req.EvidenceDir)
	if err != nil {
		return sliceObjectiveLedgerDoc{}, nil, fmt.Errorf("%s: %w", typologypack.CriterionIDRoleGrounding, err)
	}

	rlmContextPath := filepath.Join(req.EvidenceDir, "package_rlm_context.md")
	wholeContext, readErr := os.ReadFile(rlmContextPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return sliceObjectiveLedgerDoc{}, nil, fmt.Errorf("%s: read package_rlm_context.md: %w", typologypack.CriterionIDRoleGrounding, readErr)
	}

	kept := map[string]sliceObjectiveLedgerEntry{}
	var lastIssues []string
	for attempt := 1; attempt <= maxTypologyRefineAttempts; attempt++ {
		var pending []ledgerSliceTarget
		for _, t := range allTargets {
			if _, ok := kept[t.id]; ok {
				continue
			}
			pending = append(pending, t)
		}
		if len(pending) == 0 {
			break
		}

		type result struct {
			entry sliceObjectiveLedgerEntry
			issue string
			err   error
		}
		results := make([]result, len(pending))
		sem := make(chan struct{}, ledgerRLMWorkers)
		var wg sync.WaitGroup
		for i, t := range pending {
			wg.Add(1)
			go func(i int, t ledgerSliceTarget) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				entry, issue, err := groundOneSliceLedger(ctx, caller, req, t, string(wholeContext), byPath, rolesDoc, filterIssuesForSlice(lastIssues, t.id))
				results[i] = result{entry: entry, issue: issue, err: err}
			}(i, t)
		}
		wg.Wait()

		var issues []string
		for _, r := range results {
			if r.err != nil {
				return sliceObjectiveLedgerDoc{}, nil, r.err
			}
			if strings.TrimSpace(r.issue) != "" {
				issues = append(issues, r.issue)
				continue
			}
			if id := strings.TrimSpace(r.entry.ID); id != "" {
				kept[id] = r.entry
			}
		}
		if len(issues) == 0 {
			break
		}
		lastIssues = issues
		if attempt == maxTypologyRefineAttempts {
			return sliceObjectiveLedgerDoc{}, lastIssues, nil
		}
	}

	slices := make([]sliceObjectiveLedgerEntry, 0, len(kept))
	for _, t := range allTargets {
		if e, ok := kept[t.id]; ok {
			slices = append(slices, e)
		}
	}
	doc := sliceObjectiveLedgerDoc{Slices: slices}
	if err := validateObjectiveLedgerDoc(doc); err != nil {
		return sliceObjectiveLedgerDoc{}, nil, err
	}
	// Soft-drop claim codes outside owned is=[] (alignLedger does the same). Keeps seed
	// from failing closed on a single over-wide RLM claim after evidence already grounded.
	for i := range doc.Slices {
		doc.Slices[i].Claims = filterClaimsToOwnedIs(doc.Slices[i].Claims, doc.Slices[i].OwnedPaths, byPath)
	}
	if consIssues := validateLedgerAgainstConstraints(doc, req.Constraints, rolesDoc); len(consIssues) > 0 {
		return sliceObjectiveLedgerDoc{}, consIssues, nil
	}
	if req.DigestCache != nil {
		logf("INFO", "%s", cache.FormatStatsLine(req.DigestCache.Stats()))
	}
	return doc, nil, nil
}
```

#### clusterAuditRequest (type)

```go
type clusterAuditRequest struct {
	AnalysisDir    string
	EvidenceDir    string
	Proposed       []proposedMerge
	Frozen         []clusterMergeVerdict
	RolesYAML      string
	Constraints    string
	MechanicalYAML string
	DigestCache    *cache.DigestStore
	DigestSkips    bool
	DigestModelID  string
	Attempt        int
}
```

#### clusterAuditResult (type)

```go
type clusterAuditResult struct {
	Verdicts      []clusterMergeVerdict
	RLMIterations int
	Duration      time.Duration
	PromptTokens  int
	CompletionTok int
	TotalTokens   int
	TraceDir      string
}
```

#### clusterMergeAuditor (type)

```go
type clusterMergeAuditor interface {
	Audit(ctx context.Context, req clusterAuditRequest) (clusterAuditResult, error)
}
```

#### clusterMergeVerdict (type)

```go
type clusterMergeVerdict struct {
	ID       string   `yaml:"id"`
	Packages []string `yaml:"packages"`
	Verdict  string   `yaml:"verdict"` // accept | overlay | reject
	Reason   string   `yaml:"reason,omitempty"`
	Evidence []string `yaml:"evidence,omitempty"`
}
```

#### copyFile (func)

```go
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
```

#### copyStringMap (func)

```go
func copyStringMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
```

#### digestModelID (func)

```go
func digestModelID(cfg config.RepoConfig) string {
	for _, task := range []string{jmodules.TaskTypologySliceMeaning, jmodules.TaskTypologyInspect} {
		provider, ok, err := cfg.ResolveTaskProvider(task)
		if err != nil || !ok {
			continue
		}
		if m := strings.TrimSpace(provider.Model); m != "" {
			return m
		}
	}
	return "unknown"
}
```

#### digestSectionRunner (type)

```go
type digestSectionRunner struct {
	cc            CommitContext
	regenFeedback string
	changedFiles  string
	gen           judge.Generator
}
```

#### discoverSurveyRoots (func)

```go
func discoverSurveyRoots(dir string) (surveyRoots, error) {
	modules, err := discoverGoModules(dir)
	if err != nil {
		return surveyRoots{}, err
	}
	hasPy, err := discoverPythonRoot(dir)
	if err != nil {
		return surveyRoots{}, err
	}
	return surveyRoots{
		GoModules: modules,
		HasGo:     len(modules) > 0,
		HasPython: hasPy,
	}, nil
}
```

#### draftCatalogIdentitySHA (func)

```go
func draftCatalogIdentitySHA(draftYAML string) string {
	raw := strings.TrimSpace(draftYAML)
	if raw == "" {
		return cache.ContentSHA("")
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return cache.ContentSHA(raw)
	}
	normalizeEphemeralCatalogDoc(doc)
	out, err := yaml.Marshal(doc)
	if err != nil {
		return cache.ContentSHA(raw)
	}
	return cache.ContentSHA(string(out))
}
```

#### emptyHumanInterventionNote (func)

```go
func emptyHumanInterventionNote() string {
	return "# Human intervention\n\nNo open architecture findings after typology refine. No human boundary decisions required for this seed.\n"
}
```

#### ensureArchitectureMarkdownKeepsGroundedObjectives (func)

```go
func ensureArchitectureMarkdownKeepsGroundedObjectives(archMD, refinedYAML, constraintsYAML string) (string, error) {
	refinedYAML = strings.TrimSpace(refinedYAML)
	constraintsYAML = strings.TrimSpace(constraintsYAML)
	if refinedYAML == "" || constraintsYAML == "" {
		return archMD, nil
	}
	typo, err := loadTypologyFromYAML(refinedYAML)
	if err != nil {
		return "", fmt.Errorf("architecture grounding load catalog: %w", err)
	}
	constraints, err := parseCapabilityConstraintsYAML(constraintsYAML)
	if err != nil {
		return "", fmt.Errorf("architecture grounding parse constraints: %w", err)
	}
	missing := missingGroundedObjectives(archMD, typo, constraints)
	if len(missing) == 0 {
		return archMD, nil
	}
	var block strings.Builder
	if err := groundedObjectivesAppendTemplate.Execute(&block, missing); err != nil {
		return "", fmt.Errorf("architecture grounding render objectives: %w", err)
	}
	out := strings.TrimRight(archMD, "\n") + "\n" + block.String()
	if still := missingGroundedObjectives(out, typo, constraints); len(still) > 0 {
		return "", fmt.Errorf("%s", formatMissingGroundedObjectivesFeedback(still))
	}
	return out, nil
}
```

#### ensureStoryArchitectureBanner (func)

```go
func ensureStoryArchitectureBanner(md string) string {
	return ensureRoleBanner(md, storyArchitectureRoleMarker, storyArchitectureBanner)
}
```

#### evidenceForChronology (func)

```go
func evidenceForChronology(cc CommitContext) bool {
	if strings.TrimSpace(cc.Subject) == "" {
		return false
	}
	if len(cc.Files) == 0 && strings.TrimSpace(cc.Diff) == "" {
		return false
	}
	return true
}
```

#### extractArchitectureFindings (func)

```go
func extractArchitectureFindings(architectureMD string) []string {
	lines := strings.Split(architectureMD, "\n")
	inFindings := false
	var out []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		low := strings.ToLower(trim)
		if isArchitectureFindingsHeading(trim) {
			inFindings = true
			continue
		}
		if inFindings && strings.HasPrefix(trim, "#") {
			break
		}
		if !inFindings || trim == "" {
			continue
		}
		// Skip instructional prose that is not a finding bullet.
		if strings.HasPrefix(trim, "The following findings") {
			continue
		}
		if strings.HasPrefix(trim, "1.") || strings.HasPrefix(trim, "2.") || strings.HasPrefix(trim, "3.") {
			// Numbered remediation steps after the list, not findings.
			if strings.Contains(low, "read the relevant") || strings.Contains(low, "fix the code") || strings.Contains(low, "record a temporary") {
				break
			}
		}
		finding := ""
		switch {
		case strings.HasPrefix(trim, "- "), strings.HasPrefix(trim, "* "):
			finding = strings.TrimSpace(trim[2:])
		case strings.HasPrefix(trim, "|") && !strings.Contains(trim, "---"):
			finding = strings.TrimSpace(trim)
		default:
			if strings.Contains(trim, "`") && (strings.Contains(low, "slicebinding") || strings.Contains(low, "unmapped") || strings.Contains(low, "missing")) {
				finding = trim
			}
		}
		if finding == "" {
			continue
		}
		out = append(out, finding)
	}
	return out
}
```

#### findingFingerprint (func)

```go
func findingFingerprint(finding string) string {
	norm := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(finding))), " ")
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:8])
}
```

#### findingMatchNeedle (func)

```go
func findingMatchNeedle(finding string) string {
	f := strings.TrimSpace(finding)
	if f == "" {
		return ""
	}
	// Prefer a backticked package/slice id when present.
	if i := strings.Index(f, "`"); i >= 0 {
		rest := f[i+1:]
		if j := strings.Index(rest, "`"); j > 0 {
			return rest[:j]
		}
	}
	if len(f) > 48 {
		return f[:48]
	}
	return f
}
```

#### firstLine (func)

```go
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}
```

#### formatBootstrapStorySectionQuery (func)

```go
func formatBootstrapStorySectionQuery(repoID string, sec bootstrapStorySection, validationFeedback string) string {
	fb := ""
	if strings.TrimSpace(validationFeedback) != "" {
		fb = "\nvalidation_feedback (fix before emitting):\n" + strings.TrimSpace(validationFeedback) + "\n"
	}
	repo := strings.TrimSpace(repoID)
	if repo == "" {
		repo = "the served repository"
	}
	return fmt.Sprintf(`Write one teaching-story markdown section for served repository %s.
Subject is that product repo, not Majordomo the control plane (unless repo_id is majordomo).
Section id: %s
%s
%s
Current draft (may be a placeholder; improve from evidence):
%s

Final answer MUST be the full markdown body for this section only (start with a heading).
Do not wrap in YAML, code fences, or a markdown: | envelope.

Write present-tense, evidence-backed prose. Prefer slice objective ledger and refined Typology catalog over raw inventory.
Do not invent history. Preserve majordomo-reading markers when rewriting README.
Mission, architecture, and grounding MUST name the served product from evidence; MUST NOT describe Majordomo triage, digest, or context-branch process as the product.`,
		repo, sec.ID, sec.Instruction, fb, strings.TrimSpace(sec.Current))
}
```

#### formatFindingCommentBody (func)

```go
func formatFindingCommentBody(fingerprint, counsel string) string {
	counsel = strings.TrimSpace(counsel)
	var b strings.Builder
	if err := findingCommentBodyTmpl.Execute(&b, struct {
		Marker  string
		Counsel string
		Cleared bool
	}{
		Marker:  findingCommentMarker(fingerprint),
		Counsel: counsel,
		Cleared: strings.HasPrefix(strings.ToLower(counsel), "cleared"),
	}); err != nil {
		panic(fmt.Sprintf("finding comment template: %v", err))
	}
	return b.String()
}
```

#### formatFindingsList (func)

```go
func formatFindingsList(findings []string) string {
	if len(findings) == 0 {
		return ""
	}
	var b strings.Builder
	for i, f := range findings {
		fmt.Fprintf(&b, "%d. %s\n", i+1, f)
	}
	return strings.TrimSpace(b.String())
}
```

#### generateInterventionStep (func)

```go
func generateInterventionStep(
	ctx context.Context,
	gen judge.Generator,
	task string,
	fields map[string]interface{},
	outKey string,
	validate func(map[string]interface{}) error,
	cacheOpts interventionCacheOpts,
) (string, error) {
	fp := cache.InterventionFingerprint{
		TaskID:           task,
		ArchitectureHash: cache.ContentSHA(cacheOpts.ArchitectureMD),
		RefinedHash:      cache.ContentSHA(cacheOpts.RefinedYAML),
		VerdictsHash:     clusterVerdictsIdentitySHA(cacheOpts.VerdictsYAML),
		FindingsHash:     cache.ContentSHA(cacheOpts.FindingsList),
		FindingHash:      cache.ContentSHA(cacheOpts.Finding),
		ModelID:          cacheOpts.ModelID,
		PromptVersion:    cache.DigestInterventionPromptV1,
		SchemaVersion:    cache.DigestInterventionSchemaV2,
	}
	feedback := ""
	if v, ok := fields["validation_feedback"].(string); ok {
		feedback = strings.TrimSpace(v)
	}
	if feedback == "" && cacheOpts.Skips && cacheOpts.Store != nil {
		if hit, ok, err := cacheOpts.Store.LookupIntervention(fp); err == nil && ok && strings.TrimSpace(hit.Markdown) != "" {
			out := map[string]interface{}{outKey: hit.Markdown}
			if validate == nil || validate(out) == nil {
				cacheOpts.Store.RecordInterventionHit(hit.PromptTokens, hit.CompletionTokens, hit.TotalTokens)
				logf("INFO", "digest cache hit intervention task=%s", task)
				return strings.TrimSpace(hit.Markdown), nil
			}
		}
	}
	var lastErr error
	for attempt := 1; attempt <= maxHumanInterventionAttempts; attempt++ {
		if cacheOpts.Store != nil && attempt == 1 {
			cacheOpts.Store.RecordInterventionMiss()
		}
		stepFields := copyStringMap(fields)
		stepFields["validation_feedback"] = feedback
		out, err := gen.Generate(ctx, task, stepFields, attempt)
		if err != nil {
			return "", fmt.Errorf("%s: %w", task, err)
		}
		if strings.TrimSpace(stringField(out, outKey)) == "" {
			lastErr = fmt.Errorf("%s: %s is required", task, outKey)
			feedback = lastErr.Error()
			continue
		}
		if validate != nil {
			if err := validate(out); err != nil {
				lastErr = err
				if attempt == maxHumanInterventionAttempts {
					return "", err
				}
				feedback = err.Error()
				continue
			}
		}
		agg, err := gen.Evaluate(ctx, task, stepFields, out, attempt)
		if err != nil {
			lastErr = err
			if attempt == maxHumanInterventionAttempts {
				return "", fmt.Errorf("%s LLM evaluation: %w", task, err)
			}
			feedback = err.Error()
			continue
		}
		if !judge.EvalPassed(agg) {
			lastErr = fmt.Errorf("%s", judge.EvalFeedback(agg))
			if attempt == maxHumanInterventionAttempts {
				return "", fmt.Errorf("%s LLM evaluation failed after %d attempts:\n%s", task, maxHumanInterventionAttempts, judge.EvalFeedback(agg))
			}
			feedback = judge.EvalFeedback(agg)
			continue
		}
		markdown := strings.TrimSpace(stringField(out, outKey))
		if cacheOpts.Store != nil {
			if err := cacheOpts.Store.StoreIntervention(fp, cache.InterventionCached{Markdown: markdown}); err != nil {
				logf("WARN", "digest cache store intervention task=%s failed: %v", task, err)
			}
		}
		return markdown, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("%s exhausted retries", task)
}
```

#### glabRepoArgs (func)

```go
func glabRepoArgs(owner, name string) []string {
	if strings.Contains(owner, "/") {
		return []string{"--repo", owner + "/" + name}
	}
	return []string{"--repo", owner + "/" + name}
}
```

#### interventionCacheOpts (type)

```go
type interventionCacheOpts struct {
	Store          *cache.DigestStore
	Skips          bool
	ModelID        string
	ArchitectureMD string
	RefinedYAML    string
	VerdictsYAML   string
	FindingsList   string
	Finding        string
}
```

#### interventionCacheOptsFrom (func)

```go
func interventionCacheOptsFrom(input HumanInterventionInput, finding string) interventionCacheOpts {
	return interventionCacheOpts{
		Store:          input.DigestCache,
		Skips:          input.DigestSkips,
		ModelID:        input.DigestModelID,
		ArchitectureMD: input.ArchitectureMD,
		RefinedYAML:    input.RefinedCatalogYAML,
		VerdictsYAML:   input.ClusterMergeVerdictsYAML,
		FindingsList:   input.FindingsList,
		Finding:        finding,
	}
}
```

#### isLocalSeedMode (func)

```go
func isLocalSeedMode(opts Options) bool {
	return strings.TrimSpace(opts.LocalSeedDir) != ""
}
```

#### isPlaceholderOwner (func)

```go
func isPlaceholderOwner(owner string) bool {
	up := strings.ToUpper(strings.TrimSpace(owner))
	return strings.Contains(up, "YOUR_") || up == "YOUR_ORG"
}
```

#### isPlaceholderRepo (func)

```go
func isPlaceholderRepo(repoID string, cfg config.RepoConfig) bool {
	if strings.HasPrefix(repoID, "example-") {
		return true
	}
	return strings.Contains(strings.ToUpper(cfg.Repository.CloneURL), "YOUR_")
}
```

#### ledgerGroundingIssue (type)

```go
type ledgerGroundingIssue struct {
	msg string
}
```

#### ledgerStepRunner (type)

```go
type ledgerStepRunner struct {
	store         stepplan.Store
	caller        sliceLedgerRLMCaller
	req           sliceLedgerBuildRequest
	target        ledgerSliceTarget
	wholeContext  string
	byPath        map[string]packageCapabilityConstraint
	rolesDoc      packageRolesDoc
	sliceFeedback string
	pathByStep    map[string]string
}
```

#### ledgerStepRunner.runEvidence (method)

```go
func (r *ledgerStepRunner) runEvidence(ctx context.Context, step stepplan.Step) (*orchestration.StepRunResult, error) {
	pkgPath := r.pathByStep[step.ID]
	if pkgPath == "" {
		for _, ref := range step.InputRefs {
			if strings.TrimSpace(ref.URI) != "" {
				pkgPath = strings.TrimSpace(ref.URI)
				break
			}
		}
	}
	if pkgPath == "" {
		return nil, fmt.Errorf("evidence step %q missing package path", step.ID)
	}

	ctxMD := packageRLMContextSnippet(r.wholeContext, pkgPath)
	if strings.TrimSpace(ctxMD) == "" {
		built, err := typroles.FormatPackageRLMContextForPath(r.req.AnalysisDir, pkgPath, nil)
		if err == nil {
			ctxMD = built
		}
	}
	if strings.TrimSpace(ctxMD) == "" {
		note := packageEvidenceNote{
			Path:     pkgPath,
			Evidence: []string{"package:" + filepath.Base(pkgPath)},
			Notes:    "Package present in owned slice; symbols not extracted.",
		}
		body, err := json.Marshal(note)
		if err != nil {
			return nil, err
		}
		return &orchestration.StepRunResult{Output: body}, nil
	}

	note := mechanicalPackageEvidenceNote(pkgPath, ctxMD)
	if len(note.Evidence) == 0 {
		pkgConstraints := formatConstraintRowsForPaths([]string{pkgPath}, r.byPath)
		payload := strings.TrimSpace(ctxMD)
		if pkgConstraints != "" {
			payload = payload + "\n\n" + pkgConstraints
		}
		if len(payload) > ledgerMaxContextChars {
			payload = truncateToLedgerBudget(payload, "\n[... package context truncated ...]\n")
		}
		query := formatPackageEvidenceQuery(r.target.id, pkgPath)
		answer, _, _, _, _, err := r.caller.Complete(ctx, payload, query)
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			logf("WARN", "ledger evidence RLM soft-fail slice=%s pkg=%s: %v; using empty mechanical note", r.target.id, pkgPath, err)
		} else if parsed, parseErr := parsePackageEvidenceAnswer(pkgPath, answer); parseErr == nil {
			note = parsed
		} else {
			logf("WARN", "ledger evidence parse soft-fail slice=%s pkg=%s: %v", r.target.id, pkgPath, parseErr)
		}
	}
	if len(note.Evidence) == 0 && strings.TrimSpace(note.Notes) == "" {
		note = packageEvidenceNote{
			Path:     pkgPath,
			Evidence: []string{"package:" + filepath.Base(pkgPath)},
			Notes:    "Package present in owned slice; symbols not extracted.",
		}
	}
	body, err := json.Marshal(note)
	if err != nil {
		return nil, err
	}
	return &orchestration.StepRunResult{Output: body}, nil
}
```

#### ledgerStepRunner.runSynthesis (method)

```go
func (r *ledgerStepRunner) runSynthesis(ctx context.Context, plan *stepplan.Plan, _ stepplan.Step) (*orchestration.StepRunResult, error) {
	notes, err := r.loadEvidenceNotes(ctx, plan)
	if err != nil {
		return nil, err
	}
	if len(notes) == 0 {
		return nil, fmt.Errorf("%s: slice %q owned packages have empty RLM context; cannot ground objective",
			typologypack.CriterionIDRoleGrounding, r.target.id)
	}

	distilled := formatDistilledPackageEvidence(notes)
	if len(distilled) > ledgerMaxContextChars {
		distilled = truncateToLedgerBudget(distilled, "\n[... distilled evidence truncated ...]\n")
	}
	constraintBlock := formatConstraintRowsForPaths(r.target.paths, r.byPath)
	query := formatSliceObjectiveLedgerQuery(r.target.id, r.target.paths, constraintBlock, r.req.ClusterHintYAML, r.sliceFeedback)
	answer, _, promptTok, completionTok, totalTok, err := r.caller.Complete(ctx, distilled, query)
	if err != nil {
		return nil, err
	}
	entry, issue, err := finalizeLedgerEntryFromAnswer(r.target, answer, r.byPath, r.rolesDoc, r.req, promptTok, completionTok, totalTok)
	if err != nil {
		return nil, err
	}
	if issue != "" {
		return nil, ledgerGroundingIssue{msg: issue}
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	return &orchestration.StepRunResult{
		Output: body,
		Usage: &stepplan.TokenUsage{
			PromptTokens:     promptTok,
			CompletionTokens: completionTok,
			TotalTokens:      totalTok,
		},
	}, nil
}
```

#### loadFindingCommentBodies (func)

```go
func loadFindingCommentBodies(ctxDir string) ([]FindingCommentBody, error) {
	path := filepath.Join(ctxDir, findingCommentBodiesRel)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var file FindingCommentBodiesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("finding comment bodies decode: %w", err)
	}
	return file.Comments, nil
}
```

#### loadFindingCommentsSidecar (func)

```go
func loadFindingCommentsSidecar(ctxDir string) (FindingCommentsSidecar, error) {
	path := filepath.Join(ctxDir, findingCommentsRel)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FindingCommentsSidecar{ByFingerprint: map[string]string{}}, nil
		}
		return FindingCommentsSidecar{}, err
	}
	var s FindingCommentsSidecar
	if err := json.Unmarshal(data, &s); err != nil {
		return FindingCommentsSidecar{}, fmt.Errorf("finding comments sidecar decode: %w", err)
	}
	if s.ByFingerprint == nil {
		s.ByFingerprint = map[string]string{}
	}
	return s, nil
}
```

#### logf (func)

```go
func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

#### marshalConstraints (func)

```go
func marshalConstraints(doc packageCapabilityConstraintsDoc) (string, error) {
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return "", fmt.Errorf("capability constraints encode: %w", err)
	}
	return string(data), nil
}
```

#### materializeDigestCacheWorktree (func)

```go
func materializeDigestCacheWorktree(dir string, served *Git, branch, token, scm string) error {
	if err := cache.ValidateInferenceCacheBranch(branch); err != nil {
		return err
	}
	g := &Git{Dir: dir, Token: token, SCM: scm}
	if _, err := g.run("init"); err != nil {
		return err
	}
	if err := configureCommitIdentity(g); err != nil {
		return err
	}
	remote, err := served.trim("remote", "get-url", "origin")
	if err != nil {
		return err
	}
	if err := ensureRemote(g, remote); err != nil {
		return err
	}
	exists, err := RemoteBranchExists(served, branch)
	if err != nil {
		return err
	}
	if exists {
		if err := FetchOrigin(g, branch+":"+branch); err != nil {
			return fmt.Errorf("fetch inference cache branch: %w", err)
		}
		if err := CheckoutBranch(g, branch); err != nil {
			return err
		}
		return nil
	}
	if _, err := g.run("checkout", "--orphan", branch); err != nil {
		return err
	}
	readme := filepath.Join(dir, "README.md")
	body := "# Majordomo inference cache\n\n" +
		"Keyed review (`review/`) and digest (`digest/`) artifacts. Not teaching content.\n"
	if err := os.WriteFile(readme, []byte(body), 0o644); err != nil {
		return err
	}
	if _, err := g.run("add", "-A"); err != nil {
		return err
	}
	if _, err := g.run("commit", "-m", "seed inference cache"); err != nil {
		return err
	}
	return nil
}
```

#### mechanicalIdentitySHA (func)

```go
func mechanicalIdentitySHA(rolesYAML string) (string, error) {
	doc, err := parseRolesYAML(rolesYAML)
	if err != nil {
		return "", err
	}
	stable := packageRolesDoc{
		Packages: make([]packageRoleNode, 0, len(doc.Packages)),
		Edges:    append([]packageRoleEdge(nil), doc.Edges...),
	}
	for _, n := range doc.Packages {
		stable.Packages = append(stable.Packages, packageRoleNode{
			Path:           n.Path,
			Role:           n.Role,
			Confidence:     n.Confidence,
			InspectedStage: n.InspectedStage,
			CandidateRole:  n.CandidateRole,
			MechanicalRole: n.MechanicalRole,
			Agreement:      n.Agreement,
			Language:       n.Language,
		})
	}
	yamlOut, err := mechanicalPreClusterYAML(stable)
	if err != nil {
		return "", err
	}
	return cache.ContentSHA(yamlOut), nil
}
```

#### mechanicalPreClusterYAML (func)

```go
func mechanicalPreClusterYAML(doc packageRolesDoc) (string, error) {
	topo := roles.Topology{
		Packages: make([]roles.Node, 0, len(doc.Packages)),
		Edges:    make([]roles.Edge, 0, len(doc.Edges)),
	}
	for _, n := range doc.Packages {
		topo.Packages = append(topo.Packages, roles.Node{
			Path:           n.Path,
			Role:           n.Role,
			Confidence:     n.Confidence,
			Evidence:       n.Evidence,
			InspectedStage: n.InspectedStage,
			CandidateRole:  n.CandidateRole,
		})
	}
	for _, e := range doc.Edges {
		topo.Edges = append(topo.Edges, roles.Edge{
			From: e.From,
			To:   e.To,
			Kind: e.Kind,
		})
	}
	raw, err := yaml.Marshal(roles.BuildGrouping(topo))
	if err != nil {
		return "", fmt.Errorf("mechanical_grouping encode: %w", err)
	}
	return string(raw), nil
}
```

#### mergesFromClusterOut (func)

```go
func mergesFromClusterOut(out map[string]interface{}) ([]proposedMerge, error) {
	idsRaw := strings.TrimSpace(stringField(out, "merge_ids"))
	pkgsRaw := strings.TrimSpace(stringField(out, "merge_packages"))
	intentsRaw := strings.TrimSpace(stringField(out, "merge_intents"))

	// Backward compatible: accept []string from older stubs / XML array parse.
	if idsRaw == "" {
		idsRaw = strings.Join(stringListField(out, "merge_ids"), ",")
	}
	if pkgsRaw == "" {
		pkgsRaw = strings.Join(stringListField(out, "merge_packages"), ";")
	}
	if intentsRaw == "" {
		intentsRaw = strings.Join(stringListField(out, "merge_intents"), ",")
	}

	if isMergeNoneSentinel(idsRaw) && isMergeNoneSentinel(pkgsRaw) && isMergeNoneSentinel(intentsRaw) {
		return nil, nil
	}
	if isMergeNoneSentinel(idsRaw) || isMergeNoneSentinel(pkgsRaw) || isMergeNoneSentinel(intentsRaw) {
		return nil, fmt.Errorf("merge_ids / merge_packages / merge_intents must all be none or all carry the same number of rows")
	}

	ids := splitCommaTokens(idsRaw)
	pkgGroups := splitSemicolonGroups(pkgsRaw)
	intents := splitCommaTokens(intentsRaw)
	if len(ids) == 0 && len(pkgGroups) == 0 && len(intents) == 0 {
		return nil, nil
	}
	if len(ids) != len(pkgGroups) || len(ids) != len(intents) {
		return nil, fmt.Errorf("merge_ids (%d), merge_packages (%d), merge_intents (%d) must be the same length",
			len(ids), len(pkgGroups), len(intents))
	}
	rows := make([]proposedMerge, 0, len(ids))
	seenIDs := make(map[string]struct{}, len(ids))
	for i := range ids {
		id := strings.TrimSpace(ids[i])
		if id == "" || strings.EqualFold(id, "none") {
			return nil, fmt.Errorf("merge_ids[%d] is empty", i)
		}
		if _, ok := seenIDs[id]; ok {
			return nil, fmt.Errorf("merge_ids duplicate id %q", id)
		}
		seenIDs[id] = struct{}{}
		pkgList := normalizePackageList(splitCommaPackages(pkgGroups[i]))
		if len(pkgList) == 0 {
			return nil, fmt.Errorf("merge %q has no packages", id)
		}
		intent := coerceMergeIntent(intents[i])
		rows = append(rows, proposedMerge{ID: id, Packages: pkgList, Intent: intent})
	}
	if len(rows) > maxClusterAuditMerges {
		return nil, fmt.Errorf("cluster merge proposal has %d rows; max is %d", len(rows), maxClusterAuditMerges)
	}
	return rows, nil
}
```

#### moduleScopeExists (func)

```go
func moduleScopeExists(scope string, modules []string) bool {
	scope = filepath.ToSlash(strings.TrimSpace(scope))
	for _, mod := range modules {
		if filepath.ToSlash(mod) == scope {
			return true
		}
	}
	return false
}
```

#### mustParseRoles (func)

```go
func mustParseRoles(raw string) packageRolesDoc {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return packageRolesDoc{}
	}
	var doc packageRolesDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return packageRolesDoc{}
	}
	return doc
}
```

#### openJourneyNoFindings (func)

```go
func openJourneyNoFindings(journey string) string {
	j := strings.TrimSpace(journey)
	if j == "" {
		return "# Journey\n\n## Status\n\nOpen. No post-refine architecture findings.\n\n## Technical debt and boundary violations\n\nNone.\n"
	}
	return j
}
```

#### ownerSetIndex.contains (method)

```go
func (i ownerSetIndex) contains(key string) bool {
	_, ok := i[key]
	return ok
}
```

#### packageJudgeGenerator (type)

```go
type packageJudgeGenerator struct{}
```

#### parseBootstrapStoryMarkdownAnswer (func)

```go
func parseBootstrapStoryMarkdownAnswer(answer string) (string, error) {
	body := strings.TrimSpace(stripCodeFence(answer))
	if body == "" {
		return "", fmt.Errorf("empty bootstrap story answer")
	}
	var doc struct {
		Markdown string `yaml:"markdown"`
	}
	if err := yaml.Unmarshal([]byte(body), &doc); err == nil && strings.TrimSpace(doc.Markdown) != "" {
		return strings.TrimSpace(doc.Markdown), nil
	}
	// Models sometimes leak the old YAML envelope with an unindented body that
	// yaml.Unmarshal cannot recover. Strip common shapes, then fail closed.
	stripped := strings.TrimSpace(stripLeakedStoryMarkdownEnvelope(body))
	if stripped == "" || looksLikeStoryMarkdownEnvelope(stripped) {
		return "", fmt.Errorf("bootstrap story answer looks like a YAML/markdown envelope; return the raw markdown body for this section (start with a heading)")
	}
	if strings.Contains(stripped, "\n") || strings.HasPrefix(stripped, "#") {
		return stripped, nil
	}
	return "", fmt.Errorf("bootstrap story answer missing markdown body")
}
```

#### parseCapabilityConstraintsYAML (func)

```go
func parseCapabilityConstraintsYAML(raw string) (packageCapabilityConstraintsDoc, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return packageCapabilityConstraintsDoc{}, fmt.Errorf("capability constraints YAML is empty")
	}
	var doc packageCapabilityConstraintsDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return packageCapabilityConstraintsDoc{}, fmt.Errorf("capability constraints decode: %w", err)
	}
	return doc, nil
}
```

#### parseFindingFingerprint (func)

```go
func parseFindingFingerprint(body string) string {
	m := findingMarkerRE.FindStringSubmatch(body)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}
```

#### parseRLMRoleAnswer (func)

```go
func parseRLMRoleAnswer(text string) (role, evidence string) {
	if r, e, ok := parseRLMRoleAnswerYAML(text); ok {
		return r, e
	}
	lower := strings.ToLower(text)
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trim), "evidence:") {
			evidence = strings.TrimSpace(trim[len("evidence:"):])
		}
	}
	if m := roleTokenRE.FindStringSubmatch(text); len(m) == 3 {
		role = normalizeObservedRole(m[2])
	}
	if role == "" {
		for _, cand := range []string{
			roleEntrypoint, roleHTTPSurface, roleDTO, roleExecRunner, roleAggregator,
			roleAdapter, roleConfig, roleObservability, roleUnknown,
		} {
			if strings.Contains(lower, "role: "+cand) || strings.Contains(lower, "role="+cand) {
				role = cand
				break
			}
		}
	}
	if role == "" {
		role = roleUnknown
	}
	return role, evidence
}
```

#### proposedMerge (type)

```go
type proposedMerge struct {
	ID       string   `yaml:"id"`
	Packages []string `yaml:"packages"`
	Intent   string   `yaml:"intent"` // slice | nickname
}
```

#### readRewriteMeta (func)

```go
func readRewriteMeta(dir string) (contextstore.Meta, error) {
	return contextstore.ParseMeta(filepath.Join(dir, "meta.yaml"))
}
```

#### resolveToken (func)

```go
func resolveToken(cfg config.RepoConfig, scm, owner string) string {
	if v := config.ResolveCredential(cfg.Repository.ID, scm, owner); v != "" {
		return v
	}
	if strings.ToLower(scm) == "bitbucket" {
		return strings.TrimSpace(os.Getenv("BITBUCKET_TOKEN"))
	}
	return ""
}
```

#### rewriteChronology (func)

```go
func rewriteChronology(path string, events []contextstore.ChronologyEvent) error {
	var b strings.Builder
	b.WriteString("# Chronology\n\nNewest first.\n")
	for _, ev := range events {
		if ev.Date.IsZero() {
			continue
		}
		if strings.TrimSpace(ev.Actor) == "" {
			ev.Actor = "majordomo"
		}
		if strings.TrimSpace(ev.Source) == "" {
			ev.Source = "compaction"
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "### %s - %s - %s\n\n", ev.Date.Format("2006-01-02"), ev.Actor, ev.Source)
		fmt.Fprintf(&b, "- **Did:** %s\n", ev.Did)
		fmt.Fprintf(&b, "- **Because:** %s\n", ev.Because)
		fmt.Fprintf(&b, "- **In order to:** %s\n", ev.InOrderTo)
		fmt.Fprintf(&b, "- **Evidence:** %s\n", ev.Evidence)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
```

#### rlmBootstrapStoryGenerator (type)

```go
type rlmBootstrapStoryGenerator struct {
	caller bootstrapStoryCaller
}
```

#### rlmCompleteAdapter (type)

```go
type rlmCompleteAdapter struct {
	complete func(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error)
}
```

#### rolesIdentitySHA (func)

```go
func rolesIdentitySHA(rolesYAML string) string {
	doc, err := parseRolesYAML(rolesYAML)
	if err != nil {
		return cache.ContentSHA(rolesYAML)
	}
	parts := make([]string, 0, len(doc.Packages)*6+len(doc.Edges)*3)
	pkgs := append([]packageRoleNode(nil), doc.Packages...)
	sort.Slice(pkgs, func(i, j int) bool {
		return normalizeRolePath(pkgs[i].Path) < normalizeRolePath(pkgs[j].Path)
	})
	for _, n := range pkgs {
		parts = append(parts,
			normalizeRolePath(n.Path),
			strings.TrimSpace(n.Role),
			strings.TrimSpace(n.MechanicalRole),
			strings.TrimSpace(n.Agreement),
			strings.TrimSpace(n.CandidateRole),
			strconv.FormatFloat(n.Confidence, 'f', 4, 64),
			strconv.Itoa(n.InspectedStage),
		)
	}
	edges := append([]packageRoleEdge(nil), doc.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		a := edges[i].From + "\x00" + edges[i].To + "\x00" + edges[i].Kind
		b := edges[j].From + "\x00" + edges[j].To + "\x00" + edges[j].Kind
		return a < b
	})
	for _, e := range edges {
		parts = append(parts, e.From, e.To, e.Kind)
	}
	return cache.HashDigestParts(parts...)
}
```

#### runClusterMergeAudit (func)

```go
func runClusterMergeAudit(ctx context.Context, caller clusterAuditCaller, req clusterAuditRequest) (clusterAuditResult, error) {
	out := clusterAuditResult{}
	if auditor, ok := caller.(interface{ TraceDir() string }); ok {
		out.TraceDir = auditor.TraceDir()
	}
	if len(req.Proposed) == 0 {
		out.Verdicts = append([]clusterMergeVerdict(nil), req.Frozen...)
		return out, nil
	}

	fp := cache.ClusterAuditFingerprint{
		MergesHash:      cache.ContentSHA(formatProposedMergesForAudit(req.Proposed)),
		RolesHash:       cache.ContentSHA(req.RolesYAML),
		ConstraintsHash: cache.ContentSHA(req.Constraints),
		MechanicalHash:  cache.ContentSHA(req.MechanicalYAML),
		ModelID:         req.DigestModelID,
		PromptVersion:   cache.DigestClusterAuditPromptV1,
		SchemaVersion:   cache.DigestClusterAuditSchemaV1,
	}
	if req.DigestSkips && req.DigestCache != nil {
		if hit, ok, err := req.DigestCache.LookupClusterAudit(fp); err == nil && ok {
			req.DigestCache.RecordClusterAuditHit(hit.PromptTokens, hit.CompletionTokens, hit.TotalTokens)
			logf("INFO", "digest cache hit cluster_audit merges=%d", len(req.Proposed))
			cachedRows, alignErr := alignClusterAuditRows(fromCachedVerdicts(hit.Merges), req.Proposed)
			if alignErr != nil {
				logf("WARN", "digest cache cluster_audit realign failed: %v", alignErr)
			} else {
				out.Verdicts = append(append([]clusterMergeVerdict(nil), req.Frozen...), cachedRows...)
				out.RLMIterations = hit.RLMIterations
				out.PromptTokens = hit.PromptTokens
				out.CompletionTok = hit.CompletionTokens
				out.TotalTokens = hit.TotalTokens
				return out, nil
			}
		}
	}

	ctxMD := buildClusterAuditContext(req)
	query := formatClusterAuditQuery(req.Proposed, req.Frozen)
	start := time.Now()
	answer, iters, promptTok, completionTok, totalTok, err := caller.Complete(ctx, ctxMD, query)
	out.Duration = time.Since(start)
	out.RLMIterations = iters
	out.PromptTokens = promptTok
	out.CompletionTok = completionTok
	out.TotalTokens = totalTok
	if req.DigestCache != nil {
		req.DigestCache.RecordClusterAuditMiss()
	}
	if err != nil {
		return out, fmt.Errorf("typology_slice_grouping_audit RLM: %w", err)
	}
	audited, parseErr := parseClusterAuditAnswer(answer, req.Proposed)
	if parseErr != nil {
		return out, parseErr
	}
	if req.DigestCache != nil {
		if err := req.DigestCache.StoreClusterAudit(fp, cache.ClusterAuditCached{
			Merges:           toCachedVerdicts(audited),
			RLMIterations:    iters,
			PromptTokens:     promptTok,
			CompletionTokens: completionTok,
			TotalTokens:      totalTok,
		}); err != nil {
			logf("WARN", "digest cache store cluster_audit failed: %v", err)
		}
	}
	out.Verdicts = append(append([]clusterMergeVerdict(nil), req.Frozen...), audited...)
	logf("INFO", "typology_slice_grouping_audit attempt=%d rows=%d iterations=%d duration_ms=%d tokens=%d",
		req.Attempt, len(req.Proposed), iters, out.Duration.Milliseconds(), totalTok)
	return out, nil
}
```

#### runLocalSeed (func)

```go
func runLocalSeed(opts Options, cfg config.RepoConfig, now time.Time) (Result, error) {
	servedGit := &Git{Dir: opts.WorkDir}
	sourceSHA, err := servedGit.trim("rev-parse", "HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("local seed resolve workdir HEAD: %w", err)
	}

	ws, err := OpenLocalSeedWorkspace(opts.LocalSeedDir, opts.RepoID, sourceSHA, opts.ModuleScope, opts.AllowSourceMove, now)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = ws.Release() }()

	fromStage := normalizeResumeStage(opts.FromStage)
	if fromStage == "" {
		if ws.IsNew() {
			fromStage = LocalStageSurvey
		} else {
			fromStage = nextLocalStage(ws.Manifest.CompletedStage)
			if fromStage == "" {
				logf("INFO", "local seed workspace already complete at %s", ws.Manifest.CompletedStage)
				return Result{
					Action:         "local",
					DefaultHEAD:    sourceSHA,
					Message:        "local seed workspace already complete",
					WorkStoryDir:   chooseLocalWorkStory(opts, ws),
					LocalSeedDir:   ws.Root,
					CacheMode:      "local",
					SourceSHA:      ws.Manifest.SourceSHA,
					CompletedStage: ws.Manifest.CompletedStage,
					LocalOutDir:    ws.ContextDir(),
				}, nil
			}
		}
	}
	if fromStage == LocalStageSurvey && !ws.IsNew() && strings.TrimSpace(ws.Manifest.CompletedStage) != "" {
		return Result{}, fmt.Errorf("--from-stage survey only valid for a new workspace (completed=%q); use a fresh --local-seed-dir", ws.Manifest.CompletedStage)
	}
	if err := localStageReady(ws.Manifest.CompletedStage, fromStage); err != nil {
		return Result{}, err
	}

	if strings.TrimSpace(opts.WorkStoryDir) == "" {
		opts.WorkStoryDir = ws.WorkStoryDir()
	}
	closeTrace, err := prepareWorkStory(&opts, now)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = closeTrace() }()

	if err := ensureDigestJudge(&opts, cfg); err != nil {
		return Result{}, err
	}

	store := &cache.DigestStore{Dir: ws.CacheDir()}
	opts.DigestCache = store
	opts.DigestSkips = cfg.Cache.SkipsEnabled()
	opts.DigestModelID = digestModelID(cfg)
	logf("INFO", "cache_mode=local context_publish=disabled dir=%s skips=%v model=%s", ws.CacheDir(), opts.DigestSkips, opts.DigestModelID)
	defer func() {
		if ferr := store.Flush(); ferr != nil {
			logf("WARN", "local digest cache flush: %v", ferr)
		}
		logf("INFO", "%s", cache.FormatStatsLine(store.Stats()))
	}()

	ctxDir := ws.ContextDir()
	if ws.IsNew() || !treeHasRequiredContext(ctxDir) {
		if err := contextstore.Bootstrap(ctxDir, opts.RepoID, sourceSHA, now); err != nil {
			return Result{}, fmt.Errorf("local seed bootstrap context: %w", err)
		}
	}

	logf("INFO", "local_seed dir=%s source=%s from_stage=%s completed=%s (no forge token, no context/cache push)",
		ws.Root, shortSHA(sourceSHA), fromStage, ws.Manifest.CompletedStage)

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	analysisDir, err := cloneAnalysisRepoFn(ctx, opts.WorkDir)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(analysisDir)

	if fromStage != LocalStageSurvey {
		if err := ws.RestoreAnalysisDrafts(analysisDir); err != nil {
			return Result{}, err
		}
	}

	runErr := runLocalStages(ctx, ws, opts, cfg, analysisDir, sourceSHA, now, fromStage)
	if runErr != nil {
		_ = ws.RecordFailure(fromStage, runErr, now)
		return Result{}, runErr
	}

	diffBody := fmt.Sprintf("local seed complete repo=%s source=%s completed=%s\n", opts.RepoID, sourceSHA, ws.Manifest.CompletedStage)
	_ = ws.WriteLocalDiff(diffBody)

	return Result{
		Action:         "local",
		DefaultHEAD:    sourceSHA,
		Message:        fmt.Sprintf("local seed complete at stage %s", ws.Manifest.CompletedStage),
		WorkStoryDir:   opts.WorkStoryDir,
		LocalSeedDir:   ws.Root,
		CacheMode:      "local",
		SourceSHA:      ws.Manifest.SourceSHA,
		CompletedStage: ws.Manifest.CompletedStage,
		FromStage:      fromStage,
		LocalOutDir:    ws.ContextDir(),
	}, nil
}
```

#### runTypology (func)

```go
func runTypology(ctx context.Context, binary, dir, command, moduleScope string, extra ...string) error {
	args := []string{command, dir}
	if strings.TrimSpace(moduleScope) != "" {
		args = append(args, "--module", moduleScope)
	}
	args = append(args, extra...)
	_, err := runTypologyCapture(ctx, binary, dir, args...)
	return err
}
```

#### runTypologyCapture (func)

```go
func runTypologyCapture(ctx context.Context, binary, dir string, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("run typology: args required")
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		command := args[0]
		if producedBootstrapOutput(dir, command, args...) {
			logf("WARN", "typology %s exited non-zero after writing output: %s", command, text)
			return text, nil
		}
		return text, fmt.Errorf("run typology %s: %w\n%s", command, err, text)
	}
	return text, nil
}
```

#### saveFindingCommentsSidecar (func)

```go
func saveFindingCommentsSidecar(ctxDir string, s FindingCommentsSidecar) error {
	if s.ByFingerprint == nil {
		s.ByFingerprint = map[string]string{}
	}
	path := filepath.Join(ctxDir, findingCommentsRel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("finding comments sidecar mkdir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("finding comments sidecar encode: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
```

#### shortSHA (func)

```go
func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}
```

#### sliceCatalogAssembleRequest (type)

```go
type sliceCatalogAssembleRequest struct {
	FoldedTypo         catalog.Typology
	LedgerDoc          sliceObjectiveLedgerDoc
	EvidenceDir        string
	RolesYAML          string
	ConstraintsYAML    string
	ReadmeSnapshot     string
	ValidationFeedback string
	Kept               map[string]catalog.Slice
}
```

#### sliceCatalogAssembleResult (type)

```go
type sliceCatalogAssembleResult struct {
	Fragments     []catalog.Slice
	Issues        []string
	PromptTokens  int
	CompletionTok int
	TotalTokens   int
	RLMIterations int
	Duration      time.Duration
}
```

#### sliceCatalogAssembler (type)

```go
type sliceCatalogAssembler interface {
	AssembleSlices(ctx context.Context, req sliceCatalogAssembleRequest) (sliceCatalogAssembleResult, error)
}
```

#### sliceLedgerBuildRequest (type)

```go
type sliceLedgerBuildRequest struct {
	AnalysisDir     string
	EvidenceDir     string
	DraftTypo       catalog.Typology
	Constraints     packageCapabilityConstraintsDoc
	ClusterHintYAML string
	DigestCache     *cache.DigestStore
	DigestSkips     bool
	DigestModelID   string
}
```

#### sliceObjectiveLedgerBuilder (type)

```go
type sliceObjectiveLedgerBuilder interface {
	BuildSliceLedger(ctx context.Context, req sliceLedgerBuildRequest) (sliceObjectiveLedgerDoc, []string, error)
}
```

#### sliceObjectiveLedgerDoc (type)

```go
type sliceObjectiveLedgerDoc struct {
	Slices []sliceObjectiveLedgerEntry `yaml:"slices"`
}
```

#### splitOwnerName (func)

```go
func splitOwnerName(cloneURL string) (owner, name string) {
	path := strings.TrimSuffix(cloneURL, ".git")
	path = strings.TrimPrefix(path, "https://")
	path = strings.TrimPrefix(path, "http://")
	if i := strings.Index(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i], path[i+1:]
	}
	return "", path
}
```

#### stickyVerdictMap (type)

```go
type stickyVerdictMap map[string]clusterMergeVerdict
```

#### stickyVerdictMap.get (method)

```go
func (m stickyVerdictMap) get(packages []string) (clusterMergeVerdict, bool) {
	if m == nil {
		return clusterMergeVerdict{}, false
	}
	v, ok := m[packageSetKey(packages)]
	return v, ok
}
```

#### stickyVerdictMap.put (method)

```go
func (m stickyVerdictMap) put(v clusterMergeVerdict) {
	if m == nil {
		return
	}
	key := packageSetKey(v.Packages)
	if key == "" {
		return
	}
	v.Packages = normalizePackageList(v.Packages)
	m[key] = v
}
```

#### stringField (func)

```go
func stringField(out map[string]interface{}, key string) string {
	text, ok := out[key].(string)
	if !ok {
		return ""
	}
	return text
}
```

#### stropBootstrapStoryRLM (type)

```go
type stropBootstrapStoryRLM struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
	}
}
```

#### stropClusterMergeAuditor (type)

```go
type stropClusterMergeAuditor struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
	}
	traceDir string
}
```

#### stropPackageRoleRLM (type)

```go
type stropPackageRoleRLM struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
	}
}
```

#### stropSliceCatalogRLM (type)

```go
type stropSliceCatalogRLM struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
	}
	traceDir string
}
```

#### stropSliceObjectiveLedgerRLM (type)

```go
type stropSliceObjectiveLedgerRLM struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
	}
}
```

#### stubSliceLedgerCaller (type)

```go
type stubSliceLedgerCaller struct {
	answer string
	err    error
}
```

#### surveyWithTypologyPythonOnly (func)

```go
func surveyWithTypologyPythonOnly(ctx context.Context, input BootstrapSurveyInput) error {
	version, err := typologyVersion(ctx, input.TypologyBinary, input.AnalysisDir)
	if err != nil {
		return err
	}

	contractsLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "package_contracts.md")
	if err := os.MkdirAll(filepath.Dir(contractsLocal), 0o755); err != nil {
		return err
	}
	// Empty module scope: typology Harvest detects Python roots without go.mod.
	if err := runTypology(ctx, input.TypologyBinary, input.AnalysisDir, "contracts", "",
		"--out", contractsLocal); err != nil {
		return err
	}

	contractsPath := filepath.Join(input.EvidenceDir, "package_contracts.md")
	if err := copyFile(contractsLocal, contractsPath); err != nil {
		return fmt.Errorf("bootstrap survey: copy package contracts: %w", err)
	}
	rolesLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", packageRolesRel)
	if _, err := os.Stat(rolesLocal); err != nil {
		return fmt.Errorf("bootstrap survey: package roles missing after contracts: %w", err)
	}
	rolesPath := filepath.Join(input.EvidenceDir, packageRolesRel)
	if err := copyFile(rolesLocal, rolesPath); err != nil {
		return fmt.Errorf("bootstrap survey: copy package roles: %w", err)
	}
	if err := ensureRolesNonEmpty(rolesPath); err != nil {
		return fmt.Errorf("bootstrap survey: python harvest: %w", err)
	}
	rlmLocal := filepath.Join(input.AnalysisDir, "tmp", "typology", "package_rlm_context.md")
	rlmPath := filepath.Join(input.EvidenceDir, "package_rlm_context.md")
	if _, err := os.Stat(rlmLocal); err == nil {
		if err := copyFile(rlmLocal, rlmPath); err != nil {
			return fmt.Errorf("bootstrap survey: copy package RLM context: %w", err)
		}
	}

	archDraft := filepath.Join(input.AnalysisDir, "tmp", "typology", "architecture_draft.md")
	if err := os.MkdirAll(filepath.Dir(archDraft), 0o755); err != nil {
		return err
	}
	brief := "# Architecture\n\nPresent-tense snapshot seeded from Typology Python package harvest.\n\n"
	if err := os.WriteFile(archDraft, []byte(brief), 0o644); err != nil {
		return err
	}
	if err := polishTypologyArchitectureBrief(archDraft); err != nil {
		return err
	}
	if err := prependObservedRolesBrief(archDraft, rolesPath); err != nil {
		return err
	}
	if err := copyFile(archDraft, filepath.Join(input.EvidenceDir, contextstore.TypologyArchitectureBriefPath)); err != nil {
		return fmt.Errorf("bootstrap survey: copy architecture brief: %w", err)
	}

	manifest := contextstore.TypologyManifest{
		RepoID:                input.RepoID,
		SourceSHA:             input.SourceSHA,
		GeneratedAt:           input.GeneratedAt.UTC().Format(time.RFC3339),
		TypologyVersion:       strings.TrimSpace(version),
		Mode:                  contextstore.TypologyModeDiscover,
		SnapshotPath:          "snapshot.yaml",
		ArchitecturePath:      contextstore.TypologyArchitectureBriefPath,
		RefineStatus:          contextstore.TypologyRefinePending,
		PackageContractsPath:  "package_contracts.md",
		PackageRolesPath:      packageRolesRel,
		PackageRLMContextPath: "package_rlm_context.md",
		ClusterProposalPath:   "slice_grouping_proposal.yaml",
		RefinedSnapshotPath:   refinedSnapshotRel,
		JourneyPath:           "journey.md",
	}
	return writeTypologyManifest(input.EvidenceDir, manifest)
}
```

#### touchStorySections (func)

```go
func touchStorySections(ctxDir string, cc CommitContext) error {
	if len(cc.Files) == 0 {
		return nil
	}
	for _, name := range []string{"weaknesses.md"} {
		if !anyPathMatches(cc.Files, []string{"**/*.go", "**/*.py", "**/internal/**"}) {
			continue
		}
		path := filepath.Join(ctxDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		note := fmt.Sprintf("\n- Observed in `%s`: %s\n", shortSHA(cc.SHA), cc.Subject)
		if !strings.Contains(string(data), cc.SHA) {
			if err := os.WriteFile(path, append(data, []byte(note)...), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
```

#### typologyVersion (func)

```go
func typologyVersion(ctx context.Context, binary, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, "version")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run typology version: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
```

#### validateBootstrapStoryEvidenceMap (func)

```go
func validateBootstrapStoryEvidenceMap(input BootstrapStoryInput) error {
	fields := map[string]any{
		"repo_id":         input.RepoID,
		"readme_snapshot": input.ReadmeSnapshot,
	}
	required := []string{"repo_id", "readme_snapshot"}
	if strings.TrimSpace(input.TypologyRefinedCatalog) != "" || strings.TrimSpace(input.TypologySliceObjectiveLedger) != "" {
		fields["typology_refined_catalog"] = input.TypologyRefinedCatalog
		fields["slice_meaning_ledger"] = input.TypologySliceObjectiveLedger
		required = append(required, "typology_refined_catalog", "slice_meaning_ledger")
	}
	return stropvalidation.ValidateRequiredInputs(required)(context.Background(), fields, nil)
}
```

#### validateBootstrapStoryOutput (func)

```go
func validateBootstrapStoryOutput(out BootstrapStoryOutput) error {
	for name, text := range map[string]string{
		"readme_md":       out.ReadmeMD,
		"mission_md":      out.MissionMD,
		"architecture_md": out.ArchitectureMD,
		"conventions_md":  out.ConventionsMD,
		"weaknesses_md":   out.WeaknessesMD,
		"chronology_md":   out.ChronologyMD,
		"grounding_md":    out.GroundingMD,
	} {
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("bootstrap story output %s is required", name)
		}
	}
	return nil
}
```

#### validateBootstrapStorySection (func)

```go
func validateBootstrapStorySection(input BootstrapStoryInput, id, text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("bootstrap story output %s is required", id)
	}
	if err := rejectMajordomoAsProduct(input.RepoID, id, text); err != nil {
		return err
	}
	if id == "architecture" {
		return validateArchitectureGroundedObjectives(input, text)
	}
	return nil
}
```

#### validateDigestModeOptions (func)

```go
func validateDigestModeOptions(opts Options) error {
	if isLocalSeedMode(opts) {
		return validateLocalSeedOptions(opts)
	}
	return validateResumeOptions(opts)
}
```

#### validateJourneyFindings (func)

```go
func validateJourneyFindings(findings []string, journeyMD string) error {
	if len(findings) == 0 {
		return nil
	}
	if journeyStatusClaimsComplete(journeyMD) {
		return fmt.Errorf("human intervention: journey Status must not claim complete while architecture findings remain")
	}
	if err := validateNamedFindingCoverage(findings, "journey debt", journeyMD); err != nil {
		return err
	}
	if !journeyHasDebtTable(journeyMD) {
		return fmt.Errorf("human intervention: journey must include a technical debt table when findings remain")
	}
	return nil
}
```

#### validateNamedFindingCoverage (func)

```go
func validateNamedFindingCoverage(findings []string, name, body string) error {
	if len(findings) == 0 {
		return nil
	}
	lowBody := strings.ToLower(body)
	for _, f := range findings {
		needles := findingCoverageNeedles(f)
		if len(needles) == 0 {
			continue
		}
		matched := false
		for _, needle := range needles {
			if strings.Contains(lowBody, strings.ToLower(needle)) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("human intervention: finding %q missing from %s", f, name)
		}
	}
	return nil
}
```

#### writeFallbackSurvey (func)

```go
func writeFallbackSurvey(input BootstrapSurveyInput) error {
	architecture, err := fallbackArchitecture(input.AnalysisDir)
	if err != nil {
		return err
	}
	architecture = ensureTypologyArchitectureBanner(architecture)
	if err := os.WriteFile(filepath.Join(input.EvidenceDir, contextstore.TypologyArchitectureBriefPath), []byte(architecture), 0o644); err != nil {
		return err
	}
	manifest := contextstore.TypologyManifest{
		RepoID:           input.RepoID,
		SourceSHA:        input.SourceSHA,
		GeneratedAt:      input.GeneratedAt.UTC().Format(time.RFC3339),
		Mode:             contextstore.TypologyModeFallback,
		ArchitecturePath: contextstore.TypologyArchitectureBriefPath,
		RefineStatus:     contextstore.TypologyRefineSkipped,
	}
	return writeTypologyManifest(input.EvidenceDir, manifest)
}
```

#### writeFileAtomic (func)

```go
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
```

#### writeMeta (func)

```go
func writeMeta(dir string, meta contextstore.Meta) error {
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.yaml"), data, 0o644); err != nil {
		return err
	}
	return contextstore.ValidateTree(dir)
}
```


## ./internal/contextgate
- package: `contextgate`
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: Action, ActionDone, ActionIgnore, ActionReject, ActionWhy, Comment, DefaultPrefix, ErrNotImplemented, FileStore, Gate, ParsedComment, Sidecar, Status, StatusBlockedWhy, StatusDone, StatusOpen, StatusRejected
- exportedFuncs: ApplyComments, LoadSidecar, NewGate, ParseComment, RegenOptions, SaveSidecar, SidecarPath, SyncFromComments
- exportedMethods: FileStore.AppendStatus, FileStore.Load, FileStore.Save, Gate.NormalizeReject, Sidecar.ReadyToMerge, Sidecar.RegenRequested
- unexportedDecls: evalRow, sidecarName
- unexportedFuncs: (none)
- unexportedMethods: (none)
- errorTypes: ErrNotImplemented
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextgate/gate.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextgate/parse.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextgate/state.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextgate/store.go

### Exported bodies

#### RegenOptions (func)

```go
func RegenOptions(reason string) (regenerate.RegenerateOptions, error) {
	return humanreview.RegenOptionsFromComment(context.Background(), humanreview.PassthroughNormalizer{}, reason)
}
```

#### Gate (type)

```go
type Gate struct {
	prefix string
}
```

#### NewGate (func)

```go
func NewGate(prefix string) *Gate {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return &Gate{prefix: prefix}
}
```

#### Gate.NormalizeReject (method)

```go
func (g *Gate) NormalizeReject(reason string) (string, error) {
	opts, err := RegenOptions(reason)
	if err != nil {
		return "", err
	}
	if opts.Message == "" {
		return "", fmt.Errorf("empty reject reason")
	}
	return opts.Message, nil
}
```

#### Action (type)

```go
type Action int
```

#### Comment (type)

```go
type Comment struct {
	Body     string
	Author   string
	PostedAt string
}
```

#### ParsedComment (type)

```go
type ParsedComment struct {
	Action  Action
	Payload string // reason for reject/why; empty for done
}
```

#### ParseComment (func)

```go
func ParseComment(body, prefix string) ParsedComment {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	line := strings.TrimSpace(body)
	if !strings.HasPrefix(line, prefix) {
		return ParsedComment{Action: ActionIgnore}
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	switch {
	case strings.HasPrefix(rest, "reject "):
		return ParsedComment{Action: ActionReject, Payload: strings.TrimSpace(strings.TrimPrefix(rest, "reject "))}
	case rest == "done" || strings.HasPrefix(rest, "done "):
		return ParsedComment{Action: ActionDone}
	case strings.HasPrefix(rest, "why "):
		return ParsedComment{Action: ActionWhy, Payload: strings.TrimSpace(strings.TrimPrefix(rest, "why "))}
	default:
		return ParsedComment{Action: ActionIgnore}
	}
}
```

#### ApplyComments (func)

```go
func ApplyComments(comments []Comment, prefix string) (rejectReason string, done bool, why string) {
	for _, c := range comments {
		p := ParseComment(c.Body, prefix)
		switch p.Action {
		case ActionReject:
			if strings.TrimSpace(p.Payload) != "" {
				rejectReason = p.Payload
				done = false
			}
		case ActionDone:
			done = true
		case ActionWhy:
			if strings.TrimSpace(p.Payload) != "" {
				why = p.Payload
			}
		}
	}
	return rejectReason, done, why
}
```

#### Status (type)

```go
type Status string
```

#### Sidecar (type)

```go
type Sidecar struct {
	Status         Status `json:"status"`
	RejectReason   string `json:"reject_reason,omitempty"`
	ConversationAt string `json:"conversation_at,omitempty"`
	PRNumber       string `json:"pr_number,omitempty"`
	UpdatedAt      string `json:"updated_at"`
}
```

#### SidecarPath (func)

```go
func SidecarPath(dir string) string {
	return filepath.Join(dir, sidecarName)
}
```

#### LoadSidecar (func)

```go
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
```

#### SaveSidecar (func)

```go
func SaveSidecar(dir string, s Sidecar) error {
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(SidecarPath(dir), data, 0o644)
}
```

#### SyncFromComments (func)

```go
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
```

#### Sidecar.RegenRequested (method)

```go
func (s Sidecar) RegenRequested() bool {
	return s.Status == StatusRejected && strings.TrimSpace(s.RejectReason) != ""
}
```

#### Sidecar.ReadyToMerge (method)

```go
func (s Sidecar) ReadyToMerge() bool {
	return s.Status == StatusDone
}
```

#### FileStore (type)

```go
type FileStore struct {
	Path string
}
```

#### FileStore.Load (method)

```go
func (f *FileStore) Load() ([]evalRow, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []evalRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
```

#### FileStore.Save (method)

```go
func (f *FileStore) Save(rows []evalRow) error {
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(f.Path, data, 0o644)
}
```

#### FileStore.AppendStatus (method)

```go
func (f *FileStore) AppendStatus(id, status string) error {
	rows, err := f.Load()
	if err != nil {
		return err
	}
	rows = append(rows, evalRow{ID: id, Status: status})
	return f.Save(rows)
}
```

### Private one-hop bodies

#### evalRow (type)

```go
type evalRow struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
```


## ./internal/contextstore
- package: `contextstore`
- packageDoc: Package contextstore validates the served-repo context branch tree (meta.yaml, chronology.md, and required dossier files).
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: ChronologyEvent, CurrentSchemaVersion, Meta, RequiredFiles, StoryReadingOrder, TypologyAppendixFiles, TypologyArchitectureBriefPath, TypologyManifest, TypologyModeDiscover, TypologyModeFallback, TypologyModeReuse, TypologyReadingIndexPath, TypologyReadingOrder, TypologyRefineComplete, TypologyRefinePending, TypologyRefineSkipped
- exportedFuncs: AppendChronologyEvent, ApplyReadingPath, Bootstrap, EnsureReadingNav, EnsureRootReadingTOC, ParseChronology, ParseChronologyFile, ParseMeta, ParseTypologyManifest, ValidateContextBranch, ValidateMeta, ValidateTree, ValidateTypologyManifest
- exportedMethods: (none)
- unexportedDecls: bulletRE, contextBranchRE, fieldBecause, fieldDid, fieldEvidence, fieldInOrderTo, headingRE, pendingEvent, readingNavEnd, readingNavStart, readingNavTmpl, readingTOCEnd, readingTOCStart, readingTmplFuncs, requiredFields, rootReadingTOCTmpl, typologyReadingIndexTmpl
- unexportedFuncs: applyNavChain, bootstrapAgenting, dirExists, existingRelPaths, fileExists, formatReadingNav, formatRootReadingTOC, insertAfterFirstHeading, mdCode, mustRender, relativeMarkdownLink, stripMarkedSection, stripReadingNav, validateAgenting, validateNewestFirst, validateRelativeEvidencePath, validateTypologyEvidence, validateTypologyEvidenceFile, validateTypologySnapshot, verifyReadingNav, writeTypologyReadingIndex
- unexportedMethods: pendingEvent.finish
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextstore/bootstrap.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextstore/branch.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextstore/chronology.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextstore/chronology_write.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextstore/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextstore/meta.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextstore/reading_path.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextstore/tree.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/contextstore/typology.go

### Exported bodies

#### Bootstrap (func)

```go
func Bootstrap(dir, repoID, cursorSHA string, digestAt time.Time) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("bootstrap: dir is required")
	}
	if strings.TrimSpace(repoID) == "" {
		return fmt.Errorf("bootstrap: repo_id is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("bootstrap mkdir: %w", err)
	}
	ts := digestAt.UTC().Format(time.RFC3339)
	meta := Meta{
		SchemaVersion: CurrentSchemaVersion,
		RepoID:        repoID,
		LastMergedSHA: strings.TrimSpace(cursorSHA),
		LastDigestAt:  ts,
	}
	metaBytes, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("bootstrap meta.yaml: %w", err)
	}
	files := map[string]string{
		"README.md": `# Context branch

This orphan branch holds project understanding for the served repo.
Never merge these files into the default branch.
Context updates are PRs whose base is this branch.

<!-- majordomo-reading-toc:start -->
## Reading order

1. [README.md](README.md) (this file)
2. [mission.md](mission.md)
3. [architecture.md](architecture.md)
4. [conventions.md](conventions.md)
5. [weaknesses.md](weaknesses.md)
6. [chronology.md](chronology.md)
7. ` + "`evidence/typology/`" + ` — appears after a Typology survey seed

<!-- majordomo-reading-toc:end -->

## Story vs Typology evidence

- Root markdown (` + "`mission.md`" + `, ` + "`architecture.md`" + `, …) is the **teaching story**.
- ` + "`evidence/typology/`" + ` holds Typology **seed evidence** for this digest proposal (for example ` + "`architecture_brief.md`" + `). That brief is not the teaching story and not the confirmed ` + "`.typology/`" + ` catalog.
`,
		"meta.yaml": string(metaBytes),
		"mission.md": `# Mission

Describe what this repo exists to do.
`,
		"architecture.md": `# Architecture

> **Teaching story.** Living project architecture for humans and review grounding. Not Typology seed evidence (see ` + "`evidence/typology/architecture_brief.md`" + `).

Describe the high-level shape of the system.
`,
		"conventions.md": `# Conventions

Describe how contributors work in this repo.
`,
		"weaknesses.md": `# Weaknesses

Known gaps and risks worth remembering.
`,
		"chronology.md": `# Chronology

Newest first.
`,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("bootstrap %s: %w", name, err)
		}
	}
	if err := bootstrapAgenting(dir); err != nil {
		return err
	}
	if err := ApplyReadingPath(dir); err != nil {
		return err
	}
	return ValidateTree(dir)
}
```

#### ValidateContextBranch (func)

```go
func ValidateContextBranch(branch string) error {
	if !contextBranchRE.MatchString(branch) {
		return fmt.Errorf("context branch %q must match majordomo-context/<repo-id>", branch)
	}
	return nil
}
```

#### ChronologyEvent (type)

```go
type ChronologyEvent struct {
	Date      time.Time
	Actor     string
	Source    string
	Did       string
	Because   string
	InOrderTo string
	Evidence  string
	Heading   string
}
```

#### ParseChronologyFile (func)

```go
func ParseChronologyFile(path string) ([]ChronologyEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read chronology.md: %w", err)
	}
	defer f.Close()
	events, err := ParseChronology(f)
	if err != nil {
		return nil, err
	}
	return events, nil
}
```

#### ParseChronology (func)

```go
func ParseChronology(r io.Reader) ([]ChronologyEvent, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var events []ChronologyEvent
	var cur *pendingEvent
	lineNo := 0

	flush := func() error {
		if cur == nil {
			return nil
		}
		ev, err := cur.finish()
		if err != nil {
			return err
		}
		events = append(events, ev)
		cur = nil
		return nil
	}

	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), " \t")
		if strings.HasPrefix(line, "### ") {
			if err := flush(); err != nil {
				return nil, err
			}
			m := headingRE.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("chronology.md:%d: heading must be ### YYYY-MM-DD - actor - source", lineNo)
			}
			day, err := time.Parse("2006-01-02", m[1])
			if err != nil {
				return nil, fmt.Errorf("chronology.md:%d: invalid date %q: %w", lineNo, m[1], err)
			}
			cur = &pendingEvent{
				line:    lineNo,
				heading: line,
				date:    day,
				actor:   strings.TrimSpace(m[2]),
				source:  strings.TrimSpace(m[3]),
				fields:  map[string]string{},
			}
			continue
		}
		if cur == nil {
			continue
		}
		if line == "" {
			continue
		}
		bm := bulletRE.FindStringSubmatch(line)
		if bm == nil {
			return nil, fmt.Errorf("chronology.md:%d: expected a Did/Because/In order to/Evidence bullet", lineNo)
		}
		key, val := bm[1], strings.TrimSpace(bm[2])
		if _, ok := cur.fields[key]; ok {
			return nil, fmt.Errorf("chronology.md:%d: duplicate %s", lineNo, key)
		}
		if val == "" {
			return nil, fmt.Errorf("chronology.md:%d: %s is empty", lineNo, key)
		}
		cur.fields[key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read chronology.md: %w", err)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if err := validateNewestFirst(events); err != nil {
		return nil, err
	}
	return events, nil
}
```

#### AppendChronologyEvent (func)

```go
func AppendChronologyEvent(dir string, ev ChronologyEvent) error {
	if strings.TrimSpace(ev.Did) == "" || strings.TrimSpace(ev.Because) == "" ||
		strings.TrimSpace(ev.InOrderTo) == "" || strings.TrimSpace(ev.Evidence) == "" {
		return fmt.Errorf("chronology event requires Did, Because, In order to, and Evidence")
	}
	if ev.Date.IsZero() {
		ev.Date = time.Now().UTC()
	}
	if strings.TrimSpace(ev.Actor) == "" {
		ev.Actor = "majordomo"
	}
	if strings.TrimSpace(ev.Source) == "" {
		ev.Source = "digest"
	}
	heading := fmt.Sprintf("### %s - %s - %s", ev.Date.Format("2006-01-02"), ev.Actor, ev.Source)
	block := heading + "\n\n" +
		fmt.Sprintf("- **Did:** %s\n", ev.Did) +
		fmt.Sprintf("- **Because:** %s\n", ev.Because) +
		fmt.Sprintf("- **In order to:** %s\n", ev.InOrderTo) +
		fmt.Sprintf("- **Evidence:** %s\n", ev.Evidence)

	path := filepath.Join(dir, "chronology.md")
	existing, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read chronology.md: %w", err)
	}
	body := strings.TrimRight(string(existing), " \t\n")
	if !strings.HasPrefix(body, "# Chronology") {
		body = "# Chronology\n\nNewest first.\n"
	}
	var out strings.Builder
	out.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		out.WriteByte('\n')
	}
	out.WriteByte('\n')
	out.WriteString(block)
	if err := os.WriteFile(path, []byte(out.String()), 0o644); err != nil {
		return err
	}
	_, err = ParseChronologyFile(path)
	return err
}
```

#### Meta (type)

```go
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
```

#### ParseMeta (func)

```go
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
```

#### ValidateMeta (func)

```go
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
```

#### EnsureReadingNav (func)

```go
func EnsureReadingNav(md string, prevLabel, prevHref, nextLabel, nextHref, tocHref string) string {
	body := stripReadingNav(md)
	banner := formatReadingNav(prevLabel, prevHref, nextLabel, nextHref, tocHref)
	return insertAfterFirstHeading(body, banner)
}
```

#### EnsureRootReadingTOC (func)

```go
func EnsureRootReadingTOC(md string, typologyPresent bool) string {
	body := stripMarkedSection(md, readingTOCStart, readingTOCEnd)
	section := formatRootReadingTOC(typologyPresent)
	return insertAfterFirstHeading(body, section)
}
```

#### ApplyReadingPath (func)

```go
func ApplyReadingPath(ctxDir string) error {
	if strings.TrimSpace(ctxDir) == "" {
		return fmt.Errorf("reading path: ctx dir is required")
	}
	evidenceDir := filepath.Join(ctxDir, "evidence", "typology")
	typologyPresent := dirExists(evidenceDir)
	if typologyPresent {
		if err := writeTypologyReadingIndex(evidenceDir); err != nil {
			return err
		}
	}

	readmePath := filepath.Join(ctxDir, "README.md")
	if fileExists(readmePath) {
		data, err := os.ReadFile(readmePath)
		if err != nil {
			return fmt.Errorf("reading path read README: %w", err)
		}
		updated := EnsureRootReadingTOC(string(data), typologyPresent)
		if err := os.WriteFile(readmePath, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("reading path write README toc: %w", err)
		}
	}

	storyChain := existingRelPaths(ctxDir, StoryReadingOrder)
	if typologyPresent {
		storyChain = append(storyChain, filepath.ToSlash(filepath.Join("evidence", "typology", TypologyReadingIndexPath)))
	}
	if err := applyNavChain(ctxDir, storyChain, "README.md", false); err != nil {
		return err
	}

	if typologyPresent {
		typoChain := existingRelPaths(evidenceDir, TypologyReadingOrder)
		// Handoff back to the story TOC after the last typology briefing file.
		absChain := make([]string, 0, len(typoChain)+1)
		for _, rel := range typoChain {
			absChain = append(absChain, filepath.ToSlash(filepath.Join("evidence", "typology", rel)))
		}
		absChain = append(absChain, "README.md")
		if err := applyNavChain(ctxDir, absChain, filepath.ToSlash(filepath.Join("evidence", "typology", TypologyReadingIndexPath)), true); err != nil {
			return err
		}
	}

	return verifyReadingNav(ctxDir, typologyPresent)
}
```

#### ValidateTree (func)

```go
func ValidateTree(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("context validate: dir is required")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("context tree %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("context tree %s is not a directory", dir)
	}
	for _, name := range RequiredFiles {
		p := filepath.Join(dir, name)
		st, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("context tree missing %s: %w", name, err)
		}
		if st.IsDir() {
			return fmt.Errorf("context tree %s is a directory, want a file", name)
		}
	}
	meta, err := ParseMeta(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		return err
	}
	if err := ValidateMeta(meta); err != nil {
		return err
	}
	_, err = ParseChronologyFile(filepath.Join(dir, "chronology.md"))
	if err != nil {
		return err
	}
	if err := validateTypologyEvidence(dir); err != nil {
		return err
	}
	return validateAgenting(dir)
}
```

#### TypologyManifest (type)

```go
type TypologyManifest struct {
	RepoID                           string `yaml:"repo_id,omitempty"`
	SourceSHA                        string `yaml:"source_sha"`
	GeneratedAt                      string `yaml:"generated_at,omitempty"`
	TypologyVersion                  string `yaml:"typology_version,omitempty"`
	Mode                             string `yaml:"mode"`
	ModuleScope                      string `yaml:"module_scope,omitempty"`
	SnapshotPath                     string `yaml:"snapshot_path,omitempty"`
	ArchitecturePath                 string `yaml:"architecture_path"`
	RefineStatus                     string `yaml:"refine_status,omitempty"`
	GraphPath                        string `yaml:"graph_path,omitempty"`
	PackageContractsPath             string `yaml:"package_contracts_path,omitempty"`
	PackageRolesPath                 string `yaml:"package_roles_path,omitempty"`
	PackageRLMContextPath            string `yaml:"package_rlm_context_path,omitempty"`
	PackageCapabilityConstraintsPath string `yaml:"package_capability_constraints_path,omitempty"`
	SliceObjectiveClaimsPath         string `yaml:"slice_objective_claims_path,omitempty"`
	SliceObjectiveLedgerPath         string `yaml:"slice_objective_ledger_path,omitempty"`
	ClusterProposalPath              string `yaml:"cluster_proposal_path,omitempty"`
	ClusterMergeVerdictsPath         string `yaml:"cluster_merge_verdicts_path,omitempty"`
	MechanicalGroupingPath           string `yaml:"mechanical_grouping_path,omitempty"`
	RefinedSnapshotPath              string `yaml:"refined_snapshot_path,omitempty"`
	JourneyPath                      string `yaml:"journey_path,omitempty"`
	HumanInterventionPath            string `yaml:"human_intervention_path,omitempty"`
}
```

#### ParseTypologyManifest (func)

```go
func ParseTypologyManifest(path string) (TypologyManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TypologyManifest{}, fmt.Errorf("read typology manifest: %w", err)
	}
	var m TypologyManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return TypologyManifest{}, fmt.Errorf("parse typology manifest: %w", err)
	}
	return m, nil
}
```

#### ValidateTypologyManifest (func)

```go
func ValidateTypologyManifest(m TypologyManifest) error {
	if strings.TrimSpace(m.RepoID) == "" {
		return fmt.Errorf("typology manifest repo_id is required")
	}
	if strings.TrimSpace(m.SourceSHA) == "" {
		return fmt.Errorf("typology manifest source_sha is required")
	}
	if ts := strings.TrimSpace(m.GeneratedAt); ts != "" {
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			return fmt.Errorf("typology manifest generated_at %q is not RFC3339: %w", m.GeneratedAt, err)
		}
	}
	mode := strings.ToLower(strings.TrimSpace(m.Mode))
	switch mode {
	case TypologyModeDiscover, TypologyModeReuse, TypologyModeFallback:
	default:
		return fmt.Errorf("typology manifest mode %q is not supported", m.Mode)
	}
	if err := validateRelativeEvidencePath(m.ArchitecturePath, "architecture_path"); err != nil {
		return err
	}
	if strings.TrimSpace(m.SnapshotPath) != "" {
		if err := validateRelativeEvidencePath(m.SnapshotPath, "snapshot_path"); err != nil {
			return err
		}
	}
	refine := strings.ToLower(strings.TrimSpace(m.RefineStatus))
	switch refine {
	case "", TypologyRefinePending, TypologyRefineComplete, TypologyRefineSkipped:
	default:
		return fmt.Errorf("typology manifest refine_status %q is not supported", m.RefineStatus)
	}
	if mode == TypologyModeFallback {
		return nil
	}
	switch refine {
	case TypologyRefinePending:
		if err := validateRelativeEvidencePath(m.GraphPath, "graph_path"); err != nil {
			return err
		}
		if err := validateRelativeEvidencePath(m.PackageContractsPath, "package_contracts_path"); err != nil {
			return err
		}
		if err := validateRelativeEvidencePath(m.PackageRolesPath, "package_roles_path"); err != nil {
			return err
		}
	case TypologyRefineComplete:
		if strings.TrimSpace(m.SnapshotPath) == "" {
			return fmt.Errorf("typology manifest snapshot_path is required for mode %q", m.Mode)
		}
		for _, pair := range []struct {
			path, field string
		}{
			{m.GraphPath, "graph_path"},
			{m.PackageContractsPath, "package_contracts_path"},
			{m.PackageRolesPath, "package_roles_path"},
			{m.PackageCapabilityConstraintsPath, "package_capability_constraints_path"},
			{m.SliceObjectiveClaimsPath, "slice_objective_claims_path"},
			{m.SliceObjectiveLedgerPath, "slice_objective_ledger_path"},
			{m.ClusterProposalPath, "cluster_proposal_path"},
			{m.RefinedSnapshotPath, "refined_snapshot_path"},
			{m.JourneyPath, "journey_path"},
			{m.HumanInterventionPath, "human_intervention_path"},
		} {
			if err := validateRelativeEvidencePath(pair.path, pair.field); err != nil {
				return err
			}
		}
		if p := strings.TrimSpace(m.ClusterMergeVerdictsPath); p != "" {
			if err := validateRelativeEvidencePath(p, "cluster_merge_verdicts_path"); err != nil {
				return err
			}
		}
		if p := strings.TrimSpace(m.MechanicalGroupingPath); p != "" {
			if err := validateRelativeEvidencePath(p, "mechanical_grouping_path"); err != nil {
				return err
			}
		}
	default:
		// Legacy evidence: snapshot + architecture only.
		if strings.TrimSpace(m.SnapshotPath) == "" {
			return fmt.Errorf("typology manifest snapshot_path is required for mode %q", m.Mode)
		}
	}
	return nil
}
```

### Private one-hop bodies

#### applyNavChain (func)

```go
func applyNavChain(ctxDir string, relChain []string, tocRel string, typologyLinks bool) error {
	for i, rel := range relChain {
		// Terminal handoff to root README is only a Next target for typology files;
		// do not rewrite root README a second time in the typology pass.
		if typologyLinks && rel == "README.md" {
			continue
		}
		// Reading-nav banners are markdown-only; never inject them into YAML evidence.
		if !strings.HasSuffix(strings.ToLower(rel), ".md") {
			continue
		}
		path := filepath.Join(ctxDir, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading path read %s: %w", rel, err)
		}
		var prevLabel, prevHref, nextLabel, nextHref string
		if i > 0 {
			prevHref = relativeMarkdownLink(rel, relChain[i-1])
			prevLabel = filepath.Base(relChain[i-1])
		}
		if i+1 < len(relChain) {
			nextHref = relativeMarkdownLink(rel, relChain[i+1])
			nextLabel = filepath.Base(relChain[i+1])
		}
		tocHref := relativeMarkdownLink(rel, tocRel)
		updated := EnsureReadingNav(string(data), prevLabel, prevHref, nextLabel, nextHref, tocHref)
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("reading path write %s: %w", rel, err)
		}
	}
	return nil
}
```

#### bootstrapAgenting (func)

```go
func bootstrapAgenting(dir string) error {
	indexPath := filepath.Join(dir, "agenting", "index.yaml")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return err
	}
	index := `packs:
  overview:
    modes: [files, summary, technical, digest]
`
	if err := os.WriteFile(indexPath, []byte(index), 0o644); err != nil {
		return fmt.Errorf("bootstrap agenting/index.yaml: %w", err)
	}
	overviewDir := filepath.Join(dir, "agenting", "overview")
	if err := os.MkdirAll(overviewDir, 0o755); err != nil {
		return err
	}
	grounding := `# Overview

High-level project grounding for review. Digest expands this from mission.md and architecture.md.
`
	path := filepath.Join(overviewDir, "GROUNDING.md")
	if err := os.WriteFile(path, []byte(grounding), 0o644); err != nil {
		return fmt.Errorf("bootstrap agenting/overview/GROUNDING.md: %w", err)
	}
	return nil
}
```

#### dirExists (func)

```go
func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
```

#### existingRelPaths (func)

```go
func existingRelPaths(base string, names []string) []string {
	var out []string
	for _, name := range names {
		if fileExists(filepath.Join(base, name)) {
			out = append(out, name)
		}
	}
	return out
}
```

#### fileExists (func)

```go
func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
```

#### formatReadingNav (func)

```go
func formatReadingNav(prevLabel, prevHref, nextLabel, nextHref, tocHref string) string {
	var parts []string
	if strings.TrimSpace(prevHref) != "" {
		label := strings.TrimSpace(prevLabel)
		if label == "" {
			label = prevHref
		}
		parts = append(parts, fmt.Sprintf("[Prev: %s](%s)", label, prevHref))
	}
	if strings.TrimSpace(nextHref) != "" {
		label := strings.TrimSpace(nextLabel)
		if label == "" {
			label = nextHref
		}
		parts = append(parts, fmt.Sprintf("[Next: %s](%s)", label, nextHref))
	}
	toc := strings.TrimSpace(tocHref)
	if toc == "" {
		toc = "README.md"
	}
	parts = append(parts, fmt.Sprintf("[TOC](%s)", toc))
	return mustRender(readingNavTmpl, struct {
		Start string
		End   string
		Trail string
	}{
		Start: readingNavStart,
		End:   readingNavEnd,
		Trail: strings.Join(parts, " · "),
	})
}
```

#### formatRootReadingTOC (func)

```go
func formatRootReadingTOC(typologyPresent bool) string {
	return mustRender(rootReadingTOCTmpl, struct {
		Start           string
		End             string
		TypologyPresent bool
	}{
		Start:           readingTOCStart,
		End:             readingTOCEnd,
		TypologyPresent: typologyPresent,
	})
}
```

#### insertAfterFirstHeading (func)

```go
func insertAfterFirstHeading(md, block string) string {
	body := strings.TrimSpace(md)
	block = strings.TrimSpace(block) + "\n"
	if body == "" {
		return block
	}
	lines := strings.Split(body, "\n")
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "#") {
		var b strings.Builder
		b.WriteString(strings.TrimRight(lines[0], "\r"))
		b.WriteString("\n\n")
		b.WriteString(block)
		if !strings.HasSuffix(block, "\n") {
			b.WriteString("\n")
		}
		if len(lines) > 1 {
			rest := strings.TrimLeft(strings.Join(lines[1:], "\n"), "\n")
			if rest != "" {
				b.WriteString("\n")
				b.WriteString(rest)
				if !strings.HasSuffix(rest, "\n") {
					b.WriteString("\n")
				}
			}
		}
		return b.String()
	}
	return block + "\n" + body + "\n"
}
```

#### pendingEvent (type)

```go
type pendingEvent struct {
	line    int
	heading string
	date    time.Time
	actor   string
	source  string
	fields  map[string]string
}
```

#### pendingEvent.finish (method)

```go
func (p *pendingEvent) finish() (ChronologyEvent, error) {
	for _, key := range requiredFields {
		if strings.TrimSpace(p.fields[key]) == "" {
			return ChronologyEvent{}, fmt.Errorf("chronology.md:%d: missing **%s:**", p.line, key)
		}
	}
	return ChronologyEvent{
		Date:      p.date,
		Actor:     p.actor,
		Source:    p.source,
		Did:       p.fields[fieldDid],
		Because:   p.fields[fieldBecause],
		InOrderTo: p.fields[fieldInOrderTo],
		Evidence:  p.fields[fieldEvidence],
		Heading:   p.heading,
	}, nil
}
```

#### stripMarkedSection (func)

```go
func stripMarkedSection(md, start, end string) string {
	for {
		s := strings.Index(md, start)
		if s < 0 {
			return md
		}
		e := strings.Index(md[s:], end)
		if e < 0 {
			return strings.TrimSpace(md[:s]) + "\n"
		}
		e = s + e + len(end)
		for e < len(md) && (md[e] == '\n' || md[e] == '\r') {
			e++
		}
		md = md[:s] + md[e:]
	}
}
```

#### stripReadingNav (func)

```go
func stripReadingNav(md string) string {
	return stripMarkedSection(md, readingNavStart, readingNavEnd)
}
```

#### validateAgenting (func)

```go
func validateAgenting(dir string) error {
	indexPath := filepath.Join(dir, agenting.IndexRelPath)
	if _, err := os.Stat(indexPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("agenting index: %w", err)
	}
	idx, err := agenting.LoadIndex(dir)
	if err != nil {
		return err
	}
	for _, id := range idx.PackIDs() {
		grounding := filepath.Join(dir, "agenting", id, agenting.GroundingName)
		st, err := os.Stat(grounding)
		if err != nil {
			return fmt.Errorf("agenting pack %q missing %s: %w", id, agenting.GroundingName, err)
		}
		if st.IsDir() {
			return fmt.Errorf("agenting pack %q: %s is a directory", id, agenting.GroundingName)
		}
	}
	return nil
}
```

#### validateNewestFirst (func)

```go
func validateNewestFirst(events []ChronologyEvent) error {
	for i := 1; i < len(events); i++ {
		if events[i].Date.After(events[i-1].Date) {
			return fmt.Errorf("chronology.md: events must be newest first (entry %d is dated after entry %d)", i+1, i)
		}
	}
	return nil
}
```

#### validateRelativeEvidencePath (func)

```go
func validateRelativeEvidencePath(path, field string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return fmt.Errorf("typology manifest %s is required", field)
	}
	if filepath.IsAbs(p) {
		return fmt.Errorf("typology manifest %s must be relative, got %q", field, path)
	}
	clean := filepath.Clean(p)
	if clean == "." || strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+"..") {
		return fmt.Errorf("typology manifest %s must stay within evidence/typology, got %q", field, path)
	}
	return nil
}
```

#### validateTypologyEvidence (func)

```go
func validateTypologyEvidence(dir string) error {
	evidenceDir := filepath.Join(dir, "evidence", "typology")
	st, err := os.Stat(evidenceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("typology evidence: %w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("typology evidence %s is not a directory", evidenceDir)
	}

	readmePath := filepath.Join(evidenceDir, TypologyReadingIndexPath)
	if st, err := os.Stat(readmePath); err != nil || st.IsDir() {
		return fmt.Errorf("typology evidence missing %s reading index", TypologyReadingIndexPath)
	}

	manifestPath := filepath.Join(evidenceDir, "manifest.yaml")
	manifest, err := ParseTypologyManifest(manifestPath)
	if err != nil {
		return err
	}
	if err := ValidateTypologyManifest(manifest); err != nil {
		return err
	}
	if err := validateTypologyEvidenceFile(evidenceDir, manifest.ArchitecturePath); err != nil {
		return err
	}
	if strings.TrimSpace(manifest.SnapshotPath) != "" {
		if err := validateTypologyEvidenceFile(evidenceDir, manifest.SnapshotPath); err != nil {
			return err
		}
		if err := validateTypologySnapshot(filepath.Join(evidenceDir, manifest.SnapshotPath)); err != nil {
			return err
		}
	}
	refine := strings.ToLower(strings.TrimSpace(manifest.RefineStatus))
	if refine == TypologyRefineComplete {
		for _, rel := range []string{
			manifest.GraphPath,
			manifest.PackageContractsPath,
			manifest.PackageRolesPath,
			manifest.PackageCapabilityConstraintsPath,
			manifest.SliceObjectiveClaimsPath,
			manifest.SliceObjectiveLedgerPath,
			manifest.ClusterProposalPath,
			manifest.RefinedSnapshotPath,
			manifest.JourneyPath,
			manifest.HumanInterventionPath,
		} {
			if err := validateTypologyEvidenceFile(evidenceDir, rel); err != nil {
				return err
			}
		}
		if p := strings.TrimSpace(manifest.ClusterMergeVerdictsPath); p != "" {
			if err := validateTypologyEvidenceFile(evidenceDir, p); err != nil {
				return err
			}
		}
		if err := validateTypologySnapshot(filepath.Join(evidenceDir, manifest.RefinedSnapshotPath)); err != nil {
			return err
		}
	}
	if refine == TypologyRefinePending {
		for _, rel := range []string{manifest.GraphPath, manifest.PackageContractsPath, manifest.PackageRolesPath, manifest.PackageRLMContextPath} {
			if strings.TrimSpace(rel) == "" {
				continue
			}
			if err := validateTypologyEvidenceFile(evidenceDir, rel); err != nil {
				return err
			}
		}
	}
	return nil
}
```

#### verifyReadingNav (func)

```go
func verifyReadingNav(ctxDir string, typologyPresent bool) error {
	check := func(rel string) error {
		path := filepath.Join(ctxDir, filepath.FromSlash(rel))
		if !fileExists(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(data), readingNavStart) {
			return fmt.Errorf("reading path: guided file %s missing nav banner", rel)
		}
		return nil
	}
	for _, rel := range StoryReadingOrder {
		// Root README carries the TOC section; nav is still applied and verified.
		if err := check(rel); err != nil {
			return err
		}
	}
	if !typologyPresent {
		return nil
	}
	for _, name := range TypologyReadingOrder {
		if name == "pr_priority.md" && !fileExists(filepath.Join(ctxDir, "evidence", "typology", name)) {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("evidence", "typology", name))
		if !fileExists(filepath.Join(ctxDir, filepath.FromSlash(rel))) {
			continue
		}
		if err := check(rel); err != nil {
			return err
		}
	}
	return nil
}
```

#### writeTypologyReadingIndex (func)

```go
func writeTypologyReadingIndex(evidenceDir string) error {
	appendix := make([]string, 0, len(TypologyAppendixFiles))
	for _, name := range TypologyAppendixFiles {
		if fileExists(filepath.Join(evidenceDir, name)) {
			appendix = append(appendix, name)
		}
	}
	body := mustRender(typologyReadingIndexTmpl, struct {
		Appendix []string
	}{Appendix: appendix})
	path := filepath.Join(evidenceDir, TypologyReadingIndexPath)
	return os.WriteFile(path, []byte(body), 0o644)
}
```


## ./internal/diff
- package: `diff`
- packageDoc: Package diff builds combined diffs from staging manifests.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- deliveryHint: dto
- mechanicalRole: dto
- mechanicalConfidence: 0.90
- mechanicalEvidence: json_tags, decl_heavy_export_surface
- exportedDecls: BuildAllOptions
- exportedFuncs: BuildAll
- exportedMethods: (none)
- unexportedDecls: manifestFile, reviewableEntry
- unexportedFuncs: serializeAgentContext, writeEntry
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/diff/alldiffs.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/diff/doc.go

### Exported bodies

#### BuildAllOptions (type)

```go
type BuildAllOptions struct {
	Manifest string
	Output   string
	// Cap truncates each file's diff to at most Cap lines. Nil means no cap.
	Cap *int
}
```

#### BuildAll (func)

```go
func BuildAll(opts BuildAllOptions) error {
	if opts.Manifest == "" || opts.Output == "" {
		return fmt.Errorf("manifest and output paths required")
	}
	raw, err := os.ReadFile(opts.Manifest)
	if err != nil {
		return fmt.Errorf("cannot read manifest %s: %w", opts.Manifest, err)
	}
	var m manifestFile
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("cannot parse manifest %s: %w", opts.Manifest, err)
	}
	if m.Reviewable == nil {
		return fmt.Errorf("manifest %s has no 'reviewable' array", opts.Manifest)
	}

	if err := os.MkdirAll(filepath.Dir(opts.Output), 0o755); err != nil {
		return err
	}
	f, err := os.Create(opts.Output)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range m.Reviewable {
		if err := writeEntry(f, entry, opts.Cap); err != nil {
			return err
		}
	}
	return nil
}
```

### Private one-hop bodies

#### manifestFile (type)

```go
type manifestFile struct {
	Reviewable []reviewableEntry `json:"reviewable"`
}
```

#### writeEntry (func)

```go
func writeEntry(f *os.File, entry reviewableEntry, capLines *int) error {
	if _, err := fmt.Fprintf(f, "=== FILE: %s ===\n", entry.File); err != nil {
		return err
	}
	if ctx := serializeAgentContext(entry.AgentContext); ctx != "" {
		if _, err := fmt.Fprintf(f, "=== AGENT CONTEXT: %s ===\n", ctx); err != nil {
			return err
		}
	}

	data, err := os.ReadFile(entry.InputFile)
	var lines []string
	if err != nil {
		lines = nil
	} else {
		text := string(data)
		if text == "" {
			lines = []string{}
		} else {
			lines = strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
			// Python splitlines() drops a trailing empty line from a final newline.
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
		}
	}

	if capLines == nil || len(lines) <= *capLines {
		if len(lines) > 0 {
			if _, err := f.WriteString(strings.Join(lines, "\n")); err != nil {
				return err
			}
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
	} else {
		capN := *capLines
		if _, err := f.WriteString(strings.Join(lines[:capN], "\n")); err != nil {
			return err
		}
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
		omitted := len(lines) - capN
		if _, err := fmt.Fprintf(f, "[... %d lines omitted — diff cap is %d lines]\n", omitted, capN); err != nil {
			return err
		}
	}
	_, err = f.WriteString("\n")
	return err
}
```


## ./internal/filereview
- package: `filereview`
- packageDoc: Package filereview is the Prepare → Judge → Validate → Assemble state machine for per-file PR review batches.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: validation
- mechanicalConfidence: 0.90
- mechanicalEvidence: validate_export, validation_report
- exportedDecls: Finding, JudgeFunc, Options, Report, Reviewable, Severity, SeverityCritical, SeverityInfo, SeverityWarn
- exportedFuncs: Assemble, CollectReports, FormatMarkdown, LoadReviewables, ParseMarkdownReport, ParseSeverity, PerFileDir, Run, ValidateReports
- exportedMethods: Severity.Tag
- unexportedDecls: fileHeaderRe, findingRe, noIssuesRe
- unexportedFuncs: writeFeedback
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/filereview/assemble.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/filereview/batch.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/filereview/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/filereview/parse.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/filereview/prepare.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/filereview/types.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/filereview/validate.go

### Exported bodies

#### CollectReports (func)

```go
func CollectReports(perFileDir string, reviewables []Reviewable) (map[string]Report, error) {
	out := make(map[string]Report, len(reviewables))
	for _, r := range reviewables {
		path := filepath.Join(perFileDir, r.Slug+".md")
		rep, err := ParseMarkdownReport(path, r.Slug)
		if err != nil {
			return nil, fmt.Errorf("filereview collect %s: %w", r.Slug, err)
		}
		if rep.File == rep.Slug && r.File != "" {
			rep.File = r.File
		}
		rep.Slug = r.Slug
		out[r.Slug] = rep
	}
	return out, nil
}
```

#### Assemble (func)

```go
func Assemble(skillOut string, reports map[string]Report, reviewables []Reviewable) error {
	perFile := PerFileDir(skillOut)
	if err := os.MkdirAll(perFile, 0o755); err != nil {
		return err
	}
	ordered := make([]Report, 0, len(reviewables))
	for _, r := range reviewables {
		rep := reports[r.Slug]
		ordered = append(ordered, rep)
		md := FormatMarkdown(rep)
		if err := os.WriteFile(filepath.Join(perFile, r.Slug+".md"), []byte(md), 0o644); err != nil {
			return fmt.Errorf("filereview assemble md %s: %w", r.Slug, err)
		}
	}
	data, err := json.MarshalIndent(map[string]any{"reports": ordered}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(skillOut, "findings.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("filereview assemble findings.json: %w", err)
	}
	return nil
}
```

#### JudgeFunc (type)

```go
type JudgeFunc func() error
```

#### Options (type)

```go
type Options struct {
	StagingDir string // batch staging (contains manifest.json)
	SkillOut   string // <output>/<skill>
	MaxRetries int    // Validate failures; default 2 (3 attempts total with first)
	Judge      JudgeFunc
	Logf       func(format string, args ...any)
}
```

#### Run (func)

```go
func Run(opts Options) error {
	if opts.Judge == nil {
		return fmt.Errorf("filereview: Judge required")
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 2
	}
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	manifest := filepath.Join(opts.StagingDir, "manifest.json")
	reviewables, err := LoadReviewables(manifest)
	if err != nil {
		return err
	}
	logf("filereview prepare: %d reviewable(s)", len(reviewables))

	perFile := PerFileDir(opts.SkillOut)
	attempts := opts.MaxRetries + 1
	var lastValidate error
	for attempt := 1; attempt <= attempts; attempt++ {
		logf("filereview judge: attempt %d/%d", attempt, attempts)
		if err := opts.Judge(); err != nil {
			return fmt.Errorf("filereview judge: %w", err)
		}
		reports, err := CollectReports(perFile, reviewables)
		if err != nil {
			lastValidate = err
			logf("filereview validate: %v", err)
			if err := writeFeedback(opts.StagingDir, err); err != nil {
				return err
			}
			continue
		}
		if err := ValidateReports(reviewables, reports); err != nil {
			lastValidate = err
			logf("filereview validate: %v", err)
			if err := writeFeedback(opts.StagingDir, err); err != nil {
				return err
			}
			continue
		}
		if err := Assemble(opts.SkillOut, reports, reviewables); err != nil {
			return err
		}
		logf("filereview assemble: ok (%d report(s))", len(reports))
		return nil
	}
	return fmt.Errorf("filereview: exhausted retries: %w", lastValidate)
}
```

#### ParseMarkdownReport (func)

```go
func ParseMarkdownReport(path, fallbackSlug string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if fallbackSlug != "" {
		slug = fallbackSlug
	}
	rep := Report{Slug: slug, File: slug}
	noIssues := false
	for _, line := range strings.Split(string(data), "\n") {
		if m := fileHeaderRe.FindStringSubmatch(line); m != nil {
			rep.File = strings.TrimSpace(m[1])
			continue
		}
		if noIssuesRe.MatchString(line) {
			noIssues = true
			continue
		}
		if m := findingRe.FindStringSubmatch(line); m != nil {
			sev, err := ParseSeverity(m[1])
			if err != nil {
				return Report{}, err
			}
			text := strings.TrimSpace(m[2])
			if text == "" {
				return Report{}, fmt.Errorf("filereview: empty finding text in %s", path)
			}
			rep.Findings = append(rep.Findings, Finding{Severity: sev, Text: text})
		}
	}
	rep.NoIssues = noIssues && len(rep.Findings) == 0
	return rep, nil
}
```

#### FormatMarkdown (func)

```go
func FormatMarkdown(rep Report) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(rep.File)
	b.WriteString("\n\n")
	if len(rep.Findings) == 0 {
		b.WriteString("No issues found.\n")
		return b.String()
	}
	for _, f := range rep.Findings {
		b.WriteString("- [")
		b.WriteString(f.Severity.Tag())
		b.WriteString("] ")
		b.WriteString(f.Text)
		b.WriteString("\n")
	}
	return b.String()
}
```

#### LoadReviewables (func)

```go
func LoadReviewables(manifestPath string) ([]Reviewable, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("filereview prepare: read manifest: %w", err)
	}
	var raw struct {
		Reviewable []map[string]any `json:"reviewable"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("filereview prepare: decode manifest: %w", err)
	}
	out := make([]Reviewable, 0, len(raw.Reviewable))
	seen := map[string]struct{}{}
	for _, t := range raw.Reviewable {
		file, fileOK := t["file"].(string)
		slug, slugOK := t["slug"].(string)
		if !fileOK || !slugOK || file == "" || slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, Reviewable{File: file, Slug: slug})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("filereview prepare: no reviewables in %s", manifestPath)
	}
	return out, nil
}
```

#### PerFileDir (func)

```go
func PerFileDir(skillOut string) string {
	return filepath.Join(skillOut, "per-file")
}
```

#### Severity (type)

```go
type Severity string
```

#### ParseSeverity (func)

```go
func ParseSeverity(tag string) (Severity, error) {
	switch tag {
	case "CRITICAL", "critical":
		return SeverityCritical, nil
	case "WARN", "warn":
		return SeverityWarn, nil
	case "INFO", "info":
		return SeverityInfo, nil
	default:
		return "", fmt.Errorf("filereview: unknown severity %q", tag)
	}
}
```

#### Severity.Tag (method)

```go
func (s Severity) Tag() string {
	switch s {
	case SeverityCritical:
		return "CRITICAL"
	case SeverityWarn:
		return "WARN"
	case SeverityInfo:
		return "INFO"
	default:
		return string(s)
	}
}
```

#### Finding (type)

```go
type Finding struct {
	Severity Severity `json:"severity"`
	Text     string   `json:"text"`
}
```

#### Report (type)

```go
type Report struct {
	File     string    `json:"file"`
	Slug     string    `json:"slug"`
	Findings []Finding `json:"findings"`
	// NoIssues is true when the report explicitly has no findings (allowed).
	NoIssues bool `json:"no_issues,omitempty"`
}
```

#### Reviewable (type)

```go
type Reviewable struct {
	File string `json:"file"`
	Slug string `json:"slug"`
}
```

#### ValidateReports (func)

```go
func ValidateReports(reviewables []Reviewable, reports map[string]Report) error {
	var errs []string
	for _, r := range reviewables {
		rep, ok := reports[r.Slug]
		if !ok {
			errs = append(errs, fmt.Sprintf("missing report for slug %q (file %q)", r.Slug, r.File))
			continue
		}
		if strings.TrimSpace(rep.File) == "" {
			errs = append(errs, fmt.Sprintf("%s: empty file field", r.Slug))
		}
		if strings.TrimSpace(rep.Slug) == "" {
			errs = append(errs, fmt.Sprintf("%s: empty slug", r.Slug))
		}
		if len(rep.Findings) == 0 && !rep.NoIssues {
			errs = append(errs, fmt.Sprintf("%s: no findings and no explicit no-issues marker", r.Slug))
		}
		for i, f := range rep.Findings {
			if f.Severity != SeverityCritical && f.Severity != SeverityWarn && f.Severity != SeverityInfo {
				errs = append(errs, fmt.Sprintf("%s finding[%d]: bad severity %q", r.Slug, i, f.Severity))
			}
			if strings.TrimSpace(f.Text) == "" {
				errs = append(errs, fmt.Sprintf("%s finding[%d]: empty text", r.Slug, i))
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("filereview validate: %s", strings.Join(errs, "; "))
}
```

### Private one-hop bodies

#### writeFeedback (func)

```go
func writeFeedback(stagingDir string, validateErr error) error {
	path := filepath.Join(stagingDir, "filereview_feedback.md")
	body := "# File-review Validate feedback\n\n" + validateErr.Error() + "\n"
	return os.WriteFile(path, []byte(body), 0o644)
}
```


## ./internal/githttps
- package: `githttps`
- packageDoc: Package githttps builds git -c http.extraHeader args for forge HTTPS remotes.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: (none)
- exportedFuncs: ExtraHeaderArgs, InferSCM
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: (none)
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/githttps/auth.go

### Exported bodies

#### ExtraHeaderArgs (func)

```go
func ExtraHeaderArgs(token, scm string) []string {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	var header string
	switch strings.ToLower(strings.TrimSpace(scm)) {
	case "gitlab":
		basic := base64.StdEncoding.EncodeToString([]byte("oauth2:" + token))
		header = "Authorization: Basic " + basic
	case "bitbucket":
		basic := base64.StdEncoding.EncodeToString([]byte("x-token-auth:" + token))
		header = "Authorization: Basic " + basic
	default:
		basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		header = "Authorization: Basic " + basic
	}
	return []string{"-c", "http.extraHeader=" + header}
}
```

#### InferSCM (func)

```go
func InferSCM(remoteURL string) string {
	u := strings.ToLower(remoteURL)
	switch {
	case strings.Contains(u, "gitlab"):
		return "gitlab"
	case strings.Contains(u, "bitbucket"):
		return "bitbucket"
	default:
		return "github"
	}
}
```


## ./internal/judge
- package: `judge`
- packageDoc: Package judge is the Majordomo boundary onto strop JobRunner and evaluation packs.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: pipeline
- mechanicalConfidence: 0.80
- mechanicalEvidence: pipeline_registry_param, pipeline_runner_type
- exportedDecls: DispatchMode, DispatchModeFiles, DispatchModeFinalize, DispatchModeProse, DispatchModeScore, DispatchModeSummary, DispatchModeTechScore, DispatchModeTechnical, DispatchModeTechnicalDeep, DispatchOptions, ErrNotReady, ErrStropJudgeNotReady, FileReviewOptions, Generator, MinEvalPassScore, Runtime, RuntimeOptions
- exportedFuncs: AllGeneratorTasks, DefaultModuleRetryConfig, DefaultRuntime, DigestTasks, Dispatch, EnsureRuntimeFromConfig, EnsureStropReady, EvalFeedback, EvalPassed, Evaluate, FileReviewBatch, Generate, LLMConfigured, NewJobRunner, NewRuntime, RegisterPacks, ResetRegistryForTests, ResolveGatewayProvider, ResolveProvider, ReviewTasks, SetDefaultRuntime, SharedRunner, StoryLLMAvailable, StropReady, WrapLLMWithRetry
- exportedMethods: Runtime.Evaluate, Runtime.Generate, Runtime.Ready, Runtime.TaskModel, mapInput.EvaluationMap, mapInput.GetVersion, mapInput.ToMap, nopLogger.Debug, nopLogger.Error, nopLogger.Info, nopLogger.Warn, nopLogger.WithError, nopLogger.WithField, nopLogger.WithFields, retryLLM.Generate, retryLLM.GenerateWithContent, singleRoleInfo.ConsolidatorKey, singleRoleInfo.ConsolidatorName, singleRoleInfo.EvaluatorName, singleRoleInfo.EvaluatorWeight, singleRoleInfo.HasEvaluator
- unexportedDecls: defaultModuleRetryAttempts, defaultModuleTimeout, defaultRuntime, defaultRuntimeErr, defaultRuntimeMu, defaultRuntimeOnce, digestEvalSpec, digestEvalSpecs, errGatewayUnavailable, finalizeIndexTemplate, finalizeReportData, finalizeSummaryTemplate, mapInput, nopLogger, registryErr, registryOnce, retryLLM, sharedReg, sharedRunner, singleRoleInfo
- unexportedFuncs: discardEventChannel, ensureRegistry, fileExists, newMapInput, readGrounding, readReviewableInput, readStagingContext, registerDigestEvaluationWorkflow, renderFinalizeReports, resolveGatewayOrUnavailable, resolveTaskProviderConfig, withLLMRetry, writeFinalizeOutputs
- unexportedMethods: (none)
- errorTypes: ErrNotReady, ErrStropJudgeNotReady
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/eval_register.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/filereview.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/input.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/llm_retry.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/mode.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/provider.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/registry.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/runner.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/runtime.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/strop_dispatch.go

### Exported bodies

#### singleRoleInfo.EvaluatorName (method)

```go
func (s singleRoleInfo) EvaluatorName(key evaluation.EvaluatorKey) string {
	if key == s.evaluatorKey {
		return s.evaluatorLabel
	}
	return key.String()
}
```

#### singleRoleInfo.HasEvaluator (method)

```go
func (s singleRoleInfo) HasEvaluator(key evaluation.EvaluatorKey) bool {
	return key == s.evaluatorKey
}
```

#### singleRoleInfo.EvaluatorWeight (method)

```go
func (s singleRoleInfo) EvaluatorWeight(key evaluation.EvaluatorKey) float64 {
	if key == s.evaluatorKey {
		return 1.0
	}
	return 0
}
```

#### singleRoleInfo.ConsolidatorKey (method)

```go
func (s singleRoleInfo) ConsolidatorKey() evaluation.ConsolidatorKey { return s.consolidatorKey }
```

#### singleRoleInfo.ConsolidatorName (method)

```go
func (s singleRoleInfo) ConsolidatorName() string { return s.consolidatorLabel }
```

#### EvalPassed (func)

```go
func EvalPassed(eval *evaluation.AggregatedEvaluation) bool {
	return eval != nil && eval.WeightedScore >= MinEvalPassScore
}
```

#### EvalFeedback (func)

```go
func EvalFeedback(eval *evaluation.AggregatedEvaluation) string {
	if eval == nil {
		return "evaluation returned no result"
	}
	if fb := strings.TrimSpace(eval.ConsolidatedFeedback); fb != "" {
		return fb
	}
	return fmt.Sprintf("weighted_score=%.2f (below pass threshold %.1f)", eval.WeightedScore, MinEvalPassScore)
}
```

#### FileReviewOptions (type)

```go
type FileReviewOptions struct {
	Context    context.Context
	StagingDir string
	SkillOut   string
}
```

#### FileReviewBatch (func)

```go
func FileReviewBatch(opts FileReviewOptions) error {
	if err := EnsureStropReady(); err != nil {
		return err
	}
	reviewables, err := filereview.LoadReviewables(filepath.Join(opts.StagingDir, "manifest.json"))
	if err != nil {
		return err
	}
	perFile := filereview.PerFileDir(opts.SkillOut)
	if err := os.MkdirAll(perFile, 0o755); err != nil {
		return err
	}
	grounding := readGrounding(opts.StagingDir)
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	for _, r := range reviewables {
		diff, err := readReviewableInput(opts.StagingDir, r)
		if err != nil {
			return err
		}
		out, err := Generate(ctx, jmodules.TaskFileReview, map[string]interface{}{
			"file_path":    r.File,
			"slug":         r.Slug,
			"diff_content": diff,
			"grounding":    grounding,
		}, 1)
		if err != nil {
			return fmt.Errorf("filereview %s: %w", r.Slug, err)
		}
		md, ok := out["markdown"].(string)
		if !ok {
			return fmt.Errorf("filereview %s: output missing string field markdown", r.Slug)
		}
		if strings.TrimSpace(md) == "" {
			md = filereview.FormatMarkdown(filereview.Report{File: r.File, Slug: r.Slug, NoIssues: true})
		}
		path := filepath.Join(perFile, r.Slug+".md")
		if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
			return err
		}
	}
	return nil
}
```

#### mapInput.ToMap (method)

```go
func (m mapInput) ToMap() map[string]interface{} { return m.fields }
```

#### mapInput.EvaluationMap (method)

```go
func (m mapInput) EvaluationMap() map[string]interface{} { return m.fields }
```

#### mapInput.GetVersion (method)

```go
func (m mapInput) GetVersion() int { return m.version }
```

#### DefaultModuleRetryConfig (func)

```go
func DefaultModuleRetryConfig() interceptors.RetryConfig {
	return interceptors.RetryConfig{
		MaxAttempts: defaultModuleRetryAttempts,
		Delay:       2 * time.Second,
		MaxBackoff:  30 * time.Second,
		Backoff:     2.0,
	}
}
```

#### WrapLLMWithRetry (func)

```go
func WrapLLMWithRetry(llm core.LLM, cfg interceptors.RetryConfig) core.LLM {
	if llm == nil {
		return nil
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaultModuleRetryAttempts
	}
	if cfg.Delay <= 0 {
		cfg.Delay = 2 * time.Second
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = 2.0
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 30 * time.Second
	}
	return &retryLLM{LLM: llm, cfg: cfg}
}
```

#### retryLLM.Generate (method)

```go
func (r *retryLLM) Generate(ctx context.Context, prompt string, opts ...core.GenerateOption) (*core.LLMResponse, error) {
	return withLLMRetry(ctx, r.cfg, "Generate", func() (*core.LLMResponse, error) {
		return r.LLM.Generate(ctx, prompt, opts...)
	})
}
```

#### retryLLM.GenerateWithContent (method)

```go
func (r *retryLLM) GenerateWithContent(ctx context.Context, content []core.ContentBlock, opts ...core.GenerateOption) (*core.LLMResponse, error) {
	return withLLMRetry(ctx, r.cfg, "GenerateWithContent", func() (*core.LLMResponse, error) {
		return r.LLM.GenerateWithContent(ctx, content, opts...)
	})
}
```

#### ResolveProvider (func)

```go
func ResolveProvider() (stropdspy.ProviderConfig, error) {
	return ResolveGatewayProvider(strings.TrimSpace(os.Getenv("MAJORDOMO_MODEL")))
}
```

#### LLMConfigured (func)

```go
func LLMConfigured() bool {
	_, err := aigateway.NewAccountFromEnv()
	return err == nil
}
```

#### StropReady (func)

```go
func StropReady() bool {
	_, _, err := ensureRegistry()
	return err == nil
}
```

#### EnsureStropReady (func)

```go
func EnsureStropReady() error {
	_, _, err := ensureRegistry()
	if err != nil {
		return ErrNotReady
	}
	return nil
}
```

#### SharedRunner (func)

```go
func SharedRunner() (*runner.JobRunner, error) {
	_, jr, err := ensureRegistry()
	return jr, err
}
```

#### Generate (func)

```go
func Generate(ctx context.Context, task string, fields map[string]interface{}, version int) (map[string]interface{}, error) {
	rt, err := DefaultRuntime()
	if err != nil {
		return nil, err
	}
	return rt.Generate(ctx, task, fields, version)
}
```

#### Evaluate (func)

```go
func Evaluate(
	ctx context.Context,
	task string,
	inputFields, outputFields map[string]interface{},
	version int,
) (*evaluation.AggregatedEvaluation, error) {
	rt, err := DefaultRuntime()
	if err != nil {
		return nil, err
	}
	return rt.Evaluate(ctx, task, inputFields, outputFields, version)
}
```

#### StoryLLMAvailable (func)

```go
func StoryLLMAvailable() bool {
	return StropReady()
}
```

#### ResetRegistryForTests (func)

```go
func ResetRegistryForTests() {
	registryOnce = sync.Once{}
	registryErr = nil
	sharedReg = nil
	sharedRunner = nil
	defaultRuntimeOnce = sync.Once{}
	defaultRuntimeMu.Lock()
	defaultRuntime = nil
	defaultRuntimeErr = nil
	defaultRuntimeMu.Unlock()
	aigateway.ResetForTests()
}
```

#### RegisterPacks (func)

```go
func RegisterPacks(r *criteria.CriterionRegistry) {
	summarypack.Register(r)
	techpack.Register(r)
	digestpack.Register(r)
	bootstrappack.Register(r)
	typologypack.Register(r)
}
```

#### NewJobRunner (func)

```go
func NewJobRunner(
	reg *registry.ModuleRegistry,
	learning runner.LearningServiceForGeneration,
	formatter runner.ExampleFormatter,
	logger stroplog.Logger,
) *runner.JobRunner {
	if logger == nil {
		logger = nopLogger{}
	}
	return runner.NewJobRunner(reg, learning, formatter, logger)
}
```

#### nopLogger.WithField (method)

```go
func (nopLogger) WithField(string, interface{}) stroplog.Logger { return nopLogger{} }
```

#### nopLogger.WithFields (method)

```go
func (nopLogger) WithFields(map[string]interface{}) stroplog.Logger {
	return nopLogger{}
}
```

#### nopLogger.WithError (method)

```go
func (nopLogger) WithError(error) stroplog.Logger { return nopLogger{} }
```

#### nopLogger.Debug (method)

```go
func (nopLogger) Debug(...interface{})            {}
```

#### nopLogger.Info (method)

```go
func (nopLogger) Info(...interface{})             {}
```

#### nopLogger.Warn (method)

```go
func (nopLogger) Warn(...interface{})             {}
```

#### nopLogger.Error (method)

```go
func (nopLogger) Error(...interface{})            {}
```

#### DigestTasks (func)

```go
func DigestTasks() []string {
	return []string{
		jmodules.TaskTypologyInspect,
		jmodules.TaskTypologySliceGrouping,
		jmodules.TaskTypologySliceCatalog,
		jmodules.TaskTypologyInterventionJourney,
		jmodules.TaskTypologyInterventionBrief,
		jmodules.TaskTypologyInterventionWeaknesses,
		jmodules.TaskTypologyInterventionPRPriority,
		jmodules.TaskTypologyFindingComment,
		jmodules.TaskBootstrapStory,
		jmodules.TaskDigestStory,
	}
}
```

#### ReviewTasks (func)

```go
func ReviewTasks() []string {
	return []string{jmodules.TaskFileReview, jmodules.TaskSummary, jmodules.TaskTechnical}
}
```

#### AllGeneratorTasks (func)

```go
func AllGeneratorTasks() []string {
	return []string{
		jmodules.TaskFileReview,
		jmodules.TaskTypologyInspect,
		jmodules.TaskTypologySliceGrouping,
		jmodules.TaskTypologySliceCatalog,
		jmodules.TaskTypologyHumanIntervention,
		jmodules.TaskTypologyInterventionJourney,
		jmodules.TaskTypologyInterventionBrief,
		jmodules.TaskTypologyInterventionWeaknesses,
		jmodules.TaskTypologyInterventionPRPriority,
		jmodules.TaskTypologyFindingComment,
		jmodules.TaskBootstrapStory,
		jmodules.TaskDigestStory,
		jmodules.TaskSummary,
		jmodules.TaskTechnical,
	}
}
```

#### Generator (type)

```go
type Generator interface {
	Generate(ctx context.Context, task string, fields map[string]interface{}, version int) (map[string]interface{}, error)
	Evaluate(ctx context.Context, task string, inputFields, outputFields map[string]interface{}, version int) (*evaluation.AggregatedEvaluation, error)
	Ready() bool
	TaskModel(task string) string
}
```

#### Runtime (type)

```go
type Runtime struct {
	reg    *registry.ModuleRegistry
	runner *runner.JobRunner
	models map[string]string
}
```

#### RuntimeOptions (type)

```go
type RuntimeOptions struct {
	// Tasks limits registration; empty means all known generator tasks.
	Tasks []string
	// FallbackModel overrides MAJORDOMO_MODEL for embedded-gateway fallback.
	FallbackModel string
	// RunReport configures strop runreport for Judge Generate/Evaluate (digest turns this on).
	RunReport runreport.Config
}
```

#### NewRuntime (func)

```go
func NewRuntime(ctx context.Context, cfg config.RepoConfig, opts RuntimeOptions) (*Runtime, error) {
	strict := len(opts.Tasks) > 0
	tasks := opts.Tasks
	if !strict {
		tasks = AllGeneratorTasks()
	}

	reg := registry.NewModuleRegistry()
	llmFactory := factory.NewLLMFactory(func(modelID string, providerType string) {
		reg.RegisterModelProvider(modelID, providerType)
	}, defaultModuleTimeout)
	llmFactory.SetInstrumentHTTP(observability.InstrumentHTTPClient)

	otelOn := true
	if v := os.Getenv("MAJORDOMO_OTEL_ENABLED"); v == "0" {
		otelOn = false
	}
	svc := os.Getenv("MAJORDOMO_OTEL_SERVICE_NAME")
	if svc == "" {
		svc = observability.DefaultServiceName
	}
	retryConfig := DefaultModuleRetryConfig()
	interceptorSetup := factory.NewInterceptorSetup(
		otelOn, svc, &retryConfig, defaultModuleTimeout,
		dspyTracing.OpenInferenceModuleInterceptor,
		nil,
		reg.GetModelProvider,
		reg.GetModuleModel,
		func(moduleName, modelID string) { reg.RegisterModuleModel(moduleName, modelID) },
		nil,
		opts.RunReport,
	)
	// Evidence keys that must be non-empty when the CoT path runs (mirror mandatory outputs).
	// RLM bootstrap story validates a named map before Complete; this covers legacy CoT injection.
	interceptorSetup.RegisterRequiredInputs(jmodules.TaskBootstrapStory, []string{
		"repo_id", "readme_snapshot",
	})
	interceptorSetup.RegisterRequiredInputs(jmodules.TaskTypologySliceGrouping, []string{
		"repo_id", "package_roles", "mechanical_grouping_yaml", "readme_snapshot",
	})
	interceptorSetup.RegisterRequiredInputs(jmodules.TaskTypologySliceCatalog, []string{
		"repo_id", "slice_meaning_ledger_yaml", "package_roles", "readme_snapshot",
	})
	configurator := factory.NewModuleConfigurator(llmFactory, interceptorSetup, nil)
	genFactory := factory.NewGeneratorFactory(configurator)
	evalFactory := factory.NewEvaluatorFactory(configurator)

	RegisterPacks(criteria.DefaultRegistry())

	rt := &Runtime{
		reg:    reg,
		models: make(map[string]string, len(tasks)),
	}

	ctors := map[string]func() core.Module{
		jmodules.TaskFileReview:                     jmodules.FileReviewModule,
		jmodules.TaskTypologyInspect:                jmodules.TypologyInspectModule,
		jmodules.TaskTypologySliceGrouping:                jmodules.TypologyClusterModule,
		jmodules.TaskTypologySliceCatalog:                 jmodules.TypologyRefineModule,
		jmodules.TaskTypologyHumanIntervention:      jmodules.TypologyHumanInterventionModule,
		jmodules.TaskTypologyInterventionJourney:    jmodules.TypologyInterventionJourneyModule,
		jmodules.TaskTypologyInterventionBrief:      jmodules.TypologyInterventionBriefModule,
		jmodules.TaskTypologyInterventionWeaknesses: jmodules.TypologyInterventionWeaknessesModule,
		jmodules.TaskTypologyInterventionPRPriority: jmodules.TypologyInterventionPRPriorityModule,
		jmodules.TaskTypologyFindingComment:         jmodules.TypologyFindingCommentModule,
		jmodules.TaskBootstrapStory:                 jmodules.BootstrapStoryModule,
		jmodules.TaskDigestStory:                    jmodules.DigestStoryModule,
		jmodules.TaskSummary:                        jmodules.SummaryModule,
		jmodules.TaskTechnical:                      jmodules.TechnicalModule,
	}

	providers := make(map[string]stropdspy.ProviderConfig, len(tasks))
	for _, task := range tasks {
		ctor, ok := ctors[task]
		if !ok {
			return nil, fmt.Errorf("judge runtime: unknown task %q", task)
		}
		provider, err := resolveTaskProviderConfig(cfg, task, opts.FallbackModel)
		if err != nil {
			if !strict && errors.Is(err, errGatewayUnavailable) {
				continue
			}
			return nil, fmt.Errorf("judge runtime: resolve %s: %w", task, err)
		}
		mod, err := genFactory.CreateGenerator(ctx, provider, func() (core.Module, error) {
			return ctor(), nil
		}, task, nil)
		if err != nil {
			return nil, fmt.Errorf("judge runtime: register %s: %w", task, err)
		}
		reg.RegisterGenerator(task, mod)
		rt.models[task] = provider.Model
		providers[task] = provider
	}

	if len(rt.models) == 0 {
		return nil
// ... truncated
```

#### Runtime.Generate (method)

```go
func (rt *Runtime) Generate(ctx context.Context, task string, fields map[string]interface{}, version int) (map[string]interface{}, error) {
	if rt == nil || rt.runner == nil {
		return nil, ErrNotReady
	}
	cfg := runner.GenerationConfig{
		ModuleName:   task,
		JobName:      task,
		StepName:     task,
		ErrorMessage: task,
	}
	out, err := rt.runner.Generate(ctx, cfg, newMapInput(fields, version), nil)
	llmusage.RecordExecutionState(ctx, task)
	return out, err
}
```

#### Runtime.Evaluate (method)

```go
func (rt *Runtime) Evaluate(
	ctx context.Context,
	task string,
	inputFields, outputFields map[string]interface{},
	version int,
) (*evaluation.AggregatedEvaluation, error) {
	if rt == nil || rt.runner == nil {
		return nil, ErrNotReady
	}
	// strop v0.2.4 EvaluateStream blocks forever on nil eventChan (send on nil channel).
	// Drain a buffered channel until strop is bumped with the nil-safe JobRunner path.
	eventChan, stop := discardEventChannel()
	defer stop()
	out, err := rt.runner.EvaluateWorkflow(ctx, task, newMapInput(inputFields, version), outputFields, eventChan)
	llmusage.RecordExecutionState(ctx, task)
	return out, err
}
```

#### Runtime.Ready (method)

```go
func (rt *Runtime) Ready() bool {
	return rt != nil && rt.runner != nil
}
```

#### Runtime.TaskModel (method)

```go
func (rt *Runtime) TaskModel(task string) string {
	if rt == nil {
		return ""
	}
	return rt.models[task]
}
```

#### SetDefaultRuntime (func)

```go
func SetDefaultRuntime(rt *Runtime) {
	defaultRuntimeMu.Lock()
	defer defaultRuntimeMu.Unlock()
	defaultRuntime = rt
	defaultRuntimeErr = nil
	if rt != nil {
		sharedReg = rt.reg
		sharedRunner = rt.runner
	}
}
```

#### DefaultRuntime (func)

```go
func DefaultRuntime() (*Runtime, error) {
	defaultRuntimeMu.RLock()
	rt := defaultRuntime
	err := defaultRuntimeErr
	defaultRuntimeMu.RUnlock()
	if rt != nil || err != nil {
		return rt, err
	}

	defaultRuntimeOnce.Do(func() {
		built, buildErr := NewRuntime(context.Background(), config.RepoConfig{}, RuntimeOptions{})
		defaultRuntimeMu.Lock()
		defer defaultRuntimeMu.Unlock()
		if buildErr != nil {
			defaultRuntimeErr = buildErr
			return
		}
		defaultRuntime = built
		sharedReg = built.reg
		sharedRunner = built.runner
	})

	defaultRuntimeMu.RLock()
	defer defaultRuntimeMu.RUnlock()
	return defaultRuntime, defaultRuntimeErr
}
```

#### EnsureRuntimeFromConfig (func)

```go
func EnsureRuntimeFromConfig(cfg config.RepoConfig, opts RuntimeOptions) (*Runtime, error) {
	rt, err := NewRuntime(context.Background(), cfg, opts)
	if err != nil {
		return nil, err
	}
	SetDefaultRuntime(rt)
	return rt, nil
}
```

#### ResolveGatewayProvider (func)

```go
func ResolveGatewayProvider(model string) (stropdspy.ProviderConfig, error) {
	gw, err := aigateway.Ensure()
	if err != nil {
		return stropdspy.ProviderConfig{}, err
	}
	account, err := aigateway.NewAccountFromEnv()
	if err != nil {
		return stropdspy.ProviderConfig{}, fmt.Errorf("resolve gateway account: %w", err)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(os.Getenv("MAJORDOMO_MODEL"))
	}
	if model == "" {
		model = aigateway.LogicalModel(account)
	}
	return stropdspy.ProviderConfig{
		APIKey:    aigateway.DummyAPIKey,
		Model:     model,
		BaseURL:   gw.BaseURL(),
		APISchema: "openai",
		Timeout:   "120s",
	}, nil
}
```

#### DispatchMode (type)

```go
type DispatchMode string
```

#### DispatchOptions (type)

```go
type DispatchOptions struct {
	Context    context.Context
	PRNumber   string
	StagingDir string
	OutputDir  string
	Mode       DispatchMode
}
```

#### Dispatch (func)

```go
func Dispatch(opts DispatchOptions) error {
	if err := EnsureStropReady(); err != nil {
		return err
	}
	if opts.StagingDir == "" || opts.OutputDir == "" {
		return fmt.Errorf("judge dispatch requires staging-dir and output-dir")
	}
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	stagingContext, err := readStagingContext(opts.StagingDir)
	if err != nil && opts.Mode != DispatchModeProse && opts.Mode != DispatchModeFinalize {
		return err
	}
	pipelineOut := filepath.Dir(opts.OutputDir)
	if filepath.Base(opts.OutputDir) == "pr-review-technical-deep" {
		pipelineOut = filepath.Dir(opts.OutputDir)
	}

	switch opts.Mode {
	case DispatchModeFiles:
		return FileReviewBatch(FileReviewOptions{
			Context:    ctx,
			StagingDir: opts.StagingDir,
			SkillOut:   opts.OutputDir,
		})
	case DispatchModeSummary:
		out, err := Generate(ctx, jmodules.TaskSummary, map[string]interface{}{
			"staging_context": stagingContext,
		}, 1)
		if err != nil {
			return err
		}
		md, ok := out["summary_md"].(string)
		if !ok {
			return fmt.Errorf("judge dispatch: summary output missing string field summary_md")
		}
		return os.WriteFile(filepath.Join(pipelineOut, "summary.md"), []byte(md), 0o644)
	case DispatchModeScore:
		return os.WriteFile(filepath.Join(pipelineOut, "score.md"), []byte("SCORE: 20\n"), 0o644)
	case DispatchModeTechnical:
		out, err := Generate(ctx, jmodules.TaskTechnical, map[string]interface{}{
			"staging_context": stagingContext,
		}, 1)
		if err != nil {
			return err
		}
		md, ok := out["technical_md"].(string)
		if !ok {
			return fmt.Errorf("judge dispatch: technical output missing string field technical_md")
		}
		return os.WriteFile(filepath.Join(pipelineOut, "technical.md"), []byte(md), 0o644)
	case DispatchModeTechScore:
		return os.WriteFile(filepath.Join(pipelineOut, "tech-score.md"), []byte("SCORE: 20\n"), 0o644)
	case DispatchModeFinalize:
		return writeFinalizeOutputs(opts.PRNumber, opts.StagingDir, opts.OutputDir)
	case DispatchModeProse:
		// Per-file markdown is already Validate→Assemble shaped; no OpenCode rewrite.
		return nil
	case DispatchModeTechnicalDeep:
		if stagingContext == "" {
			stagingContext, err = readStagingContext(opts.StagingDir)
			if err != nil {
				return err
			}
		}
		out, err := Generate(ctx, jmodules.TaskTechnical, map[string]interface{}{
			"staging_context": stagingContext,
		}, 1)
		if err != nil {
			return err
		}
		md, ok := out["technical_md"].(string)
		if !ok {
			return fmt.Errorf("judge dispatch: technical output missing string field technical_md")
		}
		if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(opts.OutputDir, "tech-deep.md"), []byte(md), 0o644)
	default:
		return FileReviewBatch(FileReviewOptions{StagingDir: opts.StagingDir, SkillOut: opts.OutputDir})
	}
}
```

### Private one-hop bodies

#### discardEventChannel (func)

```go
func discardEventChannel() (streaming.EventChannel, func()) {
	ch := make(streaming.EventChannel, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	return ch, func() {
		close(ch)
		<-done
	}
}
```

#### ensureRegistry (func)

```go
func ensureRegistry() (*registry.ModuleRegistry, *runner.JobRunner, error) {
	rt, err := DefaultRuntime()
	if err != nil {
		registryErr = err
		return nil, nil, err
	}
	return rt.reg, rt.runner, nil
}
```

#### mapInput (type)

```go
type mapInput struct {
	fields  map[string]interface{}
	version int
}
```

#### newMapInput (func)

```go
func newMapInput(fields map[string]interface{}, version int) mapInput {
	if fields == nil {
		fields = map[string]interface{}{}
	}
	return mapInput{fields: fields, version: version}
}
```

#### nopLogger (type)

```go
type nopLogger struct{}
```

#### readGrounding (func)

```go
func readGrounding(stagingDir string) string {
	dir := filepath.Join(stagingDir, ".grounding")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var parts []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n\n")
}
```

#### readReviewableInput (func)

```go
func readReviewableInput(stagingDir string, r filereview.Reviewable) (string, error) {
	data, err := os.ReadFile(filepath.Join(stagingDir, "manifest.json"))
	if err != nil {
		return "", err
	}
	var raw struct {
		Reviewable []map[string]any `json:"reviewable"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", err
	}
	inputRel := ""
	for _, row := range raw.Reviewable {
		slug, slugOK := row["slug"].(string)
		inputFile, inputFileOK := row["input_file"].(string)
		if slugOK && inputFileOK && slug == r.Slug {
			inputRel = inputFile
			break
		}
	}
	if inputRel == "" {
		return fmt.Sprintf("(no staged diff for %s)", r.File), nil
	}
	b, err := os.ReadFile(filepath.Join(stagingDir, inputRel))
	if err != nil {
		return "", fmt.Errorf("read input %s: %w", inputRel, err)
	}
	return string(b), nil
}
```

#### readStagingContext (func)

```go
func readStagingContext(stagingDir string) (string, error) {
	var parts []string
	manifest := filepath.Join(stagingDir, "manifest.json")
	if b, err := os.ReadFile(manifest); err == nil {
		parts = append(parts, string(b))
	}
	if err := filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".json") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+".grounding"+string(filepath.Separator)) {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(stagingDir, path)
			if err != nil {
				return err
			}
			parts = append(parts, fmt.Sprintf("--- %s ---\n%s", rel, string(b)))
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(stagingDir, path)
		if err != nil {
			return err
		}
		parts = append(parts, fmt.Sprintf("--- %s ---\n%s", rel, string(b)))
		return nil
	}); err != nil {
		return "", fmt.Errorf("judge dispatch: walk staging context: %w", err)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("judge dispatch: empty staging context in %s", stagingDir)
	}
	return strings.Join(parts, "\n\n"), nil
}
```

#### resolveTaskProviderConfig (func)

```go
func resolveTaskProviderConfig(cfg config.RepoConfig, task, fallbackModel string) (stropdspy.ProviderConfig, error) {
	if explicit, ok, err := cfg.ResolveTaskProvider(task); err != nil {
		return stropdspy.ProviderConfig{}, err
	} else if ok {
		if explicit.UsesEmbeddedGateway() {
			return resolveGatewayOrUnavailable(explicit.Model)
		}
		return explicit.ToStrop(), nil
	}
	model := strings.TrimSpace(fallbackModel)
	if model == "" {
		if pipe, ok := cfg.PipelineNamed("pr-review"); ok {
			model = strings.TrimSpace(pipe.Model)
		}
	}
	return resolveGatewayOrUnavailable(model)
}
```

#### retryLLM (type)

```go
type retryLLM struct {
	core.LLM
	cfg interceptors.RetryConfig
}
```

#### singleRoleInfo (type)

```go
type singleRoleInfo struct {
	evaluatorKey      evaluation.EvaluatorKey
	consolidatorKey   evaluation.ConsolidatorKey
	evaluatorLabel    string
	consolidatorLabel string
}
```

#### withLLMRetry (func)

```go
func withLLMRetry(ctx context.Context, cfg interceptors.RetryConfig, op string, call func() (*core.LLMResponse, error)) (*core.LLMResponse, error) {
	var lastErr error
	delay := cfg.Delay
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resp, err := call()
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if attempt == cfg.MaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		delay = time.Duration(float64(delay) * cfg.Backoff)
		if cfg.MaxBackoff > 0 && delay > cfg.MaxBackoff {
			delay = cfg.MaxBackoff
		}
	}
	return nil, fmt.Errorf("llm %s failed after %d attempts: %w", op, cfg.MaxAttempts, lastErr)
}
```

#### writeFinalizeOutputs (func)

```go
func writeFinalizeOutputs(prNumber, stagingDir, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	summaryPath := filepath.Join(outputDir, "summary.md")
	indexPath := filepath.Join(outputDir, "index.md")
	if fileExists(summaryPath) && fileExists(indexPath) {
		return nil
	}

	skill := filepath.Base(stagingDir)
	baseBranch := "unknown"
	var files []string
	var excluded []string
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var raw struct {
			BaseBranch string           `json:"base_branch"`
			Reviewable []map[string]any `json:"reviewable"`
			Excluded   []any            `json:"excluded"`
		}
		if json.Unmarshal(data, &raw) == nil {
			if raw.BaseBranch != "" {
				baseBranch = raw.BaseBranch
			}
			for _, row := range raw.Reviewable {
				if f, ok := row["file"].(string); ok && f != "" {
					files = append(files, f)
				}
			}
			for _, e := range raw.Excluded {
				switch v := e.(type) {
				case string:
					excluded = append(excluded, v)
				case map[string]any:
					if f, ok := v["file"].(string); ok && f != "" {
						excluded = append(excluded, f)
					}
				}
			}
		}
	}
	// Prefer findings.json for file list when present.
	if findingsPath := filepath.Join(outputDir, "findings.json"); fileExists(findingsPath) {
		if data, err := os.ReadFile(findingsPath); err == nil {
			var wrap struct {
				Reports []struct {
					File string `json:"file"`
				} `json:"reports"`
			}
			if json.Unmarshal(data, &wrap) == nil && len(wrap.Reports) > 0 {
				files = nil
				for _, r := range wrap.Reports {
					if r.File != "" {
						files = append(files, r.File)
					}
				}
			}
		}
	}

	reviewedAt := time.Now().UTC().Format(time.RFC3339)
	if ts, err := os.ReadFile(filepath.Join(stagingDir, "review_timestamp.txt")); err == nil {
		reviewedAt = strings.TrimSpace(string(ts))
	}

	summary, index, err := renderFinalizeReports(finalizeReportData{
		PRNumber:   prNumber,
		Skill:      skill,
		BaseBranch: baseBranch,
		ReviewedAt: reviewedAt,
		Files:      files,
		Excluded:   excluded,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(summaryPath, []byte(summary), 0o644); err != nil {
		return err
	}
	return os.WriteFile(indexPath, []byte(index), 0o644)
}
```


## ./internal/judge/evaluation/bootstrap
- package: `bootstrap`
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: CriterionIDEvidencedOnly, CriterionIDHonestSeed, CriterionIDPreservesForm, CriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: (none)
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/evaluation/bootstrap/pack.go

### Exported bodies

#### Register (func)

```go
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDEvidencedOnly,
		Name:        "Bootstrap claims evidenced",
		Description: `Every new claim in the bootstrap story is traceable to the survey evidence, README, or layout snapshot.`,
		Scoring: `2 points: All additions cite supplied evidence.
0 points: Any invented or unsupported claim.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDPreservesForm,
		Name:        "Bootstrap preserves section form",
		Description: `Updated sections keep the expected markdown structure for the file.`,
		Scoring: `1 point: Valid markdown for the section type.
0 points: Broken structure or wrong section content.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDHonestSeed,
		Name:        "Bootstrap is honest about seeding",
		Description: `Seed-time prose must read like a fresh baseline, not a reconstruction of history.`,
		Scoring: `1 point: The text clearly states it is a seed baseline or otherwise avoids invented history.
0 points: The text implies historical reconstruction that is not evidenced.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
}
```


## ./internal/judge/evaluation/digest
- package: `digest`
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: CriterionIDEvidencedOnly, CriterionIDPreservesForm, CriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: (none)
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/evaluation/digest/pack.go

### Exported bodies

#### Register (func)

```go
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDEvidencedOnly,
		Name:        "Digest claims evidenced",
		Description: `Every new claim in the updated section is traceable to the commit diff or subject.`,
		Scoring: `2 points: All additions cite diff-visible evidence.
0 points: Any invented or unsupported claim.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDPreservesForm,
		Name:        "Digest preserves section shape",
		Description: `Updated section keeps markdown structure appropriate to the section id.`,
		Scoring: `1 point: Valid markdown for the section type.
0 points: Broken structure or wrong section content.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
}
```


## ./internal/judge/evaluation/summary
- package: `summary`
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: CriterionIDCallerFacingH3s, CriterionIDH2Structure, CriterionIDJudgmentH3, CriterionIDNoEmDashConnectors, CriterionIDNoFilenamesInProse, CriterionIDNoGenericPhrases, CriterionIDNoPrescriptiveFix, CriterionIDTeamConsequence, CriterionIDWhatGotBuiltBlocks, CriterionIDWhyNamesArtifact, CriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: init
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/evaluation/summary/pack.go

### Exported bodies

#### Register (func)

```go
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDH2Structure,
		Name:        "Summary H2 structure",
		Description: `summary.md has exactly the five required H2 headings in order, with no extras.`,
		Scoring: `2 points: Exact five H2s in order.
0 points: Missing, renamed, reordered, or extra H2.`,
		Examples:  `Required: Why This PR Exists, What Got Built, Low-Risk Changes, Requires Human Judgment, Where to Focus in the Diff.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDWhatGotBuiltBlocks,
		Name:        "What Got Built H3 and before/after",
		Description: `What Got Built has at least one H3 and a Before/After fenced code pair.`,
		Scoring: `2 points: H3 plus Before/After pair present.
0 points: Missing H3 or missing Before/After pair.`,
		Examples:  `First fence opens with # Before:; second with # After:.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDJudgmentH3,
		Name:        "Judgment concerns as H3",
		Description: `Every concern under Requires Human Judgment is an H3.`,
		Scoring: `2 points: All concerns use ### headings.
0 points: Any concern uses bullets or plain paragraphs.`,
		Examples:  `Look for lines beginning with ### under the judgment section.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoGenericPhrases,
		Name:        "No prohibited generic phrases",
		Description: `Summary avoids banned generic quality phrases.`,
		Scoring: `1 point: None of the banned phrases appear.
0 points: Any banned phrase appears.`,
		Examples:  `Banned: better error handling, more robust, cleaner code, improved maintainability, improved readability, more maintainable, better organized.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoEmDashConnectors,
		Name:        "No em dash connectors",
		Description: `No sentence uses an em dash as a clause connector.`,
		Scoring: `1 point: No em dash connectors.
0 points: Any em dash connector found.`,
		Examples:  `Title clarifications with em dashes are OK; clause connectors are not.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDWhyNamesArtifact,
		Name:        "Why names a specific artifact",
		Description: `Why This PR Exists opening names a class, method, pattern, or concrete gap.`,
		Scoring: `2 points: Specific artifact named.
0 points: Only generic motivation.`,
		Examples:  `Good: names CmsController or session recovery gap.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoFilenamesInProse,
		Name:        "No filenames in narrative prose",
		Description: `File names stay out of flowing narrative (allowed in fences, bullets, skip entries).`,
		Scoring: `2 points: No narrative filename embeds.
0 points: Filename inside narrative sentence.`,
		Examples:  `Code fences and Diff Focus skip lines may name files.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDCallerFacingH3s,
		Name:        "Caller-facing What Got Built H3s",
		Description: `What Got Built H3s are caller-facing or tests; no internal-only headings or count-stuffed titles.`,
		Scoring: `2 points: All H3s caller-facing or tests.
0 points: Internal-only H3 or count in heading.`,
		Examples:  `Fail: graph wiring H3 with no call site; H3 with "113 Screen subclasses".`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Re
// ... truncated
```


## ./internal/judge/evaluation/tech
- package: `tech`
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: CriterionIDBlankLinesFields, CriterionIDChecklistSkip, CriterionIDConfirmYesNo, CriterionIDDeclarativeH3, CriterionIDFourFields, CriterionIDH2Structure, CriterionIDNoEmDashConnectors, CriterionIDNoGenericPhrases, CriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: init
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/evaluation/tech/pack.go

### Exported bodies

#### Register (func)

```go
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDH2Structure,
		Name:        "Tech H2 structure",
		Description: `tech-review.md has exactly Correctness Risks, Verification Checklist, Test Coverage Gaps in order.`,
		Scoring: `2 points: Exact three H2s in order.
0 points: Missing, renamed, reordered, or extra H2.`,
		Examples:  `No other H2 headings allowed.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDBlankLinesFields,
		Name:        "Blank lines between risk fields",
		Description: `Each risk entry separates Does/Trigger/Consequence/Confirm with blank lines.`,
		Scoring: `3 points: All consecutive field pairs separated by a blank line.
0 points: Any two field lines consecutive without a blank line.`,
		Examples:  `Between Does and Trigger, Trigger and Consequence, Consequence and Confirm.`,
		MaxPoints: 3.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDFourFields,
		Name:        "Exactly four fields per risk",
		Description: `Each Correctness Risks H3 has Does, Trigger, Consequence, Confirm in that order.`,
		Scoring: `2 points: All entries have exactly those four fields in order.
0 points: Missing, extra, or wrong order.`,
		Examples:  `Labeled **Does:** **Trigger:** **Consequence:** **Confirm:**.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDConfirmYesNo,
		Name:        "Confirm is a yes/no question",
		Description: `Every Confirm ends with ? and starts with an interrogative (Does, Is, Are, Has, Will, Did, Can, Would).`,
		Scoring: `2 points: All Confirm fields valid.
0 points: Any Confirm fails the rule.`,
		Examples:  `Good: Does the handler still run on nil session?`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDDeclarativeH3,
		Name:        "Risk H3s are declarative",
		Description: `Correctness Risks H3s are failure-mode statements, not questions.`,
		Scoring: `2 points: No H3 ends with ? or reads as a question.
0 points: Any interrogative H3.`,
		Examples:  `Fail: "### Will nil session panic?"`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDChecklistSkip,
		Name:        "Checklist ends with Skip",
		Description: `Last Verification Checklist bullet begins with Skip:.`,
		Scoring: `1 point: Last bullet is a Skip entry.
0 points: Last bullet is not Skip:.`,
		Examples:  `Skip: docs-only paths.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoEmDashConnectors,
		Name:        "No em dash connectors",
		Description: `No sentence uses an em dash as a clause connector.`,
		Scoring: `1 point: No em dash connectors.
0 points: Any em dash connector found.`,
		Examples:  `Title clarifications are OK.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoGenericPhrases,
		Name:        "No prohibited generic phrases",
		Description: `Tech review avoids banned hedging and generic quality phrases.`,
		Scoring: `1 point: None of the banned phrases appear.
0 points: Any banned phrase appears.`,
		Examples:  `Banned: may cause issues, could be a problem, needs review, might break, better error handling, more robust, cleaner code, improved maintainability.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
}
```


## ./internal/judge/evaluation/typology
- package: `typology`
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: ClusterCriterionIDs, CriterionIDAdapterSurfaces, CriterionIDClusterCounsel, CriterionIDClusterDelivery, CriterionIDDebtWhenFindings, CriterionIDInterventionCounsel, CriterionIDInterventionCoverage, CriterionIDInterventionNoInvent, CriterionIDInterventionStatus, CriterionIDInterventionTutorVoice, CriterionIDJourneyConsistent, CriterionIDJourneyCounsel, CriterionIDObjectives, CriterionIDRoleGrounding, CriterionIDSliceOwnership, CriterionIDSurfaces, CriterionIDs, FindingCommentCriterionIDs, InterventionBriefCriterionIDs, InterventionCriterionIDs, InterventionJourneyCriterionIDs, InterventionPRPriorityCriterionIDs
- exportedFuncs: Register
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: (none)
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/evaluation/typology/pack.go

### Exported bodies

#### Register (func)

```go
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDSurfaces,
		Name:        "Interaction packages on surfaces",
		Description: `User-facing entrypoint and server packages (from observed roles) belong under surfaces[]. Path words such as dashboard are not evidence. exec_runner packages are not interaction surfaces.`,
		Scoring: `2 points: entrypoint and server packages sit on surfaces when present; exec_runner stays off kind: cli.
0 points: Observed interaction roles remain only under owns[], or exec_runner is placed under kind: cli.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDDebtWhenFindings,
		Name:        "Debt recorded when findings remain",
		Description: `When the architecture brief still lists findings, the journey notes must include a non-empty technical debt / boundary violations table.`,
		Scoring: `2 points: Findings are empty, or journey debt table has at least one concrete row.
0 points: Findings remain and journey has no debt table content.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDObjectives,
		Name:        "Slice objectives are substantive",
		Description: `Every slice has a concrete business objective that states why the context exists. Template lines such as "Provide X functionality" fail.`,
		Scoring: `2 points: All slices have a non-empty, non-template business objective.
0 points: Any slice is missing an objective or uses hollow Provide-X-functionality wording.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDAdapterSurfaces,
		Name:        "Exec adapters are not CLI surfaces",
		Description: `Packages that wrap process execution (for example cliexec) are adapters under owns[], not kind: cli surfaces. Demoting them off CLI must keep them claimed under owns[]; never drop the package.`,
		Scoring: `2 points: Exec-adapter packages are owned domain/infrastructure components under owns[], not kind: cli surfaces, and remain claimed.
0 points: An exec-adapter package is placed under a kind: cli surface, or is omitted from the catalog entirely.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDJourneyConsistent,
		Name:        "Journey debt matches status",
		Description: `When Status claims refinement or merges are complete, debt rows must not still instruct Merge into as pending work.`,
		Scoring: `1 point: Journey status and debt table agree on what remains open.
0 points: Status says complete while debt still lists Merge into actions.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDJourneyCounsel,
		Name:        "Journey decisions and debt argue",
		Description: `journey_md decisions say what was rejected and why. Open debt rows carry smell, alternatives with a cost, and a lean. Hollow mitigations such as "Approve binding or refactor" fail.`,
		Scoring: `2 points: Decisions and open debt rows argue with alternatives and a lean.
0 points: Inventory-only debt, generic approve-or-refactor mitigations, or decisions without a rejected alternative.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDRoleGrounding,
		Name:        "Objectives respect capability constraints",
		Description: `Slice objectives must match the evidence-first slice_objective_ledger, and derived objective_claims must not intersect the must_not union of owned packages from package_capability_constraints (roles + fills_dto edges). dto/data_shape packages must not clai
// ... truncated
```


## ./internal/judge/modules
- package: `modules`
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: TaskBootstrapStory, TaskDigestStory, TaskFileReview, TaskSummary, TaskTechnical, TaskTypologyFindingComment, TaskTypologyHumanIntervention, TaskTypologyInspect, TaskTypologyInterventionBrief, TaskTypologyInterventionJourney, TaskTypologyInterventionPRPriority, TaskTypologyInterventionWeaknesses, TaskTypologySliceCatalog, TaskTypologySliceGrouping, TaskTypologySliceGroupingAudit, TaskTypologySliceMeaning
- exportedFuncs: BootstrapStoryModule, DigestStoryModule, FileReviewModule, SummaryModule, TechnicalModule, TypologyClusterModule, TypologyFindingCommentModule, TypologyHumanInterventionModule, TypologyInspectModule, TypologyInterventionBriefModule, TypologyInterventionJourneyModule, TypologyInterventionPRPriorityModule, TypologyInterventionWeaknessesModule, TypologyRefineModule
- exportedMethods: (none)
- unexportedDecls: _, consultantCounselContract
- unexportedFuncs: bootstrapStoryModule, digestStoryModule, fileReviewModule, in, newGenerator, out, summaryModule, technicalModule, typologyClusterModule, typologyFindingCommentModule, typologyHumanInterventionModule, typologyInspectModule, typologyInterventionBriefModule, typologyInterventionJourneyModule, typologyInterventionPRPriorityModule, typologyInterventionSharedInputs, typologyInterventionWeaknessesModule, typologyRefineModule
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/modules/export.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/modules/signatures.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/judge/modules/tasks.go

### Exported bodies

#### FileReviewModule (func)

```go
func FileReviewModule() core.Module { return fileReviewModule() }
```

#### DigestStoryModule (func)

```go
func DigestStoryModule() core.Module { return digestStoryModule() }
```

#### BootstrapStoryModule (func)

```go
func BootstrapStoryModule() core.Module { return bootstrapStoryModule() }
```

#### TypologyClusterModule (func)

```go
func TypologyClusterModule() core.Module { return typologyClusterModule() }
```

#### TypologyRefineModule (func)

```go
func TypologyRefineModule() core.Module { return typologyRefineModule() }
```

#### TypologyInspectModule (func)

```go
func TypologyInspectModule() core.Module { return typologyInspectModule() }
```

#### TypologyHumanInterventionModule (func)

```go
func TypologyHumanInterventionModule() core.Module { return typologyHumanInterventionModule() }
```

#### TypologyInterventionJourneyModule (func)

```go
func TypologyInterventionJourneyModule() core.Module { return typologyInterventionJourneyModule() }
```

#### TypologyInterventionBriefModule (func)

```go
func TypologyInterventionBriefModule() core.Module { return typologyInterventionBriefModule() }
```

#### TypologyInterventionWeaknessesModule (func)

```go
func TypologyInterventionWeaknessesModule() core.Module {
	return typologyInterventionWeaknessesModule()
}
```

#### TypologyInterventionPRPriorityModule (func)

```go
func TypologyInterventionPRPriorityModule() core.Module {
	return typologyInterventionPRPriorityModule()
}
```

#### TypologyFindingCommentModule (func)

```go
func TypologyFindingCommentModule() core.Module { return typologyFindingCommentModule() }
```

#### SummaryModule (func)

```go
func SummaryModule() core.Module { return summaryModule() }
```

#### TechnicalModule (func)

```go
func TechnicalModule() core.Module { return technicalModule() }
```

### Private one-hop bodies

#### bootstrapStoryModule (func)

```go
func bootstrapStoryModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("source_sha", "Default branch HEAD used as the bootstrap source"),
			in("generated_at", "RFC3339 bootstrap time"),
			in("evidence_mode", "Survey mode, such as discover, reuse, or fallback"),
			in("module_scope", "Typology module scope, when available"),
			in("readme_snapshot", "Current README snapshot from the served repo"),
			in("typology_manifest", "Typology evidence manifest YAML"),
			in("typology_architecture", "Post-refine Typology architecture brief or fallback architecture survey"),
			in("typology_refined_catalog", "Refined Typology catalog YAML proposal, when available"),
			in("slice_meaning_ledger", "Evidence-first slice meaning ledger YAML when catalog assemble ran"),
			in("typology_journey", "Compressed typology journey notes and boundary debt, when available"),
			in("repo_layout", "Top-level repo layout and notable evidence files"),
			in("current_readme", "Current bootstrap README placeholder"),
			in("current_mission", "Current bootstrap mission placeholder"),
			in("current_architecture", "Current bootstrap architecture placeholder"),
			in("current_conventions", "Current bootstrap conventions placeholder"),
			in("current_weaknesses", "Current bootstrap weaknesses placeholder"),
			in("current_chronology", "Current bootstrap chronology placeholder"),
			in("current_grounding", "Current bootstrap agenting grounding placeholder"),
			in("validation_feedback", "Optional evaluator feedback to fix on retry"),
		},
		[]core.OutputField{
			out("readme_md", "Updated context-branch README markdown"),
			out("mission_md", "Updated mission markdown"),
			out("architecture_md", "Updated architecture markdown"),
			out("conventions_md", "Updated conventions markdown"),
			out("weaknesses_md", "Updated weaknesses markdown"),
			out("chronology_md", "Updated chronology markdown"),
			out("grounding_md", "Updated agenting grounding markdown"),
		},
	).WithInstruction(`Seed the context branch for the served repository (repo_id), not Majordomo the control plane (unless repo_id is majordomo).
Write all outputs as present-tense, user-facing markdown about that product.
Prefer the slice objective ledger and refined Typology catalog over raw package inventory when they are present.
The README should describe the context branch and its seed origin for the served repo.
README MUST keep a ## Reading order section (story path then evidence/typology). Preserve <!-- majordomo-reading-toc:start --> / <!-- majordomo-reading-toc:end --> and <!-- majordomo-reading-nav:start --> / <!-- majordomo-reading-nav:end --> blocks when present; digest re-applies them if dropped.
Mission, architecture, conventions, and weaknesses must be evidence-backed and should not mention historical events that are not in the supplied evidence.
Mission, architecture, and grounding MUST name the served product from evidence; MUST NOT describe Majordomo triage, digest, or context-branch process as the product.
Architecture should describe proposed bounded contexts (slices), surfaces, and known boundary debt from the journey notes and ledger.
Root architecture_md is the teaching story for humans and review grounding. Keep the Typology evidence brief (typology_architecture input) as source material; do not pretend it is the confirmed catalog.
Do not copy hollow template slice objectives; paraphrase into concrete teaching language grounded in the ledger, catalog, and README.
Chronology must stay honest, with at most a single explicit seed marker. Do not reconstruct past decisions.
The grounding output should summarize the accepted mission and architecture for agenting on the served product.
If evidence is thin, keep the section minimal rather than inventing details.
When validation_feedback is present, fix those issues before emitting.
Preserve each file's markdown shape and heading conventions. Preserve major
// ... truncated
```

#### digestStoryModule (func)

```go
func digestStoryModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("section_id", "Story section id (mission, architecture, conventions, weaknesses)"),
			in("current_text", "Current section markdown"),
			in("commit_subject", "First-parent commit subject"),
			in("commit_diff", "Capped commit diff"),
			in("changed_files", "Comma-separated changed paths"),
			in("regen_feedback", "Optional gate reject feedback to address"),
		},
		[]core.OutputField{out("updated_text", "Updated section markdown; preserve structure; only add evidenced claims")},
	).WithInstruction(`Update one teaching-story section after a default-branch commit.
Amend only when the commit diff supports a concrete claim. If nothing applies, return current_text unchanged.
Never invent architecture or risks. Chronology is handled separately.
Preserve <!-- majordomo-reading-nav:start --> / <!-- majordomo-reading-nav:end --> banners when present.`)
	return newGenerator(sig, TaskDigestStory)
}
```

#### fileReviewModule (func)

```go
func fileReviewModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("file_path", "Repository-relative path under review"),
			in("slug", "Stable slug for the per-file report filename"),
			in("diff_content", "Staged diff or file content for review"),
			in("grounding", "Optional grounding markdown from agenting packs"),
		},
		[]core.OutputField{out("markdown", "Per-file review markdown with # header and - [SEVERITY] findings or 'No issues found.'")},
	).WithInstruction(`Review one changed file. Output markdown only.
Use exactly one H1 with the file path, then bullet findings as - [CRITICAL|WARN|INFO] text.
If there are no issues, write "No issues found." after the header.
Do not invent issues; only cite evidence from the diff.`)
	return newGenerator(sig, TaskFileReview)
}
```

#### in (func)

```go
func in(name, desc string) core.InputField {
	return core.InputField{Field: core.NewField(name, core.WithDescription(desc))}
}
```

#### summaryModule (func)

```go
func summaryModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("staging_context", "Summary staging context and diffs"),
		},
		[]core.OutputField{out("summary_md", "PR summary markdown matching Majordomo summary rubric")},
	).WithInstruction("Write a PR summary following Majordomo summary structure and rubric.")
	return newGenerator(sig, TaskSummary)
}
```

#### technicalModule (func)

```go
func technicalModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("staging_context", "Technical review staging context"),
		},
		[]core.OutputField{out("technical_md", "Technical review markdown")},
	).WithInstruction("Write a technical PR review following Majordomo tech rubric.")
	return newGenerator(sig, TaskTechnical)
}
```

#### typologyClusterModule (func)

```go
func typologyClusterModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("module_scope", "Typology module scope"),
			in("draft_catalog_yaml", "Raw Typology discover draft YAML"),
			in("graph_text", "typology show graph output"),
			in("package_contracts", "Per-package public contracts from typology contracts"),
			in("package_roles", "Observed package role topology YAML: role, confidence, evidence, labeled edges. Folder names are not evidence."),
			in("package_capability_constraints", "Durable is/must_not capability codes per package (and filled_by from fills_dto edges). Factual; MUST NOT contradict."),
			in("mechanical_grouping_yaml", "Deterministic door-walk seed YAML: door-private vs shared vs unreached, libraries, product clumps."),
			in("architecture_draft", "Architecture brief for the raw draft"),
			in("repo_layout", "Top-level layout names"),
			in("readme_snapshot", "Served-repo README: product purpose and delivery commands"),
			in("validation_feedback", "Optional prior structure-validation feedback to fix"),
		},
		[]core.OutputField{
			// Flat strings (not XML arrays): empty [] fails strop mandatory validation.
			// When proposing no folds, emit the literal "none" in each field.
			// Go splits and zips into slice_grouping_proposal.yaml.
			out("merge_ids", "Comma-separated free-form merge nickname ids, or the literal none when proposing no folds"),
			out("merge_packages", "Semicolon-separated package groups (comma-separated paths inside each group), same order as merge_ids; or none"),
			out("merge_intents", "Comma-separated intents: each value MUST be exactly the literal slice or nickname (same order as merge_ids; never repeat the merge id); or none"),
		},
	).WithInstruction(`You are the unattended Typology slice-grouping pass for Majordomo context digest.
A discover draft is package-level inventory. package_roles is the factual observed topology. Grouping is an optional overlay and MUST NOT contradict package_roles.
package_capability_constraints is factual is/is-not prior. MUST NOT contradict it.

Order of evidence (MUST):
1. package_roles: each package already has role + confidence + evidence from code (entrypoint, server, dto, exec_runner, aggregator, adapter, config, observability, unknown).
2. package_capability_constraints: portable is / must_not codes and filled_by from fills_dto edges.
3. mechanical_grouping_yaml: deterministic door-walk seed. It is authoritative for door-private vs shared vs unreached facts; the LLM must not silently override those facts.
4. package_contracts and readme_snapshot: supporting facts.
5. graph_text: coupling and wiring only. Labeled edges in package_roles (fills_dto, uses_runner, serves_server, composes, reads_config) explain imports.
6. Folder and path words (dashboard, board, cli, server) are NEVER evidence and MUST NOT relabel a node.

Hard rules from observed roles and the door-walk seed:
- entrypoint and server are distinct doors. MUST NOT merge them. Cross-door wiring is a note, not ownership.
- Door-private packages may form product slices for that door only.
- Shared-across-doors packages must not be claimed as sole ownership for one door. Shared is not the same as library; only dto/config/exec_runner/observability (and similar technical roles) are library by role.
- Unreached packages MUST NOT be auto-owned; argue or leave debt.
- dto packages are shared data contracts; MUST NOT merge them into an aggregator or call them the product domain.
- aggregator packages build page/domain data; MUST NOT label them kind: ui or "the website".
- exec_runner packages are technical runners; MUST NOT put them under the entrypoint's domain just because the entrypoint also imports them.
- observability packages boot tracing/metrics; they are not config.
- fills_dto edges mean adapters fill JSON types; they are NOT "forge depends on the UI".
- uses_runner edges mean a package shells out through
// ... truncated
```

#### typologyFindingCommentModule (func)

```go
func typologyFindingCommentModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("architecture_md", "Post-refine Typology architecture brief"),
			in("refined_catalog_yaml", "Refined Typology catalog YAML"),
			in("journey_md", "Updated journey markdown"),
			in("human_intervention_md", "Operator briefing"),
			in("finding", "One open architecture finding to discuss on the context PR"),
			in("validation_feedback", "Optional prior validation feedback to fix"),
		},
		[]core.OutputField{
			out("comment_md", "Tutor-voice PR comment body for this single finding"),
		},
	).WithInstruction(`You write one context-PR comment for a single open architecture finding so humans can discuss it in-thread.
` + consultantCounselContract + `

Rules:
- Cover only the given finding. Mention enough of the finding text (or a backticked id from it) that coverage checks match.
- Tutor voice: situation, smell/risk, alternatives with a cost, recommended lean.
- MUST NOT invent catalog YAML or ask humans to rubber-stamp mechanical slice-to-library bindings.
- MUST NOT say "see journey_md" for the real argument.
- Output markdown only in comment_md (no HTML markers; the host adds those).
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyFindingComment)
}
```

#### typologyHumanInterventionModule (func)

```go
func typologyHumanInterventionModule() *dspymodules.DirectivesCoT {
	// Legacy alias: brief-only so old task names keep a registered module.
	return typologyInterventionBriefModule()
}
```

#### typologyInspectModule (func)

```go
func typologyInspectModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("package_path", "Repository-relative package path being inspected"),
			in("package_contracts", "Contract row for this package if available"),
			in("package_source", "Go source files for this package only"),
			in("candidate_role", "Optional mechanical candidate role"),
			in("current_evidence", "Mechanical evidence ids already known"),
		},
		[]core.OutputField{
			out("role", "One of: entrypoint, server, dto, exec_runner, aggregator, adapter, config, observability, unknown"),
			out("evidence", "Short symbol-based evidence quotes; never the directory name"),
		},
	).WithInstruction(`Classify one Go package into an observed role from its source and contracts only.
Allowed roles: entrypoint, server, dto, exec_runner, aggregator, adapter, config, observability, unknown.
MUST NOT use the directory or folder name as evidence (ignore words like dashboard, board, cli, server in the path).
	Cite exported symbols and import paths only (os/exec, net/http, google.golang.org/grpc, go.opentelemetry.io, gopkg.in/yaml.v3, go:embed, JSON/YAML tags, ServeHTTP, Register*Server).
MUST NOT invent a role from English function names. If unsure, return role unknown.`)
	return newGenerator(sig, TaskTypologyInspect)
}
```

#### typologyInterventionBriefModule (func)

```go
func typologyInterventionBriefModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		typologyInterventionSharedInputs(),
		[]core.OutputField{
			out("human_intervention_md", "Tutor-voice operator briefing of priority decisions humans must make"),
		},
	).WithInstruction(`You write the operator human-intervention briefing after Typology refine.
Humans give direction and leadership. Surface architecture findings the unattended refine must NOT invent away.
` + consultantCounselContract + `

Rules:
- findings_list is authoritative. Every finding MUST appear in human_intervention_md.
- Output markdown only. MUST NOT invent sliceBindings, libraries[] rows, rewrite package ownership, or invent catalog YAML.
- Majordomo completes evidenced slice-to-library SliceBindings in the proposal catalog. MUST NOT ask humans to rubber-stamp those mechanical edges.
- Frame remaining findings as normative human decisions: keep library placement, fold into a domain slice, decouple, approve slice-to-slice binding, merge slices, or accept temporary debt.
- Tutor briefing: situation, smell/risk, alternatives with a cost, recommended lean, and evidence pointers. MUST NOT punt to journey_md.
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyInterventionBrief)
}
```

#### typologyInterventionJourneyModule (func)

```go
func typologyInterventionJourneyModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		typologyInterventionSharedInputs(),
		[]core.OutputField{
			out("journey_md", "Updated journey with open Status and debt covering every finding"),
		},
	).WithInstruction(`You write typology journey notes after refine and architecture so open findings cannot hide.
` + consultantCounselContract + `

Rules:
- findings_list is authoritative. Every finding MUST appear in the Technical debt and boundary violations table.
- When findings_list is non-empty, journey Status MUST stay open (not complete/completed).
- When debt rows still say Merge into, journey Status MUST stay open (not complete/completed).
- Journey MUST include Status, decisions already taken, and a debt table that names every finding with smell, alternatives with a cost, and a lean.
- MUST NOT flatten debt rows to hollow "Approve binding or refactor".
- MUST NOT invent catalog YAML, sliceBindings, or libraries membership.
- Output markdown only in journey_md.
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyInterventionJourney)
}
```

#### typologyInterventionPRPriorityModule (func)

```go
func typologyInterventionPRPriorityModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		append(typologyInterventionSharedInputs(),
			in("human_intervention_md", "Operator briefing already produced for these findings"),
		),
		[]core.OutputField{
			out("pr_priority_md", "Context PR summary markdown in tutor voice for a cold reader"),
		},
	).WithInstruction(`You write the GitHub context-PR summary a cold reader sees first, in a tutor voice.
` + consultantCounselContract + `

Rules:
- findings_list is authoritative. Every finding MUST appear in pr_priority_md.
- Assume the reader has never seen this repo. Lead with what Majordomo's Typology digest is proposing on this context branch and why it matters, then smell, alternatives, and the lean.
- Frame slice consolidations and library placements as proposed catalog models for grounding, not as work the product team already shipped.
- Gloss jargon in the same sentence. Name packages by role as well as id so coverage checks still match.
- MUST NOT use only imperative task titles such as "Formalize Config Access".
- MUST NOT dump catalog ids without a gloss. MUST NOT say "see journey_md".
- MUST NOT invent catalog YAML or ask humans to rubber-stamp mechanical slice-to-library bindings.
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyInterventionPRPriority)
}
```

#### typologyInterventionWeaknessesModule (func)

```go
func typologyInterventionWeaknessesModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		append(typologyInterventionSharedInputs(),
			in("human_intervention_md", "Operator briefing already produced for these findings"),
		),
		[]core.OutputField{
			out("weaknesses_seed_md", "Weaknesses markdown bullets for bootstrap story seeding"),
		},
	).WithInstruction(`You seed weaknesses.md from open architecture findings and the operator briefing.
` + consultantCounselContract + `

Rules:
- findings_list is authoritative. Every finding MUST appear as a weakness bullet.
- Keep the same priorities and leans as human_intervention_md, but shorter.
- Output markdown only starting with # Weaknesses.
- MUST NOT invent catalog YAML or claim findings are resolved.
When validation_feedback is present, fix those issues before emitting.`)
	return newGenerator(sig, TaskTypologyInterventionWeaknesses)
}
```

#### typologyRefineModule (func)

```go
func typologyRefineModule() *dspymodules.DirectivesCoT {
	sig := core.NewSignature(
		[]core.InputField{
			in("repo_id", "Served repository id"),
			in("module_scope", "Typology module scope"),
			in("draft_catalog_yaml", "Raw Typology discover draft YAML"),
			in("slice_grouping_proposal_yaml", "Proposed merges YAML composed from cluster CoT (id/packages/intent rows)"),
			in("slice_grouping_verdicts_yaml", "Durable cluster merge audit verdicts: accept|overlay|reject per proposed package set"),
			in("package_contracts", "Per-package public contracts from typology contracts"),
			in("package_roles", "Observed package role topology YAML: role, confidence, evidence, labeled edges"),
			in("package_capability_constraints", "Durable is/must_not capability codes per package; factual; MUST NOT contradict"),
			in("slice_meaning_ledger_yaml", "Authoritative evidence-first ledger: slice id, evidence quotes, claims, objective; copy objectives verbatim"),
			in("architecture_draft", "Architecture brief for the raw draft"),
			in("repo_layout", "Top-level layout names"),
			in("readme_snapshot", "Served-repo README: product purpose and delivery commands"),
			in("validation_feedback", "Optional ValidateStructure or boundary-evaluator feedback to fix"),
		},
		[]core.OutputField{
			// Explicit exception: full catalog stays one YAML string leaf until a follow-up splits slices into XML items.
			out("refined_catalog_yaml", "Full refined Typology catalog as a YAML document string (not a list field)"),
		},
	).WithInstruction(`You are the unattended Typology slice catalog writer for Majordomo context digest.
Apply accepted slice-grouping merges to the draft catalog and emit a complete refined typology.yaml.
package_roles, package_capability_constraints, slice_grouping_verdicts_yaml, and slice_meaning_ledger_yaml are factual. MUST NOT contradict them.
Folder names are never evidence.

slice_meaning_ledger_yaml already settled each owned slice's meaning (evidence, claims, objective).
For every ledger slice id that still owns packages, copy the ledger objective into the catalog verbatim.
When you merge draft neighborhoods into one refined slice, copy ONE contributing ledger objective verbatim (do not invent a prestige blend).
MUST NOT invent prestige objectives beyond the ledger. MUST NOT escalate data_shape slices into synchronize_state or merge_adapters stories.
Claims are produced by the ledger stage in Go; do not invent a competing claims sidecar.

Placement from roles:
- entrypoint -> kind: cli surfaces
- server -> surfaces (api, grpc, or ui when evidence includes embeds_static); NEVER fold into the CLI surface for sole importer; KEEP on the same slice as an api surface when other packages import the server
- dto -> owns[] (or a thin shared data slice); NEVER the product domain from graph position; NEVER owned by an aggregator just because that aggregator imports it
- aggregator -> owns[] of a product slice; MUST NOT kind: ui
- exec_runner -> owns[] or libraries[]; NEVER under the entrypoint domain solely because cmd imports it
- observability -> owns[] or libraries[]; NEVER config
- adapter / config -> owns[] or libraries[] as fits

Fold packages into one slice or libraries[].owns[] ONLY when slice_grouping_verdicts_yaml marks that package set verdict: accept.
overlay and reject rows stay separate package owners; nicknames MUST NOT disguise as libraries[].owns[].
MUST NOT invent "forge depends on UI" or "localgit depends on CLI" smells from false ownership.

Catalog rules:
- Every slice MUST have a non-empty business objective that states why the bounded context exists in one concrete sentence.
- MUST NOT use hollow template objectives such as "Provide X functionality", "Provide X capabilities", or "Provide X services".
- Each slice MUST use at most one owns block, one surfaces block, and one libraries block. List every package for that slice inside the same block instead of repeating the key.
- Components are packages under owns, under
// ... truncated
```


## ./internal/llmusage
- package: `llmusage`
- packageDoc: Package llmusage aggregates provider-reported LLM token usage for a Majordomo run.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: Collector, Summary, TaskRow
- exportedFuncs: Active, Format, FromContext, New, Pop, Push, RecordExecutionState, WithCollector
- exportedMethods: Collector.Add, Collector.AddFromTokenUsage, Collector.AddTokenUsageValue, Collector.Snapshot
- unexportedDecls: active, activeMu, ctxKey, taskBucket
- unexportedFuncs: (none)
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/llmusage/llmusage.go

### Exported bodies

#### TaskRow (type)

```go
type TaskRow struct {
	Task              string `json:"task"`
	PromptTokens      int    `json:"prompt_tokens"`
	CompletionTokens  int    `json:"completion_tokens"`
	TotalTokens       int    `json:"total_tokens"`
	Calls             int    `json:"calls"`
	MissingUsageCalls int    `json:"missing_usage_calls,omitempty"`
}
```

#### Summary (type)

```go
type Summary struct {
	Tasks             []TaskRow `json:"tasks"`
	PromptTokens      int       `json:"prompt_tokens"`
	CompletionTokens  int       `json:"completion_tokens"`
	TotalTokens       int       `json:"total_tokens"`
	Calls             int       `json:"calls"`
	MissingUsageCalls int       `json:"missing_usage_calls,omitempty"`
}
```

#### Collector (type)

```go
type Collector struct {
	mu     sync.Mutex
	byTask map[string]*taskBucket
}
```

#### New (func)

```go
func New() *Collector {
	return &Collector{byTask: make(map[string]*taskBucket)}
}
```

#### Collector.Add (method)

```go
func (c *Collector) Add(task string, prompt, completion, total int) {
	if c == nil {
		return
	}
	task = strings.TrimSpace(task)
	if task == "" {
		task = "unknown"
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	b := c.byTask[task]
	if b == nil {
		b = &taskBucket{}
		c.byTask[task] = b
	}
	b.prompt += prompt
	b.completion += completion
	b.total += total
	b.calls++
	if prompt == 0 && completion == 0 && total == 0 {
		b.missing++
	}
}
```

#### Collector.AddFromTokenUsage (method)

```go
func (c *Collector) AddFromTokenUsage(task string, usage *core.TokenUsage) {
	if c == nil {
		return
	}
	if usage == nil {
		c.Add(task, 0, 0, 0)
		return
	}
	c.Add(task, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
}
```

#### Collector.AddTokenUsageValue (method)

```go
func (c *Collector) AddTokenUsageValue(task string, usage core.TokenUsage) {
	if c == nil {
		return
	}
	c.Add(task, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
}
```

#### Collector.Snapshot (method)

```go
func (c *Collector) Snapshot() Summary {
	if c == nil {
		return Summary{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := Summary{Tasks: make([]TaskRow, 0, len(c.byTask))}
	for task, b := range c.byTask {
		out.Tasks = append(out.Tasks, TaskRow{
			Task:              task,
			PromptTokens:      b.prompt,
			CompletionTokens:  b.completion,
			TotalTokens:       b.total,
			Calls:             b.calls,
			MissingUsageCalls: b.missing,
		})
		out.PromptTokens += b.prompt
		out.CompletionTokens += b.completion
		out.TotalTokens += b.total
		out.Calls += b.calls
		out.MissingUsageCalls += b.missing
	}
	sort.Slice(out.Tasks, func(i, j int) bool { return out.Tasks[i].Task < out.Tasks[j].Task })
	return out
}
```

#### Format (func)

```go
func Format(s Summary) string {
	if s.Calls == 0 && len(s.Tasks) == 0 {
		return "LLM usage: (no LLM calls recorded)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "LLM usage total: prompt=%d completion=%d total=%d calls=%d",
		s.PromptTokens, s.CompletionTokens, s.TotalTokens, s.Calls)
	if s.MissingUsageCalls > 0 {
		fmt.Fprintf(&b, " missing_usage_calls=%d", s.MissingUsageCalls)
	}
	b.WriteByte('\n')
	b.WriteString("LLM usage by task:")
	for _, row := range s.Tasks {
		fmt.Fprintf(&b, "\n  %s: prompt=%d completion=%d total=%d calls=%d",
			row.Task, row.PromptTokens, row.CompletionTokens, row.TotalTokens, row.Calls)
		if row.MissingUsageCalls > 0 {
			fmt.Fprintf(&b, " missing_usage_calls=%d", row.MissingUsageCalls)
		}
	}
	return b.String()
}
```

#### WithCollector (func)

```go
func WithCollector(ctx context.Context, c *Collector) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, c)
}
```

#### FromContext (func)

```go
func FromContext(ctx context.Context) *Collector {
	if ctx != nil {
		if c, ok := ctx.Value(ctxKey{}).(*Collector); ok && c != nil {
			return c
		}
	}
	return Active()
}
```

#### RecordExecutionState (func)

```go
func RecordExecutionState(ctx context.Context, task string) {
	c := FromContext(ctx)
	if c == nil {
		return
	}
	state := core.GetExecutionState(ctx)
	if state == nil {
		c.Add(task, 0, 0, 0)
		return
	}
	c.AddFromTokenUsage(task, state.GetTokenUsage())
}
```

#### Push (func)

```go
func Push(c *Collector) {
	if c == nil {
		return
	}
	activeMu.Lock()
	defer activeMu.Unlock()
	active = append(active, c)
}
```

#### Pop (func)

```go
func Pop() {
	activeMu.Lock()
	defer activeMu.Unlock()
	if len(active) == 0 {
		return
	}
	active = active[:len(active)-1]
}
```

#### Active (func)

```go
func Active() *Collector {
	activeMu.Lock()
	defer activeMu.Unlock()
	if len(active) == 0 {
		return nil
	}
	return active[len(active)-1]
}
```

### Private one-hop bodies

#### ctxKey (type)

```go
type ctxKey struct{}
```

#### taskBucket (type)

```go
type taskBucket struct {
	prompt, completion, total int
	calls, missing            int
}
```


## ./internal/observability
- package: `observability`
- packageDoc: Package observability provides OpenTelemetry tracing and inference failure dumps.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: true
- importsOsExec: false
- importsGrpc: false
- importsOtel: true
- importsPrometheus: false
- mechanicalRole: observability
- mechanicalConfidence: 0.90
- mechanicalEvidence: imports_otel
- exportedDecls: Config, DefaultServiceName, Settings
- exportedFuncs: EndSpanWithStatus, Flush, Init, InstrumentHTTPClient, NewFailureDumpProcessorForTest, ResolveConfig, Shutdown, StartChainSpan, TraceIDFromContext, WrapRoundTripper
- exportedMethods: errorDetailTransport.RoundTrip, failureDumpProcessor.ForceFlush, failureDumpProcessor.OnEnd, failureDumpProcessor.OnStart, failureDumpProcessor.Shutdown, redactedError.Error, redactedError.Unwrap
- unexportedDecls: _, contextKey, defaultFailureDumpDir, dumpedEvent, dumpedSpan, dumpedTrace, embeddedHTTPURL, errorDetailTransport, failureDumpProcessor, globalInit, globalInitErr, globalTP, httpServiceLabel, redactedError, redactedSecret, traceDumpBuffer
- unexportedFuncs: attributesToMap, classifyHTTPError, httpClientSpanName, isSecretQueryName, isTimeout, newFailureDumpProcessor, otlpInsecureDefault, pruneFailureDumps, redactHTTPError, redactQueryString, redactQueryValues, redactSpanRequestURL, redactURLString, redactURLsInText, sanitizeAttrValue, snapshotEvents, snapshotSpan, statusCodeString, withErrorDetail
- unexportedMethods: failureDumpProcessor.writeLocked
- errorTypes: redactedError
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/observability/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/observability/failure_dump.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/observability/http.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/observability/otel.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/observability/redact.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/observability/tracing.go

### Exported bodies

#### NewFailureDumpProcessorForTest (func)

```go
func NewFailureDumpProcessorForTest(dir string, maxAgeHours, maxFiles int) sdktrace.SpanProcessor {
	return newFailureDumpProcessor(dir, maxAgeHours, maxFiles)
}
```

#### failureDumpProcessor.OnStart (method)

```go
func (p *failureDumpProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}
```

#### failureDumpProcessor.OnEnd (method)

```go
func (p *failureDumpProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if p == nil || s == nil {
		return
	}
	snap := snapshotSpan(s)
	traceID := snap.TraceID
	if traceID == "" {
		return
	}
	isRoot := snap.ParentSpanID == ""
	isError := s.Status().Code == codes.Error

	p.mu.Lock()
	defer p.mu.Unlock()
	buf := p.traces[traceID]
	if buf == nil {
		buf = &traceDumpBuffer{}
		p.traces[traceID] = buf
	}
	buf.spans = append(buf.spans, snap)
	if isError {
		buf.hasError = true
	}
	if !isRoot {
		return
	}
	delete(p.traces, traceID)
	if !isError {
		return
	}
	p.writeLocked(dumpedTrace{
		TraceID:           traceID,
		DumpedAt:          time.Now().UTC(),
		Reason:            "root_span_error",
		RootName:          snap.Name,
		RootStatusMessage: snap.StatusMessage,
		Spans:             buf.spans,
	})
}
```

#### failureDumpProcessor.Shutdown (method)

```go
func (p *failureDumpProcessor) Shutdown(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for tid, buf := range p.traces {
		if buf != nil && buf.hasError {
			p.writeLocked(dumpedTrace{
				TraceID:  tid,
				DumpedAt: time.Now().UTC(),
				Reason:   "shutdown_with_error_spans",
				Spans:    buf.spans,
			})
		}
		delete(p.traces, tid)
	}
	return nil
}
```

#### failureDumpProcessor.ForceFlush (method)

```go
func (p *failureDumpProcessor) ForceFlush(context.Context) error { return nil }
```

#### errorDetailTransport.RoundTrip (method)

```go
func (t *errorDetailTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	span := trace.SpanFromContext(req.Context())
	redactSpanRequestURL(span, req)
	started := time.Now()
	resp, err := t.base.RoundTrip(req)
	if !span.IsRecording() {
		return resp, redactHTTPError(err)
	}
	redactSpanRequestURL(span, req)
	span.SetAttributes(attribute.Int64("http.duration_ms", time.Since(started).Milliseconds()))
	if err != nil {
		redacted := redactHTTPError(err)
		span.SetAttributes(
			attribute.String("error.class", classifyHTTPError(redacted)),
			attribute.String("error.message", redactURLsInText(redacted.Error())),
			attribute.Bool("timeout", isTimeout(redacted)),
		)
		return resp, redacted
	}
	if resp != nil && resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, resp.Status)
		span.SetAttributes(attribute.Bool("http.error", true))
	}
	return resp, err
}
```

#### WrapRoundTripper (func)

```go
func WrapRoundTripper(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if _, ok := base.(*otelhttp.Transport); ok {
		return base
	}
	return otelhttp.NewTransport(withErrorDetail(base),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return httpClientSpanName(r)
		}),
		otelhttp.WithSpanOptions(trace.WithAttributes(
			attribute.String("openinference.span.kind", "CHAIN"),
			attribute.String("http.io", "client"),
		)),
	)
}
```

#### InstrumentHTTPClient (func)

```go
func InstrumentHTTPClient(client *http.Client) {
	if client == nil {
		return
	}
	client.Transport = WrapRoundTripper(client.Transport)
}
```

#### Settings (type)

```go
type Settings struct {
	Enabled     *bool
	Endpoint    string
	APIKey      string
	ServiceName string
	Insecure    *bool
}
```

#### Config (type)

```go
type Config struct {
	ServiceName            string
	OTLPEndpoint           string // empty skips OTLP; dumps still work when FailureDumpDir set
	OTLPAPIKey             string // Bearer token for Phoenix/Arize; empty skips auth header
	OTLPInsecure           bool   // plaintext gRPC (typical for local Phoenix)
	FailureDumpDir         string
	FailureDumpMaxAgeHours int
	FailureDumpMaxFiles    int
	Enabled                bool
}
```

#### ResolveConfig (func)

```go
func ResolveConfig(outputDir string, fromFile ...Settings) Config {
	var file Settings
	if len(fromFile) > 0 {
		file = fromFile[0]
	}

	enabled := true
	if file.Enabled != nil {
		enabled = *file.Enabled
	}
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_ENABLED")); v != "" {
		enabled = !(v == "0" || strings.EqualFold(v, "false"))
	}

	endpoint := strings.TrimSpace(file.Endpoint)
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_ENDPOINT")); v != "" {
		endpoint = v
	} else if v := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")); v != "" {
		endpoint = v
	}

	apiKey := strings.TrimSpace(file.APIKey)
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_API_KEY")); v != "" {
		apiKey = v
	} else if v := strings.TrimSpace(os.Getenv("PHOENIX_API_KEY")); v != "" {
		apiKey = v
	}

	svc := strings.TrimSpace(file.ServiceName)
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_SERVICE_NAME")); v != "" {
		svc = v
	}
	if svc == "" {
		svc = DefaultServiceName
	}

	insecure := otlpInsecureDefault(endpoint)
	if file.Insecure != nil {
		insecure = *file.Insecure
	}
	if v := strings.TrimSpace(os.Getenv("MAJORDOMO_OTEL_INSECURE")); v != "" {
		insecure = !(v == "0" || strings.EqualFold(v, "false"))
	}

	dumpDir := strings.TrimSpace(os.Getenv("MAJORDOMO_INFERENCE_DUMP_DIR"))
	if dumpDir == "" {
		if outputDir != "" {
			dumpDir = outputDir + "/logs/inference-failures"
		} else {
			// Cwd-relative scratch when no output dir; keep out of the repo tree.
			dumpDir = "tmp/logs/inference-failures"
		}
	}

	return Config{
		ServiceName:            svc,
		OTLPEndpoint:           endpoint,
		OTLPAPIKey:             apiKey,
		OTLPInsecure:           insecure,
		FailureDumpDir:         dumpDir,
		FailureDumpMaxAgeHours: 48,
		FailureDumpMaxFiles:    20,
		Enabled:                enabled,
	}
}
```

#### Init (func)

```go
func Init(cfg Config) (*sdktrace.TracerProvider, error) {
	globalInit.Do(func() {
		if !cfg.Enabled {
			return
		}
		serviceName := cfg.ServiceName
		if serviceName == "" {
			serviceName = DefaultServiceName
		}
		res, err := resource.New(context.Background(),
			resource.WithAttributes(
				semconv.ServiceNameKey.String(serviceName),
				semconv.ServiceVersionKey.String("1.0.0"),
			),
		)
		if err != nil {
			globalInitErr = fmt.Errorf("otel resource: %w", err)
			return
		}
		opts := []sdktrace.TracerProviderOption{
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.TraceIDRatioBased(1.0)),
		}
		if cfg.FailureDumpDir != "" {
			opts = append(opts, sdktrace.WithSpanProcessor(newFailureDumpProcessor(
				cfg.FailureDumpDir,
				cfg.FailureDumpMaxAgeHours,
				cfg.FailureDumpMaxFiles,
			)))
		}
		if cfg.OTLPEndpoint != "" {
			endpoint := strings.TrimPrefix(strings.TrimPrefix(cfg.OTLPEndpoint, "https://"), "http://")
			exporterOpts := []otlptracegrpc.Option{
				otlptracegrpc.WithEndpoint(endpoint),
			}
			if cfg.OTLPInsecure {
				exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
			}
			if key := strings.TrimSpace(cfg.OTLPAPIKey); key != "" {
				// gRPC metadata keys must be lowercase for Phoenix auth.
				bearer := key
				if !strings.HasPrefix(strings.ToLower(bearer), "bearer ") {
					bearer = "Bearer " + key
				}
				exporterOpts = append(exporterOpts, otlptracegrpc.WithHeaders(map[string]string{
					"authorization": bearer,
				}))
			}
			exporter, exportErr := otlptracegrpc.New(context.Background(), exporterOpts...)
			if exportErr != nil {
				globalInitErr = fmt.Errorf("otlp exporter: %w", exportErr)
				return
			}
			opts = append(opts, sdktrace.WithBatcher(
				exporter,
				sdktrace.WithBatchTimeout(2*time.Second),
				sdktrace.WithMaxExportBatchSize(512),
			))
		}
		tp := sdktrace.NewTracerProvider(opts...)
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		globalTP = tp
	})
	return globalTP, globalInitErr
}
```

#### Shutdown (func)

```go
func Shutdown(ctx context.Context) error {
	if globalTP == nil {
		return nil
	}
	err := globalTP.Shutdown(ctx)
	globalTP = nil
	return err
}
```

#### Flush (func)

```go
func Flush(ctx context.Context) error {
	if globalTP == nil {
		return nil
	}
	return globalTP.ForceFlush(ctx)
}
```

#### redactedError.Error (method)

```go
func (e *redactedError) Error() string { return e.msg }
```

#### redactedError.Unwrap (method)

```go
func (e *redactedError) Unwrap() error { return e.orig }
```

#### StartChainSpan (func)

```go
func StartChainSpan(ctx context.Context, serviceName, operationName string) (context.Context, trace.Span) {
	if serviceName == "" {
		serviceName = DefaultServiceName
	}
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, operationName, trace.WithAttributes(
		attribute.String("openinference.span.kind", "CHAIN"),
		attribute.String("command.operation", operationName),
		attribute.String("service.name", serviceName),
	))
	spanCtx := span.SpanContext()
	ctx = core.WithExecutionState(ctx)
	if state := core.GetExecutionState(ctx); state != nil {
		_ = state
		ctx = context.WithValue(ctx, contextKey("otel.trace_id"), spanCtx.TraceID().String())
		ctx = context.WithValue(ctx, contextKey("otel.span_id"), spanCtx.SpanID().String())
	}
	return ctx, span
}
```

#### EndSpanWithStatus (func)

```go
func EndSpanWithStatus(span trace.Span, err *error) {
	if span == nil {
		return
	}
	if err != nil && *err != nil {
		span.RecordError(*err)
		span.SetStatus(codes.Error, (*err).Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}
```

#### TraceIDFromContext (func)

```go
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	span := trace.SpanFromContext(ctx)
	if span != nil {
		sc := span.SpanContext()
		if sc.IsValid() {
			return sc.TraceID().String()
		}
	}
	if v, ok := ctx.Value(contextKey("otel.trace_id")).(string); ok {
		return v
	}
	return ""
}
```

### Private one-hop bodies

#### classifyHTTPError (func)

```go
func classifyHTTPError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if isTimeout(err) {
		return "timeout"
	}
	return "http_error"
}
```

#### contextKey (type)

```go
type contextKey string
```

#### dumpedTrace (type)

```go
type dumpedTrace struct {
	TraceID           string       `json:"trace_id"`
	DumpedAt          time.Time    `json:"dumped_at"`
	Reason            string       `json:"reason"`
	RootName          string       `json:"root_name,omitempty"`
	RootStatusMessage string       `json:"root_status_message,omitempty"`
	SpanCount         int          `json:"span_count"`
	Spans             []dumpedSpan `json:"spans"`
}
```

#### errorDetailTransport (type)

```go
type errorDetailTransport struct {
	base http.RoundTripper
}
```

#### failureDumpProcessor (type)

```go
type failureDumpProcessor struct {
	dir         string
	maxAgeHours int
	maxFiles    int
	mu          sync.Mutex
	traces      map[string]*traceDumpBuffer
}
```

#### failureDumpProcessor.writeLocked (method)

```go
func (p *failureDumpProcessor) writeLocked(doc dumpedTrace) {
	sort.Slice(doc.Spans, func(i, j int) bool {
		return doc.Spans[i].StartTime.Before(doc.Spans[j].StartTime)
	})
	doc.SpanCount = len(doc.Spans)
	if err := os.MkdirAll(p.dir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "Inference failure dump: mkdir %s: %v\n", p.dir, err)
		return
	}
	path := filepath.Join(p.dir, doc.TraceID+".json")
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Inference failure dump: encode %s: %v\n", doc.TraceID, err)
		return
	}
	if err := os.WriteFile(path, body, 0o640); err != nil {
		fmt.Fprintf(os.Stderr, "Inference failure dump: write %s: %v\n", path, err)
		return
	}
	fmt.Fprintf(os.Stderr, "Inference failure dump written trace_id=%s path=%s span_count=%d root_name=%s\n",
		doc.TraceID, path, doc.SpanCount, doc.RootName)
	if err := pruneFailureDumps(p.dir, p.maxAgeHours, p.maxFiles); err != nil {
		fmt.Fprintf(os.Stderr, "Inference failure dump: prune: %v\n", err)
	}
}
```

#### httpClientSpanName (func)

```go
func httpClientSpanName(r *http.Request) string {
	if r == nil || r.URL == nil {
		return httpServiceLabel + " CLIENT HTTP"
	}
	return httpServiceLabel + " CLIENT " + r.Method + " " + r.URL.Host + r.URL.Path
}
```

#### isTimeout (func)

```go
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isTimeout(urlErr.Err)
	}
	return false
}
```

#### newFailureDumpProcessor (func)

```go
func newFailureDumpProcessor(dir string, maxAgeHours, maxFiles int) *failureDumpProcessor {
	return &failureDumpProcessor{
		dir:         dir,
		maxAgeHours: maxAgeHours,
		maxFiles:    maxFiles,
		traces:      make(map[string]*traceDumpBuffer),
	}
}
```

#### otlpInsecureDefault (func)

```go
func otlpInsecureDefault(endpoint string) bool {
	host := strings.ToLower(strings.TrimSpace(endpoint))
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1"
}
```

#### redactHTTPError (func)

```go
func redactHTTPError(err error) error {
	if err == nil {
		return nil
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		out := *urlErr
		out.URL = redactURLString(urlErr.URL)
		return &out
	}
	msg := err.Error()
	redacted := redactURLsInText(msg)
	if redacted == msg {
		return err
	}
	return &redactedError{orig: err, msg: redacted}
}
```

#### redactSpanRequestURL (func)

```go
func redactSpanRequestURL(span trace.Span, req *http.Request) {
	if span == nil || !span.IsRecording() || req == nil || req.URL == nil {
		return
	}
	span.SetAttributes(attribute.String("url.full", redactURLString(req.URL.String())))
	if req.URL.RawQuery != "" {
		span.SetAttributes(attribute.String("url.query", redactQueryString(req.URL.RawQuery)))
	}
}
```

#### redactURLsInText (func)

```go
func redactURLsInText(s string) string {
	if s == "" || !strings.Contains(s, "://") {
		return s
	}
	return embeddedHTTPURL.ReplaceAllStringFunc(s, redactURLString)
}
```

#### redactedError (error)

```go
type redactedError struct {
	orig error
	msg  string
}
```

#### snapshotSpan (func)

```go
func snapshotSpan(s sdktrace.ReadOnlySpan) dumpedSpan {
	sc := s.SpanContext()
	parentID := ""
	if s.Parent().IsValid() {
		parentID = s.Parent().SpanID().String()
	}
	status := s.Status()
	return dumpedSpan{
		TraceID:       sc.TraceID().String(),
		SpanID:        sc.SpanID().String(),
		ParentSpanID:  parentID,
		Name:          s.Name(),
		Kind:          s.SpanKind().String(),
		StatusCode:    statusCodeString(status.Code),
		StatusMessage: redactURLsInText(status.Description),
		StartTime:     s.StartTime().UTC(),
		EndTime:       s.EndTime().UTC(),
		Attributes:    attributesToMap(s.Attributes()),
		Events:        snapshotEvents(s.Events()),
	}
}
```

#### traceDumpBuffer (type)

```go
type traceDumpBuffer struct {
	spans    []dumpedSpan
	hasError bool
}
```

#### withErrorDetail (func)

```go
func withErrorDetail(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if _, ok := base.(*errorDetailTransport); ok {
		return base
	}
	return &errorDetailTransport{base: base}
}
```


## ./internal/orchestrate
- package: `orchestrate`
- packageDoc: Package orchestrate runs review waves, checkpoints, finalize, and synthesis loops.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: BatchEntry, BatchPlan, Options, StageFinalize, StagePrep, StageProse, StageReport, StageSynth, StageWaves
- exportedFuncs: CheckpointPath, ChunkBatches, FileExists, IsSynthesisSkill, LoadBatchPlan, NormalizeUntil, Run, SplitBatches, TouchCheckpoint
- exportedMethods: (none)
- unexportedDecls: orchestrateRank, synthesisSkills
- unexportedFuncs: copyFile, envInt, envOr, isTimeout, runFileProse, runFileWaves, runFinalize, runOneFileBatch, runSynthesis, runSynthesisProse, runTechDeep, shouldRun
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/orchestrate/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/orchestrate/plan.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/orchestrate/run.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/orchestrate/until.go

### Exported bodies

#### BatchEntry (type)

```go
type BatchEntry struct {
	Skill      string `json:"skill"`
	BatchNum   string `json:"batch_num"`
	TaskCount  int    `json:"task_count"`
	StagingDir string `json:"staging_dir"`
}
```

#### BatchPlan (type)

```go
type BatchPlan struct {
	Batches      []BatchEntry `json:"batches"`
	Skills       []string     `json:"skills"`
	TotalBatches int          `json:"total_batches"`
}
```

#### IsSynthesisSkill (func)

```go
func IsSynthesisSkill(skill string) bool {
	_, ok := synthesisSkills[skill]
	return ok
}
```

#### LoadBatchPlan (func)

```go
func LoadBatchPlan(path string) (*BatchPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var plan BatchPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("invalid batch-plan.json: %w", err)
	}
	return &plan, nil
}
```

#### SplitBatches (func)

```go
func SplitBatches(batches []BatchEntry) (fileBatches, synthesisBatches []BatchEntry) {
	for _, b := range batches {
		if IsSynthesisSkill(b.Skill) {
			synthesisBatches = append(synthesisBatches, b)
		} else {
			fileBatches = append(fileBatches, b)
		}
	}
	return fileBatches, synthesisBatches
}
```

#### ChunkBatches (func)

```go
func ChunkBatches(batches []BatchEntry, concurrency int) [][]BatchEntry {
	if concurrency < 1 {
		concurrency = 1
	}
	var waves [][]BatchEntry
	for i := 0; i < len(batches); i += concurrency {
		end := i + concurrency
		if end > len(batches) {
			end = len(batches)
		}
		waves = append(waves, batches[i:end])
	}
	return waves
}
```

#### CheckpointPath (func)

```go
func CheckpointPath(skillOutputDir, batchNum string) string {
	return filepath.Join(skillOutputDir, "logs", "batch_"+batchNum+".done.txt")
}
```

#### FileExists (func)

```go
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

#### TouchCheckpoint (func)

```go
func TouchCheckpoint(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
```

#### Options (type)

```go
type Options struct {
	Context     context.Context
	PRNumber    string
	BaseBranch  string
	StagingDir  string
	OutputDir   string // pipeline output root, e.g. copilot-review-pr-42/pr-review
	Pipeline    string // COPILOT_PIPELINE / label (default pr-review)
	Concurrency int
	ScriptsDir  string
	SkipPrep    bool
	SkipDeep    bool
	SkipReport  bool
	Until       string // prep|waves|finalize|prose|synth|report; empty = full
	RepoRoot    string // for prep / deep; empty = cwd

	RoutingPath       string
	AgentContextPath  string
	SummaryConfigPath string
	ContextDir        string // merged context-branch checkout (agenting); optional

	// ConfigDir + RepoID load majordomo-central-config and materialize routing/agentContext
	// when RoutingPath / AgentContextPath are empty.
	ConfigDir string
	RepoID    string

	// BatchTimeout for a single dispatch; 0 → COPILOT_BATCH_TIMEOUT_MINUTES or 8m.
	BatchTimeout time.Duration

	// Injectables for tests
	Dispatch   func(agent.DispatchOptions) error
	RunSummary func(agent.SummaryLoopOptions) error
	RunTech    func(agent.TechLoopOptions) error
}
```

#### Run (func)

```go
func Run(opts Options) error {
	var owned *llmusage.Collector
	if llmusage.Active() == nil {
		owned = llmusage.New()
		llmusage.Push(owned)
		defer func() {
			snap := owned.Snapshot()
			agent.Logf("INFO", "pr=%s LLM usage summary", opts.PRNumber)
			for _, line := range strings.Split(llmusage.Format(snap), "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				agent.Logf("INFO", "%s", line)
			}
			llmusage.Pop()
		}()
	}

	if opts.PRNumber == "" || opts.StagingDir == "" || opts.OutputDir == "" {
		return fmt.Errorf("orchestrate requires --pr, --staging-dir, and --output-dir")
	}
	if opts.Pipeline == "" {
		opts.Pipeline = envOr("COPILOT_PIPELINE", "pr-review")
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = envInt("COPILOT_CONCURRENCY", 6)
	}
	if opts.BatchTimeout <= 0 {
		mins := envInt("COPILOT_BATCH_TIMEOUT_MINUTES", 8)
		opts.BatchTimeout = time.Duration(mins) * time.Minute
	}
	until, err := NormalizeUntil(opts.Until)
	if err != nil {
		return err
	}
	opts.Until = until

	useDefaultDispatch := opts.Dispatch == nil
	if useDefaultDispatch {
		opts.Dispatch = agent.Dispatch
	}
	if opts.RunSummary == nil {
		opts.RunSummary = agent.RunSummaryLoop
	}
	if opts.RunTech == nil {
		opts.RunTech = agent.RunTechLoop
	}

	logf := agent.Logf
	logf("INFO", "========== majordomo orchestrate ==========")
	if opts.Until != "" {
		logf("INFO", "until: %s", opts.Until)
	}
	logf("INFO", "PR: %s  pipeline: %s  concurrency: %d  judge: strop", opts.PRNumber, opts.Pipeline, opts.Concurrency)

	if !opts.SkipPrep {
		if opts.BaseBranch == "" {
			return fmt.Errorf("--base-branch required unless --skip-prep")
		}
		routingPath, agentContextPath, cfg, err := config.ResolvePrepPaths(
			opts.ConfigDir, opts.RepoID, opts.Pipeline,
			config.MaterializeDirForStaging(opts.StagingDir),
			opts.RoutingPath, opts.AgentContextPath,
		)
		if err != nil {
			return fmt.Errorf("central config: %w", err)
		}
		if opts.ConfigDir != "" && opts.RepoID != "" {
			if err := config.ApplyPipelineModelEnv(cfg, opts.Pipeline); err != nil {
				return fmt.Errorf("apply pipeline model environment: %w", err)
			}
		}
		logf("INFO", "Running prep against %s → %s", opts.BaseBranch, opts.StagingDir)
		err = staging.Run(staging.Options{
			BaseBranch:        opts.BaseBranch,
			StagingDir:        opts.StagingDir,
			RoutingPath:       routingPath,
			AgentContextPath:  agentContextPath,
			SummaryConfigPath: opts.SummaryConfigPath,
			RepoRoot:          opts.RepoRoot,
			ContextDir:        staging.ResolveContextDir(opts.ContextDir),
		})
		if err != nil {
			if errors.Is(err, staging.ErrNothingToReview) {
				logf("INFO", "prep: nothing to review — skipping")
				return nil
			}
			return fmt.Errorf("prep: %w", err)
		}
	} else if opts.ConfigDir != "" && opts.RepoID != "" {
		cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
		if err != nil {
			return fmt.Errorf("central config: %w", err)
		}
		if err := config.ApplyPipelineModelEnv(cfg, opts.Pipeline); err != nil {
			return fmt.Errorf("apply pipeline model environment: %w", err)
		}
	}

	if !shouldRun(opts.Until, StageWaves) {
		logf("INFO", "until=%s: stopping after prep", opts.Until)
		return nil
	}

	if useDefaultDispatch {
		if opts.ConfigDir != "" && opts.RepoID != "" {
			cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
			if err != nil {
				return err
			}
			fallback := ""
			if pipe, ok := cfg.PipelineNamed(opts.Pipeline); ok {
				fallback = pipe.Model
			}
			if _, err := judge.EnsureRuntimeFromConfig(cfg, judge.RuntimeOptions{
				Tasks:         judge.ReviewTasks(),
				FallbackModel: fallback,
			}); err != nil {
				return err
			}
		} else if err := judge.EnsureStropReady(); err != nil {
			return err
		}
	}

	planPath := filepath.Join(opts.StagingDir, "batch-plan.json")
	plan, err := LoadBatchPlan(planPath)
	if err != nil {
		return err
	}
	fileBatches, synthBatches := SplitBatches(plan.Batches)
	logf("INFO", "%d batch(es) across skill(s): %v", len(plan.Batches)
// ... truncated
```

#### NormalizeUntil (func)

```go
func NormalizeUntil(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "publish":
		return "", nil
	case "clone", "sa":
		return "", fmt.Errorf("until %q is not an orchestrate stage (use majordomo run review)", s)
	case StagePrep, StageWaves, StageFinalize, StageProse, StageSynth, StageReport:
		return s, nil
	default:
		return "", fmt.Errorf("unknown until stage %q (prep|waves|finalize|prose|synth|report)", s)
	}
}
```

### Private one-hop bodies

#### envInt (func)

```go
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
```

#### envOr (func)

```go
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

#### shouldRun (func)

```go
func shouldRun(until, stage string) bool {
	if until == "" {
		return true
	}
	ur, okU := orchestrateRank[until]
	sr, okS := orchestrateRank[stage]
	if !okU || !okS {
		return true
	}
	return sr <= ur
}
```


## ./internal/outbound
- package: `outbound`
- packageDoc: Package outbound provides a shared retrying HTTP client for forge/SCM APIs.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: true
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: DefaultTimeout
- exportedFuncs: Client, DoWithRetry
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: (none)
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/outbound/client.go

### Exported bodies

#### Client (func)

```go
func Client(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	c := &http.Client{Timeout: timeout}
	observability.InstrumentHTTPClient(c)
	return c
}
```

#### DoWithRetry (func)

```go
func DoWithRetry(client *http.Client, req *http.Request, maxAttempts int) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("outbound: request is required")
	}
	if client == nil {
		client = Client(DefaultTimeout)
	}
	if maxAttempts < 1 {
		maxAttempts = 3
	}
	var lastErr error
	backoff := 200 * time.Millisecond
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		r := req.Clone(req.Context())
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r.Body = body
		}
		resp, err := client.Do(r)
		if err != nil {
			lastErr = err
			if attempt == maxAttempts {
				return nil, err
			}
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				if closeErr := resp.Body.Close(); closeErr != nil {
					return nil, fmt.Errorf("discard retry response body: %w; close body: %v", err, closeErr)
				}
				return nil, fmt.Errorf("discard retry response body: %w", err)
			}
			if err := resp.Body.Close(); err != nil {
				return nil, fmt.Errorf("close retry response body: %w", err)
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if attempt == maxAttempts {
				return nil, lastErr
			}
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return resp, nil
	}
	return nil, lastErr
}
```


## ./internal/poll
- package: `poll`
- packageDoc: Package poll discovers open PRs/MRs via SCM APIs and compares head_sha against a local poll cursor (.poll-cache; Actions cache in the tower).
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: true
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: Options, PendingReview, Result
- exportedFuncs: Run
- exportedMethods: (none)
- unexportedDecls: majordomoInternalBranchPrefixes, openPR, pollSummaryData, pollSummaryTemplate, pollSummaryTmpl, repoOutcome, repoRow
- unexportedFuncs: defaultCloneURL, encodeGitLabProject, formatASCIISummary, gitlabAPIBase, gitlabProjectRef, isMajordomoInternalBranch, linkRelNext, listGitHubPRs, listGitLabMRs, listOpenPRs, logf, newGitLabMRListRequest, parseClonePath, readResponseBody, short, splitOwnerName, writePollSummary
- unexportedMethods: repoOutcome.continuousLabel, repoOutcome.path
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/poll/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/poll/poll.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/poll/scm.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/poll/summary.go

### Exported bodies

#### PendingReview (type)

```go
type PendingReview struct {
	RepoID      string `json:"repo_id"`
	SCM         string `json:"scm"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	PRNumber    string `json:"pr"`
	HeadSHA     string `json:"head_sha"`
	BaseBranch  string `json:"base_branch"`
	CloneURL    string `json:"clone_url"`
	ReviewID    string `json:"review_id"`
	PublishMode string `json:"publish_mode,omitempty"`
}
```

#### Result (type)

```go
type Result struct {
	GeneratedAt string          `json:"generated_at"`
	Reviews     []PendingReview `json:"reviews"`
}
```

#### Options (type)

```go
type Options struct {
	ConfigDir string
	CursorDir string // local cursor store (Actions cache); default .poll-cache
	OutPath   string // write JSON here; empty → stdout
	// ListPRs injectable for tests
	ListPRs func(cfg config.RepoConfig, token string) ([]openPR, error)
}
```

#### Run (func)

```go
func Run(opts Options) error {
	if opts.ConfigDir == "" {
		return fmt.Errorf("--config-dir required")
	}
	if opts.CursorDir == "" {
		opts.CursorDir = ".poll-cache"
	}
	// Capture before defaulting so production empty-token skips still work.
	listInjected := opts.ListPRs != nil
	if opts.ListPRs == nil {
		opts.ListPRs = listOpenPRs
	}

	cfgs, err := config.LoadAll(opts.ConfigDir)
	if err != nil {
		return err
	}
	logf("INFO", "========== majordomo poll ==========")
	logf("INFO", "config dir: %s (%d repo(s))", opts.ConfigDir, len(cfgs))

	result := Result{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Reviews:     []PendingReview{},
	}

	outcomes := make([]repoOutcome, 0, len(cfgs))
	var listErrs []string
	for _, cfg := range cfgs {
		scm := strings.ToLower(cfg.SCM)
		if scm == "" {
			scm = "github"
		}

		owner, name := cfg.Repository.Owner, cfg.Repository.Name
		if owner == "" || name == "" {
			o2, n2 := splitOwnerName(parseClonePath(cfg.Repository.CloneURL))
			if owner == "" {
				owner = o2
			}
			if name == "" {
				name = n2
			}
		}

		out := repoOutcome{
			RepoID:     cfg.Repository.ID,
			SCM:        scm,
			Owner:      owner,
			Name:       name,
			Continuous: cfg.Review.ContinuousRunsEnabled(),
		}

		if !cfg.Trigger.PollEnabled() {
			out.Status = "poll_disabled"
			logf("INFO", "%s: skip (poll disabled)", cfg.Repository.ID)
			outcomes = append(outcomes, out)
			continue
		}

		token := config.ResolveCredential(cfg.Repository.ID, scm, owner)
		// When ListPRs is injected (tests), allow empty token.
		if token == "" && !listInjected {
			hint := config.CredentialHint(cfg.Repository.ID, scm, owner)
			out.Status = "no_credential"
			out.Detail = hint
			logf("WARN", "%s: no credential (set %s) - skipping", cfg.Repository.ID, hint)
			outcomes = append(outcomes, out)
			continue
		}

		prs, err := opts.ListPRs(cfg, token)
		if err != nil {
			msg := fmt.Sprintf("%s: list PRs failed: %v", cfg.Repository.ID, err)
			logf("WARN", "%s", msg)
			listErrs = append(listErrs, msg)
			out.Status = "list_error"
			out.Detail = err.Error()
			outcomes = append(outcomes, out)
			continue
		}

		cursorPath := filepath.Join(opts.CursorDir, cfg.Repository.ID, "poll-cursor.json")
		cursor, err := cache.ReadPollCursor(cursorPath)
		if err != nil {
			return err
		}
		cursor.RepoID = cfg.Repository.ID

		pendingHere := 0
		skippedHere := 0
		for _, pr := range prs {
			prNum := fmt.Sprintf("%d", pr.Number)
			if isMajordomoInternalBranch(pr.BaseBranch, pr.HeadBranch) {
				skippedHere++
				logf("INFO", "%s#%s: skip majordomo-internal branch (base=%s head=%s)",
					cfg.Repository.ID, prNum, pr.BaseBranch, pr.HeadBranch)
				continue
			}
			continuous := out.Continuous
			if !cache.ShouldReview(cursor, prNum, pr.HeadSHA, continuous) {
				skippedHere++
				if continuous {
					logf("INFO", "%s#%s: head unchanged (%s)", cfg.Repository.ID, prNum, short(pr.HeadSHA))
				} else {
					logf("INFO", "%s#%s: already reviewed (enableContinuousRuns=false)", cfg.Repository.ID, prNum)
				}
				continue
			}
			clone := cfg.Repository.CloneURL
			if clone == "" {
				clone = defaultCloneURL(scm, cfg.SCMAPI.BaseURL, owner, name)
			}
			reviewID := fmt.Sprintf("%s:%s/%s:%s:%s", scm, owner, name, prNum, pr.HeadSHA)
			result.Reviews = append(result.Reviews, PendingReview{
				RepoID:      cfg.Repository.ID,
				SCM:         scm,
				Owner:       owner,
				Name:        name,
				PRNumber:    prNum,
				HeadSHA:     pr.HeadSHA,
				BaseBranch:  pr.BaseBranch,
				CloneURL:    clone,
				ReviewID:    reviewID,
				PublishMode: cfg.EffectivePublishMode(),
			})
			pendingHere++
			logf("INFO", "%s#%s: needs review @ %s", cfg.Repository.ID, prNum, short(pr.HeadSHA))
		}

		out.Status = "polled"
		out.Open = len(prs)
		out.Pending = pendingHere
		out.Skipped = skippedHere
		logf("INFO", "%s: %s %s/%s open=%d pending=%d skip=%d",
			cfg.Repository.ID, scm, owner, name, out.Open, out.Pending, out.Skipped)
		outcomes = append(outcomes, ou
// ... truncated
```

### Private one-hop bodies

#### defaultCloneURL (func)

```go
func defaultCloneURL(scm, apiBase, owner, name string) string {
	if owner == "" || name == "" {
		return ""
	}
	host := "github.com"
	if scm == "gitlab" {
		host = "gitlab.com"
	}
	if apiBase != "" {
		b := strings.TrimRight(apiBase, "/")
		b = strings.TrimPrefix(b, "https://")
		b = strings.TrimPrefix(b, "http://")
		if i := strings.Index(b, "/"); i >= 0 {
			b = b[:i]
		}
		if b != "" {
			host = b
		}
	}
	return fmt.Sprintf("https://%s/%s/%s.git", host, owner, name)
}
```

#### isMajordomoInternalBranch (func)

```go
func isMajordomoInternalBranch(base, head string) bool {
	for _, p := range majordomoInternalBranchPrefixes {
		if strings.HasPrefix(base, p) || strings.HasPrefix(head, p) {
			return true
		}
	}
	return false
}
```

#### listOpenPRs (func)

```go
func listOpenPRs(cfg config.RepoConfig, token string) ([]openPR, error) {
	scm := strings.ToLower(cfg.SCM)
	if scm == "" {
		scm = "github"
	}
	switch scm {
	case "github":
		return listGitHubPRs(cfg, token)
	case "gitlab":
		return listGitLabMRs(cfg, token)
	default:
		return nil, fmt.Errorf("scm %q not supported in poll (github|gitlab)", scm)
	}
}
```

#### logf (func)

```go
func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

#### openPR (type)

```go
type openPR struct {
	Number     int
	HeadSHA    string
	BaseBranch string
	HeadBranch string
}
```

#### parseClonePath (func)

```go
func parseClonePath(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "ssh://")
	u = strings.TrimPrefix(u, "git@")
	u = strings.Replace(u, ":", "/", 1)
	if i := strings.Index(u, "/"); i >= 0 {
		return strings.Trim(u[i+1:], "/")
	}
	return ""
}
```

#### repoOutcome (type)

```go
type repoOutcome struct {
	RepoID     string
	SCM        string
	Owner      string
	Name       string
	Status     string // polled | no_credential | poll_disabled | list_error
	Open       int
	Pending    int
	Skipped    int // open but not queued (cursor)
	Detail     string
	Continuous bool
}
```

#### repoOutcome.path (method)

```go
func (o repoOutcome) path() string {
	if o.Owner == "" && o.Name == "" {
		return "-"
	}
	if o.Name == "" {
		return o.Owner
	}
	if o.Owner == "" {
		return o.Name
	}
	return o.Owner + "/" + o.Name
}
```

#### short (func)

```go
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
```

#### splitOwnerName (func)

```go
func splitOwnerName(path string) (owner, name string) {
	path = strings.Trim(path, "/")
	if path == "" {
		return "", ""
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		return "", parts[0]
	}
	return strings.Join(parts[:len(parts)-1], "/"), parts[len(parts)-1]
}
```


## ./internal/publish
- package: `publish`
- packageDoc: Package publish posts PR/MR summaries via forge CLIs (gh, glab) or Bitbucket HTTP.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: true
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: exec_runner
- mechanicalConfidence: 0.80
- mechanicalEvidence: imports_os_exec, exports_run_surface
- exportedDecls: CLIRunner, Marker, Mode, ModeAuto, ModeComment, ModeDescription, Options
- exportedFuncs: HasBodyContent, Run
- exportedMethods: (none)
- unexportedDecls: legacyMarker
- unexportedFuncs: defaultCLIRunner, firstNonEmpty, ghComment, ghEditBody, ghRepoArgs, ghViewBody, gitlabRepoSpec, glabNote, glabRepoArgs, glabUpdateDesc, glabViewDesc, httpJSON, logf, ownedByMajordomo, publishBitbucket, publishGitHub, publishGitLab, writeTempBody
- unexportedMethods: Options.client, Options.fillFromEnv, Options.reviewLinks, Options.runCLI, Options.withLinks
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/publish/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/publish/publish.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/publish/scm.go

### Exported bodies

#### Mode (type)

```go
type Mode string
```

#### CLIRunner (type)

```go
type CLIRunner func(name string, args []string, env []string) (stdout string, err error)
```

#### Options (type)

```go
type Options struct {
	SCM         string // github | gitlab | bitbucket
	PRNumber    string
	SummaryFile string
	Mode        Mode
	// Bitbucket Server (HTTP)
	BitbucketURL   string
	BitbucketToken string
	BBProject      string
	BBRepo         string
	// RepoID is the majordomo-central-config id (optional; enables MAJORDOMO_CREDENTIAL_ override).
	RepoID string
	// GitHub (gh CLI) — owner/repo also used for GitLab path projects
	GitHubToken string
	GitHubOwner string
	GitHubRepo  string
	// GitLab (glab CLI)
	GitLabToken     string
	GitLabHost      string // e.g. gitlab.com or gitlab.example.com
	GitLabProjectID string // optional numeric id; else owner/name
	// Artifact URLs (optional links footer)
	SummaryArtifactURL     string
	SummaryHTMLArtifactURL string
	TechReviewArtifactURL  string
	TechDeepArtifactURL    string
	SAArtifactURLsJSON     string
	HTTPClient             *http.Client
	// Runner overrides CLI execution (tests). Empty → exec on PATH.
	Runner CLIRunner
}
```

#### HasBodyContent (func)

```go
func HasBodyContent(summary string) bool {
	commentLine := regexp.MustCompile(`^<!--.*-->$`)
	for _, line := range strings.Split(summary, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") || commentLine.MatchString(s) {
			continue
		}
		return true
	}
	return false
}
```

#### Run (func)

```go
func Run(opts Options) error {
	opts.Mode = Mode(strings.ToLower(string(opts.Mode)))
	switch opts.Mode {
	case ModeAuto, ModeComment, ModeDescription:
	default:
		return fmt.Errorf("mode must be auto|comment|description, got %q", opts.Mode)
	}
	data, err := os.ReadFile(opts.SummaryFile)
	if err != nil {
		return fmt.Errorf("summary file not found: %w", err)
	}
	summary := string(data)
	if !HasBodyContent(summary) {
		logf("INFO", "summary.md contains no body content — skipping publish")
		return nil
	}
	opts.fillFromEnv()
	switch strings.ToLower(opts.SCM) {
	case "github":
		return publishGitHub(opts, summary)
	case "gitlab":
		return publishGitLab(opts, summary)
	case "bitbucket":
		return publishBitbucket(opts, summary)
	default:
		return fmt.Errorf("unsupported scm %q (github|gitlab|bitbucket)", opts.SCM)
	}
}
```

### Private one-hop bodies

#### Options.fillFromEnv (method)

```go
func (o *Options) fillFromEnv() {
	if o.BitbucketURL == "" {
		o.BitbucketURL = os.Getenv("BITBUCKET_URL")
	}
	if o.BitbucketToken == "" {
		o.BitbucketToken = os.Getenv("BITBUCKET_TOKEN")
	}
	if o.BBProject == "" {
		o.BBProject = os.Getenv("BB_PROJECT")
	}
	if o.BBRepo == "" {
		o.BBRepo = os.Getenv("BB_REPO")
	}
	if o.RepoID == "" {
		o.RepoID = firstNonEmpty(os.Getenv("MAJORDOMO_REPO_ID"), os.Getenv("REPO_ID"))
	}
	if o.GitHubOwner == "" {
		o.GitHubOwner = os.Getenv("GITHUB_REPOSITORY_OWNER")
		if o.GitHubOwner == "" {
			if repo := os.Getenv("GITHUB_REPOSITORY"); strings.Contains(repo, "/") {
				parts := strings.SplitN(repo, "/", 2)
				o.GitHubOwner, o.GitHubRepo = parts[0], parts[1]
			}
		}
	}
	if o.GitHubRepo == "" {
		if repo := os.Getenv("GITHUB_REPOSITORY"); strings.Contains(repo, "/") {
			o.GitHubRepo = strings.SplitN(repo, "/", 2)[1]
		}
	}
	if o.GitLabHost == "" {
		o.GitLabHost = firstNonEmpty(os.Getenv("GITLAB_HOST"), os.Getenv("GLAB_HOST"))
	}
	if o.GitLabProjectID == "" {
		o.GitLabProjectID = firstNonEmpty(os.Getenv("GITLAB_PROJECT_ID"), os.Getenv("CI_PROJECT_ID"))
	}
	if o.GitHubOwner == "" || o.GitHubRepo == "" {
		if path := firstNonEmpty(os.Getenv("GITLAB_REPO"), os.Getenv("CI_PROJECT_PATH")); strings.Contains(path, "/") {
			parts := strings.Split(path, "/")
			if o.GitHubOwner == "" {
				o.GitHubOwner = strings.Join(parts[:len(parts)-1], "/")
			}
			if o.GitHubRepo == "" {
				o.GitHubRepo = parts[len(parts)-1]
			}
		}
	}
	// Served-repo tokens: per-repo override, then per-org (no unqualified GH_TOKEN / GITLAB_TOKEN).
	if o.GitHubToken == "" {
		o.GitHubToken = config.ResolveCredential(o.RepoID, "github", o.GitHubOwner)
	}
	if o.GitLabToken == "" {
		o.GitLabToken = config.ResolveCredential(o.RepoID, "gitlab", o.GitHubOwner)
	}
	if o.SummaryArtifactURL == "" {
		o.SummaryArtifactURL = os.Getenv("SUMMARY_ARTIFACT_URL")
	}
	if o.SummaryHTMLArtifactURL == "" {
		o.SummaryHTMLArtifactURL = os.Getenv("SUMMARY_HTML_ARTIFACT_URL")
	}
	if o.TechReviewArtifactURL == "" {
		o.TechReviewArtifactURL = os.Getenv("TECH_REVIEW_ARTIFACT_URL")
	}
	if o.TechDeepArtifactURL == "" {
		o.TechDeepArtifactURL = os.Getenv("TECH_DEEP_ARTIFACT_URL")
	}
	if o.SAArtifactURLsJSON == "" {
		o.SAArtifactURLsJSON = os.Getenv("SA_ARTIFACT_URLS")
	}
}
```

#### logf (func)

```go
func logf(level, format string, args ...any) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

#### publishBitbucket (func)

```go
func publishBitbucket(opts Options, summary string) error {
	if opts.BitbucketURL == "" || opts.BitbucketToken == "" || opts.BBProject == "" || opts.BBRepo == "" {
		return fmt.Errorf("bitbucket publish requires BITBUCKET_URL, BITBUCKET_TOKEN, BB_PROJECT, BB_REPO")
	}
	prURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%s",
		strings.TrimRight(opts.BitbucketURL, "/"), opts.BBProject, opts.BBRepo, opts.PRNumber)
	c := opts.client()
	logf("INFO", "========== Publishing PR summary to Bitbucket (mode: %s) ==========", opts.Mode)

	body := Marker + "\n" + opts.withLinks(summary)
	legacyBody := legacyMarker + "\n" + opts.withLinks(summary)

	putDesc := func(prMeta map[string]any, description string) error {
		payload := map[string]any{
			"version":     prMeta["version"],
			"title":       prMeta["title"],
			"description": description,
			"toRef":       prMeta["toRef"],
		}
		reviewers := []map[string]any{}
		if raw, ok := prMeta["reviewers"].([]any); ok {
			for _, r := range raw {
				if m, ok := r.(map[string]any); ok {
					if u, ok := m["user"]; ok {
						reviewers = append(reviewers, map[string]any{"user": u})
					}
				}
			}
		}
		payload["reviewers"] = reviewers
		_, _, err := httpJSON(c, "PUT", prURL, opts.BitbucketToken, payload)
		return err
	}

	postComment := func(text string) error {
		_, _, err := httpJSON(c, "POST", prURL+"/comments", opts.BitbucketToken, map[string]any{"text": text})
		return err
	}

	switch opts.Mode {
	case ModeComment:
		return postComment(opts.withLinks(summary))
	case ModeDescription:
		prMeta, _, err := httpJSON(c, "GET", prURL, opts.BitbucketToken, nil)
		if err != nil {
			return err
		}
		return putDesc(prMeta, body)
	case ModeAuto:
		prMeta, _, err := httpJSON(c, "GET", prURL, opts.BitbucketToken, nil)
		if err != nil {
			return err
		}
		current, ok := prMeta["description"].(string)
		if !ok {
			current = ""
		}
		if ownedByMajordomo(current) {
			logf("INFO", "claiming/updating PR description")
			if strings.Contains(current, legacyMarker) && !strings.Contains(current, Marker) {
				return putDesc(prMeta, legacyBody)
			}
			return putDesc(prMeta, body)
		}
		logf("INFO", "PR description has user content — posting link comment")
		if opts.SummaryArtifactURL == "" {
			return fmt.Errorf("SUMMARY_ARTIFACT_URL required when PR description is owned by someone else")
		}
		links := opts.reviewLinks(opts.SummaryArtifactURL)
		return postComment("🤖 **Majordomo PR Review complete** — " + strings.Join(links, " · "))
	}
	return nil
}
```

#### publishGitHub (func)

```go
func publishGitHub(opts Options, summary string) error {
	if opts.GitHubToken == "" || opts.GitHubOwner == "" || opts.GitHubRepo == "" {
		return fmt.Errorf("github publish requires token and owner/repo (set %s)",
			config.CredentialHint(opts.RepoID, "github", opts.GitHubOwner))
	}
	repo := opts.GitHubOwner + "/" + opts.GitHubRepo
	env := append([]string{}, os.Environ()...)
	env = append(env, "GH_TOKEN="+opts.GitHubToken, "GITHUB_TOKEN="+opts.GitHubToken)

	body := Marker + "\n" + opts.withLinks(summary)
	logf("INFO", "========== Publishing PR summary to GitHub via gh (mode: %s) ==========", opts.Mode)

	switch opts.Mode {
	case ModeComment:
		return ghComment(opts, env, repo, body)
	case ModeDescription:
		return ghEditBody(opts, env, repo, body)
	case ModeAuto:
		current, err := ghViewBody(opts, env, repo)
		if err != nil {
			return err
		}
		if ownedByMajordomo(current) {
			logf("INFO", "claiming/updating PR description")
			return ghEditBody(opts, env, repo, body)
		}
		logf("INFO", "PR description has user content — posting link comment")
		artifact := opts.SummaryArtifactURL
		if artifact == "" {
			return fmt.Errorf("SUMMARY_ARTIFACT_URL required when PR description is owned by someone else")
		}
		links := opts.reviewLinks(artifact)
		msg := "🤖 **Majordomo PR Review complete** — " + strings.Join(links, " · ")
		return ghComment(opts, env, repo, msg)
	}
	return nil
}
```

#### publishGitLab (func)

```go
func publishGitLab(opts Options, summary string) error {
	if opts.GitLabToken == "" {
		return fmt.Errorf("gitlab publish requires token (set %s)",
			config.CredentialHint(opts.RepoID, "gitlab", opts.GitHubOwner))
	}
	repo := gitlabRepoSpec(opts)
	if repo == "" {
		return fmt.Errorf("gitlab publish requires owner/repo or GITLAB_PROJECT_ID / GITLAB_REPO")
	}
	env := append([]string{}, os.Environ()...)
	env = append(env, "GITLAB_TOKEN="+opts.GitLabToken, "GLAB_TOKEN="+opts.GitLabToken)
	if opts.GitLabHost != "" {
		host := strings.TrimPrefix(strings.TrimPrefix(opts.GitLabHost, "https://"), "http://")
		host = strings.TrimRight(host, "/")
		env = append(env, "GITLAB_HOST="+host)
	}

	body := Marker + "\n" + opts.withLinks(summary)
	logf("INFO", "========== Publishing MR summary to GitLab via glab (mode: %s) ==========", opts.Mode)

	switch opts.Mode {
	case ModeComment:
		return glabNote(opts, env, repo, body)
	case ModeDescription:
		return glabUpdateDesc(opts, env, repo, body)
	case ModeAuto:
		current, err := glabViewDesc(opts, env, repo)
		if err != nil {
			return err
		}
		if ownedByMajordomo(current) {
			logf("INFO", "claiming/updating MR description")
			return glabUpdateDesc(opts, env, repo, body)
		}
		logf("INFO", "MR description has user content — posting link note")
		artifact := opts.SummaryArtifactURL
		if artifact == "" {
			return fmt.Errorf("SUMMARY_ARTIFACT_URL required when MR description is owned by someone else")
		}
		links := opts.reviewLinks(artifact)
		msg := "🤖 **Majordomo PR Review complete** — " + strings.Join(links, " · ")
		return glabNote(opts, env, repo, msg)
	}
	return nil
}
```


## ./internal/report
- package: `report`
- packageDoc: Package report converts review findings to JUnit XML and Markdown reports to HTML.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: (none)
- exportedFuncs: BuildTestsuite, ConvertMarkdownToHTML, ConvertMarkdownToHTMLCLI, ConvertToJUnit, DeriveTitle, Sanitize
- exportedMethods: (none)
- unexportedDecls: fileHeaderRe, finding, findingRe, h1Re, htmlTemplate, mdRenderer, metaRe, nameMax, saDuplicateRe, skipFilenames, tagStrip, xmlFailure, xmlIllegalRe, xmlSkipped, xmlSuite, xmlTestCase, xmlTestsuites, xmlText
- unexportedFuncs: parseReport, titleCase
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/report/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/report/html.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/report/junit.go

### Exported bodies

#### DeriveTitle (func)

```go
func DeriveTitle(mdPath, htmlBody string) string {
	if m := h1Re.FindStringSubmatch(htmlBody); m != nil {
		raw := tagStrip.ReplaceAllString(m[1], "")
		return strings.TrimSpace(raw)
	}
	stem := strings.TrimSuffix(filepath.Base(mdPath), filepath.Ext(mdPath))
	stem = strings.ReplaceAll(stem, "-", " ")
	stem = strings.ReplaceAll(stem, "_", " ")
	return titleCase(stem)
}
```

#### ConvertMarkdownToHTML (func)

```go
func ConvertMarkdownToHTML(mdPath, htmlPath string) error {
	source, err := os.ReadFile(mdPath)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := mdRenderer.Convert(source, &buf); err != nil {
		return err
	}
	body := buf.String()
	title := DeriveTitle(mdPath, body)
	page := strings.Replace(htmlTemplate, "__TITLE__", title, 1)
	page = strings.Replace(page, "__BODY__", body, 1)
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(htmlPath, []byte(page), 0o644)
}
```

#### ConvertMarkdownToHTMLCLI (func)

```go
func ConvertMarkdownToHTMLCLI(mdPath, htmlPath string) error {
	if _, err := os.Stat(mdPath); err != nil {
		return fmt.Errorf("ERROR: input file not found: %s", mdPath)
	}
	if err := ConvertMarkdownToHTML(mdPath, htmlPath); err != nil {
		return err
	}
	fmt.Printf("Converted: %s → %s\n", mdPath, htmlPath)
	return nil
}
```

#### Sanitize (func)

```go
func Sanitize(text string) string {
	// Strip illegal control chars; also drop unpaired surrogates by filtering runes.
	cleaned := xmlIllegalRe.ReplaceAllString(text, "")
	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
```

#### BuildTestsuite (func)

```go
func BuildTestsuite(skillDir, pipelineName, skillName string) (xmlSuite, error) {
	perFileDir := filepath.Join(skillDir, "per-file")
	var reportFiles []string
	if info, err := os.Stat(perFileDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(perFileDir)
		if err != nil {
			return xmlSuite{}, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			if _, skip := skipFilenames[e.Name()]; skip {
				continue
			}
			if strings.HasSuffix(e.Name(), "_session.md") {
				continue
			}
			reportFiles = append(reportFiles, filepath.Join(perFileDir, e.Name()))
		}
		sort.Strings(reportFiles)
	}

	suite := xmlSuite{
		Name:      "Copilot PR Review — " + pipelineName + "/" + skillName,
		Tests:     "0",
		Failures:  "0",
		Errors:    "0",
		Skipped:   "0",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
	total, failures, skipped := 0, 0, 0

	for _, reportPath := range reportFiles {
		reviewedFile, meta, findings, err := parseReport(reportPath)
		if err != nil {
			return xmlSuite{}, err
		}
		classname := "copilot_review." + pipelineName + "." + skillName + "." +
			strings.ReplaceAll(strings.ReplaceAll(reviewedFile, "\\", "/"), "/", ".")
		var metaLines []string
		for _, m := range meta {
			metaLines = append(metaLines, m[0]+": "+m[1])
		}
		metaBlock := strings.Join(metaLines, "\n")
		preamble := strings.TrimRight("File: "+reviewedFile+"\n"+metaBlock+"\n", "\n") + "\n"

		if len(findings) == 0 {
			suite.Cases = append(suite.Cases, xmlTestCase{
				Classname: classname,
				Name:      reviewedFile,
				Time:      "0",
				SystemOut: &xmlText{Text: Sanitize(preamble + "\nNo issues found.")},
			})
			total++
			continue
		}

		for _, f := range findings {
			if saDuplicateRe.MatchString(f.Text) {
				continue
			}
			name := f.Text
			if utf8.RuneCountInString(name) > nameMax {
				runes := []rune(name)
				name = string(runes[:nameMax-3]) + "..."
			}
			tc := xmlTestCase{
				Classname: classname,
				Name:      "[" + f.Level + "] " + name,
				Time:      "0",
				SystemOut: &xmlText{Text: Sanitize(preamble + "\n[" + f.Level + "] " + f.Text)},
			}
			body := Sanitize(preamble + "\n[" + f.Level + "] " + f.Text)
			if f.Level == "CRITICAL" {
				tc.Failure = &xmlFailure{
					Message: Sanitize(f.Text),
					Type:    f.Level,
					Text:    body,
				}
				failures++
			} else {
				tc.Skipped = &xmlSkipped{
					Message: Sanitize(f.Text),
					Text:    body,
				}
				skipped++
			}
			suite.Cases = append(suite.Cases, tc)
			total++
		}
	}

	suite.Tests = strconv.Itoa(total)
	suite.Failures = strconv.Itoa(failures)
	suite.Skipped = strconv.Itoa(skipped)
	return suite, nil
}
```

#### ConvertToJUnit (func)

```go
func ConvertToJUnit(reviewOutputDir, junitOutputDir string) error {
	info, err := os.Stat(reviewOutputDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	if err := os.MkdirAll(junitOutputDir, 0o755); err != nil {
		return err
	}

	pipelineEntries, err := os.ReadDir(reviewOutputDir)
	if err != nil {
		return err
	}
	sort.Slice(pipelineEntries, func(i, j int) bool {
		return pipelineEntries[i].Name() < pipelineEntries[j].Name()
	})

	for _, pe := range pipelineEntries {
		if !pe.IsDir() {
			continue
		}
		pipelineName := pe.Name()
		pipelineDir := filepath.Join(reviewOutputDir, pipelineName)
		skillEntries, err := os.ReadDir(pipelineDir)
		if err != nil {
			return err
		}
		sort.Slice(skillEntries, func(i, j int) bool {
			return skillEntries[i].Name() < skillEntries[j].Name()
		})
		for _, se := range skillEntries {
			if !se.IsDir() {
				continue
			}
			skillName := se.Name()
			skillDir := filepath.Join(pipelineDir, skillName)
			suite, err := BuildTestsuite(skillDir, pipelineName, skillName)
			if err != nil {
				return err
			}
			doc := xmlTestsuites{Suites: []xmlSuite{suite}}
			data, err := xml.MarshalIndent(doc, "", "  ")
			if err != nil {
				return err
			}
			out := append([]byte(xml.Header), data...)
			out = append(out, '\n')
			outPath := filepath.Join(junitOutputDir, "copilot-review-"+pipelineName+"-"+skillName+".xml")
			if err := os.WriteFile(outPath, out, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
```

### Private one-hop bodies

#### finding (type)

```go
type finding struct {
	Level string
	Text  string
}
```

#### parseReport (func)

```go
func parseReport(path string) (reviewedFile string, meta [][2]string, findings []finding, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, nil, err
	}
	reviewedFile = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, line := range strings.Split(string(data), "\n") {
		if m := fileHeaderRe.FindStringSubmatch(line); m != nil {
			reviewedFile = strings.TrimSpace(m[1])
			continue
		}
		if m := metaRe.FindStringSubmatch(line); m != nil {
			meta = append(meta, [2]string{strings.TrimSpace(m[1]), strings.TrimSpace(m[2])})
			continue
		}
		if m := findingRe.FindStringSubmatch(line); m != nil {
			findings = append(findings, finding{Level: m[1], Text: strings.TrimSpace(m[2])})
		}
	}
	return reviewedFile, meta, findings, nil
}
```

#### titleCase (func)

```go
func titleCase(s string) string {
	parts := strings.Fields(s)
	for i, p := range parts {
		if p == "" {
			continue
		}
		lower := strings.ToLower(p)
		parts[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}
```

#### xmlFailure (type)

```go
type xmlFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}
```

#### xmlSkipped (type)

```go
type xmlSkipped struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}
```

#### xmlSuite (type)

```go
type xmlSuite struct {
	Name      string        `xml:"name,attr"`
	Tests     string        `xml:"tests,attr"`
	Failures  string        `xml:"failures,attr"`
	Errors    string        `xml:"errors,attr"`
	Skipped   string        `xml:"skipped,attr"`
	Timestamp string        `xml:"timestamp,attr"`
	Cases     []xmlTestCase `xml:"testcase"`
}
```

#### xmlTestCase (type)

```go
type xmlTestCase struct {
	Classname string      `xml:"classname,attr"`
	Name      string      `xml:"name,attr"`
	Time      string      `xml:"time,attr"`
	SystemOut *xmlText    `xml:"system-out,omitempty"`
	Failure   *xmlFailure `xml:"failure,omitempty"`
	Skipped   *xmlSkipped `xml:"skipped,omitempty"`
}
```

#### xmlTestsuites (type)

```go
type xmlTestsuites struct {
	XMLName xml.Name   `xml:"testsuites"`
	Suites  []xmlSuite `xml:"testsuite"`
}
```

#### xmlText (type)

```go
type xmlText struct {
	Text string `xml:",chardata"`
}
```


## ./internal/reviewrun
- package: `reviewrun`
- packageDoc: Package reviewrun is the local/CI review job: clone, SA, orchestrate, optional publish.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: exec_runner
- mechanicalConfidence: 0.80
- mechanicalEvidence: imports_os_exec, exports_run_surface
- exportedDecls: Options, StageClone, StageFinalize, StagePrep, StageProse, StagePublish, StageReport, StageSA, StageSynth, StageWaves
- exportedFuncs: ParseUntil, Run
- exportedMethods: (none)
- unexportedDecls: jobRank
- unexportedFuncs: authConfigArgs, checkoutSHA, emptyUntil, ensureServedRepo, fetchBase, findSummary, git, gitAllowFail, gitTrim, isGitRepo, logf, maybeContextDir, orchestrateUntil, recordCursor, resolveDefaultBranch, resolveHEAD, runClone, runOrchestrate, runPublish, runSA, shaMatch, shortSHA, shouldRun, splitOwnerName
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/reviewrun/clone.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/reviewrun/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/reviewrun/git.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/reviewrun/run.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/reviewrun/stage.go

### Exported bodies

#### Options (type)

```go
type Options struct {
	ConfigDir   string
	RepoID      string
	PRNumber    string
	HeadSHA     string
	BaseBranch  string
	CloneURL    string
	WorkDir     string
	StagingDir  string
	OutputDir   string
	ScriptsDir  string
	ContextDir  string
	CursorDir   string
	Until       string
	Publish     bool
	SkipDeep    bool
	SkipReport  bool
	Concurrency int

	// Injectables for tests.
	Clone       func() error
	SA          func(sa.Options) error
	Orchestrate func(orchestrate.Options) error
	PublishFn   func(publish.Options) error
}
```

#### Run (func)

```go
func Run(opts Options) (err error) {
	usage := llmusage.New()
	llmusage.Push(usage)
	defer func() {
		snap := usage.Snapshot()
		logf("INFO", "repo=%s pr=%s LLM usage summary", opts.RepoID, opts.PRNumber)
		for _, line := range strings.Split(llmusage.Format(snap), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			logf("INFO", "%s", line)
		}
		llmusage.Pop()
	}()

	if strings.TrimSpace(opts.ConfigDir) == "" || strings.TrimSpace(opts.RepoID) == "" {
		return fmt.Errorf("run review requires --config-dir and --repo-id")
	}
	if strings.TrimSpace(opts.PRNumber) == "" {
		return fmt.Errorf("run review requires --pr")
	}

	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join("review-output", opts.RepoID, "pr-review")
	}

	cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	obs := cfg.Observability.Expand()
	otelCfg := observability.ResolveConfig(opts.OutputDir, observability.Settings{
		Enabled:     obs.Enabled,
		Endpoint:    obs.Endpoint,
		APIKey:      obs.APIKey,
		ServiceName: obs.ServiceName,
		Insecure:    obs.Insecure,
	})
	if _, otelErr := observability.Init(otelCfg); otelErr != nil {
		logf("WARN", "otel init: %v", otelErr)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observability.Flush(flushCtx)
		_ = observability.Shutdown(flushCtx)
	}()

	ctx, span := observability.StartChainSpan(context.Background(), otelCfg.ServiceName, "majordomo.run.review")
	defer observability.EndSpanWithStatus(span, &err)

	until, err := ParseUntil(opts.Until)
	if err != nil {
		return err
	}
	opts.Until = until

	scm := strings.ToLower(strings.TrimSpace(cfg.SCM))
	if scm == "" {
		scm = "github"
	}
	owner, name := cfg.Repository.Owner, cfg.Repository.Name
	cloneURL := strings.TrimSpace(opts.CloneURL)
	if cloneURL == "" {
		cloneURL = strings.TrimSpace(cfg.Repository.CloneURL)
	}
	if owner == "" || name == "" {
		o, n := splitOwnerName(cloneURL)
		if owner == "" {
			owner = o
		}
		if name == "" {
			name = n
		}
	}
	token := config.ResolveCredential(cfg.Repository.ID, scm, owner)
	if token == "" && strings.ToLower(scm) == "bitbucket" {
		token = strings.TrimSpace(os.Getenv("BITBUCKET_TOKEN"))
	}

	workdir := strings.TrimSpace(opts.WorkDir)
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("workdir: %w", err)
		}
		if isGitRepo(wd) {
			workdir = wd
		} else {
			workdir = filepath.Join(wd, "served", "repo")
		}
	}
	workdir, err = filepath.Abs(workdir)
	if err != nil {
		return fmt.Errorf("workdir abs: %w", err)
	}
	opts.WorkDir = workdir

	if opts.StagingDir == "" {
		opts.StagingDir = filepath.Join("staging", opts.RepoID+"-pr-"+opts.PRNumber)
	}
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join("review-output", opts.RepoID, "pr-review")
	}
	opts.StagingDir, err = filepath.Abs(opts.StagingDir)
	if err != nil {
		return fmt.Errorf("staging-dir: %w", err)
	}
	opts.OutputDir, err = filepath.Abs(opts.OutputDir)
	if err != nil {
		return fmt.Errorf("output-dir: %w", err)
	}
	if err := os.MkdirAll(opts.StagingDir, 0o755); err != nil {
		return fmt.Errorf("mkdir staging: %w", err)
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir output: %w", err)
	}

	logf("INFO", "========== majordomo run review ==========")
	logf("INFO", "repo=%s pr=%s until=%s publish=%v", opts.RepoID, opts.PRNumber, emptyUntil(opts.Until), opts.Publish)

	if err := runClone(opts, token, scm, cloneURL); err != nil {
		return err
	}
	if opts.HeadSHA == "" {
		opts.HeadSHA = resolveHEAD(opts.WorkDir)
	}
	if opts.BaseBranch == "" && isGitRepo(opts.WorkDir) {
		opts.BaseBranch = resolveDefaultBranch(opts.WorkDir, token, scm)
	}

	if !shouldRun(opts.Until, StageSA) {
		logf("INFO", "until=%s: stopping after clone", opts.Until)
		return nil
	}

	if err := runSA(opts); err != nil {
		logf("WARN", "sa: %v (continuing)", err)
	}
	if !should
// ... truncated
```

#### ParseUntil (func)

```go
func ParseUntil(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return "", nil
	case StageClone, StageSA, StagePrep, StageWaves, StageFinalize, StageProse, StageSynth, StageReport, StagePublish:
		return s, nil
	default:
		return "", fmt.Errorf("unknown until stage %q (clone|sa|prep|waves|finalize|prose|synth|report|publish)", s)
	}
}
```

### Private one-hop bodies

#### emptyUntil (func)

```go
func emptyUntil(s string) string {
	if s == "" {
		return "(full)"
	}
	return s
}
```

#### git (func)

```go
func git(dir, token, scm string, args ...string) (string, error) {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, authConfigArgs(token, scm)...)
	if strings.TrimSpace(dir) != "" {
		cmdArgs = append(cmdArgs, "-C", dir)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}
```

#### isGitRepo (func)

```go
func isGitRepo(dir string) bool {
	_, code := gitAllowFail(dir, "", "", "rev-parse", "--is-inside-work-tree")
	return code == 0
}
```

#### logf (func)

```go
func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

#### resolveDefaultBranch (func)

```go
func resolveDefaultBranch(dir, token, scm string) string {
	out, code := gitAllowFail(dir, token, scm, "symbolic-ref", "refs/remotes/origin/HEAD")
	if code == 0 {
		const prefix = "refs/remotes/origin/"
		if strings.HasPrefix(out, prefix) {
			return strings.TrimPrefix(out, prefix)
		}
	}
	for _, name := range []string{"main", "master"} {
		_, c := gitAllowFail(dir, token, scm, "rev-parse", "--verify", "origin/"+name)
		if c == 0 {
			return name
		}
	}
	return ""
}
```

#### resolveHEAD (func)

```go
func resolveHEAD(dir string) string {
	head, err := gitTrim(dir, "", "", "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return head
}
```

#### runClone (func)

```go
func runClone(opts Options, token, scm, cloneURL string) error {
	if opts.Clone != nil {
		return opts.Clone()
	}
	head := opts.HeadSHA
	return ensureServedRepo(opts, token, scm, cloneURL, head, opts.BaseBranch)
}
```

#### runSA (func)

```go
func runSA(opts Options) error {
	if opts.SA != nil {
		return opts.SA(sa.Options{
			ConfigDir:  opts.ConfigDir,
			RepoID:     opts.RepoID,
			RepoRoot:   opts.WorkDir,
			BaseBranch: opts.BaseBranch,
			ScriptsDir: opts.ScriptsDir,
		})
	}
	if opts.BaseBranch == "" {
		logf("INFO", "sa skipped: no base branch")
		return nil
	}
	return sa.Run(sa.Options{
		ConfigDir:  opts.ConfigDir,
		RepoID:     opts.RepoID,
		RepoRoot:   opts.WorkDir,
		BaseBranch: opts.BaseBranch,
		ScriptsDir: opts.ScriptsDir,
	})
}
```

#### shouldRun (func)

```go
func shouldRun(until, stage string) bool {
	if until == "" {
		return true
	}
	ur, okU := jobRank[until]
	sr, okS := jobRank[stage]
	if !okU || !okS {
		return true
	}
	return sr <= ur
}
```

#### splitOwnerName (func)

```go
func splitOwnerName(cloneURL string) (owner, name string) {
	path := strings.TrimSuffix(cloneURL, ".git")
	path = strings.TrimPrefix(path, "https://")
	path = strings.TrimPrefix(path, "http://")
	if i := strings.Index(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i], path[i+1:]
	}
	return "", path
}
```


## ./internal/sa
- package: `sa`
- packageDoc: Package sa runs staticAnalysis tools from central config against changed files.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: exec_runner
- mechanicalConfidence: 0.80
- mechanicalEvidence: imports_os_exec, exports_run_surface
- exportedDecls: Options, ToolRunner
- exportedFuncs: Run
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: defaultToolRunner, filterFiles, logf, resolveScriptsDir
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/sa/sa.go

### Exported bodies

#### ToolRunner (type)

```go
type ToolRunner func(scriptPath, slug, image, command, repoRoot string, files []string) error
```

#### Options (type)

```go
type Options struct {
	ConfigDir   string
	RepoID      string
	RepoRoot    string
	BaseBranch  string
	ScriptsDir  string
	ImagePrefix string
	// Runner overrides script execution (tests).
	Runner ToolRunner
	// ChangedFiles injectable for tests; empty → git diff via staging.SetupGit.
	ChangedFiles []string
}
```

#### Run (func)

```go
func Run(opts Options) error {
	if opts.ConfigDir == "" || opts.RepoID == "" {
		return fmt.Errorf("sa requires --config-dir and --repo-id")
	}
	if opts.BaseBranch == "" {
		return fmt.Errorf("sa requires --base-branch")
	}
	cfg, err := config.LoadMerged(opts.ConfigDir, opts.RepoID)
	if err != nil {
		return err
	}
	if len(cfg.StaticAnalysis) == 0 {
		logf("INFO", "no staticAnalysis tools configured for %s — skipping", opts.RepoID)
		return nil
	}

	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return err
	}

	files := opts.ChangedFiles
	if files == nil {
		setup, err := staging.SetupGit(opts.BaseBranch, "", repoRoot)
		if err != nil {
			return fmt.Errorf("list changed files: %w", err)
		}
		files = setup.AllFiles
	}
	logf("INFO", "========== majordomo sa ==========")
	logf("INFO", "repo %s: %d changed file(s), %d tool(s)", opts.RepoID, len(files), len(cfg.StaticAnalysis))

	scriptsDir := opts.ScriptsDir
	if scriptsDir == "" {
		scriptsDir, err = resolveScriptsDir(repoRoot)
		if err != nil {
			return err
		}
	}
	scriptPath := filepath.Join(scriptsDir, "run-sa-tool.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("run-sa-tool.sh not found at %s: %w", scriptPath, err)
	}

	runner := opts.Runner
	if runner == nil {
		runner = defaultToolRunner
	}

	for _, tool := range cfg.StaticAnalysis {
		slug := config.ResolveSAToolSlug(tool)
		matched := filterFiles(files, tool.Glob)
		if len(matched) == 0 {
			logf("INFO", "skip %s: no files match %q", slug, tool.Glob)
			continue
		}
		image := config.ResolveSAImage(tool, opts.ImagePrefix)
		cmd := strings.TrimSpace(tool.Command)
		if cmd == "" {
			logf("WARN", "skip %s: empty command", slug)
			continue
		}
		logf("INFO", "run %s image=%s files=%d", slug, image, len(matched))
		if err := runner(scriptPath, slug, image, cmd, repoRoot, matched); err != nil {
			logf("WARN", "%s: %v (continuing)", slug, err)
		}
	}
	return nil
}
```

### Private one-hop bodies

#### defaultToolRunner (func)

```go
func defaultToolRunner(scriptPath, slug, image, command, repoRoot string, files []string) error {
	args := append([]string{slug, image, command, repoRoot}, files...)
	cmd := exec.Command(scriptPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run-sa-tool.sh %s: %w", slug, err)
	}
	return nil
}
```

#### filterFiles (func)

```go
func filterFiles(files []string, glob string) []string {
	glob = strings.TrimSpace(glob)
	if glob == "" {
		return append([]string(nil), files...)
	}
	var out []string
	for _, f := range files {
		if staging.MatchGlob(glob, f) {
			out = append(out, f)
		}
	}
	return out
}
```

#### logf (func)

```go
func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

#### resolveScriptsDir (func)

```go
func resolveScriptsDir(repoRoot string) (string, error) {
	candidates := []string{
		filepath.Join(repoRoot, "pipelines", "scripts"),
		filepath.Join(repoRoot, ".majordomo", "pipelines", "scripts"),
	}
	if v := os.Getenv("MAJORDOMO_SCRIPTS"); v != "" {
		candidates = append([]string{v}, candidates...)
	}
	wd, _ := os.Getwd()
	dir := wd
	for i := 0; i < 8 && dir != ""; i++ {
		candidates = append(candidates,
			filepath.Join(dir, "pipelines", "scripts"),
			filepath.Join(dir, ".majordomo", "pipelines", "scripts"),
		)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "run-sa-tool.sh")); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("run-sa-tool.sh not found (set --scripts-dir or MAJORDOMO_SCRIPTS)")
}
```


## ./internal/satools
- package: `satools`
- packageDoc: Package satools builds local SA tool Docker images for Dockerfile validation.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: exec_runner
- mechanicalConfidence: 0.80
- mechanicalEvidence: imports_os_exec, exports_run_surface
- exportedDecls: Options
- exportedFuncs: Run
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: discoverDockerfiles, findBuildScript, imageTag, printResult, resolveRepoRoot, runBuild, runCmd, saToolsDir, setEnv, toolName, unsetEnv, workspaceRoot
- unexportedMethods: (none)
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/satools/satools.go

### Exported bodies

#### Options (type)

```go
type Options struct {
	DryRun  bool
	Verbose bool
	Corp    bool
	// RepoRoot is the majordomo checkout (directory containing scripts/ or go.mod).
	// Empty → discover from cwd.
	RepoRoot string
	// Runner overrides command execution (tests).
	Runner func(name string, args []string, env []string, dir string) (stdout, stderr string, err error)
}
```

#### Run (func)

```go
func Run(opts Options) error {
	repoRoot, err := resolveRepoRoot(opts.RepoRoot)
	if err != nil {
		return err
	}
	workspace := workspaceRoot(repoRoot)
	saDir := saToolsDir(repoRoot)
	dockerfiles, err := discoverDockerfiles(saDir)
	if err != nil {
		return err
	}
	if len(dockerfiles) == 0 {
		return fmt.Errorf("no Dockerfiles found in %s", saDir)
	}

	if opts.Corp && !opts.DryRun {
		if os.Getenv("REGISTRY_USER") == "" || os.Getenv("REGISTRY_TOKEN") == "" || os.Getenv("PACKAGE_REGISTRY_HOST") == "" {
			return fmt.Errorf("--corp requires PACKAGE_REGISTRY_HOST, REGISTRY_USER, and REGISTRY_TOKEN")
		}
	}

	mode := "public"
	if opts.Corp {
		mode = "corp"
	}
	tools := make([]string, 0, len(dockerfiles))
	for _, df := range dockerfiles {
		tools = append(tools, toolName(df))
	}
	fmt.Printf("SA Tool Image Builder\nMode:      %s\nContext:   %s\nTools:     %s\nDry-run:   %v\n\n",
		mode, workspace, strings.Join(tools, ", "), opts.DryRun)

	if opts.DryRun {
		for _, df := range dockerfiles {
			fmt.Printf("  [dry-run] would build sa-%s (%s) from %s\n", toolName(df), mode, df)
		}
		return nil
	}

	buildSh, err := findBuildScript(repoRoot, workspace)
	if err != nil {
		return err
	}

	results := map[string]bool{}
	var names []string
	for _, df := range dockerfiles {
		tool := toolName(df)
		names = append(names, tool)
		tag := imageTag(tool)
		fmt.Printf("Building %s (%s) ...\n", tag, mode)
		ok, output := runBuild(opts, buildSh, df, workspace, tag, tool)
		results[tool] = ok
		printResult(tool, ok, output, opts.Verbose)
	}

	sort.Strings(names)
	passed := 0
	for _, n := range names {
		if results[n] {
			passed++
		}
	}
	fmt.Printf("\nResults: %d/%d passed\n", passed, len(names))
	for _, n := range names {
		status := "FAIL"
		if results[n] {
			status = "PASS"
		}
		fmt.Printf("  %s  sa-%s\n", status, n)
	}
	if passed < len(names) {
		return fmt.Errorf("%d/%d SA tool builds failed", len(names)-passed, len(names))
	}
	return nil
}
```

### Private one-hop bodies

#### discoverDockerfiles (func)

```go
func discoverDockerfiles(saDir string) ([]string, error) {
	entries, err := os.ReadDir(saDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".Dockerfile") {
			out = append(out, filepath.Join(saDir, name))
		}
	}
	sort.Strings(out)
	return out, nil
}
```

#### findBuildScript (func)

```go
func findBuildScript(repoRoot, workspace string) (string, error) {
	candidates := []string{
		filepath.Join(repoRoot, "pipelines", "scripts", "build-copilot-image.sh"),
		filepath.Join(workspace, ".majordomo", "pipelines", "scripts", "build-copilot-image.sh"),
		filepath.Join(workspace, "pipelines", "scripts", "build-copilot-image.sh"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("build-copilot-image.sh not found (searched under %s)", repoRoot)
}
```

#### imageTag (func)

```go
func imageTag(tool string) string {
	return "sa-" + tool + ":local-test"
}
```

#### printResult (func)

```go
func printResult(tool string, success bool, output []string, verbose bool) {
	marker := "✗"
	status := "FAIL"
	if success {
		marker = "✓"
		status = "PASS"
	}
	fmt.Printf("  %s sa-%s: %s\n", marker, tool, status)
	if !success || verbose {
		for _, line := range output {
			fmt.Printf("      %s\n", line)
		}
	}
}
```

#### resolveRepoRoot (func)

```go
func resolveRepoRoot(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Clean(explicit), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "dockerfiles", "sa-tools")); err == nil {
			return dir, nil
		}
		// Vendored as .majordomo under a parent workspace.
		if filepath.Base(dir) == ".majordomo" {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return wd, nil
}
```

#### runBuild (func)

```go
func runBuild(opts Options, buildSh, dockerfile, workspace, tag, tool string) (bool, []string) {
	dockerfileArg := dockerfile
	if rel, err := filepath.Rel(workspace, dockerfile); err == nil {
		dockerfileArg = rel
	}
	env := append([]string{}, os.Environ()...)
	target := "public"
	if opts.Corp {
		target = "corp"
	}
	env = setEnv(env, "DOCKER_BUILD_TARGET", target)
	env = setEnv(env, "SKIP_PUSH", "true")
	if !opts.Corp {
		env = unsetEnv(env, "PACKAGE_REGISTRY_HOST")
	}
	args := []string{buildSh, "local", "sa-" + tool, "local-test", dockerfileArg}
	stdout, stderr, err := runCmd(opts, "bash", args, env, workspace)
	lines := strings.Split(strings.TrimRight(stdout+stderr, "\n"), "\n")
	if err == nil {
		full := "local/sa-" + tool + ":local-test"
		_, _, _ = runCmd(opts, "docker", []string{"tag", full, tag}, os.Environ(), "")
	}
	return err == nil, lines
}
```

#### saToolsDir (func)

```go
func saToolsDir(repoRoot string) string {
	primary := filepath.Join(repoRoot, "dockerfiles", "sa-tools")
	if st, err := os.Stat(primary); err == nil && st.IsDir() {
		return primary
	}
	vendored := filepath.Join(filepath.Dir(repoRoot), ".majordomo", "dockerfiles", "sa-tools")
	if st, err := os.Stat(vendored); err == nil && st.IsDir() {
		return vendored
	}
	return primary
}
```

#### toolName (func)

```go
func toolName(dockerfile string) string {
	base := filepath.Base(dockerfile)
	return strings.TrimSuffix(base, ".Dockerfile")
}
```

#### workspaceRoot (func)

```go
func workspaceRoot(repoRoot string) string {
	if st, err := os.Stat(filepath.Join(repoRoot, "dockerfiles", "sa-tools")); err == nil && st.IsDir() {
		return repoRoot
	}
	parent := filepath.Dir(repoRoot)
	if st, err := os.Stat(filepath.Join(parent, ".majordomo", "dockerfiles", "sa-tools")); err == nil && st.IsDir() {
		return parent
	}
	return repoRoot
}
```


## ./internal/staging
- package: `staging`
- packageDoc: Package staging ports git-diff-prep: classify, cluster, batch, write manifest.
- hasMain: false
- jsonTags: true
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: exec_runner
- mechanicalConfidence: 0.80
- mechanicalEvidence: imports_os_exec, exports_run_surface
- exportedDecls: AgentContext, BatchEntry, CrossSkillBatchDir, CrossSkillBatchNum, DefaultRouting, ErrFatal, ErrNothingToReview, GitError, GitRunner, GitTimeout, MaxCombinedLines, MaxDiffLines, MaxStageFilenameBytes, ModeDiffChunk, ModeDiffOnly, ModeFullAndDiff, Options, RoutingRule, SetupGitResult, Task
- exportedFuncs: AttachGrounding, BuildStagingFilename, ChunkLines, ClassifyFile, CollectSAFindings, ContextForFile, DetectSADir, FileSlug, GetSubmoduleExclusions, IsExcluded, IsExcludedWithExtra, LoadAgentContextConfig, LoadRouting, LoadSummaryConfig, MatchGlob, ParseNameStatus, ParseSubmoduleStatusLines, ResolveContextDir, ResolveRoutingPersonas, Run, SetupGit, StageCrossSkillBatches, StageFile, StageSkillBatches, WriteBatchPlan
- exportedMethods: ErrFatal.Error, GitError.Error
- unexportedDecls: addedFileGuidance, excludePatterns, nonWord
- unexportedFuncs: batchSizeFromEnv, bytesReader, classifyFiles, copyFile, copyMap, decodeJSONObjectOrder, fatalf, fmtBatchDir, init, loadScopedForm, logf, manifestChangedFiles, matchStarSlash, patchManifestGrounding, requiredTaskString, resolveContextRules, resolveRules, sortStrings, stageReviewableFiles, tasksToMaps, trimSpace, truncateUTF8, writeJSON
- unexportedMethods: GitRunner.diff, GitRunner.run, GitRunner.runAllowFail, GitRunner.showHEAD, GitRunner.timeout
- errorTypes: ErrFatal, ErrNothingToReview, GitError
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/batch.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/bytes.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/context.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/contextdir.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/cross_skill.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/exclude.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/filename.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/git.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/grounding.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/prep.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/routing.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/stage_file.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/staging/types.go

### Exported bodies

#### BatchEntry (type)

```go
type BatchEntry struct {
	Skill      string `json:"skill"`
	BatchNum   string `json:"batch_num"`
	TaskCount  int    `json:"task_count"`
	StagingDir string `json:"staging_dir"`
}
```

#### StageSkillBatches (func)

```go
func StageSkillBatches(
	tasks []Task,
	reviewAgents map[string][]string,
	excluded []string,
	stagingDir, repoRoot, baseBranch, refspec string,
	batchSize int,
) ([]BatchEntry, []string, error) {
	bySkill := map[string][]Task{}
	skillOrder := []string{}
	for _, task := range tasks {
		agent, err := requiredTaskString(task, "agent")
		if err != nil {
			return nil, nil, err
		}
		if _, ok := bySkill[agent]; !ok {
			skillOrder = append(skillOrder, agent)
		}
		bySkill[agent] = append(bySkill[agent], task)
	}

	docChanged := []string{}
	for _, t := range tasks {
		f, err := requiredTaskString(t, "file")
		if err != nil {
			return nil, nil, err
		}
		if strings.HasSuffix(f, ".md") {
			docChanged = append(docChanged, f)
		}
	}
	var corpusIndex []map[string]any
	if len(docChanged) > 0 {
		corpusIndex = cluster.BuildCorpusIndex(repoRoot)
		logf("INFO", "Corpus index: %d .md file(s) indexed", len(corpusIndex))
	}

	batchEntries := []BatchEntry{}
	for _, skill := range skillOrder {
		skillTasks := bySkill[skill]
		skillStaging := filepath.Join(stagingDir, skill)
		if err := os.MkdirAll(skillStaging, 0o755); err != nil {
			return nil, nil, err
		}
		skillManifest := map[string]any{
			"base_branch":   baseBranch,
			"refspec":       refspec,
			"skill_dir":     skill,
			"review_agents": map[string][]string{skill: reviewAgents[skill]},
			"reviewable":    skillTasks,
			"excluded":      excluded,
		}
		if err := writeJSON(filepath.Join(skillStaging, "manifest.json"), skillManifest); err != nil {
			return nil, nil, err
		}

		skillMD := []string{}
		for _, t := range skillTasks {
			f, err := requiredTaskString(t, "file")
			if err != nil {
				return nil, nil, err
			}
			if strings.HasSuffix(f, ".md") {
				skillMD = append(skillMD, f)
			}
		}
		skillHasMD := len(skillMD) > 0
		var skillDocClusters [][]string
		var skillReverseLinks map[string][]string
		var batches [][]map[string]any
		asMaps := tasksToMaps(skillTasks)
		if skillHasMD {
			clusterBatches, err := cluster.DocClusterAwareBatches(asMaps, batchSize, repoRoot)
			if err != nil {
				return nil, nil, err
			}
			batches = clusterBatches
			for _, c := range cluster.ClusterDocs(skillMD, repoRoot) {
				if len(c) > 1 {
					skillDocClusters = append(skillDocClusters, c)
				}
			}
			skillReverseLinks = cluster.ReverseLinks(skillMD, repoRoot)
		} else {
			clusterBatches, err := cluster.DepClusterAwareBatches(asMaps, batchSize, repoRoot)
			if err != nil {
				return nil, nil, err
			}
			batches = clusterBatches
		}

		for batchIdx, batchSlice := range batches {
			batchNum := batchIdx + 1
			batchDir := filepath.Join(skillStaging, fmtBatchDir(batchNum))
			if err := os.MkdirAll(batchDir, 0o755); err != nil {
				return nil, nil, err
			}
			batchManifest := map[string]any{
				"base_branch":   baseBranch,
				"refspec":       refspec,
				"skill_dir":     skill,
				"review_agents": map[string][]string{skill: reviewAgents[skill]},
				"reviewable":    batchSlice,
				"excluded":      excluded,
			}
			if skillHasMD {
				batchManifest["doc_clusters"] = skillDocClusters
				batchManifest["reverse_links"] = skillReverseLinks
			}
			if err := writeJSON(filepath.Join(batchDir, "manifest.json"), batchManifest); err != nil {
				return nil, nil, err
			}
			for _, task := range batchSlice {
				inputFile, err := requiredTaskString(task, "input_file")
				if err != nil {
					return nil, nil, err
				}
				src := filepath.Join(stagingDir, inputFile)
				dst := filepath.Join(batchDir, inputFile)
				if err := copyFile(src, dst); err != nil {
					// missing source is skipped like Python (only copy if exists)
					continue
				}
			}
			if skillHasMD && len(corpusIndex) > 0 {
				if err := writeJSON(filepath.Join(batchDir, "corpus-index.json"), corpusIndex); err != nil {
					return nil, nil, err
				}
			}
			dirs := map[string]struct{}{}
			for _, t := range batchSlice {
				f, err := requiredTaskString(t, "file")
				if err != nil {
					return nil, nil, err
				}
				dirs
// ... truncated
```

#### AgentContext (type)

```go
type AgentContext struct {
	Global map[string]any
	Scoped map[string]any
}
```

#### LoadAgentContextConfig (func)

```go
func LoadAgentContextConfig(path string) (AgentContext, error) {
	empty := AgentContext{Global: map[string]any{}, Scoped: map[string]any{}}
	if path == "" {
		return empty, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty, nil
		}
		return empty, fatalf("Failed to load agent context config (%v)", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return empty, fatalf("Failed to load agent context config (%v)", err)
	}
	if _, hasGlobal := raw["global"]; hasGlobal {
		return loadScopedForm(raw, path)
	}
	if _, hasScoped := raw["scoped"]; hasScoped {
		return loadScopedForm(raw, path)
	}
	logf("INFO", "Loaded legacy flat agent context: %s", path)
	return AgentContext{Global: raw, Scoped: map[string]any{}}, nil
}
```

#### ContextForFile (func)

```go
func ContextForFile(filePath string, agentContext AgentContext, repoRoot string) (map[string]any, error) {
	globalRaw := agentContext.Global
	if globalRaw == nil {
		return nil, fatalf("agentContext.global must be an object")
	}
	globalCtx, err := resolveContextRules(globalRaw, repoRoot, "agentContext.global")
	if err != nil {
		return nil, err
	}
	scopedRaw := agentContext.Scoped
	if scopedRaw == nil {
		return nil, fatalf("agentContext.scoped must be an object")
	}

	var matchedGlob string
	var matchedCtxRaw map[string]any
	for glob, scopedCtx := range scopedRaw {
		if !MatchGlob(glob, filePath) {
			continue
		}
		m, ok := scopedCtx.(map[string]any)
		if !ok {
			return nil, fatalf("agentContext.scoped['%s'] must be an object", glob)
		}
		matchedGlob = glob
		matchedCtxRaw = m
		break
	}
	if matchedGlob == "" {
		return globalCtx, nil
	}
	scopedCtx, err := resolveContextRules(matchedCtxRaw, repoRoot, "agentContext.scoped['"+matchedGlob+"']")
	if err != nil {
		return nil, err
	}
	merged := copyMap(globalCtx)
	for key, value := range scopedCtx {
		if key == "customRules" {
			continue
		}
		merged[key] = value
	}
	gRules, ok := globalCtx["customRules"].([]any)
	if !ok {
		return nil, fatalf("agentContext.global customRules must be a list")
	}
	sRules, ok := scopedCtx["customRules"].([]any)
	if !ok {
		return nil, fatalf("agentContext.scoped['%s'] customRules must be a list", matchedGlob)
	}
	combined := append([]any{}, gRules...)
	combined = append(combined, sRules...)
	merged["customRules"] = combined
	return merged, nil
}
```

#### LoadSummaryConfig (func)

```go
func LoadSummaryConfig(path string) (map[string]any, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fatalf("--summary-config file not found: %s", path)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fatalf("invalid summary-config JSON: %v", err)
	}
	return out, nil
}
```

#### ResolveContextDir (func)

```go
func ResolveContextDir(flag string) string {
	if s := strings.TrimSpace(flag); s != "" {
		return s
	}
	return strings.TrimSpace(os.Getenv("MAJORDOMO_CONTEXT_DIR"))
}
```

#### StageCrossSkillBatches (func)

```go
func StageCrossSkillBatches(
	g *GitRunner,
	allFiles []string,
	saDir, stagingDir, repoRoot, baseBranch, refspec string,
	fileStatus map[string]string,
	statusPairs [][2]string,
	agentContext AgentContext,
	extraExcl []*regexp.Regexp,
	summaryConfig map[string]any,
) ([]BatchEntry, []string, error) {
	summarySkill := "pr-review-summary"
	technicalSkill := "pr-review-technical"
	blastRadiusSkill := "pr-review-blast-radius"

	summaryStaging := filepath.Join(stagingDir, summarySkill, CrossSkillBatchDir)
	if err := os.MkdirAll(summaryStaging, 0o755); err != nil {
		return nil, nil, err
	}

	summaryTasks := []Task{}
	for _, file := range allFiles {
		if IsExcludedWithExtra(file, extraExcl) {
			continue
		}
		full := filepath.Join(repoRoot, filepath.FromSlash(file))
		if _, err := os.Stat(full); err != nil {
			continue
		}
		slug := FileSlug(file)
		fileDiff, err := g.diff(refspec, file)
		if err != nil {
			logf("WARN", "  summary: git error for %s: %v", file, err)
			continue
		}
		if strings.TrimSpace(fileDiff) == "" {
			continue
		}
		inputFile := BuildStagingFilename(slug, "")
		if err := os.WriteFile(filepath.Join(summaryStaging, inputFile), []byte(fileDiff), 0o644); err != nil {
			return nil, nil, err
		}
		ctx, err := ContextForFile(file, agentContext, repoRoot)
		if err != nil {
			return nil, nil, err
		}
		status := fileStatus[file]
		if status == "" {
			status = "M"
		}
		summaryTasks = append(summaryTasks, Task{
			"file": file, "slug": slug, "mode": ModeDiffOnly,
			"chunk": nil, "total_chunks": nil, "input_file": inputFile,
			"agent": summarySkill, "status": status, "agent_context": ctx,
		})
	}

	summaryFiles := make([]string, 0, len(summaryTasks))
	for _, t := range summaryTasks {
		summaryFiles = append(summaryFiles, t["file"].(string))
	}
	summaryClusters := cluster.ClusterFiles(summaryFiles, repoRoot)
	summaryDepClusters := [][]string{}
	for _, c := range summaryClusters {
		if len(c) > 1 {
			summaryDepClusters = append(summaryDepClusters, c)
		}
	}
	logf("INFO", "Summary dep clusters: %d multi-file cluster(s)", len(summaryDepClusters))

	summaryReverseDeps := cluster.ReverseDeps(summaryFiles, repoRoot)
	logf("INFO", "Summary reverse deps: %d changed file(s) have external importers", len(summaryReverseDeps))

	summarySA := map[string]string{}
	if saDir != "" {
		entries, _ := os.ReadDir(saDir)
		names := []string{}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
				names = append(names, e.Name())
			}
		}
		sortStrings(names)
		for _, name := range names {
			data, err := os.ReadFile(filepath.Join(saDir, name))
			if err != nil {
				continue
			}
			summarySA[strings.TrimSuffix(name, ".txt")] = string(data)
		}
		if len(summarySA) > 0 {
			logf("INFO", "Summary SA: %d tool output(s) embedded in manifest", len(summarySA))
		}
	}

	statusBreakdown := map[string]int{}
	for _, p := range statusPairs {
		statusBreakdown[p[0]]++
	}
	sc := summaryConfig
	if sc == nil {
		sc = map[string]any{}
	}
	summaryManifest := map[string]any{
		"base_branch":      baseBranch,
		"refspec":          refspec,
		"skill_dir":        summarySkill,
		"review_agents":    map[string][]string{summarySkill: summaryFiles},
		"reviewable":       summaryTasks,
		"excluded":         []string{},
		"dep_clusters":     summaryDepClusters,
		"reverse_deps":     summaryReverseDeps,
		"static_analysis":  summarySA,
		"status_breakdown": statusBreakdown,
		"summary_config":   sc,
	}
	if err := writeJSON(filepath.Join(summaryStaging, "manifest.json"), summaryManifest); err != nil {
		return nil, nil, err
	}
	logf("INFO", "Summary batch: %d file(s) staged in %s", len(summaryTasks), summaryStaging)

	blastEntries := []BatchEntry{}
	if len(summaryReverseDeps) > 0 {
		blastStaging := filepath.Join(stagingDir, blastRadiusSkill, CrossSkillBatchDir)
		if err := os.MkdirAll(blastStaging, 0o755); err != nil {
			return nil, nil, err
		}
		for _, task := range summaryTasks {
			inputFile := task["input_file"].(stri
// ... truncated
```

#### WriteBatchPlan (func)

```go
func WriteBatchPlan(entries []BatchEntry, skills []string, stagingDir string) error {
	plan := map[string]any{
		"batches":       entries,
		"skills":        skills,
		"total_batches": len(entries),
	}
	path := filepath.Join(stagingDir, "batch-plan.json")
	if err := writeJSON(path, plan); err != nil {
		return err
	}
	logf("INFO", "Batch plan: %d batch(es) → %s", len(entries), path)
	return nil
}
```

#### IsExcluded (func)

```go
func IsExcluded(file string) bool {
	for _, p := range excludePatterns {
		if p.MatchString(file) {
			return true
		}
	}
	return false
}
```

#### ParseSubmoduleStatusLines (func)

```go
func ParseSubmoduleStatusLines(stdout string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "+-U")
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			p := strings.TrimRight(parts[1], "/")
			patterns = append(patterns, regexp.MustCompile("^"+regexp.QuoteMeta(p)+"/"))
		}
	}
	return patterns
}
```

#### MatchGlob (func)

```go
func MatchGlob(pattern, name string) bool {
	pattern = strings.ReplaceAll(pattern, "**", "*")
	return matchStarSlash(pattern, name)
}
```

#### FileSlug (func)

```go
func FileSlug(file string) string {
	return nonWord.ReplaceAllString(file, "-")
}
```

#### BuildStagingFilename (func)

```go
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
```

#### ParseNameStatus (func)

```go
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
```

#### ChunkLines (func)

```go
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
```

#### GitError (error)

```go
type GitError struct {
	Args string
	Err  string
}
```

#### GitError.Error (method)

```go
func (e *GitError) Error() string {
	return fmt.Sprintf("git %s failed: %s", e.Args, e.Err)
}
```

#### GitRunner (type)

```go
type GitRunner struct {
	Dir     string
	Timeout time.Duration
}
```

#### SetupGitResult (type)

```go
type SetupGitResult struct {
	Refspec     string
	RepoRoot    string
	AllFiles    []string
	FileStatus  map[string]string
	StatusPairs [][2]string
	ExtraExcl   []*regexp.Regexp
}
```

#### SetupGit (func)

```go
func SetupGit(baseBranch, stagingDir, workDir string) (*SetupGitResult, error) {
	g := &GitRunner{Dir: workDir}
	extra := GetSubmoduleExclusions(g)

	cwd := workDir
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fatalf("cannot get cwd: %v", err)
		}
	}
	// Best-effort safe.directory (may fail without write access to global gitconfig).
	if err := exec.Command("git", "config", "--global", "--add", "safe.directory", cwd).Run(); err != nil {
		logf("WARN", "could not configure git safe.directory for %s: %v", cwd, err)
	}

	refspec := fmt.Sprintf("origin/%s...HEAD", baseBranch)
	logf("INFO", "%s", strings.Repeat("=", 50))
	logf("INFO", "majordomo prep")
	logf("INFO", "%s", strings.Repeat("=", 50))
	logf("INFO", "Base branch:  %s", baseBranch)
	logf("INFO", "Refspec:      %s", refspec)
	logf("INFO", "Staging dir:  %s", stagingDir)

	shallowOut, shallowCode := g.runAllowFail("rev-parse", "--is-shallow-repository")
	if shallowCode != 0 {
		logf("WARN", "could not determine whether repository is shallow")
	}
	isShallow := strings.TrimSpace(shallowOut) == "true"
	logf("INFO", "Shallow clone: %v", isShallow)

	mbOut, mbCode := g.runAllowFail("merge-base", "origin/"+baseBranch, "HEAD")
	if mbCode != 0 {
		return nil, fatalf(
			"No common ancestor found between 'origin/%s' and HEAD. "+
				"The feature branch and '%s' have disconnected git histories. "+
				"Ensure '%s' in the target repo was pushed from the same origin "+
				"as the feature branch so a merge base exists.",
			baseBranch, baseBranch, baseBranch,
		)
	}
	logf("INFO", "Merge base: %s", strings.TrimSpace(mbOut))

	repoRootOut, err := g.run("rev-parse", "--show-toplevel")
	if err != nil {
		logf("ERROR", "%v", err)
		return nil, fatalf("%v", err)
	}
	repoRoot := strings.TrimSpace(repoRootOut)

	changedRaw, err := g.run("diff", "-z", "--name-status", refspec)
	if err != nil {
		logf("ERROR", "%v", err)
		return nil, fatalf("%v", err)
	}

	statusPairs := ParseNameStatus(changedRaw)
	allFiles := make([]string, 0, len(statusPairs))
	fileStatus := make(map[string]string, len(statusPairs))
	for _, p := range statusPairs {
		allFiles = append(allFiles, p[1])
		fileStatus[p[1]] = p[0]
	}

	logf("INFO", "Raw diff -z --name-status output (%d files):", len(allFiles))
	if len(allFiles) == 0 {
		logf("INFO", "  (empty)")
	} else {
		for _, p := range statusPairs {
			logf("INFO", "  [%s] %s", p[0], p[1])
		}
	}
	if len(allFiles) == 0 {
		logf("WARN", "No changes detected against origin/%s", baseBranch)
		return nil, ErrNothingToReview
	}

	return &SetupGitResult{
		Refspec:     refspec,
		RepoRoot:    repoRoot,
		AllFiles:    allFiles,
		FileStatus:  fileStatus,
		StatusPairs: statusPairs,
		ExtraExcl:   extra,
	}, nil
}
```

#### GetSubmoduleExclusions (func)

```go
func GetSubmoduleExclusions(g *GitRunner) []*regexp.Regexp {
	out, code := g.runAllowFail("submodule", "status", "--cached")
	if code != 0 || strings.TrimSpace(out) == "" {
		return nil
	}
	return ParseSubmoduleStatusLines(out)
}
```

#### IsExcludedWithExtra (func)

```go
func IsExcludedWithExtra(file string, extra []*regexp.Regexp) bool {
	if IsExcluded(file) {
		return true
	}
	for _, p := range extra {
		if p.MatchString(file) {
			return true
		}
	}
	return false
}
```

#### AttachGrounding (func)

```go
func AttachGrounding(contextDir string, batches []BatchEntry) error {
	contextDir = filepath.Clean(contextDir)
	idx, err := agenting.LoadIndex(contextDir)
	if err != nil {
		return fmt.Errorf("agenting: %w", err)
	}
	for _, b := range batches {
		manifestPath := filepath.Join(b.StagingDir, "manifest.json")
		files, err := manifestChangedFiles(manifestPath)
		if err != nil {
			return err
		}
		mode := agenting.ModeForSkill(b.Skill)
		packIDs := agenting.Select(idx, mode, files)
		if len(packIDs) == 0 {
			logf("INFO", "Grounding: %s/%s — no packs for mode %s", b.Skill, b.BatchNum, mode)
			continue
		}
		staged, err := agenting.Stage(contextDir, b.StagingDir, packIDs)
		if err != nil {
			return err
		}
		if err := patchManifestGrounding(manifestPath, staged); err != nil {
			return err
		}
		logf("INFO", "Grounding: %s/%s — packs %v (mode %s)", b.Skill, b.BatchNum, packIDs, mode)
	}
	return nil
}
```

#### Run (func)

```go
func Run(opts Options) error {
	if opts.BatchSize <= 0 {
		opts.BatchSize = batchSizeFromEnv()
	}
	workDir := opts.RepoRoot
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return fatalf("cannot get cwd: %v", err)
		}
	}

	if err := os.MkdirAll(opts.StagingDir, 0o755); err != nil {
		return fatalf("cannot create staging dir: %v", err)
	}

	routing, personaPaths, err := LoadRouting(opts.RoutingPath)
	if err != nil {
		return err
	}
	summaryConfig, err := LoadSummaryConfig(opts.SummaryConfigPath)
	if err != nil {
		return err
	}

	setup, err := SetupGit(opts.BaseBranch, opts.StagingDir, workDir)
	if err != nil {
		return err
	}

	agentContext, err := LoadAgentContextConfig(opts.AgentContextPath)
	if err != nil {
		return err
	}
	personas, err := ResolveRoutingPersonas(personaPaths, setup.RepoRoot)
	if err != nil {
		return err
	}

	reviewable, excluded := classifyFiles(setup.AllFiles, routing, setup.ExtraExcl)
	if len(reviewable) == 0 {
		logf("WARN", "All changed files excluded — nothing to review")
		return ErrNothingToReview
	}

	saDir := DetectSADir(workDir)
	g := &GitRunner{Dir: workDir}
	tasks, reviewAgents, excluded, err := stageReviewableFiles(
		g, reviewable, excluded, routing, setup.Refspec, opts.StagingDir,
		setup.RepoRoot, saDir, opts.BaseBranch, setup.FileStatus, agentContext, personas,
	)
	if err != nil {
		return err
	}

	batchEntries, codeSkillNames, err := StageSkillBatches(
		tasks, reviewAgents, excluded, opts.StagingDir, setup.RepoRoot,
		opts.BaseBranch, setup.Refspec, opts.BatchSize,
	)
	if err != nil {
		return err
	}

	crossEntries, crossSkills, err := StageCrossSkillBatches(
		g, setup.AllFiles, saDir, opts.StagingDir, setup.RepoRoot,
		opts.BaseBranch, setup.Refspec, setup.FileStatus, setup.StatusPairs,
		agentContext, setup.ExtraExcl, summaryConfig,
	)
	if err != nil {
		return err
	}
	for i := len(crossEntries) - 1; i >= 0; i-- {
		batchEntries = append([]BatchEntry{crossEntries[i]}, batchEntries...)
	}
	batchPlanSkills := append(append([]string{}, crossSkills...), codeSkillNames...)
	if err := WriteBatchPlan(batchEntries, batchPlanSkills, opts.StagingDir); err != nil {
		return err
	}
	if strings.TrimSpace(opts.ContextDir) != "" {
		if err := AttachGrounding(opts.ContextDir, batchEntries); err != nil {
			return err
		}
	}
	return nil
}
```

#### RoutingRule (type)

```go
type RoutingRule struct {
	Agent string
	Globs []string
}
```

#### LoadRouting (func)

```go
func LoadRouting(routingPath string) ([]RoutingRule, map[string]string, error) {
	if routingPath == "" {
		return DefaultRouting, map[string]string{}, nil
	}
	data, err := os.ReadFile(routingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultRouting, map[string]string{}, nil
		}
		logf("WARN", "Failed to load routing config (%v) — using defaults", err)
		return DefaultRouting, map[string]string{}, nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		logf("WARN", "Failed to load routing config (%v) — using defaults", err)
		return DefaultRouting, map[string]string{}, nil
	}
	rules := make([]RoutingRule, 0, len(raw))
	personaPaths := map[string]string{}
	// Preserve JSON object key order is not guaranteed; for parity with Python 3.7+
	// insertion order we re-parse with ordered decoder when needed. For typical
	// small configs, iterate via json.Decoder with UseNumber for stability.
	ordered, err := decodeJSONObjectOrder(data)
	if err != nil {
		logf("WARN", "Failed to load routing config (%v) — using defaults", err)
		return DefaultRouting, map[string]string{}, nil
	}
	for _, agent := range ordered {
		value := raw[agent]
		switch v := value.(type) {
		case []any:
			globs := make([]string, 0, len(v))
			for _, g := range v {
				s, ok := g.(string)
				if !ok {
					return nil, nil, fatalf("routing['%s'] must be a list of globs or a map with 'globs'", agent)
				}
				globs = append(globs, s)
			}
			rules = append(rules, RoutingRule{Agent: agent, Globs: globs})
		case map[string]any:
			globsRaw, ok := v["globs"]
			if !ok {
				return nil, nil, fatalf("routing['%s'] must be a list of globs or a map with 'globs'", agent)
			}
			globsList, ok := globsRaw.([]any)
			if !ok {
				return nil, nil, fatalf("routing['%s'].globs must be a list", agent)
			}
			globs := make([]string, 0, len(globsList))
			for _, g := range globsList {
				s, ok := g.(string)
				if !ok {
					return nil, nil, fatalf("routing['%s'].globs must be a list", agent)
				}
				globs = append(globs, s)
			}
			rules = append(rules, RoutingRule{Agent: agent, Globs: globs})
			if persona, exists := v["persona"]; exists && persona != nil {
				ps, ok := persona.(string)
				if !ok {
					return nil, nil, fatalf("routing['%s'].persona must be a string path", agent)
				}
				if ps != "" {
					personaPaths[agent] = ps
				}
			}
		default:
			return nil, nil, fatalf("routing['%s'] must be a list of globs or a map with 'globs'", agent)
		}
	}
	logf("INFO", "Loaded routing config: %s", routingPath)
	return rules, personaPaths, nil
}
```

#### ClassifyFile (func)

```go
func ClassifyFile(file string, routing []RoutingRule) string {
	for _, rule := range routing {
		for _, pat := range rule.Globs {
			if MatchGlob(pat, file) {
				return rule.Agent
			}
		}
	}
	return ""
}
```

#### ResolveRoutingPersonas (func)

```go
func ResolveRoutingPersonas(personaPaths map[string]string, repoRoot string) (map[string]string, error) {
	resolved := map[string]string{}
	for agent, relPath := range personaPaths {
		relPath = trimSpace(relPath)
		if relPath == "" {
			return nil, fatalf("routing['%s'].persona has an empty path", agent)
		}
		full := filepath.Join(repoRoot, relPath)
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			return nil, fatalf("routing['%s'].persona file not found: %s", agent, relPath)
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, fatalf("routing['%s'].persona failed reading %s: %v", agent, relPath, err)
		}
		text := trimSpace(string(data))
		if text == "" {
			return nil, fatalf("routing['%s'].persona file is empty: %s", agent, relPath)
		}
		resolved[agent] = text
		logf("INFO", "  Loaded persona for %s: %s", agent, relPath)
	}
	return resolved, nil
}
```

#### Task (type)

```go
type Task map[string]any
```

#### CollectSAFindings (func)

```go
func CollectSAFindings(file, saDir string) string {
	info, err := os.Stat(saDir)
	if err != nil || !info.IsDir() {
		return ""
	}
	matchCandidates := []string{file, "/workspace/" + file}
	entries, err := os.ReadDir(saDir)
	if err != nil {
		return ""
	}
	names := make([]string, 0)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	findings := make([]string, 0)
	for _, name := range names {
		tool := strings.TrimSuffix(name, ".txt")
		data, err := os.ReadFile(filepath.Join(saDir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			for _, candidate := range matchCandidates {
				if strings.Contains(line, candidate) {
					findings = append(findings, tool+": "+line)
					break
				}
			}
		}
	}
	if len(findings) == 0 {
		return ""
	}
	return "\n=== STATIC ANALYSIS ===\n" + strings.Join(findings, "\n") + "\n=== END STATIC ANALYSIS ===\n"
}
```

#### DetectSADir (func)

```go
func DetectSADir(workDir string) string {
	sa := ".sa"
	if workDir != "" {
		sa = filepath.Join(workDir, ".sa")
	}
	info, err := os.Stat(sa)
	if err != nil || !info.IsDir() {
		logf("INFO", "Static analysis: .sa/ not present — skipping SA embedding")
		return ""
	}
	entries, _ := os.ReadDir(sa)
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			n++
		}
	}
	logf("INFO", "Static analysis: %d tool output(s) found in .sa/", n)
	return sa
}
```

#### StageFile (func)

```go
func StageFile(
	g *GitRunner,
	file, refspec, stagingDir, repoRoot, agent, saDir, status string,
) ([]Task, error) {
	fullPath := filepath.Join(repoRoot, filepath.FromSlash(file))
	if _, err := os.Stat(fullPath); err != nil {
		logf("WARN", "Skipping deleted/missing file: %s", file)
		return nil, nil
	}
	slug := FileSlug(file)
	fileDiff, err := g.diff(refspec, file)
	if err != nil {
		return nil, err
	}
	diffLines := strings.Count(fileDiff, "\n")
	fileContent := g.showHEAD(file)
	contentLines := strings.Count(fileContent, "\n")
	combinedLines := contentLines + diffLines

	saSection := ""
	if saDir != "" {
		saSection = CollectSAFindings(file, saDir)
	}
	guidance := ""
	if status == "A" {
		guidance = addedFileGuidance
	}

	tasks := make([]Task, 0, 1)
	if combinedLines <= MaxCombinedLines {
		logf("INFO", "  %s: full_and_diff (%d lines)", file, combinedLines)
		inputText := guidance + "=== CURRENT FILE (" + file + ") ===\n" + fileContent + "\n=== DIFF ===\n" + fileDiff + saSection
		inputFile := BuildStagingFilename(slug, "")
		if err := os.WriteFile(filepath.Join(stagingDir, inputFile), []byte(inputText), 0o644); err != nil {
			return nil, err
		}
		tasks = append(tasks, Task{
			"file": file, "slug": slug, "mode": ModeFullAndDiff,
			"chunk": nil, "total_chunks": nil, "input_file": inputFile,
			"agent": agent, "status": status,
		})
	} else if diffLines <= MaxDiffLines {
		logf("INFO", "  %s: diff_only (file %d lines, diff %d lines)", file, contentLines, diffLines)
		inputFile := BuildStagingFilename(slug, "")
		if err := os.WriteFile(filepath.Join(stagingDir, inputFile), []byte(guidance+fileDiff+saSection), 0o644); err != nil {
			return nil, err
		}
		tasks = append(tasks, Task{
			"file": file, "slug": slug, "mode": ModeDiffOnly,
			"chunk": nil, "total_chunks": nil, "input_file": inputFile,
			"agent": agent, "status": status,
		})
	} else {
		chunks := ChunkLines(fileDiff, MaxDiffLines)
		total := len(chunks)
		logf("INFO", "  %s: diff_chunk — %d chunks (file %d lines, diff %d lines)", file, total, contentLines, diffLines)
		for idx, chunkText := range chunks {
			i := idx + 1
			inputFile := BuildStagingFilename(slug, fmt.Sprintf("-chunk%03d", i))
			chunkSA := ""
			if i == total {
				chunkSA = saSection
			}
			chunkGuidance := ""
			if i == 1 {
				chunkGuidance = guidance
			}
			if err := os.WriteFile(filepath.Join(stagingDir, inputFile), []byte(chunkGuidance+chunkText+chunkSA), 0o644); err != nil {
				return nil, err
			}
			tasks = append(tasks, Task{
				"file": file, "slug": slug, "mode": ModeDiffChunk,
				"chunk": i, "total_chunks": total, "input_file": inputFile,
				"agent": agent, "status": status,
			})
		}
	}
	return tasks, nil
}
```

#### ErrFatal (error)

```go
type ErrFatal struct {
	Msg string
}
```

#### ErrFatal.Error (method)

```go
func (e *ErrFatal) Error() string { return e.Msg }
```

#### Options (type)

```go
type Options struct {
	BaseBranch        string
	StagingDir        string
	RoutingPath       string
	AgentContextPath  string
	SummaryConfigPath string
	RepoRoot          string // empty = cwd
	BatchSize         int
	ContextDir        string // merged context-branch checkout (agenting packs); optional
}
```

### Private one-hop bodies

#### GitRunner.diff (method)

```go
func (g *GitRunner) diff(refspec, file string) (string, error) {
	return g.run("diff", refspec, "--", file)
}
```

#### GitRunner.run (method)

```go
func (g *GitRunner) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &GitError{Args: strings.Join(args, " "), Err: strings.TrimSpace(stderr.String())}
	}
	return stdout.String(), nil
}
```

#### GitRunner.runAllowFail (method)

```go
func (g *GitRunner) runAllowFail(args ...string) (string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), g.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return stdout.String(), ee.ExitCode()
		}
		return stdout.String(), 1
	}
	return stdout.String(), 0
}
```

#### GitRunner.showHEAD (method)

```go
func (g *GitRunner) showHEAD(file string) string {
	out, code := g.runAllowFail("show", "HEAD:"+filepath.ToSlash(file))
	if code != 0 {
		return ""
	}
	return out
}
```

#### batchSizeFromEnv (func)

```go
func batchSizeFromEnv() int {
	if v := os.Getenv("MAJORDOMO_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if v := os.Getenv("COPILOT_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 15
}
```

#### classifyFiles (func)

```go
func classifyFiles(allFiles []string, routing []RoutingRule, extra []*regexp.Regexp) (reviewable, excluded []string) {
	patternExcluded := []string{}
	unrouted := []string{}
	for _, file := range allFiles {
		if IsExcludedWithExtra(file, extra) {
			patternExcluded = append(patternExcluded, file)
		} else if ClassifyFile(file, routing) != "" {
			reviewable = append(reviewable, file)
		} else {
			unrouted = append(unrouted, file)
		}
	}
	excluded = append(patternExcluded, unrouted...)
	logf("INFO", "Changed files: %d", len(allFiles))
	logf("INFO", "Reviewable:    %d", len(reviewable))
	logf("INFO", "Excluded:      %d (%d unrouted, %d pattern-excluded)",
		len(excluded), len(unrouted), len(patternExcluded))
	return reviewable, excluded
}
```

#### copyFile (func)

```go
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
```

#### copyMap (func)

```go
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
```

#### decodeJSONObjectOrder (func)

```go
func decodeJSONObjectOrder(data []byte) ([]string, error) {
	dec := json.NewDecoder(bytesReader(data))
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return nil, fatalf("routing root must be an object")
	}
	keys := []string{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, fatalf("invalid routing key")
		}
		keys = append(keys, key)
		var skip any
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	_, _ = dec.Token() // closing }
	return keys, nil
}
```

#### fatalf (func)

```go
func fatalf(format string, args ...any) error {
	return &ErrFatal{Msg: fmt.Sprintf(format, args...)}
}
```

#### fmtBatchDir (func)

```go
func fmtBatchDir(n int) string {
	return fmt.Sprintf("batch_%03d", n)
}
```

#### loadScopedForm (func)

```go
func loadScopedForm(raw map[string]any, path string) (AgentContext, error) {
	globalCtx, globalOK := raw["global"].(map[string]any)
	if !globalOK || globalCtx == nil {
		if raw["global"] != nil {
			return AgentContext{}, fatalf("agentContext must define object values for 'global' and 'scoped'")
		}
		globalCtx = map[string]any{}
	}
	scopedCtx, scopedOK := raw["scoped"].(map[string]any)
	if !scopedOK || scopedCtx == nil {
		if raw["scoped"] != nil {
			return AgentContext{}, fatalf("agentContext must define object values for 'global' and 'scoped'")
		}
		scopedCtx = map[string]any{}
	}
	logf("INFO", "Loaded scoped agent context: %s", path)
	return AgentContext{Global: globalCtx, Scoped: scopedCtx}, nil
}
```

#### logf (func)

```go
func logf(level, format string, args ...any) {
	ts := time.Now().UTC().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%s] %s\n", ts, level, fmt.Sprintf(format, args...))
}
```

#### manifestChangedFiles (func)

```go
func manifestChangedFiles(manifestPath string) ([]string, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", manifestPath, err)
	}
	var raw struct {
		ReviewAgents map[string][]string `json:"review_agents"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode manifest %s: %w", manifestPath, err)
	}
	seen := map[string]struct{}{}
	var files []string
	for _, list := range raw.ReviewAgents {
		for _, f := range list {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			files = append(files, f)
		}
	}
	return files, nil
}
```

#### matchStarSlash (func)

```go
func matchStarSlash(pattern, name string) bool {
	// Recursive backtracking matcher: * matches any string including /
	var match func(p, n int) bool
	match = func(p, n int) bool {
		for p < len(pattern) {
			if pattern[p] == '*' {
				for ; p < len(pattern) && pattern[p] == '*'; p++ {
				}
				if p == len(pattern) {
					return true
				}
				for i := n; i <= len(name); i++ {
					if match(p, i) {
						return true
					}
				}
				return false
			}
			if n >= len(name) || pattern[p] != name[n] {
				// handle ?
				if pattern[p] == '?' && n < len(name) {
					p++
					n++
					continue
				}
				return false
			}
			p++
			n++
		}
		return n == len(name)
	}
	return match(0, 0)
}
```

#### patchManifestGrounding (func)

```go
func patchManifestGrounding(manifestPath string, packs []agenting.StagedPack) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	entries := make([]map[string]string, 0, len(packs))
	for _, p := range packs {
		entries = append(entries, map[string]string{"id": p.ID, "file": p.Filename})
	}
	raw["grounding_packs"] = entries
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(manifestPath, out, 0o644)
}
```

#### requiredTaskString (func)

```go
func requiredTaskString(task Task, key string) (string, error) {
	value, ok := task[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("staging task requires a non-empty string %q", key)
	}
	return value, nil
}
```

#### resolveContextRules (func)

```go
func resolveContextRules(context map[string]any, repoRoot, label string) (map[string]any, error) {
	resolved := copyMap(context)
	rawRules, exists := resolved["customRules"]
	if !exists || rawRules == nil {
		resolved["customRules"] = []any{}
		return resolved, nil
	}
	list, ok := rawRules.([]any)
	if !ok {
		return nil, fatalf("%s customRules must be a list", label)
	}
	rules, err := resolveRules(list, repoRoot, label)
	if err != nil {
		return nil, err
	}
	asAny := make([]any, len(rules))
	for i, r := range rules {
		asAny[i] = r
	}
	resolved["customRules"] = asAny
	return resolved, nil
}
```

#### sortStrings (func)

```go
func sortStrings(s []string) {
	sort.Strings(s)
}
```

#### stageReviewableFiles (func)

```go
func stageReviewableFiles(
	g *GitRunner,
	reviewable, excluded []string,
	routing []RoutingRule,
	refspec, stagingDir, repoRoot, saDir, baseBranch string,
	fileStatus map[string]string,
	agentContext AgentContext,
	personas map[string]string,
) ([]Task, map[string][]string, []string, error) {
	excluded = append([]string{}, excluded...)
	tasks := []Task{}
	reviewAgents := map[string][]string{}
	skipped := []string{}
	logf("INFO", "Included:")
	for _, file := range reviewable {
		full := filepath.Join(repoRoot, filepath.FromSlash(file))
		if _, err := os.Stat(full); err != nil {
			skipped = append(skipped, file)
			continue
		}
		agent := ClassifyFile(file, routing)
		if agent == "" {
			excluded = append(excluded, file)
			continue
		}
		reviewAgents[agent] = append(reviewAgents[agent], file)
		fileContext, err := ContextForFile(file, agentContext, repoRoot)
		if err != nil {
			return nil, nil, nil, err
		}
		status := fileStatus[file]
		if status == "" {
			status = "M"
		}
		staged, err := StageFile(g, file, refspec, stagingDir, repoRoot, agent, saDir, status)
		if err != nil {
			if _, ok := err.(*GitError); ok {
				logf("WARN", "  git error staging %s: %v", file, err)
				continue
			}
			return nil, nil, nil, err
		}
		for _, t := range staged {
			t["agent_context"] = fileContext
			if p, ok := personas[agent]; ok {
				t["persona"] = p
			}
			tasks = append(tasks, t)
		}
	}
	if len(skipped) > 0 {
		logf("INFO", "Skipped (deleted):  %d", len(skipped))
		for _, f := range skipped {
			logf("WARN", "  %s: deleted — skipped", f)
		}
	}
	logf("INFO", "Agent routing:")
	for agent, files := range reviewAgents {
		logf("INFO", "  %s: %d file(s)", agent, len(files))
	}
	manifest := map[string]any{
		"base_branch":   baseBranch,
		"refspec":       refspec,
		"review_agents": reviewAgents,
		"reviewable":    tasks,
		"excluded":      excluded,
	}
	if err := writeJSON(filepath.Join(stagingDir, "manifest.json"), manifest); err != nil {
		return nil, nil, nil, err
	}
	logf("INFO", "Staged %d review task(s) for %d file(s)", len(tasks), len(reviewable))
	logf("INFO", "Manifest: %s", filepath.Join(stagingDir, "manifest.json"))
	return tasks, reviewAgents, excluded, nil
}
```

#### tasksToMaps (func)

```go
func tasksToMaps(tasks []Task) []map[string]any {
	out := make([]map[string]any, len(tasks))
	for i, t := range tasks {
		out[i] = map[string]any(t)
	}
	return out
}
```

#### trimSpace (func)

```go
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}
```

#### truncateUTF8 (func)

```go
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
```

#### writeJSON (func)

```go
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
```


## ./internal/status
- package: `status`
- packageDoc: Package status posts commit/build status to GitHub and Bitbucket Server.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: true
- importsOsExec: false
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: Options, State, StateFailed, StateInProgress, StateSuccessful
- exportedFuncs: Run
- exportedMethods: (none)
- unexportedDecls: (none)
- unexportedFuncs: doJSON, first, logf, min, postBitbucket, postGitHub
- unexportedMethods: Options.client, Options.fillFromEnv
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/status/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/status/status.go

### Exported bodies

#### State (type)

```go
type State string
```

#### Options (type)

```go
type Options struct {
	SCM       string // github | bitbucket
	CommitSHA string
	State     State
	// Bitbucket
	BBBaseURL   string
	BBProject   string
	BBRepo      string
	BBToken     string
	BuildURL    string
	BuildKey    string
	BuildName   string
	Description string
	BuildNumber string
	BuildRef    string
	// GitHub
	GitHubToken string
	GitHubOwner string
	GitHubRepo  string
	Context     string // GitHub status context
	HTTPClient  *http.Client
}
```

#### Run (func)

```go
func Run(opts Options) error {
	opts.State = State(strings.ToUpper(string(opts.State)))
	switch opts.State {
	case StateInProgress, StateSuccessful, StateFailed:
	default:
		return fmt.Errorf("state must be INPROGRESS|SUCCESSFUL|FAILED, got %q", opts.State)
	}
	if opts.CommitSHA == "" {
		return fmt.Errorf("commit SHA required")
	}
	opts.fillFromEnv()
	switch strings.ToLower(opts.SCM) {
	case "github":
		return postGitHub(opts)
	case "bitbucket":
		return postBitbucket(opts)
	default:
		return fmt.Errorf("unsupported scm %q (github|bitbucket)", opts.SCM)
	}
}
```

### Private one-hop bodies

#### Options.fillFromEnv (method)

```go
func (o *Options) fillFromEnv() {
	if o.BBBaseURL == "" {
		o.BBBaseURL = first(os.Getenv("BB_BASE_URL"), os.Getenv("BITBUCKET_URL"))
	}
	if o.BBProject == "" {
		o.BBProject = first(os.Getenv("BB_PROJECT_KEY"), os.Getenv("BB_PROJECT"))
	}
	if o.BBRepo == "" {
		o.BBRepo = first(os.Getenv("BB_REPO_SLUG"), os.Getenv("BB_REPO"))
	}
	if o.BBToken == "" {
		o.BBToken = os.Getenv("BITBUCKET_TOKEN")
	}
	if o.BuildURL == "" {
		o.BuildURL = os.Getenv("BUILD_URL")
	}
	if o.BuildKey == "" {
		o.BuildKey = os.Getenv("BB_BUILD_KEY")
	}
	if o.BuildName == "" {
		o.BuildName = os.Getenv("BB_BUILD_NAME")
	}
	if o.Description == "" {
		o.Description = os.Getenv("BB_BUILD_DESCRIPTION")
	}
	if o.BuildNumber == "" {
		o.BuildNumber = os.Getenv("BB_BUILD_NUMBER")
	}
	if o.BuildRef == "" {
		o.BuildRef = os.Getenv("BB_BUILD_REF")
	}
	if o.GitHubToken == "" {
		o.GitHubToken = first(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))
	}
	if o.GitHubOwner == "" || o.GitHubRepo == "" {
		if repo := os.Getenv("GITHUB_REPOSITORY"); strings.Contains(repo, "/") {
			parts := strings.SplitN(repo, "/", 2)
			if o.GitHubOwner == "" {
				o.GitHubOwner = parts[0]
			}
			if o.GitHubRepo == "" {
				o.GitHubRepo = parts[1]
			}
		}
	}
	if o.Context == "" {
		o.Context = first(os.Getenv("GITHUB_STATUS_CONTEXT"), "majordomo")
	}
}
```

#### postBitbucket (func)

```go
func postBitbucket(opts Options) error {
	if opts.BBBaseURL == "" || opts.BBProject == "" || opts.BBRepo == "" || opts.BBToken == "" {
		return fmt.Errorf("bitbucket status requires BB_BASE_URL, BB_PROJECT_KEY, BB_REPO_SLUG, BITBUCKET_TOKEN")
	}
	if opts.BuildURL == "" || opts.BuildKey == "" || opts.BuildName == "" || opts.Description == "" {
		return fmt.Errorf("bitbucket status requires BUILD_URL, BB_BUILD_KEY, BB_BUILD_NAME, BB_BUILD_DESCRIPTION")
	}
	parent := opts.BuildKey
	key := opts.BuildKey
	if opts.BuildNumber != "" {
		key = opts.BuildKey + "#" + opts.BuildNumber
	}
	url := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/commits/%s/builds",
		strings.TrimRight(opts.BBBaseURL, "/"), opts.BBProject, opts.BBRepo, opts.CommitSHA)
	payload := map[string]string{
		"state":       string(opts.State),
		"key":         key,
		"parent":      parent,
		"name":        opts.BuildName,
		"url":         opts.BuildURL,
		"description": opts.Description,
	}
	if opts.BuildNumber != "" {
		payload["buildNumber"] = opts.BuildNumber
	}
	if opts.BuildRef != "" {
		payload["ref"] = opts.BuildRef
	}
	logf("INFO", "========== Bitbucket build status %s @ %s ==========", opts.State, opts.CommitSHA[:min(7, len(opts.CommitSHA))])
	return doJSON(opts.client(), "POST", url, opts.BBToken, payload)
}
```

#### postGitHub (func)

```go
func postGitHub(opts Options) error {
	if opts.GitHubToken == "" || opts.GitHubOwner == "" || opts.GitHubRepo == "" {
		return fmt.Errorf("github status requires GITHUB_TOKEN and GITHUB_REPOSITORY")
	}
	ghState := "pending"
	switch opts.State {
	case StateSuccessful:
		ghState = "success"
	case StateFailed:
		ghState = "failure"
	case StateInProgress:
		ghState = "pending"
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/statuses/%s", opts.GitHubOwner, opts.GitHubRepo, opts.CommitSHA)
	payload := map[string]string{
		"state":       ghState,
		"context":     opts.Context,
		"description": first(opts.Description, string(opts.State)),
	}
	if opts.BuildURL != "" {
		payload["target_url"] = opts.BuildURL
	}
	logf("INFO", "========== GitHub status %s @ %s ==========", ghState, opts.CommitSHA[:min(7, len(opts.CommitSHA))])
	return doJSON(opts.client(), "POST", url, opts.GitHubToken, payload)
}
```


## ./internal/submodule
- package: `submodule`
- packageDoc: Package submodule ports scripts/submodule.py: interactive .majordomo manager.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: exec_runner
- mechanicalConfidence: 0.80
- mechanicalEvidence: imports_os_exec, exports_run_surface
- exportedDecls: Options
- exportedFuncs: Run
- exportedMethods: (none)
- unexportedDecls: manager, pipelinesBranch, worktreeDir
- unexportedFuncs: (none)
- unexportedMethods: manager.buildOpsMenu, manager.cmdPinCommit, manager.cmdSwitchBranch, manager.cmdUpdate, manager.cmdUpdateViaWorktree, manager.confirmAndReset, manager.currentBranch, manager.currentSHA, manager.findParentRepoRoot, manager.findSubmoduleRoot, manager.getSubmoduleName, manager.git, manager.gitDir, manager.isDirty, manager.isGitlinkInIndex, manager.opsMenuLoop, manager.out, manager.printf, manager.prompt, manager.promptOffBranchContext, manager.pullWithRecovery, manager.pushToOrigin, manager.readKey, manager.remoteBranches, manager.remoteTrackingSHA, manager.resetWorkingTree, manager.selectBranch
- errorTypes: (none)
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/submodule/commands.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/submodule/git.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/submodule/menu.go

### Exported bodies

#### Options (type)

```go
type Options struct {
	// StartDir is used to locate the submodule root (default: cwd).
	StartDir string
	// In/Out for prompts (tests).
	In  io.Reader
	Out io.Writer
	// GitRunner overrides git execution (tests). nil → real git.
	GitRunner func(args []string, cwd string, check bool) (string, error)
	// Prompt reads a line after printing prompt (tests). nil → stdin.
	Prompt func(prompt string) (string, error)
	// ReadKey reads a single choice character (tests). nil → line-based fallback.
	ReadKey func(prompt string) (string, error)
}
```

#### Run (func)

```go
func Run(opts Options) error {
	m := &manager{opts: opts}
	root, err := m.findSubmoduleRoot()
	if err != nil {
		return err
	}
	m.submoduleRoot = root
	m.parentRoot = m.findParentRepoRoot(root)
	m.submoduleName = m.getSubmoduleName()

	if m.parentRoot != "" {
		parentBranch, err := m.currentBranch(m.parentRoot)
		if err != nil {
			return err
		}
		if parentBranch != pipelinesBranch {
			return m.promptOffBranchContext(parentBranch)
		}
	}
	return m.opsMenuLoop()
}
```

### Private one-hop bodies

#### manager (type)

```go
type manager struct {
	opts          Options
	submoduleRoot string
	parentRoot    string // empty if none
	submoduleName string
}
```

#### manager.currentBranch (method)

```go
func (m *manager) currentBranch(repoRoot string) (string, error) {
	out, err := m.git([]string{"symbolic-ref", "--short", "HEAD"}, repoRoot, true)
	if err != nil {
		return "(detached HEAD)", nil
	}
	return out, nil
}
```

#### manager.findParentRepoRoot (method)

```go
func (m *manager) findParentRepoRoot(submoduleRoot string) string {
	parentCand := filepath.Dir(submoduleRoot)
	out, err := m.git([]string{"rev-parse", "--show-toplevel"}, parentCand, true)
	if err != nil {
		return ""
	}
	parent := out
	if parent == submoduleRoot {
		return ""
	}
	rel, err := filepath.Rel(parent, submoduleRoot)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	indexEntry, _ := m.git([]string{"ls-files", "--stage", rel}, parent, false)
	if strings.HasPrefix(indexEntry, "160000") {
		return parent
	}
	gitDirRaw, _ := m.git([]string{"rev-parse", "--git-dir"}, parent, false)
	if gitDirRaw == "" {
		return ""
	}
	gitDir := gitDirRaw
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(parent, gitDirRaw)
	}
	if st, err := os.Stat(filepath.Join(gitDir, "modules", rel)); err == nil && st.IsDir() {
		return parent
	}
	return ""
}
```

#### manager.findSubmoduleRoot (method)

```go
func (m *manager) findSubmoduleRoot() (string, error) {
	start := m.opts.StartDir
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	out, err := m.git([]string{"rev-parse", "--show-toplevel"}, start, true)
	if err != nil {
		return "", fmt.Errorf("could not determine submodule root — not inside a git repo")
	}
	return out, nil
}
```

#### manager.getSubmoduleName (method)

```go
func (m *manager) getSubmoduleName() string {
	if m.parentRoot == "" {
		return filepath.Base(m.submoduleRoot)
	}
	rel, err := filepath.Rel(m.parentRoot, m.submoduleRoot)
	if err != nil {
		return filepath.Base(m.submoduleRoot)
	}
	return filepath.ToSlash(rel)
}
```

#### manager.git (method)

```go
func (m *manager) git(args []string, cwd string, check bool) (string, error) {
	if m.opts.GitRunner != nil {
		return m.opts.GitRunner(args, cwd, check)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		if !check {
			return out, nil
		}
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
```

#### manager.opsMenuLoop (method)

```go
func (m *manager) opsMenuLoop() error {
	for {
		currentBranch, err := m.currentBranch(m.submoduleRoot)
		if err != nil {
			return err
		}
		m.printf("\n%s\n", m.buildOpsMenu(currentBranch))
		choice, err := m.readKey("Choice: ")
		if err != nil {
			return err
		}
		var changed bool
		switch choice {
		case "q":
			return nil
		case "1":
			changed, err = m.cmdUpdate()
		case "2":
			changed, err = m.cmdSwitchBranch()
		case "3":
			changed, err = m.cmdPinCommit()
		default:
			m.printf("Invalid choice.\n")
			continue
		}
		if err != nil {
			return err
		}
		if changed && m.parentRoot != "" {
			raw, err := m.prompt("\nPush to origin and exit? (y/N): ")
			if err != nil {
				return err
			}
			if strings.ToLower(strings.TrimSpace(raw)) == "y" {
				return m.pushToOrigin()
			}
		}
	}
}
```

#### manager.prompt (method)

```go
func (m *manager) prompt(p string) (string, error) {
	if m.opts.Prompt != nil {
		return m.opts.Prompt(p)
	}
	m.printf("%s", p)
	in := m.opts.In
	if in == nil {
		in = os.Stdin
	}
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", nil
	}
	return scanner.Text(), nil
}
```

#### manager.promptOffBranchContext (method)

```go
func (m *manager) promptOffBranchContext(currentParentBranch string) error {
	warning := fmt.Sprintf(
		"⚠️  OFF-BRANCH WARNING  ⚠️\nParent repo is on '%s', not '%s'.\nAny direct commits will land on '%s'.",
		currentParentBranch, pipelinesBranch, currentParentBranch,
	)
	m.printf("\n%s\n\n", warning)
	m.printf("1. 🔒 Safe  — update '%s' via isolated worktree\n", pipelinesBranch)
	m.printf("2. ⚡ Direct — I know what I'm doing (operate on '%s')\n", currentParentBranch)
	m.printf("q. Quit\n")
	for {
		choice, err := m.readKey("\nContext: ")
		if err != nil {
			return err
		}
		switch choice {
		case "q":
			return nil
		case "1":
			_, err := m.cmdUpdateViaWorktree()
			return err
		case "2":
			return m.opsMenuLoop()
		default:
			m.printf("Invalid choice — enter 1, 2, or q.\n")
		}
	}
}
```


## ./internal/workspace
- package: `workspace`
- packageDoc: Package workspace is the cwd-bounded port for explore/edit tools.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: Allow, AllowDigest, AllowNone, AllowTechDeep, ErrDenied, ErrEscape, Local, Match, Port, Stub, Tool, ToolEdit, ToolGrep, ToolRead, ToolShell
- exportedFuncs: Guard, NewLocal, NewStub
- exportedMethods: Local.Edit, Local.Grep, Local.Read, Local.Shell, Stub.Edit, Stub.Grep, Stub.Read, Stub.Shell, Tool.String, guarded.Edit, guarded.Grep, guarded.Read, guarded.Shell
- unexportedDecls: guarded
- unexportedFuncs: grepFile, resolvePath
- unexportedMethods: Stub.key, guarded.check
- errorTypes: ErrDenied, ErrEscape
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/workspace/doc.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/workspace/local.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/workspace/stub.go, ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/workspace/workspace.go

### Exported bodies

#### Local (type)

```go
type Local struct {
	Root string
}
```

#### NewLocal (func)

```go
func NewLocal(root string) (*Local, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("workspace: root %q is not a directory", abs)
	}
	return &Local{Root: abs}, nil
}
```

#### Local.Read (method)

```go
func (l *Local) Read(_ context.Context, path string) ([]byte, error) {
	full, err := resolvePath(l.Root, path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("workspace read %q: %w", path, err)
	}
	return data, nil
}
```

#### Local.Grep (method)

```go
func (l *Local) Grep(_ context.Context, pattern, path string) ([]Match, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("workspace grep: %w", err)
	}
	full, err := resolvePath(l.Root, path)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("workspace grep %q: %w", path, err)
	}
	var out []Match
	if fi.IsDir() {
		err = filepath.WalkDir(full, func(p string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(l.Root, p)
			if err != nil {
				return err
			}
			ms, err := grepFile(re, rel, p)
			if err != nil {
				return err
			}
			out = append(out, ms...)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("workspace grep %q: %w", path, err)
		}
		return out, nil
	}
	rel, err := filepath.Rel(l.Root, full)
	if err != nil {
		return nil, err
	}
	return grepFile(re, rel, full)
}
```

#### Local.Edit (method)

```go
func (l *Local) Edit(_ context.Context, path string, content []byte) error {
	full, err := resolvePath(l.Root, path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("workspace edit %q: %w", path, err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return fmt.Errorf("workspace edit %q: %w", path, err)
	}
	return nil
}
```

#### Local.Shell (method)

```go
func (l *Local) Shell(ctx context.Context, argv []string) (stdout, stderr []byte, err error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("workspace shell: empty argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = l.Root
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	return outBuf.Bytes(), errBuf.Bytes(), runErr
}
```

#### Stub (type)

```go
type Stub struct {
	mu    sync.Mutex
	Files map[string][]byte
	// ShellResult is returned from Shell when set; otherwise Shell errors.
	ShellStdout []byte
	ShellStderr []byte
	ShellErr    error
}
```

#### NewStub (func)

```go
func NewStub() *Stub {
	return &Stub{Files: map[string][]byte{}}
}
```

#### Stub.Read (method)

```go
func (s *Stub) Read(_ context.Context, p string) ([]byte, error) {
	k, err := s.key(p)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.Files[k]
	if !ok {
		return nil, fmt.Errorf("workspace read %q: not found", p)
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}
```

#### Stub.Grep (method)

```go
func (s *Stub) Grep(_ context.Context, pattern, p string) ([]Match, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("workspace grep: %w", err)
	}
	prefix, err := s.key(p)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Match
	for name, data := range s.Files {
		if name != prefix && !strings.HasPrefix(name, prefix+"/") {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				out = append(out, Match{Path: name, Line: i + 1, Content: line})
			}
		}
	}
	return out, nil
}
```

#### Stub.Edit (method)

```go
func (s *Stub) Edit(_ context.Context, p string, content []byte) error {
	k, err := s.key(p)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(content))
	copy(cp, content)
	s.Files[k] = cp
	return nil
}
```

#### Stub.Shell (method)

```go
func (s *Stub) Shell(_ context.Context, argv []string) ([]byte, []byte, error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("workspace shell: empty argv")
	}
	if s.ShellErr != nil || s.ShellStdout != nil || s.ShellStderr != nil {
		return s.ShellStdout, s.ShellStderr, s.ShellErr
	}
	return nil, nil, fmt.Errorf("workspace stub: Shell not configured")
}
```

#### Tool (type)

```go
type Tool int
```

#### Tool.String (method)

```go
func (t Tool) String() string {
	switch t {
	case ToolRead:
		return "Read"
	case ToolGrep:
		return "Grep"
	case ToolEdit:
		return "Edit"
	case ToolShell:
		return "Shell"
	default:
		return fmt.Sprintf("Tool(%d)", int(t))
	}
}
```

#### Match (type)

```go
type Match struct {
	Path    string
	Line    int
	Content string
}
```

#### Port (type)

```go
type Port interface {
	Read(ctx context.Context, path string) ([]byte, error)
	Grep(ctx context.Context, pattern, path string) ([]Match, error)
	Edit(ctx context.Context, path string, content []byte) error
	Shell(ctx context.Context, argv []string) (stdout, stderr []byte, err error)
}
```

#### Allow (type)

```go
type Allow map[Tool]bool
```

#### Guard (func)

```go
func Guard(inner Port, allow Allow) Port {
	return &guarded{inner: inner, allow: allow}
}
```

#### guarded.Read (method)

```go
func (g *guarded) Read(ctx context.Context, path string) ([]byte, error) {
	if err := g.check(ToolRead); err != nil {
		return nil, err
	}
	return g.inner.Read(ctx, path)
}
```

#### guarded.Grep (method)

```go
func (g *guarded) Grep(ctx context.Context, pattern, path string) ([]Match, error) {
	if err := g.check(ToolGrep); err != nil {
		return nil, err
	}
	return g.inner.Grep(ctx, pattern, path)
}
```

#### guarded.Edit (method)

```go
func (g *guarded) Edit(ctx context.Context, path string, content []byte) error {
	if err := g.check(ToolEdit); err != nil {
		return err
	}
	return g.inner.Edit(ctx, path, content)
}
```

#### guarded.Shell (method)

```go
func (g *guarded) Shell(ctx context.Context, argv []string) ([]byte, []byte, error) {
	if err := g.check(ToolShell); err != nil {
		return nil, nil, err
	}
	return g.inner.Shell(ctx, argv)
}
```

### Private one-hop bodies

#### Stub.key (method)

```go
func (s *Stub) key(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("workspace: empty path")
	}
	if path.IsAbs(p) || strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%w: absolute path %q", ErrEscape, p)
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: %q", ErrEscape, p)
	}
	return clean, nil
}
```

#### grepFile (func)

```go
func grepFile(re *regexp.Regexp, rel, full string) ([]Match, error) {
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Match
	sc := bufio.NewScanner(f)
	// Allow long lines in source files.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if re.MatchString(text) {
			out = append(out, Match{Path: filepath.ToSlash(rel), Line: line, Content: text})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
```

#### guarded (type)

```go
type guarded struct {
	inner Port
	allow Allow
}
```

#### guarded.check (method)

```go
func (g *guarded) check(t Tool) error {
	if g.allow[t] {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrDenied, t)
}
```

#### resolvePath (func)

```go
func resolvePath(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("workspace: empty path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: absolute path %q", ErrEscape, rel)
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrEscape, rel)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	full := filepath.Join(absRoot, cleanRel)
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	relOut, err := filepath.Rel(absRoot, absFull)
	if err != nil {
		return "", err
	}
	if relOut == ".." || strings.HasPrefix(relOut, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrEscape, rel)
	}
	return absFull, nil
}
```


## ./internal/workspace/opencode
- package: `opencode`
- packageDoc: Package opencode is the OpenCode CLI adapter for workspace.Port.
- hasMain: false
- jsonTags: false
- goEmbed: false
- embedsStatic: false
- importsNetHTTP: false
- importsOsExec: true
- importsGrpc: false
- importsOtel: false
- importsPrometheus: false
- mechanicalRole: unknown
- mechanicalConfidence: 0.00
- exportedDecls: Adapter, ErrNotImplemented
- exportedFuncs: New
- exportedMethods: Adapter.ChildEnv, Adapter.Edit, Adapter.Grep, Adapter.Read, Adapter.Shell
- unexportedDecls: _
- unexportedFuncs: (none)
- unexportedMethods: (none)
- errorTypes: ErrNotImplemented
- files: ../../../../../../../var/folders/6h/b51xqt3568q2p96564n8qzh40000gn/T/majordomo-typology-4109167456/internal/workspace/opencode/adapter.go

### Exported bodies

#### Adapter (type)

```go
type Adapter struct {
	Local *workspace.Local
	Bin   string // open code binary name; default "opencode" when wired
}
```

#### New (func)

```go
func New(root, bin string) (*Adapter, error) {
	loc, err := workspace.NewLocal(root)
	if err != nil {
		return nil, err
	}
	if bin == "" {
		bin = "opencode"
	}
	return &Adapter{Local: loc, Bin: bin}, nil
}
```

#### Adapter.Read (method)

```go
func (a *Adapter) Read(ctx context.Context, path string) ([]byte, error) {
	return a.Local.Read(ctx, path)
}
```

#### Adapter.Grep (method)

```go
func (a *Adapter) Grep(ctx context.Context, pattern, path string) ([]workspace.Match, error) {
	return a.Local.Grep(ctx, pattern, path)
}
```

#### Adapter.Edit (method)

```go
func (a *Adapter) Edit(ctx context.Context, path string, content []byte) error {
	return a.Local.Edit(ctx, path, content)
}
```

#### Adapter.ChildEnv (method)

```go
func (a *Adapter) ChildEnv(parent []string) ([]string, error) {
	if parent == nil {
		parent = os.Environ()
	}
	return aigateway.PrepareChildEnv(parent)
}
```

#### Adapter.Shell (method)

```go
func (a *Adapter) Shell(ctx context.Context, argv []string) (stdout, stderr []byte, err error) {
	if a == nil {
		return nil, nil, fmt.Errorf("workspace/opencode: nil adapter")
	}
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("workspace/opencode: empty argv")
	}
	env, err := a.ChildEnv(os.Environ())
	if err != nil {
		return nil, nil, fmt.Errorf("workspace/opencode: child env: %w", err)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	if a.Local != nil {
		cmd.Dir = a.Local.Root
	}
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return out, nil, fmt.Errorf("workspace/opencode shell: %w", runErr)
	}
	return out, nil, nil
}
```



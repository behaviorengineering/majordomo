package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/majordomo/internal/llmusage"
	"github.com/behaviorengineering/majordomo/internal/observability"
	stropdspy "github.com/behaviorengineering/strop/dspy"
	"github.com/behaviorengineering/strop/dspy/factory"
	typroles "github.com/behaviorengineering/typology/roles"
	"gopkg.in/yaml.v3"
)

var (
	clusterAuditVerdictRE = regexp.MustCompile(`(?im)^\s*verdict\s*[:=]\s*(accept|overlay|reject)\b`)
	clusterAuditIDRE      = regexp.MustCompile(`(?im)^\s*id\s*[:=]\s*(\S+)`)
)

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

type clusterAuditResult struct {
	Verdicts      []clusterMergeVerdict
	RLMIterations int
	Duration      time.Duration
	PromptTokens  int
	CompletionTok int
	TotalTokens   int
	TraceDir      string
}

type clusterMergeAuditor interface {
	Audit(ctx context.Context, req clusterAuditRequest) (clusterAuditResult, error)
}

type stropClusterMergeAuditor struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
	}
	traceDir string
}

func newStropClusterMergeAuditor(ctx context.Context, cfg config.RepoConfig, traceDir string) (clusterMergeAuditor, error) {
	provider, ok, err := cfg.ResolveTaskProvider(jmodules.TaskTypologyClusterAudit)
	if err != nil || !ok {
		provider, ok, err = cfg.ResolveTaskProvider(jmodules.TaskTypologyInspect)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("typology_cluster_audit provider not configured")
		}
	}
	stropProvider := provider.ToStrop()
	llmFactory := factory.NewLLMFactory(nil, ledgerRLMTimeout)
	llmFactory.SetInstrumentHTTP(observability.InstrumentHTTPClient)
	llm, err := llmFactory.CreateLLM(ctx, stropProvider)
	if err != nil {
		return nil, fmt.Errorf("typology_cluster_audit RLM LLM: %w", err)
	}
	llm = judge.WrapLLMWithRetry(llm, judge.DefaultModuleRetryConfig())
	rlmCfg := stropdspy.RLMDefaults()
	rlmCfg.MaxFullContextQueryChars = 24_000
	timeout := provider.GetTimeout(ledgerRLMTimeout)
	if timeout < ledgerRLMTimeout {
		timeout = ledgerRLMTimeout
	}
	rlmCfg.Timeout = timeout
	rlmCfg.TraceDir = strings.TrimSpace(traceDir)
	module, err := stropdspy.CreateRLMModule(llm, rlmCfg)
	if err != nil {
		return nil, err
	}
	return stropClusterMergeAuditor{
		traceDir: rlmCfg.TraceDir,
		module: rlmCompleteAdapter{complete: func(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
			answer, result, err := stropdspy.RLMComplete(ctx, module, contextPayload, query)
			if err != nil {
				return "", 0, 0, 0, 0, err
			}
			iters := 0
			prompt, completion, total := 0, 0, 0
			if result != nil {
				iters = result.Iterations
				prompt = result.Usage.PromptTokens
				completion = result.Usage.CompletionTokens
				total = result.Usage.TotalTokens
				llmusage.FromContext(ctx).AddTokenUsageValue(jmodules.TaskTypologyClusterAudit, result.Usage)
			} else {
				llmusage.FromContext(ctx).Add(jmodules.TaskTypologyClusterAudit, 0, 0, 0)
			}
			return answer, iters, prompt, completion, total, nil
		}},
	}, nil
}

func (v stropClusterMergeAuditor) Audit(ctx context.Context, req clusterAuditRequest) (clusterAuditResult, error) {
	return runClusterMergeAudit(ctx, v, req)
}

func (v stropClusterMergeAuditor) Complete(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	return v.module.Complete(ctx, contextPayload, query)
}

type clusterAuditCaller interface {
	Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
}

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
			out.Verdicts = append(append([]clusterMergeVerdict(nil), req.Frozen...), fromCachedVerdicts(hit.Merges)...)
			out.RLMIterations = hit.RLMIterations
			out.PromptTokens = hit.PromptTokens
			out.CompletionTok = hit.CompletionTokens
			out.TotalTokens = hit.TotalTokens
			return out, nil
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
		return out, fmt.Errorf("typology_cluster_audit RLM: %w", err)
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
	logf("INFO", "typology_cluster_audit attempt=%d rows=%d iterations=%d duration_ms=%d tokens=%d",
		req.Attempt, len(req.Proposed), iters, out.Duration.Milliseconds(), totalTok)
	return out, nil
}

func (v stropClusterMergeAuditor) TraceDir() string { return v.traceDir }

func toCachedVerdicts(in []clusterMergeVerdict) []cache.ClusterAuditCachedMerge {
	out := make([]cache.ClusterAuditCachedMerge, 0, len(in))
	for _, v := range in {
		out = append(out, cache.ClusterAuditCachedMerge{
			ID:       v.ID,
			Packages: append([]string(nil), v.Packages...),
			Verdict:  v.Verdict,
			Reason:   v.Reason,
			Evidence: append([]string(nil), v.Evidence...),
		})
	}
	return out
}

func fromCachedVerdicts(in []cache.ClusterAuditCachedMerge) []clusterMergeVerdict {
	out := make([]clusterMergeVerdict, 0, len(in))
	for _, v := range in {
		out = append(out, clusterMergeVerdict{
			ID:       v.ID,
			Packages: append([]string(nil), v.Packages...),
			Verdict:  v.Verdict,
			Reason:   v.Reason,
			Evidence: append([]string(nil), v.Evidence...),
		})
	}
	return out
}

func formatProposedMergesForAudit(merges []proposedMerge) string {
	var b strings.Builder
	for _, m := range merges {
		fmt.Fprintf(&b, "- id=%s intent=%s packages=[%s]\n", m.ID, m.Intent, strings.Join(m.Packages, ", "))
	}
	return b.String()
}

func buildClusterAuditContext(req clusterAuditRequest) string {
	var b strings.Builder
	b.WriteString("# Cluster merge audit context\n\n")
	b.WriteString("## Mechanical grouping\n\n")
	b.WriteString(strings.TrimSpace(req.MechanicalYAML))
	b.WriteString("\n\n## Package roles\n\n")
	b.WriteString(strings.TrimSpace(req.RolesYAML))
	b.WriteString("\n\n## Capability constraints\n\n")
	b.WriteString(strings.TrimSpace(req.Constraints))
	b.WriteString("\n\n## Package snippets\n\n")
	paths := make([]string, 0, 16)
	seen := map[string]struct{}{}
	for _, m := range req.Proposed {
		for _, p := range m.Packages {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	rlmContextPath := filepath.Join(req.EvidenceDir, "package_rlm_context.md")
	whole, _ := os.ReadFile(rlmContextPath)
	for _, p := range paths {
		snippet := packageRLMContextSnippet(string(whole), p)
		if strings.TrimSpace(snippet) == "" && req.AnalysisDir != "" {
			built, err := typroles.FormatPackageRLMContextForPath(req.AnalysisDir, p, nil)
			if err == nil {
				snippet = built
			}
		}
		if strings.TrimSpace(snippet) == "" {
			fmt.Fprintf(&b, "### %s\n\n(no RLM context)\n\n", p)
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", p, strings.TrimSpace(snippet))
	}
	return b.String()
}

func formatClusterAuditQuery(proposed []proposedMerge, frozen []clusterMergeVerdict) string {
	var b strings.Builder
	b.WriteString(`You audit Typology cluster merge proposals for Majordomo context digest.
You are a challenger, not a second clusterer. MUST NOT invent new merge ids or a better catalog map.

Inspection protocol (MUST follow in order):
1. Read the full proposed-merge list below. Do not invent rows.
2. For each row, pull that row's packages from context (roles, must_not, callers, mechanical private/shared).
3. Decide lifecycle-same vs theme-only.
4. Emit that row's verdict, why, and evidence quotes (package path + symbol).
5. Only then move to the next row.

Verdicts:
- accept: same lifecycle / same job; fold into one refined slice is earned. Evidence MUST quote every package path in the row; otherwise use overlay or reject.
- overlay: useful teaching nickname only; MUST NOT become catalog owns[]
- reject: drop the grouping

Frozen sticky verdicts (do not re-litigate):
`)
	if len(frozen) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, v := range frozen {
			fmt.Fprintf(&b, "- id=%s verdict=%s packages=[%s]\n", v.ID, v.Verdict, strings.Join(v.Packages, ", "))
		}
	}
	b.WriteString("\nProposed merges to score (every id required exactly once):\n")
	b.WriteString(formatProposedMergesForAudit(proposed))
	b.WriteString(`
Final answer MUST be a YAML list of objects (no markdown fences), one object per proposed id:
- id: <id>
  verdict: accept|overlay|reject
  reason: <one sentence>
  evidence:
    - <path + symbol quote>
`)
	return b.String()
}

func parseClusterAuditAnswer(answer string, proposed []proposedMerge) ([]clusterMergeVerdict, error) {
	body := strings.TrimSpace(stripCodeFence(answer))
	if looksLikeClusterAuditYAML(body) {
		rows, err := parseClusterAuditAnswerYAML(answer)
		if err != nil {
			return nil, err
		}
		return alignClusterAuditRows(rows, proposed)
	}
	if rows, err := parseClusterAuditAnswerYAML(answer); err == nil && len(rows) > 0 {
		return alignClusterAuditRows(rows, proposed)
	}
	return parseClusterAuditAnswerLines(answer, proposed)
}

func looksLikeClusterAuditYAML(body string) bool {
	trim := strings.TrimSpace(body)
	if trim == "" {
		return false
	}
	if strings.HasPrefix(trim, "merges:") || strings.HasPrefix(trim, "- id:") || strings.HasPrefix(trim, "-id:") {
		return true
	}
	return strings.Contains(trim, "\n- id:") || strings.Contains(trim, "\nmerges:")
}

func parseClusterAuditAnswerYAML(answer string) ([]clusterMergeVerdict, error) {
	body := strings.TrimSpace(stripCodeFence(answer))
	if body == "" {
		return nil, fmt.Errorf("empty cluster audit answer")
	}
	var rows []clusterMergeVerdict
	if err := yaml.Unmarshal([]byte(body), &rows); err != nil {
		// Also accept a document wrapper: merges: [...]
		var wrap struct {
			Merges []clusterMergeVerdict `yaml:"merges"`
		}
		if err2 := yaml.Unmarshal([]byte(body), &wrap); err2 != nil || len(wrap.Merges) == 0 {
			return nil, fmt.Errorf("cluster audit yaml: %w", err)
		}
		rows = wrap.Merges
	}
	out := make([]clusterMergeVerdict, 0, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			continue
		}
		verdict := strings.ToLower(strings.TrimSpace(row.Verdict))
		if verdict == "" {
			verdict = verdictReject
		}
		out = append(out, clusterMergeVerdict{
			ID:       id,
			Packages: normalizePackageList(row.Packages),
			Verdict:  verdict,
			Reason:   strings.TrimSpace(row.Reason),
			Evidence: row.Evidence,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("cluster audit yaml had no rows")
	}
	return out, nil
}

func alignClusterAuditRows(rows []clusterMergeVerdict, proposed []proposedMerge) ([]clusterMergeVerdict, error) {
	byID := make(map[string]clusterMergeVerdict, len(rows))
	for _, row := range rows {
		v := row
		if v.Verdict == verdictAccept && len(v.Evidence) == 0 {
			v.Verdict = verdictReject
			v.Reason = firstNonEmpty(v.Reason, "accept without evidence quotes")
		}
		byID[v.ID] = v
	}
	out := make([]clusterMergeVerdict, 0, len(proposed))
	for _, p := range proposed {
		v, ok := byID[p.ID]
		if !ok {
			out = append(out, clusterMergeVerdict{
				ID:       p.ID,
				Packages: p.Packages,
				Verdict:  verdictReject,
				Reason:   "unparsed: missing verdict for proposed id",
			})
			continue
		}
		delete(byID, p.ID)
		v.Packages = p.Packages
		v = enforceAcceptEvidenceCoverage(v)
		out = append(out, v)
	}
	if len(byID) > 0 {
		extras := make([]string, 0, len(byID))
		for id := range byID {
			extras = append(extras, id)
		}
		sort.Strings(extras)
		return out, fmt.Errorf("typology_cluster_audit returned extra ids: %s", strings.Join(extras, ", "))
	}
	return out, nil
}

func parseClusterAuditAnswerLines(answer string, proposed []proposedMerge) ([]clusterMergeVerdict, error) {
	blocks := splitAuditBlocks(answer)
	byID := make(map[string]clusterMergeVerdict, len(proposed))
	for _, block := range blocks {
		id := ""
		if m := clusterAuditIDRE.FindStringSubmatch(block); len(m) == 2 {
			id = strings.TrimSpace(m[1])
		}
		if id == "" {
			continue
		}
		verdict := ""
		if m := clusterAuditVerdictRE.FindStringSubmatch(block); len(m) == 2 {
			verdict = strings.ToLower(strings.TrimSpace(m[1]))
		}
		if verdict == "" {
			verdict = verdictReject
		}
		reason := fieldLine(block, "reason")
		evidence := collectEvidenceFromBlock(block)
		if verdict == verdictAccept && len(evidence) == 0 {
			verdict = verdictReject
			reason = firstNonEmpty(reason, "accept without evidence quotes")
		}
		byID[id] = clusterMergeVerdict{
			ID:       id,
			Verdict:  verdict,
			Reason:   reason,
			Evidence: evidence,
		}
	}
	out := make([]clusterMergeVerdict, 0, len(proposed))
	for _, p := range proposed {
		v, ok := byID[p.ID]
		if !ok {
			out = append(out, clusterMergeVerdict{
				ID:       p.ID,
				Packages: p.Packages,
				Verdict:  verdictReject,
				Reason:   "unparsed: missing verdict for proposed id",
			})
			continue
		}
		delete(byID, p.ID)
		v.Packages = p.Packages
		v = enforceAcceptEvidenceCoverage(v)
		out = append(out, v)
	}
	if len(byID) > 0 {
		extras := make([]string, 0, len(byID))
		for id := range byID {
			extras = append(extras, id)
		}
		sort.Strings(extras)
		return out, fmt.Errorf("typology_cluster_audit returned extra ids: %s", strings.Join(extras, ", "))
	}
	return out, nil
}

func splitAuditBlocks(answer string) []string {
	lines := strings.Split(answer, "\n")
	var blocks []string
	var cur strings.Builder
	flush := func() {
		s := strings.TrimSpace(cur.String())
		if s != "" {
			blocks = append(blocks, s)
		}
		cur.Reset()
	}
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if clusterAuditIDRE.MatchString(trim) && cur.Len() > 0 {
			flush()
		}
		cur.WriteString(line)
		cur.WriteByte('\n')
	}
	flush()
	return blocks
}

func fieldLine(block, key string) string {
	prefix := strings.ToLower(key) + ":"
	for _, line := range strings.Split(block, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trim), prefix) {
			return strings.TrimSpace(trim[len(prefix):])
		}
	}
	return ""
}

// collectEvidenceFromBlock reads evidence from an audit block. Models often emit:
//
//	evidence:
//	internal/localgit: Inspector.InspectPath
//	  - internal/remotegit: Fetch
//
// fieldLine alone only sees the empty rest of the evidence: line and drops quotes.
func collectEvidenceFromBlock(block string) []string {
	lines := strings.Split(block, "\n")
	collecting := false
	out := make([]string, 0, 4)
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		lower := strings.ToLower(trim)
		if strings.HasPrefix(lower, "evidence:") {
			collecting = true
			rest := strings.TrimSpace(trim[len("evidence:"):])
			if rest != "" {
				out = append(out, splitEvidenceList(rest)...)
			}
			continue
		}
		if !collecting {
			continue
		}
		if auditBlockFieldStart(lower) {
			break
		}
		item := strings.TrimSpace(strings.TrimPrefix(trim, "-"))
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func auditBlockFieldStart(lowerTrimmed string) bool {
	for _, key := range []string{"id:", "verdict:", "reason:", "packages:", "intent:"} {
		if strings.HasPrefix(lowerTrimmed, key) {
			return true
		}
	}
	return false
}

func splitEvidenceList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := regexp.MustCompile(`\s*;\s*|\s*\|\s*`).Split(raw, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 && raw != "" {
		return []string{raw}
	}
	return out
}

// enforceAcceptEvidenceCoverage demotes accept to overlay when evidence does not cite every package.
func enforceAcceptEvidenceCoverage(v clusterMergeVerdict) clusterMergeVerdict {
	if v.Verdict != verdictAccept || len(v.Packages) < 2 {
		return v
	}
	joined := strings.ToLower(strings.Join(v.Evidence, "\n"))
	var missing []string
	for _, pkg := range v.Packages {
		p := strings.TrimSpace(strings.ToLower(pkg))
		if p == "" {
			continue
		}
		if !strings.Contains(joined, p) {
			missing = append(missing, pkg)
		}
	}
	if len(missing) == 0 {
		return v
	}
	v.Verdict = verdictOverlay
	v.Reason = firstNonEmpty(v.Reason, "accept without per-package evidence quotes")
	if !strings.Contains(strings.ToLower(v.Reason), "per-package evidence") {
		v.Reason = strings.TrimSpace(v.Reason) + "; accept without per-package evidence quotes"
	}
	return v
}

func newClusterAuditorFromOpts(ctx context.Context, opts Options, analysisDir string) (clusterMergeAuditor, error) {
	if strings.TrimSpace(opts.ConfigDir) == "" || strings.TrimSpace(opts.RepoID) == "" {
		return nil, fmt.Errorf("config-dir and repo-id required for cluster audit RLM")
	}
	defaults, err := config.LoadDefaults(opts.ConfigDir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadRepoFile(opts.ConfigDir, opts.RepoID, defaults)
	if err != nil {
		return nil, err
	}
	traceDir := rlmTraceDir(inferenceWorkRoot(opts, analysisDir), jmodules.TaskTypologyClusterAudit)
	return newStropClusterMergeAuditor(ctx, cfg, traceDir)
}

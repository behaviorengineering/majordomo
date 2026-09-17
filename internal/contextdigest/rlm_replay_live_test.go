package contextdigest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/majordomo/internal/observability"
	stropdspy "github.com/behaviorengineering/strop/dspy"
	"github.com/behaviorengineering/strop/dspy/factory"
)

// rlmReplayFixture is one captured RLM TraceDir metadata + final answer span.
// Note: dspy-go TraceDir metadata currently truncates context (~503 chars with
// a trailing "..."); live replay still exercises Complete + parsers, but is not
// a full-fidelity context clone until dumps store the full payload.
type rlmReplayFixture struct {
	Source                       string                   `json:"source"`
	Task                         string                   `json:"task"`
	RepoID                       string                   `json:"repo_id"`
	Label                        string                   `json:"label"`
	Context                      string                   `json:"context"`
	Query                        string                   `json:"query"`
	ContextTruncated             bool                     `json:"context_truncated"`
	RecordedFinalAnswer          string                   `json:"recorded_final_answer"`
	RecordedFinalAnswerSynthetic bool                     `json:"recorded_final_answer_synthetic"`
	Proposed                     []rlmReplayProposedMerge `json:"proposed"`
}

type rlmReplayProposedMerge struct {
	ID       string   `json:"id"`
	Intent   string   `json:"intent"`
	Packages []string `json:"packages"`
}

func digestRLMReplayTasks() []string {
	return []string{
		jmodules.TaskTypologyInspect,
		jmodules.TaskBootstrapStory,
		jmodules.TaskTypologyClusterAudit,
		jmodules.TaskTypologyObjectiveGrounding,
	}
}

func rlmReplayFixturePath(task string) string {
	return filepath.Join("testdata", "rlm_replay", "gitboard_"+task+"_span.json")
}

func loadRLMReplayFixture(t *testing.T, task string) (rlmReplayFixture, bool) {
	t.Helper()
	path := rlmReplayFixturePath(task)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return rlmReplayFixture{}, false
		}
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var doc rlmReplayFixture
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
	if strings.TrimSpace(doc.Task) == "" {
		doc.Task = task
	}
	if strings.TrimSpace(doc.Query) == "" {
		t.Fatalf("fixture %s query empty", path)
	}
	return doc, true
}

func liveRLMReplayEnabled() bool {
	return os.Getenv("MAJORDOMO_LIVE_RLM_REPLAY") == "1"
}

// TestRLMReplayFixturesOffline loads recorded TraceDir finals through Go parsers.
func TestRLMReplayFixturesOffline(t *testing.T) {
	found := 0
	for _, task := range digestRLMReplayTasks() {
		task := task
		t.Run(task, func(t *testing.T) {
			doc, ok := loadRLMReplayFixture(t, task)
			if !ok {
				t.Skip("no fixture yet; capture TraceDir metadata+final from rlm-traces into " + rlmReplayFixturePath(task))
			}
			found++
			if strings.TrimSpace(doc.RecordedFinalAnswer) == "" {
				t.Fatal("recorded_final_answer empty")
			}
			if doc.ContextTruncated {
				t.Log("note: fixture context is truncated in TraceDir metadata")
			}
			switch task {
			case jmodules.TaskTypologyInspect:
				role, evidence := parseRLMRoleAnswer(doc.RecordedFinalAnswer)
				if role == "" {
					t.Fatalf("parse role empty; answer=%q", truncateReplayLog(doc.RecordedFinalAnswer, 200))
				}
				t.Logf("role=%s evidence=%q", role, truncateReplayLog(evidence, 120))
			case jmodules.TaskBootstrapStory:
				md, err := parseBootstrapStoryMarkdownAnswer(doc.RecordedFinalAnswer)
				if err != nil {
					t.Fatal(err)
				}
				assertBootstrapStoryMarkdownUnwrapped(t, md)
				section := doc.Label
				if section == "" {
					section = "readme"
				}
				if err := validateBootstrapStorySection(BootstrapStoryInput{RepoID: "gitboard"}, section, md); err != nil {
					t.Fatal(err)
				}
				t.Logf("section=%s markdown_len=%d", section, len(md))
			case jmodules.TaskTypologyClusterAudit:
				proposed := proposedFromRLMReplay(doc)
				if len(proposed) == 0 {
					t.Fatal("fixture proposed merges empty")
				}
				rows, err := parseClusterAuditAnswer(doc.RecordedFinalAnswer, proposed)
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) == 0 {
					t.Fatal("parsed zero verdicts")
				}
				if doc.RecordedFinalAnswerSynthetic {
					t.Log("note: recorded answer is synthetic (seed TraceDir had no final_answer)")
				}
				for _, row := range rows {
					t.Logf("verdict id=%s verdict=%s", row.ID, row.Verdict)
				}
			}
		})
	}
	if found == 0 {
		t.Fatal("expected at least one rlm_replay fixture under testdata/rlm_replay/")
	}
}

// TestBootstrapStoryEnvelopeFixturesOffline parses every bootstrap_story span
// fixture (main readme + *_envelope_span.json) and asserts YAML/markdown
// wrappers never survive parseBootstrapStoryMarkdownAnswer.
func TestBootstrapStoryEnvelopeFixturesOffline(t *testing.T) {
	pattern := filepath.Join("testdata", "rlm_replay", "gitboard_bootstrap_story*_span.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	if len(paths) == 0 {
		t.Fatalf("expected at least one fixture matching %s", pattern)
	}
	for _, path := range paths {
		path := path
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var doc rlmReplayFixture
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("decode %s: %v", path, err)
			}
			if strings.TrimSpace(doc.RecordedFinalAnswer) == "" {
				t.Fatal("recorded_final_answer empty")
			}
			md, err := parseBootstrapStoryMarkdownAnswer(doc.RecordedFinalAnswer)
			if err != nil {
				t.Fatal(err)
			}
			assertBootstrapStoryMarkdownUnwrapped(t, md)
			t.Logf("label=%s markdown_len=%d", doc.Label, len(md))
		})
	}
}

func assertBootstrapStoryMarkdownUnwrapped(t *testing.T, md string) {
	t.Helper()
	trimmed := strings.TrimSpace(md)
	if trimmed == "" {
		t.Fatal("parsed markdown empty")
	}
	if strings.Contains(md, "markdown: |") {
		t.Fatalf("parsed markdown still contains envelope marker markdown: |; got head=%q", truncateReplayLog(md, 160))
	}
	firstLine, _, _ := strings.Cut(trimmed, "\n")
	if strings.EqualFold(strings.TrimSpace(firstLine), "yaml") {
		t.Fatalf("parsed markdown still starts with yaml line; got head=%q", truncateReplayLog(md, 160))
	}
	if !strings.HasPrefix(trimmed, "#") && !strings.Contains(md, "\n#") {
		t.Fatalf("parsed markdown missing heading; got head=%q", truncateReplayLog(md, 160))
	}
}

// TestLiveDigestRLMReplay re-runs each digest RLM task that has a fixture via
// CreateRLMModule + RLMComplete. Opt-in only (CI stays offline):
//
//	MAJORDOMO_LIVE_RLM_REPLAY=1 go test ./internal/contextdigest/ \
//	  -run LiveDigestRLMReplay -count=1 -v -timeout 30m
//
// Optional: MAJORDOMO_CENTRAL_CONFIG=/path/to/majordomo-central-config
func TestLiveDigestRLMReplay(t *testing.T) {
	if !liveRLMReplayEnabled() {
		t.Skip("set MAJORDOMO_LIVE_RLM_REPLAY=1 to run live Polypus RLM replay")
	}
	if !polypusReachable(t) {
		t.Fatal("Polypus not reachable at http://127.0.0.1:1320/v1/models")
	}

	configDir := generatorReplayConfigDir(t)
	defaults, err := config.LoadDefaults(configDir)
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	cfg, err := config.LoadRepoFile(configDir, "gitboard", defaults)
	if err != nil {
		t.Fatalf("load gitboard config: %v", err)
	}

	for _, task := range digestRLMReplayTasks() {
		task := task
		t.Run(task, func(t *testing.T) {
			doc, ok := loadRLMReplayFixture(t, task)
			if !ok {
				t.Skip("no fixture; capture from rlm-traces → " + rlmReplayFixturePath(task))
			}
			if doc.ContextTruncated {
				t.Log("warning: TraceDir metadata truncated context; live result may differ from seed")
			}

			workStory := t.TempDir()
			ctx := context.Background()
			answer, iters, traceDir, err := liveRLMComplete(ctx, cfg, task, workStory, doc.Context, doc.Query)
			if err != nil {
				t.Fatalf("%s RLMComplete: %v", task, err)
			}
			t.Logf("iterations=%d answer_head=%q", iters, truncateReplayLog(answer, 240))
			t.Logf("rlm_trace_dir=%s", traceDir)

			switch task {
			case jmodules.TaskTypologyInspect:
				role, evidence := parseRLMRoleAnswer(answer)
				if role == "" {
					t.Fatalf("live role empty; answer=%q", truncateReplayLog(answer, 300))
				}
				t.Logf("role=%s evidence=%q", role, truncateReplayLog(evidence, 120))
			case jmodules.TaskBootstrapStory:
				md, err := parseBootstrapStoryMarkdownAnswer(answer)
				if err != nil {
					t.Fatal(err)
				}
				assertBootstrapStoryMarkdownUnwrapped(t, md)
				section := doc.Label
				if section == "" {
					section = "readme"
				}
				if err := validateBootstrapStorySection(BootstrapStoryInput{RepoID: "gitboard"}, section, md); err != nil {
					t.Logf("section validation (truncated context may cause soft fail): %v", err)
				}
				t.Logf("markdown_len=%d", len(md))
			case jmodules.TaskTypologyClusterAudit:
				proposed := proposedFromRLMReplay(doc)
				rows, err := parseClusterAuditAnswer(answer, proposed)
				if err != nil {
					t.Fatalf("parse live audit answer: %v\nanswer=%s", err, truncateReplayLog(answer, 500))
				}
				for _, row := range rows {
					t.Logf("verdict id=%s verdict=%s", row.ID, row.Verdict)
				}
			}

			entries, err := os.ReadDir(traceDir)
			if err != nil {
				t.Fatalf("read rlm-traces: %v", err)
			}
			if len(entries) == 0 {
				t.Fatal("expected fresh rlm-traces JSONL")
			}
		})
	}
}

func proposedFromRLMReplay(doc rlmReplayFixture) []proposedMerge {
	out := make([]proposedMerge, 0, len(doc.Proposed))
	for _, p := range doc.Proposed {
		out = append(out, proposedMerge{
			ID:       p.ID,
			Intent:   p.Intent,
			Packages: append([]string(nil), p.Packages...),
		})
	}
	return out
}

func liveRLMComplete(ctx context.Context, cfg config.RepoConfig, task, workStory, contextMD, query string) (answer string, iterations int, traceDir string, err error) {
	provider, ok, err := cfg.ResolveTaskProvider(task)
	if err != nil {
		return "", 0, "", err
	}
	if !ok {
		// Match production fallbacks.
		switch task {
		case jmodules.TaskTypologyClusterAudit, jmodules.TaskTypologyObjectiveGrounding:
			provider, ok, err = cfg.ResolveTaskProvider(jmodules.TaskTypologyInspect)
			if err != nil {
				return "", 0, "", err
			}
		}
	}
	if !ok {
		return "", 0, "", fmt.Errorf("%s provider not configured", task)
	}

	stropProvider := provider.ToStrop()
	timeout := provider.GetTimeout(ledgerRLMTimeout)
	if timeout < 3*time.Minute {
		timeout = 3 * time.Minute
	}
	llmFactory := factory.NewLLMFactory(nil, timeout)
	llmFactory.SetInstrumentHTTP(observability.InstrumentHTTPClient)
	llm, err := llmFactory.CreateLLM(ctx, stropProvider)
	if err != nil {
		return "", 0, "", fmt.Errorf("create LLM: %w", err)
	}
	llm = judge.WrapLLMWithRetry(llm, judge.DefaultModuleRetryConfig())

	traceDir = rlmTraceDir(workStory, task)
	rlmCfg := stropdspy.RLMDefaults()
	rlmCfg.MaxFullContextQueryChars = 24_000
	rlmCfg.Timeout = timeout
	rlmCfg.TraceDir = traceDir
	module, err := stropdspy.CreateRLMModule(llm, rlmCfg)
	if err != nil {
		return "", 0, "", err
	}
	answer, result, err := stropdspy.RLMComplete(ctx, module, contextMD, query)
	if err != nil {
		return "", 0, traceDir, err
	}
	iters := 0
	if result != nil {
		iters = result.Iterations
	}
	return answer, iters, traceDir, nil
}

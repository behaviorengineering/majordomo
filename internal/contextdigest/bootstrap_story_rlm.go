package contextdigest

import (
	"context"
	"fmt"
	"regexp"
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
	stropvalidation "github.com/behaviorengineering/strop/dspy/validation"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
)

type bootstrapStorySection struct {
	ID          string
	Current     string
	Setter      func(*BootstrapStoryOutput, string)
	Getter      func(BootstrapStoryOutput) string
	Instruction string
}

// rlmBootstrapStoryGenerator writes each context-branch story file via one RLM Complete.
// Go validates the markdown; there is no LLM Evaluate gate.
type rlmBootstrapStoryGenerator struct {
	caller bootstrapStoryCaller
}

type bootstrapStoryCaller interface {
	Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
}

type stropBootstrapStoryRLM struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations, promptTokens, completionTokens, totalTokens int, err error)
	}
}

func (v stropBootstrapStoryRLM) Complete(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
	return v.module.Complete(ctx, contextPayload, query)
}

func newStropBootstrapStoryRLM(ctx context.Context, cfg config.RepoConfig, workStoryDir string) (rlmBootstrapStoryGenerator, error) {
	provider, ok, err := cfg.ResolveTaskProvider(jmodules.TaskBootstrapStory)
	if err != nil {
		return rlmBootstrapStoryGenerator{}, err
	}
	if !ok {
		return rlmBootstrapStoryGenerator{}, fmt.Errorf("bootstrap_story provider not configured")
	}
	stropProvider := provider.ToStrop()
	llmFactory := factory.NewLLMFactory(nil, ledgerRLMTimeout)
	llmFactory.SetInstrumentHTTP(observability.InstrumentHTTPClient)
	llm, err := llmFactory.CreateLLM(ctx, stropProvider)
	if err != nil {
		return rlmBootstrapStoryGenerator{}, fmt.Errorf("bootstrap_story RLM LLM: %w", err)
	}
	llm = judge.WrapLLMWithRetry(llm, judge.DefaultModuleRetryConfig())
	rlmCfg := stropdspy.RLMDefaults()
	rlmCfg.MaxFullContextQueryChars = 24_000
	timeout := provider.GetTimeout(ledgerRLMTimeout)
	if timeout < ledgerRLMTimeout {
		timeout = ledgerRLMTimeout
	}
	rlmCfg.Timeout = timeout
	rlmCfg.TraceDir = rlmTraceDir(workStoryDir, jmodules.TaskBootstrapStory)
	module, err := rlmCfg.CreateModule()
	if err != nil {
		return rlmBootstrapStoryGenerator{}, err
	}
	return rlmBootstrapStoryGenerator{
		caller: stropBootstrapStoryRLM{module: rlmCompleteAdapter{complete: func(ctx context.Context, contextPayload any, query string) (string, int, int, int, int, error) {
			answer, result, err := stropdspy.RLMComplete(ctx, module, contextPayload, query)
			if err != nil {
				return "", 0, 0, 0, 0, err
			}
			iters, prompt, completion, total := 0, 0, 0, 0
			if result != nil {
				iters = result.Iterations
				prompt = result.Usage.PromptTokens
				completion = result.Usage.CompletionTokens
				total = result.Usage.TotalTokens
				llmusage.FromContext(ctx).AddTokenUsageValue(jmodules.TaskBootstrapStory, result.Usage)
			} else {
				llmusage.FromContext(ctx).Add(jmodules.TaskBootstrapStory, 0, 0, 0)
			}
			return answer, iters, prompt, completion, total, nil
		}}},
	}, nil
}

func newBootstrapStoryRLMFromOpts(ctx context.Context, opts Options, analysisDir string) (BootstrapStoryGenerator, error) {
	if strings.TrimSpace(opts.ConfigDir) == "" || strings.TrimSpace(opts.RepoID) == "" {
		return nil, fmt.Errorf("config-dir and repo-id required for bootstrap story RLM")
	}
	defaults, err := config.LoadDefaults(opts.ConfigDir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadRepoFile(opts.ConfigDir, opts.RepoID, defaults)
	if err != nil {
		return nil, err
	}
	gen, err := newStropBootstrapStoryRLM(ctx, cfg, inferenceWorkRoot(opts, analysisDir))
	if err != nil {
		return nil, err
	}
	return gen, nil
}

// Generate runs one RLM Complete per story section and validates markdown in Go.
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
			PromptVersion:    cache.DigestStoryPromptV1,
			SchemaVersion:    cache.DigestStorySchemaV2,
		}
		for attempt := 1; attempt <= maxBootstrapStoryAttempts; attempt++ {
			if attempt == 1 && strings.TrimSpace(feedback) == "" &&
				input.DigestSkips && input.DigestCache != nil {
				if hit, ok, err := input.DigestCache.LookupStory(storyFP); err == nil && ok && strings.TrimSpace(hit.Markdown) != "" {
					if err := validateBootstrapStorySection(input.RepoID, sec.ID, hit.Markdown); err == nil {
						input.DigestCache.RecordStoryHit(hit.PromptTokens, hit.CompletionTokens, hit.TotalTokens)
						logf("INFO", "digest cache hit story section=%s", sec.ID)
						sec.Setter(&out, hit.Markdown)
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
			tmp := out
			if err := validateBootstrapStorySection(input.RepoID, sec.ID, sec.Getter(tmp)); err != nil {
				lastErr = err
				if attempt == maxBootstrapStoryAttempts {
					sectionErr = err
					observability.EndSpanWithStatus(span, &sectionErr)
					return BootstrapStoryOutput{}, sectionErr
				}
				feedback = err.Error()
				continue
			}
			if input.DigestCache != nil {
				if err := input.DigestCache.StoreStory(storyFP, cache.StoryCached{
					Markdown:         markdown,
					PromptTokens:     promptTok,
					CompletionTokens: completionTok,
					TotalTokens:      totalTok,
				}); err != nil {
					logf("WARN", "digest cache store story section=%s failed: %v", sec.ID, err)
				}
			}
			feedback = ""
			lastErr = nil
			break
		}
		if lastErr != nil {
			sectionErr = lastErr
			observability.EndSpanWithStatus(span, &sectionErr)
			return BootstrapStoryOutput{}, sectionErr
		}
		observability.EndSpanWithStatus(span, &sectionErr)
	}
	if err := validateBootstrapStoryOutput(out); err != nil {
		return BootstrapStoryOutput{}, err
	}
	return out, nil
}

func bootstrapStorySections(input BootstrapStoryInput) []bootstrapStorySection {
	return []bootstrapStorySection{
		{ID: "readme", Current: input.CurrentReadme, Instruction: "Context-branch README: seed origin and reading order for the served repo. Keep ## Reading order and majordomo reading markers when present. README may mention the context branch; do not rewrite the product mission as Majordomo.", Setter: func(o *BootstrapStoryOutput, s string) { o.ReadmeMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.ReadmeMD }},
		{ID: "mission", Current: input.CurrentMission, Instruction: "Mission markdown: present-tense purpose of the served product from README + typology + slice objective ledger. MUST NOT invent a Majordomo/control-plane mission.", Setter: func(o *BootstrapStoryOutput, s string) { o.MissionMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.MissionMD }},
		{ID: "architecture", Current: input.CurrentArchitecture, Instruction: "Teaching architecture for the served product. Prefer slice objective ledger jobs and doors (entrypoint vs server). MUST NOT invent a glamorous core-value slice; MUST NOT claim confirmed .typology/.", Setter: func(o *BootstrapStoryOutput, s string) { o.ArchitectureMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.ArchitectureMD }},
		{ID: "conventions", Current: input.CurrentConventions, Instruction: "Conventions markdown grounded in evidence for the served product.", Setter: func(o *BootstrapStoryOutput, s string) { o.ConventionsMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.ConventionsMD }},
		{ID: "weaknesses", Current: input.CurrentWeaknesses, Instruction: "Weaknesses markdown from typology debt and findings; no invented history.", Setter: func(o *BootstrapStoryOutput, s string) { o.WeaknessesMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.WeaknessesMD }},
		{ID: "chronology", Current: input.CurrentChronology, Instruction: "Honest chronology with at most one explicit seed marker; do not reconstruct past decisions.", Setter: func(o *BootstrapStoryOutput, s string) { o.ChronologyMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.ChronologyMD }},
		{ID: "grounding", Current: input.CurrentGrounding, Instruction: "Agenting GROUNDING.md: cold-reader map of the served product for agents. Prefer ledger objectives. MUST NOT read as Majordomo agent-bootstrap meta or invent agent taxonomies.", Setter: func(o *BootstrapStoryOutput, s string) { o.GroundingMD = s }, Getter: func(o BootstrapStoryOutput) string { return o.GroundingMD }},
	}
}

func validateBootstrapStoryEvidenceMap(input BootstrapStoryInput) error {
	fields := map[string]any{
		"repo_id":         input.RepoID,
		"readme_snapshot": input.ReadmeSnapshot,
	}
	required := []string{"repo_id", "readme_snapshot"}
	if strings.TrimSpace(input.TypologyRefinedCatalog) != "" || strings.TrimSpace(input.TypologySliceObjectiveLedger) != "" {
		fields["typology_refined_catalog"] = input.TypologyRefinedCatalog
		fields["slice_objective_ledger"] = input.TypologySliceObjectiveLedger
		required = append(required, "typology_refined_catalog", "slice_objective_ledger")
	}
	return stropvalidation.ValidateRequiredInputs(required)(context.Background(), fields, nil)
}

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

Final answer MUST be a small YAML object (no markdown fences):
markdown: |
  <full markdown body for this section only>

Write present-tense, evidence-backed prose. Prefer slice objective ledger and refined Typology catalog over raw inventory.
Do not invent history. Preserve majordomo-reading markers when rewriting README.
Mission, architecture, and grounding MUST name the served product from evidence; MUST NOT describe Majordomo triage, digest, or context-branch process as the product.`,
		repo, sec.ID, sec.Instruction, fb, strings.TrimSpace(sec.Current))
}

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
	// Fallback: treat the whole answer as markdown when models skip the YAML wrapper.
	if strings.Contains(body, "\n") || strings.HasPrefix(body, "#") {
		return body, nil
	}
	return "", fmt.Errorf("bootstrap story answer missing markdown field")
}

func validateBootstrapStorySection(repoID, id, text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("bootstrap story output %s is required", id)
	}
	if err := rejectMajordomoAsProduct(repoID, id, text); err != nil {
		return err
	}
	return nil
}

// majordomoReadingMarkerRE matches required context-branch HTML comment markers.
// These must stay in README/mission markdown but must not trip product-voice gates.
var majordomoReadingMarkerRE = regexp.MustCompile(`(?is)<!--\s*majordomo-reading-(?:nav|toc):(?:start|end)\s*-->`)

var majordomoReadingBlockRE = regexp.MustCompile(`(?is)<!--\s*majordomo-reading-(?:nav|toc):start\s*-->.*?<!--\s*majordomo-reading-(?:nav|toc):end\s*-->`)

// stripMajordomoReadingMarkers removes reading-path marker HTML so product-voice
// checks only see teaching prose (markers themselves contain the word Majordomo).
func stripMajordomoReadingMarkers(text string) string {
	out := majordomoReadingBlockRE.ReplaceAllString(text, "")
	return majordomoReadingMarkerRE.ReplaceAllString(out, "")
}

func rejectMajordomoAsProduct(repoID, sectionID, text string) error {
	switch sectionID {
	case "mission", "architecture", "grounding":
	default:
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(repoID), "majordomo") {
		return nil
	}
	lower := strings.ToLower(stripMajordomoReadingMarkers(text))
	// Product-as-actor patterns from observed gitboard drift.
	banned := []string{
		"majordomo provides",
		"majordomo bootstrap",
		"the majordomo bootstrap",
		"majordomo establishes",
		"majordomo's",
	}
	for _, p := range banned {
		if strings.Contains(lower, p) {
			return fmt.Errorf("bootstrap story section %s treats Majordomo as the product; write for served repo %q", sectionID, repoID)
		}
	}
	// Bare Majordomo as subject line opener is also drift for non-majordomo repos.
	if strings.Contains(lower, "majordomo") && (sectionID == "mission" || sectionID == "grounding") {
		return fmt.Errorf("bootstrap story section %s must not name Majordomo when repo_id=%q", sectionID, repoID)
	}
	return nil
}

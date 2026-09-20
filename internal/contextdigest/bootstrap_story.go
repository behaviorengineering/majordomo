package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	stropvalidation "github.com/behaviorengineering/strop/pkg/dspy/validation"
	"github.com/behaviorengineering/strop/pkg/evaluation"

	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

// BootstrapStoryInput carries the evidence pack into the bootstrap LLM task.
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

const maxBootstrapStoryAttempts = 3

// BootstrapStoryOutput carries the model-authored story tree content.
type BootstrapStoryOutput struct {
	ReadmeMD       string
	MissionMD      string
	ArchitectureMD string
	ConventionsMD  string
	WeaknessesMD   string
	ChronologyMD   string
	GroundingMD    string
}

// BootstrapStoryGenerator renders the seed-time story tree from evidence.
type BootstrapStoryGenerator interface {
	Generate(ctx context.Context, input BootstrapStoryInput) (BootstrapStoryOutput, error)
}

// JudgeBootstrapStoryGenerator is retained for tests that inject a CoT generator.
// Production bootstrap uses rlmBootstrapStoryGenerator (per-section Completes, no Evaluate).
type JudgeBootstrapStoryGenerator struct {
	Gen judge.Generator
}

// Generate renders all sections via the legacy mega-CoT path (tests / explicit injection only).
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

type packageJudgeGenerator struct{}

func (packageJudgeGenerator) Generate(ctx context.Context, task string, fields map[string]interface{}, version int) (map[string]interface{}, error) {
	return judge.Generate(ctx, task, fields, version)
}

func (packageJudgeGenerator) Evaluate(
	ctx context.Context,
	task string,
	inputFields, outputFields map[string]interface{},
	version int,
) (*evaluation.AggregatedEvaluation, error) {
	return judge.Evaluate(ctx, task, inputFields, outputFields, version)
}

func (packageJudgeGenerator) Ready() bool { return judge.StoryLLMAvailable() }

func (packageJudgeGenerator) TaskModel(string) string { return "" }

func writeBootstrapStory(ctx context.Context, ctxDir, analysisDir string, at time.Time, sourceSHA string, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := assertEvidenceGroundingBeforeStory(ctxDir); err != nil {
		return err
	}
	input, err := loadBootstrapStoryInput(ctxDir, analysisDir, at, sourceSHA)
	if err != nil {
		return err
	}
	input.DigestCache = opts.DigestCache
	input.DigestSkips = opts.DigestSkips
	input.DigestModelID = opts.DigestModelID
	gen := opts.BootstrapStoryGenerator
	if gen == nil {
		rlm, rlmErr := newBootstrapStoryRLMFromOpts(ctx, opts, analysisDir)
		if rlmErr != nil {
			return fmt.Errorf("bootstrap story RLM: %w", rlmErr)
		}
		gen = rlm
	}
	out, err := gen.Generate(ctx, input)
	if err != nil {
		return err
	}
	if err := persistBootstrapStory(ctxDir, out); err != nil {
		return err
	}
	return assertArchitectureKeepsGroundedObjectives(ctxDir)
}

func loadBootstrapStoryInput(ctxDir, analysisDir string, at time.Time, sourceSHA string) (BootstrapStoryInput, error) {
	manifestPath := filepath.Join(ctxDir, "evidence", "typology", "manifest.yaml")
	manifest, err := contextstore.ParseTypologyManifest(manifestPath)
	if err != nil {
		return BootstrapStoryInput{}, err
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return BootstrapStoryInput{}, err
	}
	archPath := filepath.Join(ctxDir, "evidence", "typology", manifest.ArchitecturePath)
	architecture, err := os.ReadFile(archPath)
	if err != nil {
		return BootstrapStoryInput{}, err
	}
	input := BootstrapStoryInput{
		RepoID:               manifest.RepoID,
		SourceSHA:            sourceSHA,
		GeneratedAt:          at,
		EvidenceMode:         manifest.Mode,
		ModuleScope:          manifest.ModuleScope,
		ReadmeSnapshot:       readText(filepath.Join(analysisDir, "README.md")),
		TypologyManifest:     string(manifestBytes),
		TypologyArchitecture: string(architecture),
		RepoLayout:           strings.Join(collectTopLevelEntries(analysisDir), "\n"),
		CurrentReadme:        readText(filepath.Join(ctxDir, "README.md")),
		CurrentMission:       readText(filepath.Join(ctxDir, "mission.md")),
		CurrentArchitecture:  readText(filepath.Join(ctxDir, "architecture.md")),
		CurrentConventions:   readText(filepath.Join(ctxDir, "conventions.md")),
		CurrentWeaknesses:    readText(filepath.Join(ctxDir, "weaknesses.md")),
		CurrentChronology:    readText(filepath.Join(ctxDir, "chronology.md")),
		CurrentGrounding:     readText(filepath.Join(ctxDir, "agenting", "overview", "GROUNDING.md")),
	}
	if strings.TrimSpace(manifest.RefinedSnapshotPath) != "" {
		input.TypologyRefinedCatalog = readText(filepath.Join(ctxDir, "evidence", "typology", manifest.RefinedSnapshotPath))
	}
	if strings.TrimSpace(manifest.JourneyPath) != "" {
		input.TypologyJourney = readText(filepath.Join(ctxDir, "evidence", "typology", manifest.JourneyPath))
	}
	if strings.TrimSpace(manifest.SliceObjectiveLedgerPath) != "" {
		typoDir := filepath.Join(ctxDir, "evidence", "typology")
		primary := strings.TrimSpace(manifest.SliceObjectiveLedgerPath)
		input.TypologySliceObjectiveLedger = readEvidenceRel(typoDir, primary, legacySliceObjectiveLedgerRel, sliceObjectiveLedgerRel)
	}
	constraintsRel := strings.TrimSpace(manifest.PackageCapabilityConstraintsPath)
	if constraintsRel == "" {
		constraintsRel = packageCapabilityConstraintsRel
	}
	constraintsPath := filepath.Join(ctxDir, "evidence", "typology", constraintsRel)
	if fileExists(constraintsPath) {
		input.TypologyPackageCapabilityConstraints = readText(constraintsPath)
	}
	if err := requireBootstrapStoryEvidence(input, manifest); err != nil {
		return BootstrapStoryInput{}, err
	}
	return input, nil
}

// requireBootstrapStoryEvidence fail-closes when seed refine evidence is hollow before story RLM.
func requireBootstrapStoryEvidence(input BootstrapStoryInput, manifest contextstore.TypologyManifest) error {
	fields := map[string]any{
		"repo_id":         input.RepoID,
		"readme_snapshot": input.ReadmeSnapshot,
	}
	required := []string{"repo_id", "readme_snapshot"}
	refinedRan := strings.TrimSpace(manifest.RefinedSnapshotPath) != "" &&
		!strings.EqualFold(manifest.RefineStatus, contextstore.TypologyRefineSkipped) &&
		manifest.Mode != contextstore.TypologyModeFallback
	if refinedRan {
		fields["typology_refined_catalog"] = input.TypologyRefinedCatalog
		fields["slice_meaning_ledger"] = input.TypologySliceObjectiveLedger
		required = append(required, "typology_refined_catalog", "slice_meaning_ledger")
	}
	return stropvalidation.ValidateRequiredInputs(required)(context.Background(), fields, nil)
}

func persistBootstrapStory(ctxDir string, out BootstrapStoryOutput) error {
	if err := writeRequiredFile(filepath.Join(ctxDir, "README.md"), out.ReadmeMD); err != nil {
		return err
	}
	if err := writeRequiredFile(filepath.Join(ctxDir, "mission.md"), out.MissionMD); err != nil {
		return err
	}
	if err := writeRequiredFile(filepath.Join(ctxDir, "architecture.md"), ensureStoryArchitectureBanner(out.ArchitectureMD)); err != nil {
		return err
	}
	if err := writeRequiredFile(filepath.Join(ctxDir, "conventions.md"), out.ConventionsMD); err != nil {
		return err
	}
	if err := writeRequiredFile(filepath.Join(ctxDir, "weaknesses.md"), out.WeaknessesMD); err != nil {
		return err
	}
	if err := writeRequiredFile(filepath.Join(ctxDir, "chronology.md"), out.ChronologyMD); err != nil {
		return err
	}
	if err := writeRequiredFile(filepath.Join(ctxDir, "agenting", "overview", "GROUNDING.md"), out.GroundingMD); err != nil {
		return err
	}
	return contextstore.ApplyReadingPath(ctxDir)
}

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

func readText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func writeRequiredFile(path, text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("required content for %s is empty", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func stringField(out map[string]interface{}, key string) string {
	text, ok := out[key].(string)
	if !ok {
		return ""
	}
	return text
}

func collectTopLevelEntries(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".git") || strings.HasPrefix(name, ".cursor") || strings.HasPrefix(name, ".typology") || name == "evidence" {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

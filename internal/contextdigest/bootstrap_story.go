package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/behaviorengineering/strop/evaluation"

	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

// BootstrapStoryInput carries the evidence pack into the bootstrap LLM task.
type BootstrapStoryInput struct {
	RepoID                 string
	SourceSHA              string
	GeneratedAt            time.Time
	EvidenceMode           string
	ModuleScope            string
	ReadmeSnapshot         string
	TypologyManifest       string
	TypologyArchitecture   string
	TypologyRefinedCatalog string
	TypologyJourney        string
	RepoLayout             string
	CurrentReadme          string
	CurrentMission         string
	CurrentArchitecture    string
	CurrentConventions     string
	CurrentWeaknesses      string
	CurrentChronology      string
	CurrentGrounding       string
	ValidationFeedback     string
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

// JudgeBootstrapStoryGenerator uses an injected or process-wide judge generator.
type JudgeBootstrapStoryGenerator struct {
	Gen judge.Generator
}

// Generate renders the bootstrap story using the shared judge task with evaluate/retry.
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
		agg, err := gen.Evaluate(ctx, jmodules.TaskBootstrapStory, fields, map[string]interface{}{
			"readme_md":       res.ReadmeMD,
			"mission_md":      res.MissionMD,
			"architecture_md": res.ArchitectureMD,
			"conventions_md":  res.ConventionsMD,
			"weaknesses_md":   res.WeaknessesMD,
			"chronology_md":   res.ChronologyMD,
			"grounding_md":    res.GroundingMD,
		}, attempt)
		if err != nil {
			lastErr = err
			if attempt == maxBootstrapStoryAttempts {
				return BootstrapStoryOutput{}, fmt.Errorf("bootstrap story LLM evaluation: %w", err)
			}
			feedback = err.Error()
			continue
		}
		if !judge.EvalPassed(agg) {
			lastErr = fmt.Errorf("%s", judge.EvalFeedback(agg))
			if attempt == maxBootstrapStoryAttempts {
				return BootstrapStoryOutput{}, fmt.Errorf("bootstrap story LLM evaluation failed after %d attempts:\n%s", maxBootstrapStoryAttempts, judge.EvalFeedback(agg))
			}
			feedback = judge.EvalFeedback(agg)
			continue
		}
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

func writeBootstrapStory(ctxDir, analysisDir string, at time.Time, sourceSHA string, gen BootstrapStoryGenerator, judgeGen judge.Generator) error {
	input, err := loadBootstrapStoryInput(ctxDir, analysisDir, at, sourceSHA)
	if err != nil {
		return err
	}
	if gen == nil {
		gen = JudgeBootstrapStoryGenerator{Gen: judgeGen}
	}
	out, err := gen.Generate(context.Background(), input)
	if err != nil {
		return err
	}
	return persistBootstrapStory(ctxDir, out)
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
	return input, nil
}

func persistBootstrapStory(ctxDir string, out BootstrapStoryOutput) error {
	if err := writeText(filepath.Join(ctxDir, "README.md"), out.ReadmeMD); err != nil {
		return err
	}
	if err := writeText(filepath.Join(ctxDir, "mission.md"), out.MissionMD); err != nil {
		return err
	}
	if err := writeText(filepath.Join(ctxDir, "architecture.md"), out.ArchitectureMD); err != nil {
		return err
	}
	if err := writeText(filepath.Join(ctxDir, "conventions.md"), out.ConventionsMD); err != nil {
		return err
	}
	if err := writeText(filepath.Join(ctxDir, "weaknesses.md"), out.WeaknessesMD); err != nil {
		return err
	}
	if err := writeText(filepath.Join(ctxDir, "chronology.md"), out.ChronologyMD); err != nil {
		return err
	}
	if err := writeText(filepath.Join(ctxDir, "agenting", "overview", "GROUNDING.md"), out.GroundingMD); err != nil {
		return err
	}
	return nil
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

func writeText(path, text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("bootstrap story output for %s is required", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func stringField(out map[string]interface{}, key string) string {
	text, _ := out[key].(string)
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

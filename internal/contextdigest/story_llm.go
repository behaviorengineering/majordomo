package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/strop/orchestration"
	"github.com/behaviorengineering/strop/streaming"

	"github.com/behaviorengineering/majordomo/internal/judge"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

type storySection struct {
	id       string
	filename string
	globs    []string
}

var storySections = []storySection{
	{id: "mission", filename: "mission.md", globs: []string{"**/README*", "**/cmd/**", "**/main.go"}},
	{id: "architecture", filename: "architecture.md", globs: []string{"**/internal/**", "**/pkg/**", "**/*.go", "**/*.py"}},
	{id: "conventions", filename: "conventions.md", globs: []string{"**/*.yaml", "**/*.yml", "**/Makefile", "**/.github/**"}},
	{id: "weaknesses", filename: "weaknesses.md", globs: []string{"**/*.go", "**/*.py", "**/internal/**"}},
}

type digestSectionRunner struct {
	cc            CommitContext
	regenFeedback string
	changedFiles  string
	gen           judge.Generator
}

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
	text, _ := out["updated_text"].(string)
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

func applyStoryLLM(ctxDir string, commits []CommitContext, regenFeedback string, gen judge.Generator) error {
	if gen == nil {
		if !judge.StoryLLMAvailable() {
			return fmt.Errorf("LLM story digest unavailable")
		}
		gen = packageJudgeGenerator{}
	} else if !gen.Ready() {
		return fmt.Errorf("LLM story digest unavailable")
	}
	codec := orchestration.SectionCodec[map[string]string]{
		ToMap: func(d map[string]string) map[string]string {
			if d == nil {
				return map[string]string{}
			}
			out := make(map[string]string, len(d))
			for k, v := range d {
				out[k] = v
			}
			return out
		},
		FromMap: func(m map[string]string) map[string]string {
			if m == nil {
				return map[string]string{}
			}
			out := make(map[string]string, len(m))
			for k, v := range m {
				out[k] = v
			}
			return out
		},
	}
	sectionIDs := make([]string, 0, len(storySections))
	for _, s := range storySections {
		sectionIDs = append(sectionIDs, s.id)
	}
	def := orchestration.DocumentSectionDefinition{
		Name:         "majordomo_story",
		SectionIDs:   sectionIDs,
		MaxAttempts:  3,
		MinPassScore: judge.MinEvalPassScore,
	}

	for _, cc := range commits {
		if !sectionSignals(cc) {
			continue
		}
		seed := loadStoryDraft(ctxDir)
		runner := &digestSectionRunner{
			cc:            cc,
			regenFeedback: regenFeedback,
			changedFiles:  strings.Join(cc.Files, ", "),
			gen:           gen,
		}
		strat := orchestration.NewSectionWalkStrategy(orchestration.SectionWalkConfig[map[string]string]{
			Sections: def,
			Seed:     seed,
			Runner:   runner,
			Codec:    codec,
		})
		ctx := context.Background()
		for _, phase := range strat.Phases() {
			if _, err := strat.RunPhase(ctx, phase, regenFeedback, nil); err != nil {
				return fmt.Errorf("story section-walk %s: %w", shortSHA(cc.SHA), err)
			}
		}
		comp, err := strat.Result()
		if err != nil {
			return fmt.Errorf("story section-walk result %s: %w", shortSHA(cc.SHA), err)
		}
		draft := seed
		if comp != nil {
			if st, ok := comp.OutputState.(*orchestration.SectionWalkState[map[string]string]); ok && st != nil {
				draft = st.Draft
			}
		}
		if err := writeStoryDraft(ctxDir, draft); err != nil {
			return err
		}
	}
	return nil
}

func sectionSignals(cc CommitContext) bool {
	if len(cc.Files) == 0 && strings.TrimSpace(cc.Diff) == "" {
		return false
	}
	for _, sec := range storySections {
		if anyPathMatches(cc.Files, sec.globs) {
			return true
		}
	}
	return false
}

func loadStoryDraft(ctxDir string) map[string]string {
	draft := map[string]string{}
	for _, sec := range storySections {
		b, err := os.ReadFile(filepath.Join(ctxDir, sec.filename))
		if err != nil {
			continue
		}
		draft[sec.id] = string(b)
	}
	return draft
}

func writeStoryDraft(ctxDir string, draft map[string]string) error {
	if draft == nil {
		return nil
	}
	for _, sec := range storySections {
		text, ok := draft[sec.id]
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		path := filepath.Join(ctxDir, sec.filename)
		cur, _ := os.ReadFile(path)
		if string(cur) == text {
			continue
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
	}
	return nil
}

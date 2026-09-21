package contextdigest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/cache"
	typologypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/typology"
	"github.com/behaviorengineering/strop/pkg/orchestration"
	"github.com/behaviorengineering/strop/pkg/stepplan"
	typroles "github.com/behaviorengineering/typology/pkg/roles"
)

const (
	ledgerStepplanRelDir   = "tmp/typology/stepplan"
	ledgerSynthesisStepID  = "synthesis"
	ledgerEvidenceStepPref = "ev_"
)

// ledgerGroundingIssue is a soft validation failure for the outer refine loop.
// It must not burn RunStepPlan retry attempts (those are for transient upstream faults).
type ledgerGroundingIssue struct {
	msg string
}

func (e ledgerGroundingIssue) Error() string { return e.msg }

func classifyLedgerStepError(err error) orchestration.StepErrorClass {
	var gi ledgerGroundingIssue
	if errors.As(err, &gi) {
		return orchestration.StepErrorClass{Retryable: false}
	}
	return orchestration.ClassifyTransientStepError(err)
}

// runSliceLedgerViaStepPlan executes evidence steps + synthesis through strop RunStepPlan.
// Checkpoints live under analysisDir/tmp/typology/stepplan so resume skips finished packages.
func runSliceLedgerViaStepPlan(
	ctx context.Context,
	caller sliceLedgerRLMCaller,
	req sliceLedgerBuildRequest,
	t ledgerSliceTarget,
	wholeContext string,
	byPath map[string]packageCapabilityConstraint,
	rolesDoc packageRolesDoc,
	sliceFeedback string,
) (sliceObjectiveLedgerEntry, string, error) {
	ordered := append([]string(nil), t.paths...)
	sort.Strings(ordered)

	root := filepath.Join(req.AnalysisDir, filepath.FromSlash(ledgerStepplanRelDir))
	store, err := stepplan.NewFileStore(root)
	if err != nil {
		return sliceObjectiveLedgerEntry{}, "", fmt.Errorf("ledger stepplan store: %w", err)
	}

	steps, pathByStep, err := buildLedgerStepplanSteps(req.AnalysisDir, t.id, ordered, wholeContext)
	if err != nil {
		return sliceObjectiveLedgerEntry{}, "", err
	}
	planID := ledgerPlanID(t.id)
	plan, err := stepplan.NewPlan(planID, "slice_grounding", steps, map[string]any{
		"slice_id": t.id,
	})
	if err != nil {
		return sliceObjectiveLedgerEntry{}, "", fmt.Errorf("ledger stepplan: %w", err)
	}
	if err := store.SavePlan(ctx, plan); err != nil {
		return sliceObjectiveLedgerEntry{}, "", fmt.Errorf("ledger stepplan save: %w", err)
	}

	runner := &ledgerStepRunner{
		store:         store,
		caller:        caller,
		req:           req,
		target:        t,
		wholeContext:  wholeContext,
		byPath:        byPath,
		rolesDoc:      rolesDoc,
		sliceFeedback: sliceFeedback,
		pathByStep:    pathByStep,
	}
	_, runErr := orchestration.RunStepPlan(ctx, plan, store, runner, orchestration.StepPlanConfig{
		MaxAttemptsPerStep: 3,
		Classify:           classifyLedgerStepError,
	}, nil)
	if runErr != nil {
		var gi ledgerGroundingIssue
		if errors.As(runErr, &gi) {
			return sliceObjectiveLedgerEntry{}, gi.msg, nil
		}
		if ctx.Err() != nil {
			return sliceObjectiveLedgerEntry{}, "", fmt.Errorf("%s: slice %q objective ledger step plan failed: %w",
				typologypack.CriterionIDRoleGrounding, t.id, runErr)
		}
		return sliceObjectiveLedgerEntry{}, fmt.Sprintf(
			"%s: slice %q objective ledger step plan failed: %v",
			typologypack.CriterionIDRoleGrounding, t.id, runErr,
		), nil
	}

	cp, err := store.LoadStep(ctx, plan.ID, ledgerSynthesisStepID)
	if err != nil {
		return sliceObjectiveLedgerEntry{}, fmt.Sprintf(
			"%s: slice %q missing synthesis checkpoint: %v",
			typologypack.CriterionIDRoleGrounding, t.id, err,
		), nil
	}
	if cp.Status != stepplan.StepStatusComplete {
		return sliceObjectiveLedgerEntry{}, fmt.Sprintf(
			"%s: slice %q synthesis checkpoint status %q",
			typologypack.CriterionIDRoleGrounding, t.id, cp.Status,
		), nil
	}
	var entry sliceObjectiveLedgerEntry
	if err := json.Unmarshal(cp.Output, &entry); err != nil {
		return sliceObjectiveLedgerEntry{}, fmt.Sprintf(
			"%s: slice %q synthesis checkpoint decode: %v",
			typologypack.CriterionIDRoleGrounding, t.id, err,
		), nil
	}
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = t.id
	}
	if len(entry.OwnedPaths) == 0 {
		entry.OwnedPaths = append([]string(nil), t.paths...)
	}
	return entry, "", nil
}

func buildLedgerStepplanSteps(
	analysisDir, sliceID string,
	ordered []string,
	wholeContext string,
) ([]stepplan.Step, map[string]string, error) {
	pathByStep := make(map[string]string, len(ordered))
	steps := make([]stepplan.Step, 0, len(ordered)+1)
	for _, p := range ordered {
		stepID := ledgerEvidenceStepID(p)
		pathByStep[stepID] = p
		hash, err := packageEvidenceInputHash(analysisDir, p, wholeContext)
		if err != nil {
			return nil, nil, err
		}
		steps = append(steps, stepplan.Step{
			ID:   stepID,
			Goal: fmt.Sprintf("Distill grounding evidence for package %s", p),
			InputRefs: []stepplan.InputRef{{
				Key:  "package",
				Hash: hash,
				URI:  p,
			}},
			Budget: stepplan.Budget{
				Timeout:  ledgerEvidenceTimeout,
				MaxChars: ledgerMaxContextChars,
			},
			DoneCriteria: stepplan.DoneCriteria{
				RequiredKeys: []string{"path"},
			},
		})
	}
	synthRefs := make([]stepplan.InputRef, 0, len(ordered))
	for _, p := range ordered {
		hash, err := packageEvidenceInputHash(analysisDir, p, wholeContext)
		if err != nil {
			return nil, nil, err
		}
		synthRefs = append(synthRefs, stepplan.InputRef{
			Key:  "package",
			Hash: hash,
			URI:  p,
		})
	}
	steps = append(steps, stepplan.Step{
		ID:        ledgerSynthesisStepID,
		Goal:      fmt.Sprintf("Synthesize grounded objective for slice %s", sliceID),
		InputRefs: synthRefs,
		Budget: stepplan.Budget{
			Timeout:  ledgerSynthesisTimeout,
			MaxChars: ledgerMaxContextChars,
		},
		DoneCriteria: stepplan.DoneCriteria{
			RequiredKeys: []string{"id", "objective", "verdict", "evidence"},
		},
	})
	return steps, pathByStep, nil
}

func packageEvidenceInputHash(analysisDir, pkgPath, wholeContext string) (string, error) {
	ctxMD := packageRLMContextSnippet(wholeContext, pkgPath)
	if strings.TrimSpace(ctxMD) == "" && strings.TrimSpace(analysisDir) != "" {
		built, err := typroles.FormatPackageRLMContextForPath(analysisDir, pkgPath, nil)
		if err == nil {
			ctxMD = built
		}
	}
	sum := sha256.Sum256([]byte(pkgPath + "\n" + ctxMD))
	return hex.EncodeToString(sum[:16]), nil
}

func ledgerEvidenceStepID(pkgPath string) string {
	sum := sha256.Sum256([]byte(pkgPath))
	return ledgerEvidenceStepPref + hex.EncodeToString(sum[:8])
}

func ledgerPlanID(sliceID string) string {
	var b strings.Builder
	b.WriteString("ledger_")
	for _, r := range strings.TrimSpace(sliceID) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	id := b.String()
	if id == "ledger_" {
		return "ledger_slice"
	}
	return id
}

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

func (r *ledgerStepRunner) RunStep(ctx context.Context, plan *stepplan.Plan, step stepplan.Step) (*orchestration.StepRunResult, error) {
	if r == nil || r.caller == nil {
		return nil, fmt.Errorf("ledger step runner is nil")
	}
	if step.ID == ledgerSynthesisStepID {
		return r.runSynthesis(ctx, plan, step)
	}
	return r.runEvidence(ctx, step)
}

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

func (r *ledgerStepRunner) loadEvidenceNotes(ctx context.Context, plan *stepplan.Plan) ([]packageEvidenceNote, error) {
	notes := make([]packageEvidenceNote, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		if step.ID == ledgerSynthesisStepID {
			continue
		}
		cp, err := r.store.LoadStep(ctx, plan.ID, step.EffectiveCheckpointKey())
		if err != nil {
			if errors.Is(err, stepplan.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if cp.Status != stepplan.StepStatusComplete || len(cp.Output) == 0 {
			continue
		}
		var note packageEvidenceNote
		if err := json.Unmarshal(cp.Output, &note); err != nil {
			return nil, fmt.Errorf("decode evidence checkpoint %s: %w", step.ID, err)
		}
		if strings.TrimSpace(note.Path) == "" {
			if p := r.pathByStep[step.ID]; p != "" {
				note.Path = p
			}
		}
		notes = append(notes, note)
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].Path < notes[j].Path })
	return notes, nil
}

// finalizeLedgerEntryFromAnswer applies the same post-synthesis rules as the pre-stepplan path.
func finalizeLedgerEntryFromAnswer(
	t ledgerSliceTarget,
	answer string,
	byPath map[string]packageCapabilityConstraint,
	rolesDoc packageRolesDoc,
	req sliceLedgerBuildRequest,
	promptTok, completionTok, totalTok int,
) (sliceObjectiveLedgerEntry, string, error) {
	evidence, claims, objective, verdict, err := parseSliceObjectiveLedgerAnswer(answer)
	if err != nil {
		return sliceObjectiveLedgerEntry{}, fmt.Sprintf(
			"%s: slice %q objective ledger parse failed: %v",
			typologypack.CriterionIDRoleGrounding, t.id, err,
		), nil
	}
	unclaimedOK := sliceAllCapabilityCodesMustNot(t.paths, byPath)
	if verdict == ledgerVerdictOverclaim {
		if !unclaimedOK {
			return sliceObjectiveLedgerEntry{}, fmt.Sprintf(
				"%s: slice %q objective overclaims; cite package evidence or simplify the meaning",
				typologypack.CriterionIDRoleGrounding, t.id,
			), nil
		}
		if strings.TrimSpace(objective) == "" {
			objective = ledgerUnclaimedObjective
		}
		if len(evidence) == 0 {
			evidence = []string{"role:unknown"}
		}
		claims = nil
		verdict = ledgerVerdictGrounded
	}
	if verdict == ledgerVerdictGrounded && len(claims) == 0 {
		if !unclaimedOK {
			return sliceObjectiveLedgerEntry{}, fmt.Sprintf(
				"%s: slice %q grounded answer missing claims",
				typologypack.CriterionIDRoleGrounding, t.id,
			), nil
		}
		if strings.TrimSpace(objective) == "" {
			objective = ledgerUnclaimedObjective
		}
	}
	source := ledgerSourceRLM
	dropped := claimsNotAllowedByOwnedIs(claims, t.paths, byPath)
	claims = filterClaimsToOwnedIs(claims, t.paths, byPath)
	if verdict == ledgerVerdictGrounded && len(claims) == 0 {
		if !unclaimedOK {
			msg := fmt.Sprintf(
				"%s: slice %q has no claims allowed by owned package is=[]",
				typologypack.CriterionIDRoleGrounding, t.id,
			)
			if len(dropped) > 0 {
				msg = fmt.Sprintf("%s (dropped %v not allowed by owned package is=)", msg, dropped)
			}
			return sliceObjectiveLedgerEntry{}, msg, nil
		}
		if strings.TrimSpace(objective) == "" {
			objective = ledgerUnclaimedObjective
		}
		source = ledgerSourceUnclaimed
	} else if len(claims) == 0 {
		source = ledgerSourceUnclaimed
	}
	entry := sliceObjectiveLedgerEntry{
		ID:         t.id,
		OwnedPaths: append([]string(nil), t.paths...),
		Evidence:   evidence,
		Claims:     claims,
		Objective:  objective,
		Verdict:    verdict,
		Source:     source,
	}
	if entailIssues := rejectUnentailedClaims(t.id, claims, t.paths, req.Constraints, rolesDoc); len(entailIssues) > 0 {
		return sliceObjectiveLedgerEntry{}, strings.Join(entailIssues, "\n"), nil
	}
	if req.DigestCache != nil && entry.Verdict == ledgerVerdictGrounded {
		constraintBlock := formatConstraintRowsForPaths(t.paths, byPath)
		ownedSrc, srcErr := cache.OwnedPackagesSourceHash(req.AnalysisDir, t.paths)
		if srcErr != nil {
			return sliceObjectiveLedgerEntry{}, fmt.Sprintf(
				"%s: slice %q owned package source hash failed: %v",
				typologypack.CriterionIDRoleGrounding, t.id, srcErr,
			), nil
		}
		fp := cache.LedgerFingerprint{
			SliceID:         t.id,
			OwnedPathsHash:  cache.OwnedPathsHash(t.paths),
			ContextSHA:      ownedSrc,
			ConstraintsHash: cache.ContentSHA(constraintBlock),
			ModelID:         req.DigestModelID,
			PromptVersion:   cache.DigestLedgerPromptV1,
			SchemaVersion:   cache.DigestLedgerSchemaV2,
		}
		if err := req.DigestCache.StoreLedger(fp, cache.LedgerCachedEntry{
			ID:               entry.ID,
			OwnedPaths:       append([]string(nil), entry.OwnedPaths...),
			Evidence:         append([]string(nil), entry.Evidence...),
			Claims:           append([]string(nil), entry.Claims...),
			Objective:        entry.Objective,
			Verdict:          entry.Verdict,
			Source:           entry.Source,
			PromptTokens:     promptTok,
			CompletionTokens: completionTok,
			TotalTokens:      totalTok,
		}); err != nil {
			return sliceObjectiveLedgerEntry{}, "", fmt.Errorf("store slice %q objective ledger cache: %w", t.id, err)
		}
	}
	return entry, "", nil
}

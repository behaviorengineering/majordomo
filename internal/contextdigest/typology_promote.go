package contextdigest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/majordomo/internal/config"
	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/typology/catalog"
	"gopkg.in/yaml.v3"
)

const (
	confirmedCatalogRel = ".typology/typology.yaml"
	typologyEvidenceDir = "evidence/typology"
	typologyManifestRel = "manifest.yaml"
)

// typologyPromoteDecision is the pure outcome of comparing confirmed vs refined catalogs.
type typologyPromoteDecision struct {
	ShouldPromote bool
	SkipReason    string
	Refined       catalog.Typology
}

// decideTypologyPromote loads confirmed and refined catalogs and decides whether to open a product PR.
// mode must be contextstore.TypologyModeReuse for a promote; other modes skip.
func decideTypologyPromote(mode, confirmedPath, refinedPath string) (typologyPromoteDecision, error) {
	mode = strings.TrimSpace(mode)
	if mode != contextstore.TypologyModeReuse {
		return typologyPromoteDecision{SkipReason: "mode_" + mode}, nil
	}
	if strings.TrimSpace(confirmedPath) == "" {
		return typologyPromoteDecision{SkipReason: "missing_confirmed"}, nil
	}
	if strings.TrimSpace(refinedPath) == "" {
		return typologyPromoteDecision{SkipReason: "missing_refined"}, nil
	}
	if _, err := os.Stat(confirmedPath); err != nil {
		if os.IsNotExist(err) {
			return typologyPromoteDecision{SkipReason: "missing_confirmed"}, nil
		}
		return typologyPromoteDecision{}, fmt.Errorf("typology promote: stat confirmed: %w", err)
	}
	if _, err := os.Stat(refinedPath); err != nil {
		if os.IsNotExist(err) {
			return typologyPromoteDecision{SkipReason: "missing_refined"}, nil
		}
		return typologyPromoteDecision{}, fmt.Errorf("typology promote: stat refined: %w", err)
	}
	confirmed, err := catalog.LoadYAML(confirmedPath)
	if err != nil {
		return typologyPromoteDecision{}, fmt.Errorf("typology promote: load confirmed: %w", err)
	}
	refined, err := catalog.LoadYAML(refinedPath)
	if err != nil {
		return typologyPromoteDecision{}, fmt.Errorf("typology promote: load refined: %w", err)
	}
	equal, err := catalogsNormalizedEqual(confirmed, refined)
	if err != nil {
		return typologyPromoteDecision{}, err
	}
	if equal {
		return typologyPromoteDecision{SkipReason: "catalogs_equal", Refined: refined}, nil
	}
	return typologyPromoteDecision{ShouldPromote: true, Refined: refined}, nil
}

func catalogsNormalizedEqual(a, b catalog.Typology) (bool, error) {
	da, err := yaml.Marshal(&a)
	if err != nil {
		return false, fmt.Errorf("typology promote: marshal confirmed: %w", err)
	}
	db, err := yaml.Marshal(&b)
	if err != nil {
		return false, fmt.Errorf("typology promote: marshal refined: %w", err)
	}
	return bytes.Equal(da, db), nil
}

// typologyPromotePRBody builds the product PR description for a confirmed-catalog promote.
func typologyPromotePRBody(repoID, defaultHEAD, contextPR string, findings []string, priorityMD, interventionMD string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Typology catalog promote\n\n")
	fmt.Fprintf(&b, "Majordomo digest found that the confirmed `.typology/typology.yaml` on default ")
	fmt.Fprintf(&b, "differs from this run's refined proposal. This PR writes the refined catalog ")
	fmt.Fprintf(&b, "into the confirmed path so the living map matches the digest lean.\n\n")
	fmt.Fprintf(&b, "- **Repo:** `%s`\n", strings.TrimSpace(repoID))
	if h := strings.TrimSpace(defaultHEAD); h != "" {
		fmt.Fprintf(&b, "- **Default HEAD:** `%s`\n", h)
	}
	if pr := strings.TrimSpace(contextPR); pr != "" {
		fmt.Fprintf(&b, "- **Context update PR:** %s\n", pr)
	}
	fmt.Fprintf(&b, "\nDigest does **not** auto-merge this PR. Review the catalog diff, then merge when ready.\n")

	if len(findings) > 0 {
		fmt.Fprintf(&b, "\n### Architecture findings\n\n")
		for i, f := range findings {
			fmt.Fprintf(&b, "%d. %s\n", i+1, strings.TrimSpace(f))
		}
	}
	if p := strings.TrimSpace(priorityMD); p != "" {
		fmt.Fprintf(&b, "\n### Priority counsel\n\n%s\n", p)
	}
	if iv := strings.TrimSpace(interventionMD); iv != "" {
		// Keep the product PR readable; intervention briefs can be long.
		const max = 4000
		if len(iv) > max {
			iv = iv[:max] + "\n\n…(truncated)"
		}
		fmt.Fprintf(&b, "\n### Human intervention excerpt\n\n%s\n", iv)
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func loadTypologyManifestFromContext(ctxDir string) (contextstore.TypologyManifest, error) {
	path := filepath.Join(ctxDir, typologyEvidenceDir, typologyManifestRel)
	return contextstore.ParseTypologyManifest(path)
}

func refinedSnapshotPath(ctxDir string, m contextstore.TypologyManifest) string {
	rel := strings.TrimSpace(m.RefinedSnapshotPath)
	if rel == "" {
		rel = refinedSnapshotRel
	}
	return filepath.Join(ctxDir, typologyEvidenceDir, filepath.Base(rel))
}

func loadArchitectureFindingsFromContext(ctxDir string) []string {
	brief := filepath.Join(ctxDir, typologyEvidenceDir, contextstore.TypologyArchitectureBriefPath)
	data, err := os.ReadFile(brief)
	if err != nil {
		return nil
	}
	return extractArchitectureFindings(string(data))
}

func loadHumanInterventionMarkdown(ctxDir string) string {
	path := filepath.Join(ctxDir, typologyEvidenceDir, "human_intervention.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// promoteConfirmedTypologyResult is the forge outcome of a promote attempt.
type promoteConfirmedTypologyResult struct {
	PR         string
	SkipReason string
}

// promoteConfirmedTypology opens or restacks a product PR that writes the refined
// catalog into `.typology/typology.yaml` on the default branch.
// It never auto-merges. Callers must skip local-seed / resume paths before invoking.
func promoteConfirmedTypology(p finishParams, contextPR string) (promoteConfirmedTypologyResult, error) {
	if p.forge == nil {
		return promoteConfirmedTypologyResult{SkipReason: "no_forge"}, nil
	}
	if p.servedGit == nil {
		return promoteConfirmedTypologyResult{SkipReason: "no_served_git"}, nil
	}
	defaultBranch := strings.TrimSpace(p.defaultBranch)
	if defaultBranch == "" {
		return promoteConfirmedTypologyResult{SkipReason: "no_default_branch"}, nil
	}

	manifest, err := loadTypologyManifestFromContext(p.ctxDir)
	if err != nil {
		// Soft skip: catch-up without typology evidence should not fail digest finish.
		logf("INFO", "typology_promote=skipped reason=no_manifest err=%v", err)
		return promoteConfirmedTypologyResult{SkipReason: "no_manifest"}, nil
	}

	confirmedPath := filepath.Join(p.servedGit.Dir, confirmedCatalogRel)
	refinedPath := refinedSnapshotPath(p.ctxDir, manifest)
	decision, err := decideTypologyPromote(manifest.Mode, confirmedPath, refinedPath)
	if err != nil {
		return promoteConfirmedTypologyResult{}, err
	}
	if !decision.ShouldPromote {
		logf("INFO", "typology_promote=skipped reason=%s", decision.SkipReason)
		return promoteConfirmedTypologyResult{SkipReason: decision.SkipReason}, nil
	}

	updateBranch := config.TypologyPromoteUpdateBranch(p.cfg.Repository.ID)
	tmp, err := os.MkdirTemp("", "majordomo-typology-promote-*")
	if err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	g := &Git{Dir: tmp, Token: p.token, SCM: p.scm}
	if _, err := g.run("init"); err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: git init: %w", err)
	}
	if err := configureCommitIdentity(g); err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: commit identity: %w", err)
	}
	remote, err := p.servedGit.trim("remote", "get-url", "origin")
	if err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: origin url: %w", err)
	}
	if err := ensureRemote(g, remote); err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: ensure remote: %w", err)
	}
	if err := FetchOrigin(g, defaultBranch+":"+defaultBranch); err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: fetch default: %w", err)
	}
	// Recreate head from current default so restacks stay a clean catalog promote.
	if _, err := g.run("checkout", "-B", updateBranch, defaultBranch); err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: checkout update branch: %w", err)
	}

	outPath := filepath.Join(tmp, confirmedCatalogRel)
	if err := catalog.SaveYAML(outPath, decision.Refined); err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: write catalog: %w", err)
	}
	committed, err := CommitAll(g, "Promote refined typology catalog into .typology/typology.yaml.")
	if err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: commit: %w", err)
	}
	if !committed {
		logf("INFO", "typology_promote=skipped reason=clean_tree")
		return promoteConfirmedTypologyResult{SkipReason: "clean_tree"}, nil
	}
	if err := PushForce(g, updateBranch); err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: push: %w", err)
	}

	findings := loadArchitectureFindingsFromContext(p.ctxDir)
	body := typologyPromotePRBody(
		p.cfg.Repository.ID,
		p.defaultHEAD,
		contextPR,
		findings,
		loadPRPriorityMarkdown(p.ctxDir),
		loadHumanInterventionMarkdown(p.ctxDir),
	)
	title := fmt.Sprintf("Promote typology catalog: %s", p.cfg.Repository.ID)
	pr, err := p.forge.OpenUpdatePR(defaultBranch, updateBranch, title, body)
	if err != nil {
		return promoteConfirmedTypologyResult{}, fmt.Errorf("typology promote: open PR: %w", err)
	}
	logf("INFO", "typology_promote=opened_or_restacked pr=%s branch=%s", pr, updateBranch)
	return promoteConfirmedTypologyResult{PR: pr}, nil
}

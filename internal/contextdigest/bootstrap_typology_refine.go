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
	"github.com/behaviorengineering/majordomo/internal/contextstore"
	"github.com/behaviorengineering/majordomo/internal/judge"
	typologypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/typology"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	"github.com/behaviorengineering/strop/pkg/runreport"
	"github.com/behaviorengineering/typology/pkg/catalog"
	"gopkg.in/yaml.v3"
)

const (
	analysisDraftCatalogRel   = "tmp/typology/typology.yaml"
	analysisDraftArchRel      = "tmp/typology/architecture_draft.md"
	maxTypologyRefineAttempts = 3
	maxReadmeSnapshotRunes    = 12000
)

// TypologySlicePipeline runs unattended slice grouping, meaning, and catalog assemble.
type TypologySlicePipeline interface {
	Assemble(ctx context.Context, input TypologySlicePipelineInput) (TypologySlicePipelineOutput, error)
}

// TypologySlicePipelineInput is the evidence pack for slice grouping / meaning / catalog.
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

// TypologySlicePipelineOutput is the assembled catalog and durable grouping/meaning evidence YAML.
// Journey markdown is written later by human-intervention (after architecture).
type TypologySlicePipelineOutput struct {
	MechanicalGroupingYAML   string
	ClusterMergeProposalYAML string
	ClusterMergeVerdictsYAML string
	RefinedCatalogYAML       string
	ObjectiveLedgerYAML      string
	ObjectiveClaimsYAML      string
}

// JudgeTypologySlicePipeline runs slice grouping CoT, then meaning + per-slice catalog RLM assemble.
type JudgeTypologySlicePipeline struct {
	Gen judge.Generator
}

// Assemble runs grouping, then meaning ledger, then per-slice catalog RLM→join→sanitize→gate→LLM eval.
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
				return TypologySlicePipelineOutput{}, fmt.Errorf("typology cluster merge fields failed after %d attempts: %w", maxTypologyRefineAttempts, parseErr)
			}
			clusterFeedback = parseErr.Error()
			continue
		}
		merges = scrubForbiddenHTTPEntrypointMergeRows(merges, input.PackageRoles)
		pending, frozen := sticky.applySticky(merges)
		if len(pending) > 0 && auditor == nil {
			return TypologySlicePipelineOutput{}, fmt.Errorf("typology_slice_grouping_audit RLM is required for proposed merges but no cluster auditor is configured")
		}
		constraintsForAudit := stringField(clusterFields, "package_capability_constraints")
		auditStart := time.Now()
		audited := clusterAuditResult{Verdicts: append([]clusterMergeVerdict(nil), frozen...), TraceDir: auditMeta.TraceDir}
		if len(pending) > 0 {
			var auditErr error
			audited, auditErr = auditor.Audit(ctx, clusterAuditRequest{
				AnalysisDir:    input.AnalysisDir,
				EvidenceDir:    input.EvidenceDir,
				Proposed:       pending,
				Frozen:         frozen,
				RolesYAML:      input.PackageRoles,
				Constraints:    constraintsForAudit,
				MechanicalYAML: mechanicalGroupingYAML,
				DigestCache:    input.DigestCache,
				DigestSkips:    input.DigestSkips,
				DigestModelID:  input.DigestModelID,
				Attempt:        attempt,
			})
			if auditErr != nil {
				if attempt == maxTypologyRefineAttempts {
					return TypologySlicePipelineOutput{}, fmt.Errorf("typology cluster audit failed after %d attempts: %w", maxTypologyRefineAttempts, auditErr)
				}
				clusterFeedback = auditErr.Error()
				continue
			}
		} else if audited.Duration == 0 {
			audited.Duration = time.Since(auditStart)
		}
		for _, v := range audited.Verdicts {
			sticky.put(v)
		}
		mergeVerdicts = audited.Verdicts
		auditMeta = audited
		proposedMerges = merges
		if hasOpenClusterRejects(mergeVerdicts) && attempt < maxTypologyRefineAttempts {
			clusterFeedback = formatClusterAuditRejectFeedback(mergeVerdicts)
			continue
		}
		if !usedClusterCache && input.DigestCache != nil {
			ids, pkgs, intents := flattenMergesForCache(proposedMerges)
			if err := input.DigestCache.StoreClusterCoT(clusterFP, cache.ClusterCoTCached{
				MergeIDs:      ids,
				MergePackages: pkgs,
				MergeIntents:  intents,
			}); err != nil {
				logf("WARN", "digest cache store cluster_cot failed: %v", err)
			}
		}
		break
	}
	proposedMerges = demoteRejectedMergesList(proposedMerges, mergeVerdicts)
	proposalYAML, err := marshalClusterMergeProposal(proposedMerges)
	if err != nil {
		return TypologySlicePipelineOutput{}, err
	}
	verdictsDoc := clusterMergeVerdictsDoc{
		Attempt:       maxTypologyRefineAttempts,
		RLMIterations: auditMeta.RLMIterations,
		DurationMS:    auditMeta.Duration.Milliseconds(),
		PromptTokens:  auditMeta.PromptTokens,
		CompletionTok: auditMeta.CompletionTok,
		TotalTokens:   auditMeta.TotalTokens,
		ProposedRows:  len(mergeVerdicts),
		TraceDir:      auditMeta.TraceDir,
		Merges:        mergeVerdicts,
	}
	verdictsYAML, err := marshalClusterMergeVerdicts(verdictsDoc)
	if err != nil {
		return TypologySlicePipelineOutput{}, err
	}

	var refined string
	var ledgerYAML string
	var claimsYAML string
	feedback := input.ValidationFeedback
	constraintsYAML := input.CapabilityConstraints
	if strings.TrimSpace(constraintsYAML) == "" {
		encoded, err := marshalConstraints(constraintsDoc)
		if err != nil {
			return TypologySlicePipelineOutput{}, err
		}
		constraintsYAML = encoded
	}

	draftTypo, err := loadTypologyFromYAML(input.DraftCatalogYAML)
	if err != nil {
		return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine load draft catalog: %w", err)
	}
	needsLedger := false
	for _, s := range draftTypo.Slices {
		if len(slicePackagePaths(s)) > 0 {
			needsLedger = true
			break
		}
	}
	if needsLedger && input.LedgerBuilder == nil {
		return TypologySlicePipelineOutput{}, fmt.Errorf("%s: slice_objective_ledger RLM is required for owned packages but no ledger builder is configured",
			typologypack.CriterionIDRoleGrounding)
	}

	var ledgerDoc sliceObjectiveLedgerDoc
	if needsLedger {
		clusterForLedger := strings.TrimSpace(proposalYAML)
		if strings.TrimSpace(verdictsYAML) != "" {
			clusterForLedger = strings.TrimSpace(clusterForLedger) + "\n\n## Cluster merge verdicts\n\n" + strings.TrimSpace(verdictsYAML) + "\n"
		}
		built, issues, buildErr := input.LedgerBuilder.BuildSliceLedger(ctx, sliceLedgerBuildRequest{
			AnalysisDir:     input.AnalysisDir,
			EvidenceDir:     input.EvidenceDir,
			DraftTypo:       draftTypo,
			Constraints:     constraintsDoc,
			ClusterHintYAML: clusterForLedger,
			DigestCache:     input.DigestCache,
			DigestSkips:     input.DigestSkips,
			DigestModelID:   input.DigestModelID,
		})
		if buildErr != nil {
			return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine objective ledger failed: %w", buildErr)
		}
		if len(issues) > 0 {
			return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine objective ledger failed after %d attempts:\n%s",
				maxTypologyRefineAttempts, strings.Join(issues, "\n"))
		}
		ledgerDoc = built
		var err error
		ledgerYAML, err = marshalLedger(ledgerDoc)
		if err != nil {
			return TypologySlicePipelineOutput{}, err
		}
		claimsYAML, err = marshalClaims(claimsDocFromLedger(ledgerDoc))
		if err != nil {
			return TypologySlicePipelineOutput{}, err
		}
	}

	refineFP := cache.RefineFingerprint{
		DraftHash:       draftCatalogIdentitySHA(input.DraftCatalogYAML),
		RolesHash:       rolesIdentitySHA(input.PackageRoles),
		ConstraintsHash: cache.ContentSHA(constraintsYAML),
		LedgerHash:      cache.ContentSHA(ledgerYAML),
		VerdictsHash:    clusterVerdictsIdentitySHA(verdictsYAML),
		MechanicalHash:  mechIdentityHash,
		ModelID:         input.DigestModelID,
		PromptVersion:   cache.DigestRefinePromptV2,
		SchemaVersion:   cache.DigestRefineSchemaV5,
	}
	if input.DigestSkips && input.DigestCache != nil && strings.TrimSpace(feedback) == "" {
		if hit, ok, err := input.DigestCache.LookupRefine(refineFP); err == nil && ok && strings.TrimSpace(hit.RefinedCatalogYAML) != "" {
			if sanitized, sanErr := validateRefinedCatalogYAMLWithGraph(hit.RefinedCatalogYAML, input.DraftCatalogYAML, input.RepoID, input.PackageRoles, input.GraphText); sanErr == nil {
				input.DigestCache.RecordRefineHit(hit.PromptTokens, hit.CompletionTokens, hit.TotalTokens)
				logf("INFO", "digest cache hit refine")
				refined = sanitized
				return TypologySlicePipelineOutput{
					ClusterMergeProposalYAML: proposalYAML,
					ClusterMergeVerdictsYAML: verdictsYAML,
					MechanicalGroupingYAML:   mechanicalGroupingYAML,
					RefinedCatalogYAML:       refined,
					ObjectiveLedgerYAML:      ledgerYAML,
					ObjectiveClaimsYAML:      claimsYAML,
				}, nil
			}
		}
	}

	foldedTypo, foldErr := applyAcceptedMerges(draftTypo, mergeVerdicts)
	if foldErr != nil {
		return TypologySlicePipelineOutput{}, fmt.Errorf("typology slice catalog apply accepts: %w", foldErr)
	}
	needsCatalogRLM := len(catalogAssembleTargets(foldedTypo)) > 0
	if needsCatalogRLM && input.CatalogAssembler == nil {
		return TypologySlicePipelineOutput{}, fmt.Errorf("typology_slice_catalog RLM is required for owned packages but no catalog assembler is configured")
	}

	keptFragments := map[string]catalog.Slice{}
	for attempt := 1; attempt <= maxTypologyRefineAttempts; attempt++ {
		if input.DigestCache != nil && attempt == 1 {
			input.DigestCache.RecordRefineMiss()
		}
		refineFields := map[string]interface{}{
			"repo_id":                        input.RepoID,
			"module_scope":                   input.ModuleScope,
			"draft_catalog_yaml":             input.DraftCatalogYAML,
			"slice_grouping_proposal_yaml":   proposalYAML,
			"slice_grouping_verdicts_yaml":   verdictsYAML,
			"package_contracts":              input.PackageContracts,
			"package_roles":                  input.PackageRoles,
			"package_capability_constraints": constraintsYAML,
			"slice_meaning_ledger_yaml":      ledgerYAML,
			"architecture_draft":             input.ArchitectureDraft,
			"repo_layout":                    input.RepoLayout,
			"readme_snapshot":                input.ReadmeSnapshot,
			"validation_feedback":            feedback,
		}

		joined := foldedTypo
		if needsCatalogRLM {
			assembled, assembleErr := input.CatalogAssembler.AssembleSlices(ctx, sliceCatalogAssembleRequest{
				FoldedTypo:         foldedTypo,
				LedgerDoc:          ledgerDoc,
				EvidenceDir:        input.EvidenceDir,
				RolesYAML:          input.PackageRoles,
				ConstraintsYAML:    constraintsYAML,
				ReadmeSnapshot:     input.ReadmeSnapshot,
				ValidationFeedback: feedback,
				Kept:               keptFragments,
			})
			if assembleErr != nil {
				return TypologySlicePipelineOutput{}, fmt.Errorf("typology slice catalog RLM: %w", assembleErr)
			}
			if len(assembled.Issues) > 0 {
				for _, frag := range assembled.Fragments {
					if id := strings.TrimSpace(frag.ID); id != "" {
						keptFragments[id] = frag
					}
				}
				fb := strings.Join(assembled.Issues, "\n")
				if attempt == maxTypologyRefineAttempts {
					return TypologySlicePipelineOutput{}, fmt.Errorf("typology slice catalog RLM failed after %d attempts:\n%s", maxTypologyRefineAttempts, fb)
				}
				feedback = fb
				continue
			}
			keptFragments = map[string]catalog.Slice{}
			for _, frag := range assembled.Fragments {
				keptFragments[strings.TrimSpace(frag.ID)] = frag
			}
			var joinErr error
			joined, joinErr = joinSliceCatalog(foldedTypo, assembled.Fragments)
			if joinErr != nil {
				if attempt == maxTypologyRefineAttempts {
					return TypologySlicePipelineOutput{}, joinErr
				}
				feedback = joinErr.Error()
				continue
			}
		}
		if needsLedger {
			joined = stampLedgerObjectivesOntoCatalog(joined, ledgerDoc)
		}
		encoded, encErr := yaml.Marshal(&joined)
		if encErr != nil {
			return TypologySlicePipelineOutput{}, fmt.Errorf("typology slice catalog join encode: %w", encErr)
		}
		refined = string(encoded)
		sanitized, err := validateRefinedCatalogYAMLWithGraph(refined, input.DraftCatalogYAML, input.RepoID, input.PackageRoles, input.GraphText)
		if err != nil {
			if attempt == maxTypologyRefineAttempts {
				return TypologySlicePipelineOutput{}, err
			}
			feedback = err.Error()
			keptFragments = map[string]catalog.Slice{}
			continue
		}
		refined = sanitized
		if ok, evalFeedback := evaluateTypologyCatalogBoundaries(refined, input.PackageRoles); !ok {
			if attempt == maxTypologyRefineAttempts {
				return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine evaluation failed after %d attempts:\n%s", maxTypologyRefineAttempts, evalFeedback)
			}
			feedback = evalFeedback
			keptFragments = map[string]catalog.Slice{}
			continue
		}
		typo, err := loadTypologyFromYAML(refined)
		if err != nil {
			if attempt == maxTypologyRefineAttempts {
				return TypologySlicePipelineOutput{}, err
			}
			feedback = err.Error()
			keptFragments = map[string]catalog.Slice{}
			continue
		}
		if membershipIssues := assertAcceptedMembership(typo, draftTypo, mergeVerdicts); len(membershipIssues) > 0 {
			fb := strings.Join(membershipIssues, "\n")
			if attempt < maxTypologyRefineAttempts {
				feedback = fb
				keptFragments = map[string]catalog.Slice{}
				continue
			}
			split, changed := splitIllegalMembershipToDraftOwners(typo, draftTypo, mergeVerdicts)
			if !changed {
				return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine membership gate failed after %d attempts:\n%s", maxTypologyRefineAttempts, fb)
			}
			encoded, encErr := yaml.Marshal(&split)
			if encErr != nil {
				return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine membership split encode: %w", encErr)
			}
			refined = string(encoded)
			typo = split
		}
		if needsLedger {
			aligned, alignedClaims, alignIssues := alignLedgerToRefinedCatalog(typo, ledgerDoc, constraintsDoc)
			if len(alignIssues) > 0 {
				fb := strings.Join(alignIssues, "\n")
				if attempt == maxTypologyRefineAttempts {
					return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine evaluation failed after %d attempts:\n%s", maxTypologyRefineAttempts, fb)
				}
				feedback = fb
				keptFragments = map[string]catalog.Slice{}
				continue
			}
			ledgerDoc = aligned
			var marshalErr error
			ledgerYAML, marshalErr = marshalLedger(ledgerDoc)
			if marshalErr != nil {
				return TypologySlicePipelineOutput{}, marshalErr
			}
			claimsDoc := alignedClaims
			claimsYAML, marshalErr = marshalClaims(claimsDoc)
			if marshalErr != nil {
				return TypologySlicePipelineOutput{}, marshalErr
			}
			claimIssues := appendConstraintClaimIssues(typo, constraintsDoc, claimsDoc, nil)
			if len(claimIssues) > 0 {
				fb := strings.Join(claimIssues, "\n")
				if attempt == maxTypologyRefineAttempts {
					return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine evaluation failed after %d attempts:\n%s", maxTypologyRefineAttempts, fb)
				}
				feedback = fb
				keptFragments = map[string]catalog.Slice{}
				continue
			}
		}
		evalOut := map[string]interface{}{
			"refined_catalog_yaml":      refined,
			"slice_meaning_ledger_yaml": ledgerYAML,
			"objective_claims_yaml":     claimsYAML,
		}
		agg, err := gen.Evaluate(ctx, jmodules.TaskTypologySliceCatalog, refineFields, evalOut, attempt)
		if err != nil {
			if attempt == maxTypologyRefineAttempts {
				return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine LLM evaluation: %w", err)
			}
			feedback = err.Error()
			keptFragments = map[string]catalog.Slice{}
			continue
		}
		if !judge.EvalPassed(agg) {
			if attempt == maxTypologyRefineAttempts {
				return TypologySlicePipelineOutput{}, fmt.Errorf("typology refine LLM evaluation failed after %d attempts:\n%s", maxTypologyRefineAttempts, judge.EvalFeedback(agg))
			}
			feedback = judge.EvalFeedback(agg)
			keptFragments = map[string]catalog.Slice{}
			continue
		}
		if input.DigestCache != nil {
			if err := input.DigestCache.StoreRefine(refineFP, cache.RefineCached{RefinedCatalogYAML: refined}); err != nil {
				logf("WARN", "digest cache store refine failed: %v", err)
			}
		}
		break
	}

	return TypologySlicePipelineOutput{
		MechanicalGroupingYAML:   mechanicalGroupingYAML,
		ClusterMergeProposalYAML: proposalYAML,
		ClusterMergeVerdictsYAML: verdictsYAML,
		RefinedCatalogYAML:       refined,
		ObjectiveLedgerYAML:      ledgerYAML,
		ObjectiveClaimsYAML:      claimsYAML,
	}, nil
}

// evaluateTypologyBoundaries applies deterministic typology hard-fails before LLM EvaluateWorkflow.
// Journey debt checks apply when journeyMD is non-empty (human-intervention path).
func evaluateTypologyBoundaries(refinedYAML, journeyMD, architectureDraft, rolesYAML string) (bool, string) {
	ok, catalogIssues := evaluateTypologyCatalogBoundaries(refinedYAML, rolesYAML)
	var issues []string
	if !ok {
		issues = append(issues, strings.Split(catalogIssues, "\n")...)
	}

	if architectureHasFindings(architectureDraft) && strings.TrimSpace(journeyMD) != "" && !journeyHasDebtTable(journeyMD) {
		issues = append(issues, fmt.Sprintf("%s: architecture findings remain but journey has no technical debt / boundary violations table", typologypack.CriterionIDDebtWhenFindings))
	}
	if journeyStatusClaimsComplete(journeyMD) && journeyDebtStillSaysMerge(journeyMD) {
		issues = append(issues, fmt.Sprintf(
			"%s: journey Status claims complete but debt still lists Merge into actions; set Status to Open while Merge into rows remain, or remove those Merge into rows if the catalog already reflects the merges",
			typologypack.CriterionIDJourneyConsistent,
		))
	}

	if len(issues) == 0 {
		return true, ""
	}
	return false, strings.Join(issues, "\n")
}

// evaluateTypologyCatalogBoundaries checks catalog-only refine gates (no journey yet).
func evaluateTypologyCatalogBoundaries(refinedYAML, rolesYAML string) (bool, string) {
	var issues []string

	tmp, err := os.CreateTemp("", "majordomo-eval-*.yaml")
	if err != nil {
		return false, fmt.Sprintf("%s: temp file: %v", typologypack.CriterionIDObjectives, err)
	}
	path := tmp.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := tmp.WriteString(refinedYAML); err != nil {
		_ = tmp.Close()
		return false, fmt.Sprintf("%s: write temp: %v", typologypack.CriterionIDObjectives, err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Sprintf("%s: close temp: %v", typologypack.CriterionIDObjectives, err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		return false, fmt.Sprintf("%s: load catalog: %v", typologypack.CriterionIDObjectives, err)
	}
	roles := roleByPath(mustParseRoles(rolesYAML))

	for _, s := range typo.Slices {
		obj := strings.TrimSpace(s.Objective)
		if obj == "" {
			issues = append(issues, fmt.Sprintf("%s: slice %q missing objective", typologypack.CriterionIDObjectives, s.ID))
		} else if isHollowObjective(obj) {
			issues = append(issues, fmt.Sprintf("%s: slice %q has hollow template objective %q", typologypack.CriterionIDObjectives, s.ID, obj))
		}
		for _, c := range s.Owns {
			if looksLikeInteractionPath(c.Path, roles) {
				issues = append(issues, fmt.Sprintf("%s: package %q on slice %q has observed role %s and must sit under surfaces[]", typologypack.CriterionIDSurfaces, c.Path, s.ID, roles[normalizeRolePath(c.Path)].Role))
			}
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				if isExecAdapterPath(c.Path, roles) && surf.Kind == catalog.InteractionCLI {
					issues = append(issues, fmt.Sprintf("%s: package %q is an exec_runner and must not sit under kind: cli surface %q", typologypack.CriterionIDAdapterSurfaces, c.Path, surf.ID))
				}
			}
		}
	}

	issues = appendEvidenceGroundingIssues(typo, issues)

	if len(issues) == 0 {
		return true, ""
	}
	return false, strings.Join(issues, "\n")
}

var hollowObjectiveRe = regexp.MustCompile(`(?i)^provide\s+.+\s+(functionality|capabilities|services)\.?$`)

func isHollowObjective(objective string) bool {
	return hollowObjectiveRe.MatchString(strings.TrimSpace(objective))
}

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

func isExecAdapterPath(path string, roles map[string]packageRoleNode) bool {
	n, ok := roles[normalizeRolePath(path)]
	if !ok {
		return false
	}
	return isExecRunnerRole(n.Role)
}

func looksLikeInteractionPath(path string, roles map[string]packageRoleNode) bool {
	n, ok := roles[normalizeRolePath(path)]
	if !ok {
		return false
	}
	return isInteractionRole(n.Role)
}

func journeyStatusClaimsComplete(journey string) bool {
	lines := strings.Split(journey, "\n")
	inStatus := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		low := strings.ToLower(trim)
		if strings.HasPrefix(low, "status:") {
			idx := strings.Index(trim, ":")
			body := ""
			if idx >= 0 {
				body = strings.TrimSpace(trim[idx+1:])
			}
			return statusTextClaimsComplete(body)
		}
		if strings.HasPrefix(trim, "#") && strings.Contains(low, "status") {
			inStatus = true
			continue
		}
		if inStatus && strings.HasPrefix(trim, "#") {
			break
		}
		if !inStatus || trim == "" {
			continue
		}
		if statusTextClaimsComplete(low) {
			return true
		}
	}
	return false
}

func statusTextClaimsComplete(text string) bool {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return false
	}
	if strings.Contains(low, "not complete") ||
		strings.Contains(low, "incomplete") ||
		strings.Contains(low, "do not claim complete") {
		return false
	}
	return strings.Contains(low, "completed") || strings.Contains(low, "complete")
}

func journeyDebtStillSaysMerge(journey string) bool {
	return strings.Contains(strings.ToLower(journey), "merge into")
}

const journeyStatusOpenPendingMerges = "Open — debt still lists Merge into actions; clear or apply those rows before marking refinement done."

// reconcileJourneyStatusWithDebt downgrades a false "complete" Status when debt still
// lists Merge into actions. Keeps pending merge counsel; only fixes the contradiction.
func reconcileJourneyStatusWithDebt(journey string) string {
	journey = strings.TrimSpace(journey)
	if journey == "" {
		return journey
	}
	if !journeyStatusClaimsComplete(journey) || !journeyDebtStillSaysMerge(journey) {
		return journey
	}
	return rewriteJourneyStatusBody(journey, journeyStatusOpenPendingMerges)
}

func rewriteJourneyStatusBody(journey, newBody string) string {
	lines := strings.Split(journey, "\n")
	out := make([]string, 0, len(lines)+2)
	replaced := false
	inStatus := false
	wroteBody := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		low := strings.ToLower(trim)
		if strings.HasPrefix(low, "status:") {
			out = append(out, "Status: "+newBody)
			replaced = true
			inStatus = false
			wroteBody = true
			continue
		}
		if strings.HasPrefix(trim, "#") && strings.Contains(low, "status") {
			if inStatus && !wroteBody {
				out = append(out, newBody)
				out = append(out, "")
			}
			out = append(out, line)
			inStatus = true
			wroteBody = false
			replaced = true
			continue
		}
		if inStatus {
			if strings.HasPrefix(trim, "#") {
				if !wroteBody {
					out = append(out, newBody)
					out = append(out, "")
					wroteBody = true
				}
				inStatus = false
				out = append(out, line)
				continue
			}
			if !wroteBody {
				if trim == "" {
					continue
				}
				out = append(out, newBody)
				wroteBody = true
				continue
			}
			// Drop the remainder of the old Status body until the next heading.
			continue
		}
		out = append(out, line)
	}
	if inStatus && !wroteBody {
		out = append(out, newBody)
	}
	if !replaced {
		return "## Status\n\n" + newBody + "\n\n" + journey
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}

func architectureHasFindings(arch string) bool {
	lines := strings.Split(arch, "\n")
	inFindings := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		low := strings.ToLower(trim)
		if isArchitectureFindingsHeading(trim) {
			inFindings = true
			continue
		}
		if inFindings && strings.HasPrefix(trim, "#") {
			inFindings = false
		}
		if !inFindings || trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "-") || strings.HasPrefix(trim, "*") || strings.HasPrefix(trim, "|") {
			return true
		}
		if strings.Contains(low, "`") {
			return true
		}
	}
	return false
}

func journeyHasDebtTable(journey string) bool {
	lower := strings.ToLower(journey)
	if !strings.Contains(lower, "debt") && !strings.Contains(lower, "boundary violation") {
		return false
	}
	// Require at least one markdown table row or bullet under a debt heading.
	lines := strings.Split(journey, "\n")
	inDebt := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		low := strings.ToLower(trim)
		if strings.HasPrefix(low, "#") && (strings.Contains(low, "debt") || strings.Contains(low, "boundary")) {
			inDebt = true
			continue
		}
		if inDebt && strings.HasPrefix(trim, "#") {
			inDebt = false
		}
		if !inDebt {
			continue
		}
		if strings.HasPrefix(trim, "|") && !strings.Contains(trim, "---") && !strings.Contains(strings.ToLower(trim), "violation") {
			cells := strings.Split(trim, "|")
			content := 0
			for _, c := range cells {
				if strings.TrimSpace(c) != "" {
					content++
				}
			}
			if content >= 2 {
				return true
			}
		}
		if strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* ") {
			return true
		}
	}
	return false
}

func validateRefinedCatalogYAML(raw, draftYAML, repoID, rolesYAML string) (string, error) {
	return validateRefinedCatalogYAMLWithGraph(raw, draftYAML, repoID, rolesYAML, "")
}

func validateRefinedCatalogYAMLWithGraph(raw, draftYAML, repoID, rolesYAML, graphText string) (string, error) {
	tmp, err := os.CreateTemp("", "majordomo-refined-*.yaml")
	if err != nil {
		return "", fmt.Errorf("typology refine temp file: %w", err)
	}
	path := tmp.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := tmp.WriteString(raw); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("typology refine write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("typology refine close temp: %w", err)
	}
	typo, err := catalog.LoadYAML(path)
	if err != nil {
		return "", annotateCatalogYAMLError("typology refine load catalog", raw, err)
	}
	draft, draftAllowed, err := loadDraftCatalog(draftYAML)
	if err != nil {
		return "", err
	}
	roles := roleByPath(mustParseRoles(rolesYAML))
	typo = sanitizeRefinedCatalog(typo, draft, roles, parseGraphImporters(graphText))
	if id := strings.TrimSpace(repoID); id != "" {
		cur := strings.TrimSpace(typo.ID)
		if cur == "" || strings.HasPrefix(cur, "majordomo-typology-") || strings.Contains(cur, "/") {
			typo.ID = id
		}
	}
	allowed := make(map[string]struct{}, len(draftAllowed)+len(roles))
	for p := range draftAllowed {
		allowed[p] = struct{}{}
	}
	for p := range roles {
		if norm := normalizeRolePath(p); norm != "" {
			allowed[norm] = struct{}{}
		}
	}
	if len(allowed) > 0 {
		typo = remapInventedCatalogPaths(typo, allowed)
	}
	typo = restoreMissingDraftPackages(typo, draft, roles)
	typo = restoreMissingRolePackages(typo, roles)
	if err := catalog.SaveYAML(path, typo); err != nil {
		return "", fmt.Errorf("typology refine save sanitized catalog: %w", err)
	}
	sanitized, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("typology refine read sanitized catalog: %w", err)
	}
	if issues := typo.ValidateStructure(); len(issues) > 0 {
		var b strings.Builder
		b.WriteString("catalog.ValidateStructure failed:")
		for _, issue := range issues {
			if strings.TrimSpace(issue.Slice) != "" {
				fmt.Fprintf(&b, "\n- %s: %s", issue.Slice, issue.Message)
				continue
			}
			fmt.Fprintf(&b, "\n- %s", issue.Message)
		}
		return "", fmt.Errorf("%s", b.String())
	}
	if err := rejectInventedCatalogPaths(typo, allowed); err != nil {
		return "", err
	}
	if err := rejectMissingDraftPackages(typo, draftAllowed); err != nil {
		return "", err
	}
	return string(sanitized), nil
}

func loadDraftCatalog(draftYAML string) (catalog.Typology, map[string]struct{}, error) {
	draftYAML = strings.TrimSpace(draftYAML)
	if draftYAML == "" {
		return catalog.Typology{}, nil, nil
	}
	tmp, err := os.CreateTemp("", "majordomo-draft-*.yaml")
	if err != nil {
		return catalog.Typology{}, nil, fmt.Errorf("typology refine draft temp: %w", err)
	}
	path := tmp.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := tmp.WriteString(draftYAML); err != nil {
		_ = tmp.Close()
		return catalog.Typology{}, nil, fmt.Errorf("typology refine write draft temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return catalog.Typology{}, nil, fmt.Errorf("typology refine close draft temp: %w", err)
	}
	draft, err := catalog.LoadYAML(path)
	if err != nil {
		return catalog.Typology{}, nil, fmt.Errorf("typology refine load draft catalog: %w", err)
	}
	return draft, collectCatalogPaths(draft), nil
}

func remapInventedCatalogPaths(t catalog.Typology, allowed map[string]struct{}) catalog.Typology {
	if len(allowed) == 0 {
		return t
	}
	allowedList := make([]string, 0, len(allowed))
	for p := range allowed {
		allowedList = append(allowedList, p)
	}
	sort.Strings(allowedList)
	for i := range t.Slices {
		s := &t.Slices[i]
		for j := range s.Owns {
			if near := uniqueNearestDraftPath(s.Owns[j].Path, allowedList); near != "" {
				if normalizeCatalogPath(s.Owns[j].Path) != near {
					s.Owns[j].Path = near
				}
			}
		}
		for k := range s.Surfaces {
			for j := range s.Surfaces[k].Components {
				c := &s.Surfaces[k].Components[j]
				if near := uniqueNearestDraftPath(c.Path, allowedList); near != "" {
					if normalizeCatalogPath(c.Path) != near {
						c.Path = near
					}
				}
			}
		}
	}
	for i := range t.Libraries {
		lib := &t.Libraries[i]
		for j := range lib.Owns {
			if near := uniqueNearestDraftPath(lib.Owns[j].Path, allowedList); near != "" {
				if normalizeCatalogPath(lib.Owns[j].Path) != near {
					lib.Owns[j].Path = near
				}
			}
		}
	}
	return t
}

func rejectInventedCatalogPaths(refined catalog.Typology, allowed map[string]struct{}) error {
	if len(allowed) == 0 {
		return nil
	}
	var invented []string
	for _, p := range collectCatalogPathList(refined) {
		if _, ok := allowed[p]; ok {
			continue
		}
		invented = append(invented, p)
	}
	if len(invented) == 0 {
		return nil
	}
	allowedList := make([]string, 0, len(allowed))
	for p := range allowed {
		allowedList = append(allowedList, p)
	}
	sort.Strings(allowedList)
	sort.Strings(invented)
	var b strings.Builder
	b.WriteString("invented package paths (must copy draft/graph paths verbatim; put filesystem renames in journey debt only):")
	for _, p := range invented {
		fmt.Fprintf(&b, "\n- %s", p)
		if near := uniqueNearestDraftPath(p, allowedList); near != "" {
			fmt.Fprintf(&b, " (draft has %s)", near)
		}
	}
	return fmt.Errorf("%s", b.String())
}

func collectCatalogPaths(t catalog.Typology) map[string]struct{} {
	out := make(map[string]struct{})
	for _, p := range collectCatalogPathList(t) {
		out[p] = struct{}{}
	}
	return out
}

func collectCatalogPathList(t catalog.Typology) []string {
	var paths []string
	for _, s := range t.Slices {
		for _, c := range s.Owns {
			if n := normalizeCatalogPath(c.Path); n != "" {
				paths = append(paths, n)
			}
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				if n := normalizeCatalogPath(c.Path); n != "" {
					paths = append(paths, n)
				}
			}
		}
	}
	for _, lib := range t.Libraries {
		for _, c := range lib.Owns {
			if n := normalizeCatalogPath(c.Path); n != "" {
				paths = append(paths, n)
			}
		}
	}
	return paths
}

func normalizeCatalogPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimSuffix(p, "/")
	return p
}

// uniqueNearestDraftPath returns a draft path only when exactly one allowed path
// is an unambiguous match for the invented path.
func uniqueNearestDraftPath(invented string, allowed []string) string {
	inv := normalizeCatalogPath(invented)
	if inv == "" {
		return ""
	}
	if containsString(allowed, inv) {
		return inv
	}
	base := filepath.Base(inv)
	var byBase []string
	for _, a := range allowed {
		if filepath.Base(a) == base {
			byBase = append(byBase, a)
		}
	}
	if len(byBase) == 1 {
		return byBase[0]
	}
	invFlat := strings.ReplaceAll(inv, "/", "")
	var byFlat []string
	for _, a := range allowed {
		if strings.ReplaceAll(a, "/", "") == invFlat {
			byFlat = append(byFlat, a)
		}
	}
	if len(byFlat) == 1 {
		return byFlat[0]
	}
	var byContain []string
	for _, a := range allowed {
		aBase := filepath.Base(a)
		if aBase == "" || base == "" {
			continue
		}
		if len(aBase) < 3 || len(base) < 3 {
			continue
		}
		if strings.Contains(aBase, base) || strings.Contains(base, aBase) {
			byContain = append(byContain, a)
		}
	}
	if len(byContain) == 1 {
		return byContain[0]
	}
	return ""
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

type missingCatalogComponent struct {
	draftSliceID string
	draftLibID   string
	comp         catalog.Component
}

// restoreMissingDraftPackages reclaims draft package paths the refine LLM dropped.
// Exec adapters always land under owns[]; interaction paths reattach to surfaces.
// Draft library packages reattach to the matching refined library when present.
func restoreMissingDraftPackages(refined, draft catalog.Typology, roles map[string]packageRoleNode) catalog.Typology {
	if len(refined.Slices) == 0 {
		return refined
	}
	claimed := collectCatalogPaths(refined)
	var missing []missingCatalogComponent
	add := func(draftSliceID, draftLibID string, c catalog.Component) {
		n := normalizeCatalogPath(c.Path)
		if n == "" {
			return
		}
		if _, ok := claimed[n]; ok {
			return
		}
		missing = append(missing, missingCatalogComponent{draftSliceID: draftSliceID, draftLibID: draftLibID, comp: c})
		claimed[n] = struct{}{}
	}
	for _, s := range draft.Slices {
		for _, c := range s.Owns {
			add(s.ID, "", c)
		}
		for _, surf := range s.Surfaces {
			for _, c := range surf.Components {
				add(s.ID, "", c)
			}
		}
	}
	for _, lib := range draft.Libraries {
		for _, c := range lib.Owns {
			add("", lib.ID, c)
		}
	}
	if len(missing) == 0 {
		return refined
	}
	return attachMissingComponents(refined, roles, missing)
}

// restoreMissingRolePackages reclaims module package paths from package_roles when refine dropped them.
func restoreMissingRolePackages(refined catalog.Typology, roles map[string]packageRoleNode) catalog.Typology {
	if len(refined.Slices) == 0 || len(roles) == 0 {
		return refined
	}
	claimed := collectCatalogPaths(refined)
	var missing []missingCatalogComponent
	for path := range roles {
		path = normalizeRolePath(path)
		if path == "" || strings.HasPrefix(path, "cmd/") {
			continue
		}
		if _, ok := claimed[path]; ok {
			continue
		}
		missing = append(missing, missingCatalogComponent{
			comp: catalog.Component{ID: filepath.Base(path), Path: path},
		})
		claimed[path] = struct{}{}
	}
	if len(missing) == 0 {
		return refined
	}
	return attachMissingComponents(refined, roles, missing)
}

func attachMissingComponents(refined catalog.Typology, roles map[string]packageRoleNode, missing []missingCatalogComponent) catalog.Typology {
	sliceIdx := make(map[string]int, len(refined.Slices))
	for i, s := range refined.Slices {
		if id := strings.TrimSpace(s.ID); id != "" {
			sliceIdx[id] = i
		}
	}
	libIdx := make(map[string]int, len(refined.Libraries))
	for i, lib := range refined.Libraries {
		if id := strings.TrimSpace(lib.ID); id != "" {
			libIdx[id] = i
		}
	}

	for _, m := range missing {
		c := m.comp
		if strings.TrimSpace(c.ID) == "" {
			c.ID = filepath.Base(normalizeCatalogPath(c.Path))
		}
		if libID := strings.TrimSpace(m.draftLibID); libID != "" {
			if i, ok := libIdx[libID]; ok {
				if c.Layer == catalog.LayerInteraction {
					c.Layer = ""
				}
				if c.Layer != "" && c.Layer != catalog.LayerDomain {
					c.Layer = catalog.LayerDomain
				}
				refined.Libraries[i].Owns = append(refined.Libraries[i].Owns, c)
				continue
			}
		}
		idx := 0
		if i, ok := sliceIdx[strings.TrimSpace(m.draftSliceID)]; ok {
			idx = i
		}
		if isExecAdapterPath(c.Path, roles) || !looksLikeInteractionPath(c.Path, roles) {
			c.Layer = catalog.LayerDomain
			c.Kind = ""
			refined.Slices[idx].Owns = append(refined.Slices[idx].Owns, c)
			continue
		}
		kind := inferInteractionKind(c.Path, roles)
		placed := false
		for j := range refined.Slices[idx].Surfaces {
			if refined.Slices[idx].Surfaces[j].Kind == kind {
				refined.Slices[idx].Surfaces[j].Components = append(refined.Slices[idx].Surfaces[j].Components, c)
				placed = true
				break
			}
		}
		if !placed {
			sid := strings.TrimSpace(refined.Slices[idx].ID)
			if sid == "" {
				sid = "slice"
			}
			refined.Slices[idx].Surfaces = append(refined.Slices[idx].Surfaces, catalog.Surface{
				ID:         sid + "-" + string(kind),
				Kind:       kind,
				Components: []catalog.Component{c},
			})
		}
	}
	return refined
}

func inferInteractionKind(path string, roles map[string]packageRoleNode) catalog.InteractionKind {
	n := roles[normalizeRolePath(path)]
	switch n.Role {
	case roleEntrypoint:
		return catalog.InteractionCLI
	case roleHTTPSurface:
		for _, e := range n.Evidence {
			if e == "embeds_static" {
				return catalog.InteractionUI
			}
		}
		return catalog.InteractionAPI
	default:
		return catalog.InteractionCLI
	}
}

// normalizeCatalogSurfaceKind maps RLM aliases onto catalog InteractionKind.
// Unknown kinds return ok=false so callers can drop the surface before ValidateStructure.
func normalizeCatalogSurfaceKind(kind catalog.InteractionKind) (catalog.InteractionKind, bool) {
	switch strings.ToLower(strings.TrimSpace(string(kind))) {
	case "cli":
		return catalog.InteractionCLI, true
	case "api":
		return catalog.InteractionAPI, true
	case "ui":
		return catalog.InteractionUI, true
	case "service", "http", "https", "gateway", "server", "rpc", "grpc", "rest":
		return catalog.InteractionAPI, true
	case "command", "terminal", "cmd":
		return catalog.InteractionCLI, true
	case "web", "frontend", "browser":
		return catalog.InteractionUI, true
	default:
		return "", false
	}
}

// pruneDraftPackagesAbsentFromRoles drops draft paths the current role harvest
// does not list, so a package removed from the tree cannot stay on the map.
// An empty role file is left untouched. New packages are added later by refine.
func pruneDraftPackagesAbsentFromRoles(draftPath, rolesPath string) error {
	if _, err := os.Stat(draftPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("typology catch-up stat draft: %w", err)
	}
	if _, err := os.Stat(rolesPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("typology catch-up stat package roles: %w", err)
	}
	doc, err := loadPackageRoles(rolesPath)
	if err != nil {
		return fmt.Errorf("typology catch-up load package roles: %w", err)
	}
	if len(doc.Packages) == 0 {
		return nil
	}
	live := map[string]struct{}{}
	for _, p := range doc.Packages {
		if n := normalizeRolePath(p.Path); n != "" {
			live[n] = struct{}{}
		}
	}
	typo, err := catalog.LoadYAML(draftPath)
	if err != nil {
		return fmt.Errorf("typology catch-up load draft: %w", err)
	}
	keep := func(path string) bool {
		_, ok := live[normalizeCatalogPath(path)]
		return ok
	}
	for i := range typo.Slices {
		s := &typo.Slices[i]
		var owns []catalog.Component
		for _, c := range s.Owns {
			if keep(c.Path) {
				owns = append(owns, c)
			}
		}
		s.Owns = owns
		var surfaces []catalog.Surface
		for _, surf := range s.Surfaces {
			var comps []catalog.Component
			for _, c := range surf.Components {
				if keep(c.Path) {
					comps = append(comps, c)
				}
			}
			if len(comps) == 0 {
				continue
			}
			surf.Components = comps
			surfaces = append(surfaces, surf)
		}
		s.Surfaces = surfaces
	}
	for i := range typo.Libraries {
		lib := &typo.Libraries[i]
		var owns []catalog.Component
		for _, c := range lib.Owns {
			if keep(c.Path) {
				owns = append(owns, c)
			}
		}
		lib.Owns = owns
	}
	if err := catalog.SaveYAML(draftPath, typo); err != nil {
		return fmt.Errorf("typology catch-up save draft: %w", err)
	}
	return nil
}

func rejectMissingDraftPackages(refined catalog.Typology, allowed map[string]struct{}) error {
	if len(allowed) == 0 {
		return nil
	}
	claimed := collectCatalogPaths(refined)
	var missing []string
	for p := range allowed {
		if _, ok := claimed[p]; !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	var b strings.Builder
	b.WriteString("unmapped draft packages (must remain under owns[], surfaces[], or libraries[].owns[]; demote exec adapters into owns[], do not drop them):")
	for _, p := range missing {
		fmt.Fprintf(&b, "\n- %s", p)
	}
	return fmt.Errorf("%s", b.String())
}

// sanitizeRefinedCatalog drops dangling bindings, moves likely interaction packages
// onto surfaces, demotes exec adapters into owns[] (never drops them), strips invented
// DocPages, keeps only draft-backed libraries with a purpose, prefers slice claims over
// library duplicates, and normalizes common LLM mistakes.
func sanitizeRefinedCatalog(t, draft catalog.Typology, roles map[string]packageRoleNode, importers map[string][]string) catalog.Typology {
	seenComp := make(map[string]string)
	for i := range t.Slices {
		s := &t.Slices[i]
		var owns []catalog.Component
		var moved []catalog.Component
		for _, c := range s.Owns {
			if strings.TrimSpace(c.ID) == "" {
				continue
			}
			if owner, ok := seenComp[c.ID]; ok && owner != s.ID {
				continue
			}
			if looksLikeInteractionPath(c.Path, roles) {
				moved = append(moved, catalog.Component{ID: c.ID, Path: c.Path})
				seenComp[c.ID] = s.ID
				continue
			}
			c.Layer = catalog.LayerDomain
			seenComp[c.ID] = s.ID
			owns = append(owns, c)
		}
		s.Owns = owns

		seenKind := make(map[catalog.InteractionKind]struct{})
		seenSurfaceID := make(map[string]struct{})
		uniqueSurfaceID := func(id string, kind catalog.InteractionKind) string {
			base := strings.TrimSpace(id)
			if base == "" {
				base = s.ID + "-" + string(kind)
			}
			candidate := base
			if _, ok := seenSurfaceID[candidate]; !ok {
				seenSurfaceID[candidate] = struct{}{}
				return candidate
			}
			candidate = s.ID + "-" + string(kind)
			if _, ok := seenSurfaceID[candidate]; !ok {
				seenSurfaceID[candidate] = struct{}{}
				return candidate
			}
			for n := 2; ; n++ {
				candidate = fmt.Sprintf("%s-%s-%d", s.ID, kind, n)
				if _, ok := seenSurfaceID[candidate]; !ok {
					seenSurfaceID[candidate] = struct{}{}
					return candidate
				}
			}
		}
		var surfaces []catalog.Surface
		for _, surf := range s.Surfaces {
			kind, ok := normalizeCatalogSurfaceKind(surf.Kind)
			if !ok {
				continue
			}
			surf.Kind = kind
			if _, ok := seenKind[surf.Kind]; ok {
				continue
			}
			seenKind[surf.Kind] = struct{}{}
			var comps []catalog.Component
			for _, c := range surf.Components {
				if strings.TrimSpace(c.ID) == "" {
					continue
				}
				if owner, ok := seenComp[c.ID]; ok && owner != s.ID {
					continue
				}
				// Exec adapters are never CLI delivery surfaces; demote to owns[].
				if surf.Kind == catalog.InteractionCLI && isExecAdapterPath(c.Path, roles) {
					c.Layer = catalog.LayerDomain
					seenComp[c.ID] = s.ID
					s.Owns = append(s.Owns, c)
					continue
				}
				seenComp[c.ID] = s.ID
				comps = append(comps, c)
			}
			surf.Components = comps
			if len(comps) == 0 && surf.Kind == catalog.InteractionCLI {
				// Drop empty CLI surfaces created only for mis-placed adapters.
				delete(seenKind, surf.Kind)
				continue
			}
			surf.ID = uniqueSurfaceID(surf.ID, surf.Kind)
			surfaces = append(surfaces, surf)
		}
		if len(moved) > 0 {
			kind := catalog.InteractionCLI
			for _, c := range moved {
				kind = inferInteractionKind(c.Path, roles)
				break
			}
			if _, ok := seenKind[kind]; !ok {
				surfaces = append(surfaces, catalog.Surface{
					ID:         uniqueSurfaceID(s.ID+"-"+string(kind), kind),
					Kind:       kind,
					Components: moved,
				})
				seenKind[kind] = struct{}{}
			} else {
				for j := range surfaces {
					if surfaces[j].Kind == kind {
						surfaces[j].Components = append(surfaces[j].Components, moved...)
						break
					}
				}
			}
		}
		s.Surfaces = surfaces
		// Digest proposals never declare DocPages; those come from human emit later.
		s.Docs = catalog.DocCluster{}
	}

	draftLibs := make(map[string]catalog.Library, len(draft.Libraries))
	for _, lib := range draft.Libraries {
		id := strings.TrimSpace(lib.ID)
		if id == "" {
			continue
		}
		draftLibs[id] = lib
	}
	var libraries []catalog.Library
	keptLib := make(map[string]struct{})
	appendLibraryOwns := func(id string, candidates []catalog.Component) []catalog.Component {
		var owns []catalog.Component
		for _, c := range candidates {
			if strings.TrimSpace(c.ID) == "" {
				continue
			}
			if _, ok := seenComp[c.ID]; ok {
				continue
			}
			if c.Layer == catalog.LayerInteraction {
				continue
			}
			if c.Layer != "" && c.Layer != catalog.LayerDomain {
				c.Layer = catalog.LayerDomain
			}
			seenComp[c.ID] = id
			owns = append(owns, c)
		}
		return owns
	}
	for _, lib := range t.Libraries {
		id := strings.TrimSpace(lib.ID)
		if id == "" {
			continue
		}
		draftLib, ok := draftLibs[id]
		if !ok {
			continue // drop invented libraries not present in the discover draft
		}
		purpose := strings.TrimSpace(lib.Purpose)
		if purpose == "" {
			purpose = strings.TrimSpace(draftLib.Purpose)
		}
		if purpose == "" {
			continue
		}
		owns := appendLibraryOwns(id, lib.Owns)
		if len(owns) == 0 {
			owns = appendLibraryOwns(id, draftLib.Owns)
		}
		libraries = append(libraries, catalog.Library{ID: id, Purpose: purpose, Owns: owns})
		keptLib[id] = struct{}{}
	}
	var missingDraftLibIDs []string
	for id := range draftLibs {
		if _, ok := keptLib[id]; ok {
			continue
		}
		missingDraftLibIDs = append(missingDraftLibIDs, id)
	}
	sort.Strings(missingDraftLibIDs)
	for _, id := range missingDraftLibIDs {
		draftLib := draftLibs[id]
		purpose := strings.TrimSpace(draftLib.Purpose)
		if purpose == "" {
			continue
		}
		owns := appendLibraryOwns(id, draftLib.Owns)
		libraries = append(libraries, catalog.Library{ID: id, Purpose: purpose, Owns: owns})
		keptLib[id] = struct{}{}
	}
	t.Libraries = libraries

	slices := make(map[string]struct{}, len(t.Slices))
	for _, s := range t.Slices {
		if strings.TrimSpace(s.ID) != "" {
			slices[s.ID] = struct{}{}
		}
	}
	var sliceBindings []catalog.SliceBinding
	for _, b := range t.SliceBindings {
		if _, ok := slices[b.From]; !ok {
			continue
		}
		_, toSlice := slices[b.To]
		_, toLibrary := keptLib[b.To]
		if !toSlice && !toLibrary {
			continue
		}
		sliceBindings = append(sliceBindings, b)
	}
	t.SliceBindings = sliceBindings
	var compBindings []catalog.ComponentBinding
	for _, b := range t.ComponentBindings {
		fromOwner, okFrom := seenComp[b.From]
		toOwner, okTo := seenComp[b.To]
		if !okFrom || !okTo {
			continue
		}
		if _, fromIsLib := keptLib[fromOwner]; fromIsLib {
			// Libraries must not bind outward to slice packages or other libraries.
			if _, toIsLib := keptLib[toOwner]; toIsLib {
				if fromOwner != toOwner {
					continue
				}
			} else if _, toIsSlice := slices[toOwner]; toIsSlice {
				continue
			}
		}
		if fromOwner != toOwner {
			has := false
			for _, sb := range t.SliceBindings {
				if (sb.From == fromOwner && sb.To == toOwner) || (sb.From == toOwner && sb.To == fromOwner) {
					has = true
					break
				}
			}
			if !has {
				continue
			}
		}
		compBindings = append(compBindings, b)
	}
	t.ComponentBindings = compBindings
	return collapseHollowPackageSlices(separateHTTPSurfacesFromEntrypoint(t, roles, importers))
}

// parseGraphImporters reads Typology graph "sole importer" rows.
// Paths with several importers are omitted, so callers keep those packages on the parent slice.
func parseGraphImporters(graph string) map[string][]string {
	out := map[string][]string{}
	re := regexp.MustCompile(`(?i)sole importer:\s*only imported by\s+(\S+)`)
	for _, line := range strings.Split(graph, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.Contains(strings.ToLower(trim), "sole importer") {
			continue
		}
		pkg := ""
		if strings.HasPrefix(trim, "- ") {
			rest := strings.TrimSpace(strings.TrimPrefix(trim, "- "))
			if i := strings.Index(rest, "->"); i >= 0 {
				pkg = strings.TrimSpace(rest[:i])
			}
		}
		m := re.FindStringSubmatch(trim)
		if pkg == "" || len(m) < 2 {
			continue
		}
		from := normalizeRolePath(pkg)
		to := normalizeRolePath(strings.TrimRight(m[1], ")"))
		if from == "" || to == "" {
			continue
		}
		out[from] = append(out[from], to)
	}
	return out
}

func httpImportedOnlyBySliceEntrypoints(httpPath string, entrypoints []string, importers map[string][]string) bool {
	imps := importers[normalizeRolePath(httpPath)]
	if len(imps) == 0 {
		return false
	}
	ep := map[string]struct{}{}
	for _, p := range entrypoints {
		if n := normalizeRolePath(p); n != "" {
			ep[n] = struct{}{}
		}
	}
	if len(ep) == 0 {
		return false
	}
	for _, imp := range imps {
		if _, ok := ep[normalizeRolePath(imp)]; !ok {
			return false
		}
	}
	return true
}

func collectSliceEntrypointPaths(s catalog.Slice, roles map[string]packageRoleNode) []string {
	var out []string
	add := func(path string) {
		if roles[normalizeRolePath(path)].Role != roleEntrypoint {
			return
		}
		out = append(out, path)
	}
	for _, c := range s.Owns {
		add(c.Path)
	}
	for _, surf := range s.Surfaces {
		for _, c := range surf.Components {
			add(c.Path)
		}
	}
	return out
}

func appendHTTPSurfaceComponents(s *catalog.Slice, kind catalog.InteractionKind, comps []catalog.Component) {
	if s == nil || len(comps) == 0 {
		return
	}
	for i := range s.Surfaces {
		if s.Surfaces[i].Kind != kind {
			continue
		}
		s.Surfaces[i].Components = append(s.Surfaces[i].Components, comps...)
		return
	}
	sid := strings.TrimSpace(s.ID)
	if sid == "" {
		sid = "slice"
	}
	s.Surfaces = append(s.Surfaces, catalog.Surface{
		ID:         sid + "-" + string(kind),
		Kind:       kind,
		Components: comps,
	})
}

// separateHTTPSurfacesFromEntrypoint splits an HTTP server onto a sibling slice only when
// the graph shows that slice's entrypoint is the server's sole importer. Shared gateways
// stay on the parent as an api/ui surface beside the CLI door.
func separateHTTPSurfacesFromEntrypoint(t catalog.Typology, roles map[string]packageRoleNode, importers map[string][]string) catalog.Typology {
	if len(roles) == 0 {
		return t
	}
	usedSliceIDs := make(map[string]struct{}, len(t.Slices))
	for _, s := range t.Slices {
		if id := strings.TrimSpace(s.ID); id != "" {
			usedSliceIDs[id] = struct{}{}
		}
	}
	uniqueSliceID := func(base string) string {
		base = strings.TrimSpace(base)
		if base == "" {
			base = "server"
		}
		if _, ok := usedSliceIDs[base]; !ok {
			usedSliceIDs[base] = struct{}{}
			return base
		}
		for n := 2; ; n++ {
			candidate := fmt.Sprintf("%s-%d", base, n)
			if _, ok := usedSliceIDs[candidate]; !ok {
				usedSliceIDs[candidate] = struct{}{}
				return candidate
			}
		}
	}

	var extra []catalog.Slice
	for i := range t.Slices {
		s := &t.Slices[i]
		entrypoints := collectSliceEntrypointPaths(*s, roles)
		if len(entrypoints) == 0 {
			continue
		}
		pathRole := func(path string) string {
			return roles[normalizeRolePath(path)].Role
		}

		var keepOwns []catalog.Component
		var splitHTTP []catalog.Component
		var keepHTTP []catalog.Component
		httpKind := catalog.InteractionAPI
		for _, c := range s.Owns {
			if pathRole(c.Path) != roleHTTPSurface {
				keepOwns = append(keepOwns, c)
				continue
			}
			httpKind = inferInteractionKind(c.Path, roles)
			if httpImportedOnlyBySliceEntrypoints(c.Path, entrypoints, importers) {
				splitHTTP = append(splitHTTP, c)
				continue
			}
			keepHTTP = append(keepHTTP, c)
		}
		s.Owns = keepOwns

		var keepSurfaces []catalog.Surface
		for _, surf := range s.Surfaces {
			var keepComps []catalog.Component
			var cliHTTP []catalog.Component
			for _, c := range surf.Components {
				if pathRole(c.Path) != roleHTTPSurface {
					keepComps = append(keepComps, c)
					continue
				}
				httpKind = inferInteractionKind(c.Path, roles)
				if httpImportedOnlyBySliceEntrypoints(c.Path, entrypoints, importers) {
					splitHTTP = append(splitHTTP, c)
					continue
				}
				if surf.Kind == catalog.InteractionCLI {
					cliHTTP = append(cliHTTP, c)
					continue
				}
				keepComps = append(keepComps, c)
			}
			surf.Components = keepComps
			keepHTTP = append(keepHTTP, cliHTTP...)
			if len(keepComps) == 0 {
				continue
			}
			keepSurfaces = append(keepSurfaces, surf)
		}
		s.Surfaces = keepSurfaces
		appendHTTPSurfaceComponents(s, httpKind, keepHTTP)

		if len(splitHTTP) == 0 {
			continue
		}
		sid := uniqueSliceID(strings.TrimSpace(s.ID) + "-http")
		obj := strings.TrimSpace(s.Objective)
		if obj == "" {
			obj = "Delivery surface separated from the CLI entrypoint."
		}
		extra = append(extra, catalog.Slice{
			ID:        sid,
			Objective: obj,
			Surfaces: []catalog.Surface{{
				ID:         sid + "-" + string(httpKind),
				Kind:       httpKind,
				Components: splitHTTP,
			}},
		})
	}
	if len(extra) > 0 {
		t.Slices = append(t.Slices, extra...)
	}
	return t
}

func marshalConstraints(doc packageCapabilityConstraintsDoc) (string, error) {
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return "", fmt.Errorf("capability constraints encode: %w", err)
	}
	return string(data), nil
}

func marshalClaims(doc sliceObjectiveClaimsDoc) (string, error) {
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return "", fmt.Errorf("objective claims encode: %w", err)
	}
	return string(data), nil
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	lines = lines[1:]
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func annotateCatalogYAMLError(prefix, raw string, err error) error {
	if err == nil {
		return nil
	}
	dumpPath := dumpRefinedCatalogFailure(raw)
	snippet := yamlLineSnippet(raw, yamlErrorLine(err.Error()), 2)
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %v", prefix, err)
	if dumpPath != "" {
		fmt.Fprintf(&b, "\ndump=%s", dumpPath)
	}
	if snippet != "" {
		fmt.Fprintf(&b, "\n--- yaml context ---\n%s", snippet)
	}
	return fmt.Errorf("%s", b.String())
}

func yamlErrorLine(msg string) int {
	const marker = "yaml: line "
	i := strings.Index(msg, marker)
	if i < 0 {
		return 0
	}
	rest := msg[i+len(marker):]
	n := 0
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func yamlLineSnippet(raw string, line, radius int) string {
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return ""
	}
	if line <= 0 {
		line = 1
	}
	start := line - radius
	if start < 1 {
		start = 1
	}
	end := line + radius
	if end > len(lines) {
		end = len(lines)
	}
	var b strings.Builder
	for i := start; i <= end; i++ {
		mark := " "
		if i == line {
			mark = ">"
		}
		fmt.Fprintf(&b, "%s %4d | %s\n", mark, i, lines[i-1])
	}
	return strings.TrimRight(b.String(), "\n")
}

func dumpRefinedCatalogFailure(raw string) string {
	dir := filepath.Join("tmp", "logs", "refined-catalog-failures")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.yaml", time.Now().UnixNano()))
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		return ""
	}
	return path
}

// scrubForbiddenHTTPEntrypointMergeRows drops cluster rows that fold server packages into an entrypoint.
func scrubForbiddenHTTPEntrypointMergeRows(merges []proposedMerge, rolesYAML string) []proposedMerge {
	roles := roleByPath(mustParseRoles(rolesYAML))
	if len(roles) == 0 || len(merges) == 0 {
		return merges
	}
	out := make([]proposedMerge, 0, len(merges))
	for _, m := range merges {
		hasHTTP, hasEntry := false, false
		for _, p := range m.Packages {
			switch roles[normalizeRolePath(p)].Role {
			case roleHTTPSurface:
				hasHTTP = true
			case roleEntrypoint:
				hasEntry = true
			}
		}
		if hasHTTP && hasEntry {
			continue
		}
		out = append(out, m)
	}
	return out
}

// scrubForbiddenHTTPEntrypointMerges rewrites cluster proposals that fold server
// packages into an entrypoint slice. Returns a feedback note when a forbidden merge was found.
func scrubForbiddenHTTPEntrypointMerges(proposalMD, rolesYAML string) (string, string) {
	roles := roleByPath(mustParseRoles(rolesYAML))
	if len(roles) == 0 {
		return proposalMD, ""
	}
	var httpPaths, entryPaths []string
	for path, n := range roles {
		switch n.Role {
		case roleHTTPSurface:
			httpPaths = append(httpPaths, path)
		case roleEntrypoint:
			entryPaths = append(entryPaths, path)
		}
	}
	if len(httpPaths) == 0 || len(entryPaths) == 0 {
		return proposalMD, ""
	}
	sort.Strings(httpPaths)
	sort.Strings(entryPaths)

	lower := strings.ToLower(proposalMD)
	found := false
	for _, httpPath := range httpPaths {
		hp := strings.ToLower(httpPath)
		base := strings.ToLower(filepath.Base(httpPath))
		if !strings.Contains(lower, hp) && !strings.Contains(lower, base) {
			continue
		}
		for _, entryPath := range entryPaths {
			ep := strings.ToLower(entryPath)
			entryBase := strings.ToLower(filepath.Base(entryPath))
			// Merge language tying the http package into the entrypoint.
			mentionsEntry := strings.Contains(lower, ep) || strings.Contains(lower, entryBase) ||
				strings.Contains(lower, "entrypoint") || strings.Contains(lower, "cmd/")
			mergeCue := strings.Contains(lower, "merge") || strings.Contains(lower, "→") ||
				strings.Contains(lower, "->") || strings.Contains(lower, "into")
			if mentionsEntry && mergeCue {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return proposalMD, ""
	}

	var b strings.Builder
	b.WriteString(strings.TrimSpace(proposalMD))
	b.WriteString("\n\n## Mechanical override (Majordomo)\n\n")
	b.WriteString("Rejected folding server into entrypoint. Keep delivery on its own slice/surface.\n\n")
	for _, httpPath := range httpPaths {
		fmt.Fprintf(&b, "- MUST NOT merge `%s` (server) into an entrypoint package.\n", httpPath)
	}
	for _, entryPath := range entryPaths {
		fmt.Fprintf(&b, "- Entrypoint `%s` may import HTTP packages as wiring only.\n", entryPath)
	}
	note := "MUST NOT merge server packages into an entrypoint. Keep them on a separate delivery slice/surface; sole importer is a wiring note only."
	return b.String(), note
}

// completeEvidencedLibraryBindings loads the refined catalog and architecture brief,
// appends missing slice→library SliceBindings, and writes the local catalog when changed.
func completeEvidencedLibraryBindings(localCatalogPath, architecturePath string) (bool, error) {
	archMD, err := os.ReadFile(architecturePath)
	if err != nil {
		return false, fmt.Errorf("typology library bindings read architecture: %w", err)
	}
	findings := extractArchitectureFindings(string(archMD))
	if len(findings) == 0 {
		return false, nil
	}
	typo, err := catalog.LoadYAML(localCatalogPath)
	if err != nil {
		return false, fmt.Errorf("typology library bindings load catalog: %w", err)
	}
	updated, changed := applyEvidencedLibraryBindings(typo, findings)
	if !changed {
		return false, nil
	}
	if err := catalog.SaveYAML(localCatalogPath, updated); err != nil {
		return false, fmt.Errorf("typology library bindings save catalog: %w", err)
	}
	return true, nil
}

func refineTypologyEvidence(ctx context.Context, opts Options, analysisDir, evidenceDir string, gen TypologySlicePipeline, judgeGen judge.Generator) error {
	manifestPath := filepath.Join(evidenceDir, "manifest.yaml")
	manifest, err := contextstore.ParseTypologyManifest(manifestPath)
	if err != nil {
		return err
	}
	if manifest.Mode == contextstore.TypologyModeFallback ||
		strings.EqualFold(manifest.RefineStatus, contextstore.TypologyRefineSkipped) {
		return nil
	}
	if strings.TrimSpace(opts.TypologyBinary) == "" {
		return fmt.Errorf("typology refine: Typology binary is required")
	}

	draftPath := filepath.Join(analysisDir, analysisDraftCatalogRel)
	draftYAML, err := os.ReadFile(draftPath)
	if err != nil {
		return fmt.Errorf("typology refine read draft: %w", err)
	}
	graphText, err := os.ReadFile(filepath.Join(evidenceDir, manifest.GraphPath))
	if err != nil {
		return fmt.Errorf("typology refine read graph: %w", err)
	}
	contractsRel := strings.TrimSpace(manifest.PackageContractsPath)
	if contractsRel == "" {
		contractsRel = "package_contracts.md"
	}
	contractsText, err := os.ReadFile(filepath.Join(evidenceDir, contractsRel))
	if err != nil {
		return fmt.Errorf("typology refine read package contracts: %w", err)
	}
	rolesRel := strings.TrimSpace(manifest.PackageRolesPath)
	if rolesRel == "" {
		rolesRel = packageRolesRel
	}
	rolesPath := filepath.Join(evidenceDir, rolesRel)
	rolesText, err := os.ReadFile(rolesPath)
	if err != nil {
		return fmt.Errorf("typology refine read package roles: %w", err)
	}
	archDraft, err := os.ReadFile(filepath.Join(analysisDir, analysisDraftArchRel))
	if err != nil {
		return fmt.Errorf("typology refine read architecture draft: %w", err)
	}

	if gen == nil {
		gen = JudgeTypologySlicePipeline{Gen: judgeGen}
	}
	rolesUpdated := string(rolesText)
	if validator, err := newRLMValidatorFromOpts(ctx, opts, analysisDir); err != nil {
		logf("WARN", "typology RLM validator unavailable: %v", err)
	} else if validator != nil {
		updated, err := validatePackageRolesRLM(ctx, validator, analysisDir, evidenceDir, rolesPath, string(rolesText), opts.DigestCache, opts.DigestSkips, opts.DigestModelID)
		if err != nil {
			return err
		}
		if updated != "" {
			rolesUpdated = updated
		}
	}
	rolesText = []byte(rolesUpdated)

	constraintsDoc := buildCapabilityConstraints(mustParseRoles(string(rolesText)))
	constraintsPath := filepath.Join(evidenceDir, packageCapabilityConstraintsRel)
	if err := writeCapabilityConstraints(constraintsPath, constraintsDoc); err != nil {
		return err
	}
	constraintsYAML, err := marshalConstraints(constraintsDoc)
	if err != nil {
		return err
	}

	var ledgerBuilder sliceObjectiveLedgerBuilder
	if g, err := newLedgerBuilderFromOpts(ctx, opts, analysisDir); err != nil {
		logf("WARN", "typology objective ledger RLM unavailable: %v", err)
	} else {
		ledgerBuilder = g
	}
	var clusterAuditor clusterMergeAuditor
	if g, err := newClusterAuditorFromOpts(ctx, opts, analysisDir); err != nil {
		logf("WARN", "typology cluster audit RLM unavailable: %v", err)
	} else {
		clusterAuditor = g
	}
	var catalogAssembler sliceCatalogAssembler
	if g, err := newCatalogAssemblerFromOpts(ctx, opts, analysisDir); err != nil {
		logf("WARN", "typology slice catalog RLM unavailable: %v", err)
	} else {
		catalogAssembler = g
	}

	rrCfg := runreport.Config{
		Enabled:           true,
		RecordModuleCalls: true,
		Dir:               runreportDir(inferenceWorkRoot(opts, analysisDir)),
	}
	var reportErr error
	ctx, finishReport := runreport.StartSession(ctx, rrCfg, runreport.Meta{
		Pipeline: "context-digest",
		Job:      "typology-refine",
		EntityID: manifest.RepoID,
	})
	defer func() { finishReport(reportErr) }()
	fail := func(err error) error {
		reportErr = err
		return err
	}

	out, err := gen.Assemble(ctx, TypologySlicePipelineInput{
		RepoID:                manifest.RepoID,
		ModuleScope:           manifest.ModuleScope,
		DraftCatalogYAML:      string(draftYAML),
		GraphText:             string(graphText),
		PackageContracts:      string(contractsText),
		PackageRoles:          string(rolesText),
		CapabilityConstraints: constraintsYAML,
		ArchitectureDraft:     string(archDraft),
		RepoLayout:            strings.Join(collectTopLevelEntries(analysisDir), "\n"),
		ReadmeSnapshot:        capReadmeSnapshot(readText(filepath.Join(analysisDir, "README.md"))),
		AnalysisDir:           analysisDir,
		EvidenceDir:           evidenceDir,
		LedgerBuilder:         ledgerBuilder,
		ClusterAuditor:        clusterAuditor,
		CatalogAssembler:      catalogAssembler,
		DigestCache:           opts.DigestCache,
		DigestSkips:           opts.DigestSkips,
		DigestModelID:         opts.DigestModelID,
	})
	if err != nil {
		return fail(err)
	}

	clusterPath := filepath.Join(evidenceDir, clusterMergeProposalRel)
	if err := writeRequiredFile(clusterPath, out.ClusterMergeProposalYAML); err != nil {
		return fail(err)
	}
	if err := writeRequiredFile(filepath.Join(evidenceDir, mechanicalGroupingRel), out.MechanicalGroupingYAML); err != nil {
		return fail(err)
	}
	verdictsRel := clusterMergeVerdictsRel
	if strings.TrimSpace(out.ClusterMergeVerdictsYAML) != "" {
		if err := writeRequiredFile(filepath.Join(evidenceDir, verdictsRel), out.ClusterMergeVerdictsYAML); err != nil {
			return fail(err)
		}
	}
	refinedPath := filepath.Join(evidenceDir, manifest.RefinedSnapshotPath)
	if err := writeRequiredFile(refinedPath, out.RefinedCatalogYAML); err != nil {
		return fail(err)
	}
	journeyPath := filepath.Join(evidenceDir, manifest.JourneyPath)
	if _, err := os.Stat(journeyPath); err != nil {
		// Journey is authored after architecture by human-intervention; seed an empty stub.
		if err := writeRequiredFile(journeyPath, "# Journey\n\nPending human-intervention after architecture.\n"); err != nil {
			return fail(err)
		}
	}
	if strings.TrimSpace(out.ObjectiveLedgerYAML) != "" {
		ledgerDoc, err := parseObjectiveLedgerYAML(out.ObjectiveLedgerYAML)
		if err != nil {
			return fail(fmt.Errorf("typology refine write objective ledger: %w", err))
		}
		if err := writeObjectiveLedger(filepath.Join(evidenceDir, sliceObjectiveLedgerRel), ledgerDoc); err != nil {
			return fail(err)
		}
	}
	claimsPath := filepath.Join(evidenceDir, sliceObjectiveClaimsRel)
	if strings.TrimSpace(out.ObjectiveClaimsYAML) != "" {
		claimsDoc, err := parseObjectiveClaimsYAML(out.ObjectiveClaimsYAML)
		if err != nil {
			return fail(fmt.Errorf("typology refine write objective claims: %w", err))
		}
		if err := writeObjectiveClaims(claimsPath, claimsDoc); err != nil {
			return fail(err)
		}
	}
	snapshotPath := filepath.Join(evidenceDir, manifest.SnapshotPath)
	if err := copyFile(refinedPath, snapshotPath); err != nil {
		return fail(fmt.Errorf("typology refine copy snapshot: %w", err))
	}

	archOut := filepath.Join(evidenceDir, manifest.ArchitecturePath)
	refinedLocal := filepath.Join(analysisDir, "tmp", "typology", "refined.yaml")
	if err := copyFile(refinedPath, refinedLocal); err != nil {
		return fail(fmt.Errorf("typology refine stage catalog: %w", err))
	}
	if err := runTypology(ctx, opts.TypologyBinary, analysisDir, "architecture", manifest.ModuleScope,
		"--catalog", refinedLocal, "--out", archOut); err != nil {
		return fail(fmt.Errorf("typology refine architecture: %w", err))
	}
	if _, err := os.Stat(archOut); err != nil {
		fallbackArch := filepath.Join(analysisDir, "docs", "architecture", "typology.md")
		if copyErr := copyFile(fallbackArch, archOut); copyErr != nil {
			return fail(fmt.Errorf("typology refine architecture missing: %w", err))
		}
	}
	if err := polishTypologyArchitectureBrief(archOut); err != nil {
		return fail(err)
	}

	if changed, err := completeEvidencedLibraryBindings(refinedLocal, archOut); err != nil {
		return fail(err)
	} else if changed {
		if err := copyFile(refinedLocal, refinedPath); err != nil {
			return fail(fmt.Errorf("typology refine rewrite catalog after library bindings: %w", err))
		}
		if err := copyFile(refinedPath, snapshotPath); err != nil {
			return fail(fmt.Errorf("typology refine rewrite snapshot after library bindings: %w", err))
		}
		if err := runTypology(ctx, opts.TypologyBinary, analysisDir, "architecture", manifest.ModuleScope,
			"--catalog", refinedLocal, "--out", archOut); err != nil {
			return fail(fmt.Errorf("typology refine architecture after library bindings: %w", err))
		}
		if _, err := os.Stat(archOut); err != nil {
			fallbackArch := filepath.Join(analysisDir, "docs", "architecture", "typology.md")
			if copyErr := copyFile(fallbackArch, archOut); copyErr != nil {
				return fail(fmt.Errorf("typology refine architecture missing after library bindings: %w", err))
			}
		}
		if err := polishTypologyArchitectureBrief(archOut); err != nil {
			return fail(err)
		}
	}

	if err := flagHumanIntervention(ctx, evidenceDir, opts.HumanInterventionGenerator, judgeGen, opts); err != nil {
		return fail(err)
	}

	// Preserve human_intervention_path written by the flagger.
	updated, err := contextstore.ParseTypologyManifest(filepath.Join(evidenceDir, "manifest.yaml"))
	if err != nil {
		return fail(err)
	}
	manifest.HumanInterventionPath = updated.HumanInterventionPath
	manifest.PackageCapabilityConstraintsPath = packageCapabilityConstraintsRel
	manifest.SliceObjectiveClaimsPath = sliceObjectiveClaimsRel
	manifest.SliceObjectiveLedgerPath = sliceObjectiveLedgerRel
	manifest.ClusterProposalPath = clusterMergeProposalRel
	manifest.MechanicalGroupingPath = mechanicalGroupingRel
	if strings.TrimSpace(out.ClusterMergeVerdictsYAML) != "" {
		manifest.ClusterMergeVerdictsPath = clusterMergeVerdictsRel
	}
	manifest.RefineStatus = contextstore.TypologyRefineComplete
	return fail(writeTypologyManifest(evidenceDir, manifest))
}

func capReadmeSnapshot(text string) string {
	runes := []rune(text)
	if len(runes) <= maxReadmeSnapshotRunes {
		return text
	}
	return string(runes[:maxReadmeSnapshotRunes]) + "\n[... README truncated for typology cluster/refine ...]\n"
}

func newRLMValidatorFromOpts(ctx context.Context, opts Options, analysisDir string) (packageRoleRLMValidator, error) {
	if opts.ConfigDir == "" || opts.RepoID == "" {
		return nil, fmt.Errorf("config-dir and repo-id required for RLM validate")
	}
	defaults, err := config.LoadDefaults(opts.ConfigDir)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadRepoFile(opts.ConfigDir, opts.RepoID, defaults)
	if err != nil {
		return nil, err
	}
	return newStropPackageRoleRLM(ctx, cfg, inferenceWorkRoot(opts, analysisDir))
}

func appendUnique(list []string, item string) []string {
	for _, v := range list {
		if v == item {
			return list
		}
	}
	return append(list, item)
}

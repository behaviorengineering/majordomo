package typology

import "github.com/behaviorengineering/strop/evaluation/criteria"

const (
	CriterionIDSurfaces          criteria.CriterionID = "majordomo_typology_surfaces"
	CriterionIDDebtWhenFindings  criteria.CriterionID = "majordomo_typology_debt_when_findings"
	CriterionIDObjectives        criteria.CriterionID = "majordomo_typology_objectives"
	CriterionIDAdapterSurfaces   criteria.CriterionID = "majordomo_typology_adapter_surfaces"
	CriterionIDJourneyConsistent criteria.CriterionID = "majordomo_typology_journey_consistent"
	CriterionIDJourneyCounsel    criteria.CriterionID = "majordomo_typology_journey_counsel"
	CriterionIDRoleGrounding     criteria.CriterionID = "majordomo_typology_role_grounding"
	CriterionIDSliceOwnership    criteria.CriterionID = "majordomo_typology_slice_ownership"

	CriterionIDClusterCounsel  criteria.CriterionID = "majordomo_typology_cluster_counsel"
	CriterionIDClusterDelivery criteria.CriterionID = "majordomo_typology_cluster_delivery"

	CriterionIDInterventionCoverage   criteria.CriterionID = "majordomo_typology_intervention_coverage"
	CriterionIDInterventionNoInvent   criteria.CriterionID = "majordomo_typology_intervention_no_invent"
	CriterionIDInterventionStatus     criteria.CriterionID = "majordomo_typology_intervention_status"
	CriterionIDInterventionTutorVoice criteria.CriterionID = "majordomo_typology_intervention_tutor_voice"
	CriterionIDInterventionCounsel    criteria.CriterionID = "majordomo_typology_intervention_counsel"
)

// CriterionIDs is the typology refine boundary rubric pack.
var CriterionIDs = []criteria.CriterionID{
	CriterionIDSurfaces,
	CriterionIDDebtWhenFindings,
	CriterionIDObjectives,
	CriterionIDAdapterSurfaces,
	CriterionIDJourneyConsistent,
	CriterionIDJourneyCounsel,
	CriterionIDRoleGrounding,
	CriterionIDSliceOwnership,
}

// ClusterCriterionIDs is the typology cluster-pass counsel rubric pack.
var ClusterCriterionIDs = []criteria.CriterionID{
	CriterionIDClusterCounsel,
	CriterionIDClusterDelivery,
}

// InterventionCriterionIDs is the human-intervention flagger rubric pack (legacy combined).
var InterventionCriterionIDs = []criteria.CriterionID{
	CriterionIDInterventionCoverage,
	CriterionIDInterventionNoInvent,
	CriterionIDInterventionStatus,
	CriterionIDInterventionTutorVoice,
	CriterionIDInterventionCounsel,
}

// InterventionJourneyCriterionIDs scores journey rewrite after findings.
var InterventionJourneyCriterionIDs = []criteria.CriterionID{
	CriterionIDInterventionCoverage,
	CriterionIDInterventionStatus,
	CriterionIDInterventionCounsel,
}

// InterventionBriefCriterionIDs scores the operator briefing.
var InterventionBriefCriterionIDs = []criteria.CriterionID{
	CriterionIDInterventionCoverage,
	CriterionIDInterventionNoInvent,
	CriterionIDInterventionTutorVoice,
	CriterionIDInterventionCounsel,
}

// InterventionPRPriorityCriterionIDs scores the cold-reader PR summary.
var InterventionPRPriorityCriterionIDs = []criteria.CriterionID{
	CriterionIDInterventionCoverage,
	CriterionIDInterventionTutorVoice,
	CriterionIDInterventionCounsel,
}

// FindingCommentCriterionIDs scores one PR comment for a single finding.
var FindingCommentCriterionIDs = []criteria.CriterionID{
	CriterionIDInterventionNoInvent,
	CriterionIDInterventionTutorVoice,
	CriterionIDInterventionCounsel,
}

// Register adds typology refine rubrics onto the shared strop criterion registry.
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
		Description: `Slice objectives must match the evidence-first slice_objective_ledger, and derived objective_claims must not intersect the must_not union of owned packages from package_capability_constraints (roles + fills_dto edges). dto/data_shape packages must not claim synchronize_state or merge_adapters.`,
		Scoring: `2 points: Ledger objectives match the catalog and claims stay within allowed capabilities.
0 points: Catalog drifts from ledger, claims intersect must_not, claims are missing for constrained packages, or unknown claim codes.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDSliceOwnership,
		Name:        "One package, one owning slice",
		Description: `Each package path has one owner. Hollow slices that own no packages must not keep SliceBindings after an HTTP/entrypoint split; remount edges onto the slice that owns the packages.`,
		Scoring: `2 points: No duplicate package owners; no package-less binding holders.
0 points: Empty slice still in SliceBindings, or the same path claimed by two slices.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDClusterCounsel,
		Name:        "Cluster proposal argues merges and debt",
		Description: `cluster_proposal_md rationale and proposed merges explain why this grouping, what was rejected, cost of the alternative, and the lean. Boundary debt is not a generic mitigation column.`,
		Scoring: `2 points: Merges and debt argue with alternatives and a lean.
0 points: Merge inventory only, or debt that only says approve binding or refactor.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDClusterDelivery,
		Name:        "Cluster starts from delivery facts",
		Description: `Cluster classifies from package_roles (observed topology), the door-walk mechanical seed (door-private / shared / unreached), plus contracts. Folder names are never evidence. Sole importer is wiring. dto is not owned by an aggregator; exec_runner is not CLI furniture; aggregator is not kind: ui.`,
		Scoring: `2 points: Proposal honors package_roles and the door-walk seed (private vs shared), and treats fills_dto / uses_runner as wiring, not false ownership smells.
0 points: Merges dto into aggregator as UI domain, folds server into CLI for sole importer, ignores door-private/shared facts, ignores RLM agreement metadata when present, or labels aggregator as the website from the path word dashboard.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDInterventionCoverage,
		Name:        "Every architecture finding is flagged for humans",
		Description: `Each open architecture finding appears in journey debt, human_intervention_md, and pr_priority_md.`,
		Scoring: `2 points: Every findings_list entry is represented in debt and priority outputs.
0 points: Any finding is missing from those surfaces.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDInterventionNoInvent,
		Name:        "Do not invent architecture to clear findings",
		Description: `The flagger outputs markdown only. It must not invent catalog YAML, libraries membership, or ownership rewrites. Evidenced slice-to-library bindings belong in the proposal catalog (refine/deterministic pass), not as rubber-stamp asks. Counsel focuses on whether library placement or remaining couplings are right.`,
		Scoring: `2 points: Outputs argue normative forks (keep library, fold into slice, decouple, slice-to-slice binding, temporary debt) without inventing catalog fixes or asking humans to stamp mechanical slice-to-library edges.
0 points: Output invents bindings/libraries/ownership as if findings were resolved, or reduces counsel to "approve this slice-to-library binding".`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDInterventionStatus,
		Name:        "Status stays open while findings remain",
		Description: `When findings_list is non-empty, journey Status must not claim refinement complete.`,
		Scoring: `1 point: Status remains open while findings exist, or findings are empty.
0 points: Status claims complete while findings remain.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDInterventionTutorVoice,
		Name:        "PR priority is a cold-read tutor briefing",
		Description: `pr_priority_md (and human_intervention_md) must teach a reader who has never seen this repo. Gloss jargon in the same sentence. Lead with what Majordomo's Typology digest proposes on the context branch and why, then smell, alternatives, and a lean. Keep package or slice ids so coverage still matches. A gloss without a lean fails.`,
		Scoring: `2 points: Each finding is explained in product terms with a gloss and a lean, attributed as Majordomo/Typology context proposals.
0 points: Jargon-only bullets, imperative titles such as "Formalize Config Access", explanation without a recommended lean, or copy that sounds like a consented product-repo reorganization ("we successfully reorganized").`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDInterventionCounsel,
		Name:        "Intervention and PR priority argue",
		Description: `Each finding in pr_priority_md, human_intervention_md, and journey debt has smell, alternatives with a cost, and a lean. Attribute the speaker to Majordomo/Typology digest. MUST NOT punt to journey_md, stop at approve-or-refactor, or claim the product team already reorganized the repo.`,
		Scoring: `2 points: Counsel is self-contained with smell, alternatives, and lean, framed as context-branch proposals.
0 points: Import inventory only, "approve or refactor" as the whole advice, "see journey_md", or corporate "we reorganized / we consolidated" shipped-sounding claims.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
}

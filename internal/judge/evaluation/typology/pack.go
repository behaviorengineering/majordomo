package typology

import "github.com/behaviorengineering/strop/evaluation/criteria"

const (
	CriterionIDSurfaces          criteria.CriterionID = "majordomo_typology_surfaces"
	CriterionIDDebtWhenFindings  criteria.CriterionID = "majordomo_typology_debt_when_findings"
	CriterionIDObjectives        criteria.CriterionID = "majordomo_typology_objectives"
	CriterionIDAdapterSurfaces   criteria.CriterionID = "majordomo_typology_adapter_surfaces"
	CriterionIDJourneyConsistent criteria.CriterionID = "majordomo_typology_journey_consistent"

	CriterionIDInterventionCoverage criteria.CriterionID = "majordomo_typology_intervention_coverage"
	CriterionIDInterventionNoInvent criteria.CriterionID = "majordomo_typology_intervention_no_invent"
	CriterionIDInterventionStatus   criteria.CriterionID = "majordomo_typology_intervention_status"
)

// CriterionIDs is the typology refine boundary rubric pack.
var CriterionIDs = []criteria.CriterionID{
	CriterionIDSurfaces,
	CriterionIDDebtWhenFindings,
	CriterionIDObjectives,
	CriterionIDAdapterSurfaces,
	CriterionIDJourneyConsistent,
}

// InterventionCriterionIDs is the human-intervention flagger rubric pack.
var InterventionCriterionIDs = []criteria.CriterionID{
	CriterionIDInterventionCoverage,
	CriterionIDInterventionNoInvent,
	CriterionIDInterventionStatus,
}

// Register adds typology refine rubrics onto the shared strop criterion registry.
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDSurfaces,
		Name:        "Interaction packages on surfaces",
		Description: `User-facing CLI (cmd/), UI, HTTP/API, and dashboard packages belong under surfaces[], not domain-only owns[]. Process-exec adapters such as cliexec are not interaction surfaces.`,
		Scoring: `2 points: cmd/, http/api, ui, dashboard, and server delivery paths sit on surfaces when present.
0 points: Those delivery paths remain only under owns[] with no matching surface.`,
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
		Description: `The flagger must not invent sliceBindings or rewrite package ownership to dismiss findings. It requests human decisions.`,
		Scoring: `2 points: Outputs ask humans to approve bindings, merges, or temporary debt without inventing catalog fixes.
0 points: Output invents bindings or ownership changes as if findings were resolved.`,
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
}

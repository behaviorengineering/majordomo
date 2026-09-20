package modules

// Generator task names registered on the shared ModuleRegistry.
const (
	TaskFileReview     = "filereview"
	TaskDigestStory    = "digest_story"
	TaskBootstrapStory = "bootstrap_story"

	// Slice pipeline domain language (grouping → meaning → catalog).
	TaskTypologySliceGrouping      = "typology_slice_grouping"
	TaskTypologySliceGroupingAudit = "typology_slice_grouping_audit"
	TaskTypologySliceMeaning       = "typology_slice_meaning"
	TaskTypologySliceCatalog       = "typology_slice_catalog"

	TaskTypologyInspect                = "typology_inspect"
	TaskTypologyHumanIntervention      = "typology_human_intervention" // alias for brief module registration
	TaskTypologyInterventionJourney    = "typology_intervention_journey"
	TaskTypologyInterventionBrief      = "typology_intervention_brief"
	TaskTypologyInterventionWeaknesses = "typology_intervention_weaknesses"
	TaskTypologyInterventionPRPriority = "typology_intervention_pr_priority"
	TaskTypologyFindingComment         = "typology_finding_comment"
	TaskSummary                        = "summary"
	TaskTechnical                      = "technical"
)

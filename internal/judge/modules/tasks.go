package modules

// Generator task names registered on the shared ModuleRegistry.
const (
	TaskFileReview                     = "filereview"
	TaskDigestStory                    = "digest_story"
	TaskBootstrapStory                 = "bootstrap_story"
	TaskTypologyCluster                = "typology_cluster"
	TaskTypologyRefine                 = "typology_refine"
	TaskTypologyInspect                = "typology_inspect"
	TaskTypologyHumanIntervention      = "typology_human_intervention" // legacy alias for brief module
	TaskTypologyInterventionJourney    = "typology_intervention_journey"
	TaskTypologyInterventionBrief      = "typology_intervention_brief"
	TaskTypologyInterventionWeaknesses = "typology_intervention_weaknesses"
	TaskTypologyInterventionPRPriority = "typology_intervention_pr_priority"
	TaskTypologyFindingComment         = "typology_finding_comment"
	TaskSummary                        = "summary"
	TaskTechnical                      = "technical"
)

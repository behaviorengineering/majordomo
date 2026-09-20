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
	TaskTypologyHumanIntervention      = "typology_human_intervention" // legacy alias for brief module
	TaskTypologyInterventionJourney    = "typology_intervention_journey"
	TaskTypologyInterventionBrief      = "typology_intervention_brief"
	TaskTypologyInterventionWeaknesses = "typology_intervention_weaknesses"
	TaskTypologyInterventionPRPriority = "typology_intervention_pr_priority"
	TaskTypologyFindingComment         = "typology_finding_comment"
	TaskSummary                        = "summary"
	TaskTechnical                      = "technical"
)

// Legacy task ids (one release). Prefer the TaskTypologySlice* consts above.
// Config ResolveTaskProvider still accepts these via CanonicalTask.
const (
	LegacyTaskTypologyCluster            = "typology_cluster"
	LegacyTaskTypologyClusterAudit       = "typology_cluster_audit"
	LegacyTaskTypologyObjectiveGrounding = "typology_objective_grounding"
	LegacyTaskTypologyRefine             = "typology_refine"
)

// Deprecated aliases kept so older call sites compile during the rename window.
// New code MUST use TaskTypologySliceGrouping / Meaning / Catalog / GroupingAudit.
const (
	TaskTypologyCluster            = TaskTypologySliceGrouping
	TaskTypologyClusterAudit       = TaskTypologySliceGroupingAudit
	TaskTypologyObjectiveGrounding = TaskTypologySliceMeaning
	TaskTypologyRefine             = TaskTypologySliceCatalog
)

// CanonicalTask maps a legacy or current generator task id to the domain id.
func CanonicalTask(task string) string {
	switch task {
	case LegacyTaskTypologyCluster:
		return TaskTypologySliceGrouping
	case LegacyTaskTypologyClusterAudit:
		return TaskTypologySliceGroupingAudit
	case LegacyTaskTypologyObjectiveGrounding:
		return TaskTypologySliceMeaning
	case LegacyTaskTypologyRefine:
		return TaskTypologySliceCatalog
	default:
		return task
	}
}

// LegacyTaskAliases returns deprecated config keys that resolve to the same canonical task.
func LegacyTaskAliases(canonical string) []string {
	switch canonical {
	case TaskTypologySliceGrouping:
		return []string{LegacyTaskTypologyCluster}
	case TaskTypologySliceGroupingAudit:
		return []string{LegacyTaskTypologyClusterAudit}
	case TaskTypologySliceMeaning:
		return []string{LegacyTaskTypologyObjectiveGrounding}
	case TaskTypologySliceCatalog:
		return []string{LegacyTaskTypologyRefine}
	default:
		return nil
	}
}

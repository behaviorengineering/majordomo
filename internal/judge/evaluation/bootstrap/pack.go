package bootstrap

import "github.com/behaviorengineering/strop/evaluation/criteria"

const (
	CriterionIDEvidencedOnly criteria.CriterionID = "majordomo_bootstrap_evidenced_only"
	CriterionIDPreservesForm criteria.CriterionID = "majordomo_bootstrap_preserves_form"
	CriterionIDHonestSeed    criteria.CriterionID = "majordomo_bootstrap_honest_seed"
)

// CriterionIDs is the bootstrap story rubric pack.
var CriterionIDs = []criteria.CriterionID{
	CriterionIDEvidencedOnly,
	CriterionIDPreservesForm,
	CriterionIDHonestSeed,
}

// Register adds bootstrap story rubrics onto the shared strop criterion registry.
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDEvidencedOnly,
		Name:        "Bootstrap claims evidenced",
		Description: `Every new claim in the bootstrap story is traceable to the survey evidence, README, or layout snapshot.`,
		Scoring: `2 points: All additions cite supplied evidence.
0 points: Any invented or unsupported claim.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDPreservesForm,
		Name:        "Bootstrap preserves section form",
		Description: `Updated sections keep the expected markdown structure for the file.`,
		Scoring: `1 point: Valid markdown for the section type.
0 points: Broken structure or wrong section content.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDHonestSeed,
		Name:        "Bootstrap is honest about seeding",
		Description: `Seed-time prose must read like a fresh baseline, not a reconstruction of history.`,
		Scoring: `1 point: The text clearly states it is a seed baseline or otherwise avoids invented history.
0 points: The text implies historical reconstruction that is not evidenced.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
}

package digest

import (
	"github.com/behaviorengineering/strop/evaluation/criteria"
)

const (
	CriterionIDEvidencedOnly criteria.CriterionID = "majordomo_digest_evidenced_only"
	CriterionIDPreservesForm criteria.CriterionID = "majordomo_digest_preserves_form"
)

// CriterionIDs is the digest story section-walk rubric pack.
var CriterionIDs = []criteria.CriterionID{
	CriterionIDEvidencedOnly,
	CriterionIDPreservesForm,
}

// Register adds digest story rubrics onto the shared strop criterion registry.
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDEvidencedOnly,
		Name:        "Digest claims evidenced",
		Description: `Every new claim in the updated section is traceable to the commit diff or subject.`,
		Scoring: `2 points: All additions cite diff-visible evidence.
0 points: Any invented or unsupported claim.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDPreservesForm,
		Name:        "Digest preserves section shape",
		Description: `Updated section keeps markdown structure appropriate to the section id.`,
		Scoring: `1 point: Valid markdown for the section type.
0 points: Broken structure or wrong section content.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
}

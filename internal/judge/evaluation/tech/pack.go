package tech

import (
	"github.com/behaviorengineering/strop/evaluation/criteria"
)

// Majordomo technical-score criterion IDs (product pack; not inside strop).
const (
	CriterionIDH2Structure        criteria.CriterionID = "majordomo_tech_h2_structure"
	CriterionIDBlankLinesFields   criteria.CriterionID = "majordomo_tech_blank_lines"
	CriterionIDFourFields         criteria.CriterionID = "majordomo_tech_four_fields"
	CriterionIDConfirmYesNo       criteria.CriterionID = "majordomo_tech_confirm_yes_no"
	CriterionIDDeclarativeH3      criteria.CriterionID = "majordomo_tech_declarative_h3"
	CriterionIDChecklistSkip      criteria.CriterionID = "majordomo_tech_checklist_skip"
	CriterionIDNoEmDashConnectors criteria.CriterionID = "majordomo_tech_no_emdash"
	CriterionIDNoGenericPhrases   criteria.CriterionID = "majordomo_tech_no_generic_phrases"
)

// CriterionIDs is the ordered pack used by tech evaluation modules.
var CriterionIDs = []criteria.CriterionID{
	CriterionIDH2Structure,
	CriterionIDBlankLinesFields,
	CriterionIDFourFields,
	CriterionIDConfirmYesNo,
	CriterionIDDeclarativeH3,
	CriterionIDChecklistSkip,
	CriterionIDNoEmDashConnectors,
	CriterionIDNoGenericPhrases,
}

func init() {
	Register(criteria.DefaultRegistry())
}

// Register adds technical-score rubrics onto the shared strop criterion registry.
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDH2Structure,
		Name:        "Tech H2 structure",
		Description: `tech-review.md has exactly Correctness Risks, Verification Checklist, Test Coverage Gaps in order.`,
		Scoring: `2 points: Exact three H2s in order.
0 points: Missing, renamed, reordered, or extra H2.`,
		Examples:  `No other H2 headings allowed.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDBlankLinesFields,
		Name:        "Blank lines between risk fields",
		Description: `Each risk entry separates Does/Trigger/Consequence/Confirm with blank lines.`,
		Scoring: `3 points: All consecutive field pairs separated by a blank line.
0 points: Any two field lines consecutive without a blank line.`,
		Examples:  `Between Does and Trigger, Trigger and Consequence, Consequence and Confirm.`,
		MaxPoints: 3.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDFourFields,
		Name:        "Exactly four fields per risk",
		Description: `Each Correctness Risks H3 has Does, Trigger, Consequence, Confirm in that order.`,
		Scoring: `2 points: All entries have exactly those four fields in order.
0 points: Missing, extra, or wrong order.`,
		Examples:  `Labeled **Does:** **Trigger:** **Consequence:** **Confirm:**.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDConfirmYesNo,
		Name:        "Confirm is a yes/no question",
		Description: `Every Confirm ends with ? and starts with an interrogative (Does, Is, Are, Has, Will, Did, Can, Would).`,
		Scoring: `2 points: All Confirm fields valid.
0 points: Any Confirm fails the rule.`,
		Examples:  `Good: Does the handler still run on nil session?`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDDeclarativeH3,
		Name:        "Risk H3s are declarative",
		Description: `Correctness Risks H3s are failure-mode statements, not questions.`,
		Scoring: `2 points: No H3 ends with ? or reads as a question.
0 points: Any interrogative H3.`,
		Examples:  `Fail: "### Will nil session panic?"`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDChecklistSkip,
		Name:        "Checklist ends with Skip",
		Description: `Last Verification Checklist bullet begins with Skip:.`,
		Scoring: `1 point: Last bullet is a Skip entry.
0 points: Last bullet is not Skip:.`,
		Examples:  `Skip: docs-only paths.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoEmDashConnectors,
		Name:        "No em dash connectors",
		Description: `No sentence uses an em dash as a clause connector.`,
		Scoring: `1 point: No em dash connectors.
0 points: Any em dash connector found.`,
		Examples:  `Title clarifications are OK.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoGenericPhrases,
		Name:        "No prohibited generic phrases",
		Description: `Tech review avoids banned hedging and generic quality phrases.`,
		Scoring: `1 point: None of the banned phrases appear.
0 points: Any banned phrase appears.`,
		Examples:  `Banned: may cause issues, could be a problem, needs review, might break, better error handling, more robust, cleaner code, improved maintainability.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
}

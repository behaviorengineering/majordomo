package summary

import (
	"github.com/behaviorengineering/strop/evaluation/criteria"
)

// Majordomo summary-score criterion IDs (product pack; not inside strop).
const (
	CriterionIDH2Structure          criteria.CriterionID = "majordomo_summary_h2_structure"
	CriterionIDWhatGotBuiltBlocks   criteria.CriterionID = "majordomo_summary_what_got_built"
	CriterionIDJudgmentH3           criteria.CriterionID = "majordomo_summary_judgment_h3"
	CriterionIDNoGenericPhrases     criteria.CriterionID = "majordomo_summary_no_generic_phrases"
	CriterionIDNoEmDashConnectors   criteria.CriterionID = "majordomo_summary_no_emdash"
	CriterionIDWhyNamesArtifact     criteria.CriterionID = "majordomo_summary_why_names_artifact"
	CriterionIDNoFilenamesInProse   criteria.CriterionID = "majordomo_summary_no_filenames_in_prose"
	CriterionIDCallerFacingH3s      criteria.CriterionID = "majordomo_summary_caller_facing_h3s"
	CriterionIDNoPrescriptiveFix    criteria.CriterionID = "majordomo_summary_no_prescriptive_fix"
	CriterionIDTeamConsequence      criteria.CriterionID = "majordomo_summary_team_consequence"
)

// CriterionIDs is the ordered pack used by summary evaluation modules.
var CriterionIDs = []criteria.CriterionID{
	CriterionIDH2Structure,
	CriterionIDWhatGotBuiltBlocks,
	CriterionIDJudgmentH3,
	CriterionIDNoGenericPhrases,
	CriterionIDNoEmDashConnectors,
	CriterionIDWhyNamesArtifact,
	CriterionIDNoFilenamesInProse,
	CriterionIDCallerFacingH3s,
	CriterionIDNoPrescriptiveFix,
	CriterionIDTeamConsequence,
}

func init() {
	Register(criteria.DefaultRegistry())
}

// Register adds summary-score rubrics onto the shared strop criterion registry.
func Register(r *criteria.CriterionRegistry) {
	if r == nil {
		return
	}
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDH2Structure,
		Name:        "Summary H2 structure",
		Description: `summary.md has exactly the five required H2 headings in order, with no extras.`,
		Scoring: `2 points: Exact five H2s in order.
0 points: Missing, renamed, reordered, or extra H2.`,
		Examples:  `Required: Why This PR Exists, What Got Built, Low-Risk Changes, Requires Human Judgment, Where to Focus in the Diff.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDWhatGotBuiltBlocks,
		Name:        "What Got Built H3 and before/after",
		Description: `What Got Built has at least one H3 and a Before/After fenced code pair.`,
		Scoring: `2 points: H3 plus Before/After pair present.
0 points: Missing H3 or missing Before/After pair.`,
		Examples:  `First fence opens with # Before:; second with # After:.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDJudgmentH3,
		Name:        "Judgment concerns as H3",
		Description: `Every concern under Requires Human Judgment is an H3.`,
		Scoring: `2 points: All concerns use ### headings.
0 points: Any concern uses bullets or plain paragraphs.`,
		Examples:  `Look for lines beginning with ### under the judgment section.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoGenericPhrases,
		Name:        "No prohibited generic phrases",
		Description: `Summary avoids banned generic quality phrases.`,
		Scoring: `1 point: None of the banned phrases appear.
0 points: Any banned phrase appears.`,
		Examples:  `Banned: better error handling, more robust, cleaner code, improved maintainability, improved readability, more maintainable, better organized.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoEmDashConnectors,
		Name:        "No em dash connectors",
		Description: `No sentence uses an em dash as a clause connector.`,
		Scoring: `1 point: No em dash connectors.
0 points: Any em dash connector found.`,
		Examples:  `Title clarifications with em dashes are OK; clause connectors are not.`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDWhyNamesArtifact,
		Name:        "Why names a specific artifact",
		Description: `Why This PR Exists opening names a class, method, pattern, or concrete gap.`,
		Scoring: `2 points: Specific artifact named.
0 points: Only generic motivation.`,
		Examples:  `Good: names CmsController or session recovery gap.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoFilenamesInProse,
		Name:        "No filenames in narrative prose",
		Description: `File names stay out of flowing narrative (allowed in fences, bullets, skip entries).`,
		Scoring: `2 points: No narrative filename embeds.
0 points: Filename inside narrative sentence.`,
		Examples:  `Code fences and Diff Focus skip lines may name files.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDCallerFacingH3s,
		Name:        "Caller-facing What Got Built H3s",
		Description: `What Got Built H3s are caller-facing or tests; no internal-only headings or count-stuffed titles.`,
		Scoring: `2 points: All H3s caller-facing or tests.
0 points: Internal-only H3 or count in heading.`,
		Examples:  `Fail: graph wiring H3 with no call site; H3 with "113 Screen subclasses".`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDNoPrescriptiveFix,
		Name:        "Judgment does not prescribe fixes",
		Description: `Judgment bodies state risk and what to confirm; they do not instruct the team to perform a fix.`,
		Scoring: `1 point: No instructional fix sentences.
0 points: Any prescriptive action for the team.`,
		Examples:  `Fail: "add it as a subclass", "update REQUIRED_IDENTIFIERS".`,
		MaxPoints: 1.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
	r.Register(criteria.CriterionDescription{
		ID:          CriterionIDTeamConsequence,
		Name:        "Team consequence formula",
		Description: `After each Before/After pair, prose states what the team no longer does and what they do instead.`,
		Scoring: `2 points: Both parts present; not API docs or internals-only.
0 points: Missing formula or reads as API documentation.`,
		Examples:  `Must not list inputs, outputs, exception names as the main content.`,
		MaxPoints: 2.0,
		Category:  criteria.CriterionCategoryOutputQuality,
	})
}

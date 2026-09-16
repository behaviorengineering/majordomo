package contextdigest

import (
	"strings"
	"testing"
)

func TestParseSliceObjectiveLedgerAnswerMultilineQuotedEvidence(t *testing.T) {
	// Live model shape from gitboard reseed: section openers without YAML list dashes.
	answer := `yaml
verdict: grounded
evidence:
"exportedDecls: Client, GitHub, GitLab"
"exportedFuncs: NewGitHub, NewGitLab"
claims:
adapt_external
fill_dto
objective: Provides client implementations to interact with GitHub and GitLab remote repositories.
`
	evidence, claims, objective, verdict, err := parseSliceObjectiveLedgerAnswer(answer)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != ledgerVerdictGrounded {
		t.Fatalf("verdict=%q", verdict)
	}
	if !strings.Contains(objective, "GitHub") {
		t.Fatalf("objective=%q", objective)
	}
	if len(evidence) < 2 {
		t.Fatalf("evidence=%v", evidence)
	}
	if len(claims) < 2 {
		t.Fatalf("claims=%v", claims)
	}
}

func TestParseSliceObjectiveLedgerAnswerSameLineLists(t *testing.T) {
	answer := `verdict: grounded
evidence: InspectSync, BranchSync
claims: adapt_external
objective: Local git checkout inspection.
`
	evidence, claims, objective, verdict, err := parseSliceObjectiveLedgerAnswer(answer)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "grounded" || objective == "" || len(evidence) == 0 || len(claims) == 0 {
		t.Fatalf("got verdict=%q objective=%q evidence=%v claims=%v", verdict, objective, evidence, claims)
	}
}

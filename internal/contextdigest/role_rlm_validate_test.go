package contextdigest

import "testing"

func TestParseRLMRoleAnswer(t *testing.T) {
	role, ev := parseRLMRoleAnswer("role: adapter\nevidence: Client.Fetch")
	if role != roleAdapter {
		t.Fatalf("role=%q", role)
	}
	if ev != "Client.Fetch" {
		t.Fatalf("evidence=%q", ev)
	}
}

func TestApplyRLMAgreementMatch(t *testing.T) {
	n := packageRoleNode{Path: "internal/board", Role: roleDTO, Confidence: 0.9, Evidence: []string{"json_tags"}}
	out := applyRLMAgreement(n, roleDTO, "json tags", 2)
	if out.Agreement != agreementMatch || out.Confidence != confidenceAgreeMatch {
		t.Fatalf("%+v", out)
	}
}

func TestApplyRLMAgreementKeepsStage1(t *testing.T) {
	n := packageRoleNode{
		Path: "internal/server", Role: roleHTTPSurface, Confidence: 0.9,
		Evidence: []string{"go_embed", "delivery:ui"},
	}
	out := applyRLMAgreement(n, roleAggregator, "Collect", 3)
	if out.Role != roleHTTPSurface || out.Agreement != agreementDisagree {
		t.Fatalf("%+v", out)
	}
}

func TestApplyRLMAgreementConflict(t *testing.T) {
	n := packageRoleNode{Path: "internal/x", Role: roleExecRunner, Confidence: 0.8, Evidence: []string{"imports_os_exec"}}
	out := applyRLMAgreement(n, roleAggregator, "Collect", 4)
	if out.Role != roleUnknown || out.Confidence != confidenceConflictBar || out.Agreement != agreementDisagree {
		t.Fatalf("%+v", out)
	}
}

func TestApplyRLMAgreementFillsUnknown(t *testing.T) {
	n := packageRoleNode{Path: "internal/localgit", Role: roleUnknown, Confidence: 0}
	out := applyRLMAgreement(n, roleAdapter, "FetchOrigin", 2)
	if out.Role != roleAdapter || out.Confidence != llmInspectConfidence {
		t.Fatalf("%+v", out)
	}
}

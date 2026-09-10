package contextdigest

import (
	"context"
	"testing"
)

type stubRLMValidator map[string]struct {
	role, evidence string
}

func (s stubRLMValidator) Validate(_ context.Context, pkgPath, _, _, _ string) (string, string, int, error) {
	if v, ok := s[pkgPath]; ok {
		return v.role, v.evidence, 1, nil
	}
	return roleUnknown, "", 1, nil
}

func TestValidatePackageRolesRLMGitboardStyle(t *testing.T) {
	dir := t.TempDir()
	rolesPath := dir + "/package_roles.yaml"
	rolesYAML := `packages:
  - path: internal/board
    role: dto
    confidence: 0.9
    evidence: [json_tags]
    inspected_stage: 1
  - path: internal/server
    role: server
    confidence: 0.9
    evidence: [go_embed, delivery:ui]
    inspected_stage: 1
  - path: internal/triage
    role: unknown
    confidence: 0
    inspected_stage: 2
  - path: internal/pruneagent
    role: unknown
    confidence: 0
    inspected_stage: 2
`
	if err := writePackageRoles(rolesPath, mustParseRoles(rolesYAML)); err != nil {
		t.Fatal(err)
	}
	evidence := dir
	if err := osWrite(evidence+"/package_rlm_context.md", `# Package RLM context index

## ./internal/board
- jsonTags: true
- mechanicalRole: dto

## ./internal/server
- goEmbed: true
- mechanicalRole: server

## ./internal/triage
- mechanicalRole: unknown
### Exported bodies
#### Analyze (func)
`+"```go\nfunc Analyze() {}\n```"+`

## ./internal/pruneagent
- mechanicalRole: unknown
### Exported bodies
#### Investigate (func)
`+"```go\nfunc Investigate() {}\n```"+`
`); err != nil {
		t.Fatal(err)
	}

	stub := stubRLMValidator{
		"internal/board":      {role: roleDTO, evidence: "json"},
		"internal/server":     {role: roleAggregator, evidence: "wrong"}, // stage1 keep server
		"internal/triage":     {role: roleAdapter, evidence: "Analyze"},  // fill unknown
		"internal/pruneagent": {role: roleAggregator, evidence: "Investigate"},
	}
	updated, err := validatePackageRolesRLM(context.Background(), stub, dir, evidence, rolesPath, rolesYAML)
	if err != nil {
		t.Fatal(err)
	}
	doc := mustParseRoles(updated)
	by := roleByPath(doc)
	if by["internal/board"].Agreement != agreementMatch || by["internal/board"].Confidence != confidenceAgreeMatch {
		t.Fatalf("board=%+v", by["internal/board"])
	}
	if by["internal/server"].Role != roleHTTPSurface || by["internal/server"].Agreement != agreementDisagree {
		t.Fatalf("server=%+v", by["internal/server"])
	}
	if by["internal/triage"].Role != roleAdapter {
		t.Fatalf("triage=%+v", by["internal/triage"])
	}
	if by["internal/pruneagent"].Role != roleAggregator {
		t.Fatalf("pruneagent=%+v", by["internal/pruneagent"])
	}
}

func osWrite(path, body string) error {
	return writeText(path, body)
}

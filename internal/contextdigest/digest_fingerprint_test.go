package contextdigest

import "testing"

func TestRejectInspectRoleContradictionMechanicalFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		role string
		src  string
		want bool
	}{
		{
			name: "exec_runner via flag",
			role: roleExecRunner,
			src:  "- importsOsExec: true\n- mechanicalEvidence: imports_os_exec\n",
			want: false,
		},
		{
			name: "exec_runner missing",
			role: roleExecRunner,
			src:  "- importsOsExec: false\n",
			want: true,
		},
		{
			name: "server via flag",
			role: roleHTTPSurface,
			src:  "- importsNetHTTP: true\n- embedsStatic: true\n",
			want: false,
		},
		{
			name: "observability via flag",
			role: roleObservability,
			src:  "- importsOtel: true\n",
			want: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := rejectInspectRoleContradiction(tc.role, tc.src); got != tc.want {
				t.Fatalf("reject=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestRolesIdentitySHAIgnoresEvidenceProse(t *testing.T) {
	t.Parallel()
	a := `packages:
  - path: internal/cliexec
    role: exec_runner
    confidence: 0.8
    evidence: [imports_os_exec, "rlm:Runner.Run"]
    inspected_stage: 3
    mechanical_role: exec_runner
    agreement: match
`
	b := `packages:
  - path: internal/cliexec
    role: exec_runner
    confidence: 0.8
    evidence: [imports_os_exec, "rlm:different quote every reseed"]
    inspected_stage: 3
    mechanical_role: exec_runner
    agreement: match
`
	if rolesIdentitySHA(a) != rolesIdentitySHA(b) {
		t.Fatal("roles identity hash must ignore evidence prose")
	}
	c := `packages:
  - path: internal/cliexec
    role: unknown
    confidence: 0.8
    evidence: [imports_os_exec, "rlm:Runner.Run"]
    inspected_stage: 3
    mechanical_role: exec_runner
    agreement: disagree
`
	if rolesIdentitySHA(a) == rolesIdentitySHA(c) {
		t.Fatal("roles identity hash must change when role decision changes")
	}
}

func TestClusterVerdictsIdentitySHAIgnoresDurationAndReason(t *testing.T) {
	t.Parallel()
	a := `attempt: 3
duration_ms: 12
trace_dir: /tmp/a
merges:
  - id: git
    packages: [internal/a, internal/b]
    verdict: overlay
    reason: first wording
`
	b := `attempt: 3
duration_ms: 9999
trace_dir: /tmp/b
generated_at: 2026-01-01T00:00:00Z
merges:
  - id: git
    packages: [internal/b, internal/a]
    verdict: overlay
    reason: completely different wording
    evidence: [quote one]
`
	if clusterVerdictsIdentitySHA(a) != clusterVerdictsIdentitySHA(b) {
		t.Fatal("verdicts identity hash must ignore duration/trace/reason prose")
	}
	c := `attempt: 3
merges:
  - id: git
    packages: [internal/a, internal/b]
    verdict: accept
`
	if clusterVerdictsIdentitySHA(a) == clusterVerdictsIdentitySHA(c) {
		t.Fatal("verdicts identity hash must change when verdict changes")
	}
}

func TestDraftCatalogIdentitySHAIgnoresWorktreeIDAndBindingOrder(t *testing.T) {
	t.Parallel()
	a := `id: majordomo-typology-111
scope:
  modules: ["."]
slices:
  - id: board
    owns: [{id: b, path: internal/board}]
sliceBindings:
  - from: server
    to: board
    kind: reads
  - from: board
    to: config
    kind: reads
`
	b := `id: majordomo-typology-999
scope:
  modules: ["."]
slices:
  - id: board
    owns: [{id: b, path: internal/board}]
sliceBindings:
  - from: board
    to: config
    kind: reads
  - from: server
    to: board
    kind: reads
`
	if draftCatalogIdentitySHA(a) != draftCatalogIdentitySHA(b) {
		t.Fatal("draft identity hash must ignore typology worktree id and binding order")
	}
	c := `id: majordomo-typology-111
scope:
  modules: ["."]
slices:
  - id: other
    owns: [{id: b, path: internal/board}]
sliceBindings: []
`
	if draftCatalogIdentitySHA(a) == draftCatalogIdentitySHA(c) {
		t.Fatal("draft identity hash must change when slice ids change")
	}
}

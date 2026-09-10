package contextdigest

import "testing"

func TestRejectInspectRoleContradiction(t *testing.T) {
	t.Parallel()
	cases := []struct {
		role   string
		source string
		reject bool
	}{
		{roleDTO, "type Row struct {\n\tID string `json:\"id\"`\n}\n", false},
		{roleDTO, "type Row struct{}\nfunc Analyze() {}\n", true},
		{roleExecRunner, "import \"fmt\"\nfunc Run() {}\n", true},
		{roleExecRunner, "import \"os/exec\"\nfunc Run() {}\n", false},
		{roleConfig, "import \"go.opentelemetry.io/otel\"\nfunc Init() {}\n", true},
		{roleObservability, "import \"fmt\"\nfunc Init() {}\n", true},
		{roleObservability, "import \"go.opentelemetry.io/otel\"\nfunc Init() {}\n", false},
	}
	for _, tc := range cases {
		got := rejectInspectRoleContradiction(tc.role, tc.source)
		if got != tc.reject {
			t.Fatalf("role=%s reject=%v want %v", tc.role, got, tc.reject)
		}
	}
}

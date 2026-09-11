package contextdigest

import (
	"strings"
	"testing"

	"github.com/behaviorengineering/majordomo/internal/llmusage"
)

func TestLLMUsageSummaryFormatTwoTasks(t *testing.T) {
	t.Parallel()
	c := llmusage.New()
	c.Add("typology_refine", 100, 20, 120)
	c.Add("typology_objective_grounding", 200, 50, 250)
	text := llmusage.Format(c.Snapshot())
	if !strings.Contains(text, "total=370") {
		t.Fatalf("want grand total 370 in %q", text)
	}
	if !strings.Contains(text, "typology_refine:") || !strings.Contains(text, "typology_objective_grounding:") {
		t.Fatalf("want both tasks itemized: %q", text)
	}
}

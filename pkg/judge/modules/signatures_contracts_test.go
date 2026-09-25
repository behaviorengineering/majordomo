package modules_test

import (
	"strings"
	"testing"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	jmodules "github.com/behaviorengineering/majordomo/pkg/judge/modules"
)

func TestReviewModulesHaveNonEmptyInstructions(t *testing.T) {
	t.Parallel()
	for _, mod := range []core.Module{
		jmodules.FileReviewModule(),
		jmodules.SummaryModule(),
		jmodules.TechnicalModule(),
	} {
		inst := strings.TrimSpace(mod.GetSignature().Instruction)
		if inst == "" {
			t.Fatalf("%s empty instruction", mod.GetDisplayName())
		}
	}
}

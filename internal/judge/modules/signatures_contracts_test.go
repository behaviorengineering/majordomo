package modules_test

import (
	"testing"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
)

func TestTypologyModulesIncludePackageContractsInput(t *testing.T) {
	t.Parallel()
	for _, mod := range []core.Module{jmodules.TypologyClusterModule(), jmodules.TypologyRefineModule()} {
		found := false
		for _, in := range mod.GetSignature().Inputs {
			if in.Name == "package_contracts" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("module %s missing package_contracts input", mod.GetDisplayName())
		}
	}
}

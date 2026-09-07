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

func TestTypologyHumanInterventionModuleInputs(t *testing.T) {
	t.Parallel()
	mod := jmodules.TypologyHumanInterventionModule()
	want := map[string]bool{"findings_list": false, "architecture_md": false, "journey_md": false}
	for _, in := range mod.GetSignature().Inputs {
		if _, ok := want[in.Name]; ok {
			want[in.Name] = true
		}
	}
	for name, ok := range want {
		if !ok {
			t.Fatalf("missing input %s", name)
		}
	}
	outs := map[string]bool{"human_intervention_md": false, "pr_priority_md": false, "weaknesses_seed_md": false, "journey_md": false}
	for _, out := range mod.GetSignature().Outputs {
		if _, ok := outs[out.Name]; ok {
			outs[out.Name] = true
		}
	}
	for name, ok := range outs {
		if !ok {
			t.Fatalf("missing output %s", name)
		}
	}
}

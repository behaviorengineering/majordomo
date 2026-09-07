package modules_test

import (
	"strings"
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
	inst := strings.ToLower(mod.GetSignature().Instruction)
	for _, needle := range []string{"tutor", "cold", "libraries", "slice-to-library"} {
		if !strings.Contains(inst, needle) {
			t.Fatalf("instruction missing %q: %s", needle, mod.GetSignature().Instruction)
		}
	}
	if strings.Contains(inst, "no long preamble") {
		t.Fatalf("instruction still forbids preamble: %s", mod.GetSignature().Instruction)
	}
}

func TestTypologyRefineModuleLibrariesStance(t *testing.T) {
	t.Parallel()
	inst := strings.ToLower(jmodules.TypologyRefineModule().GetSignature().Instruction)
	for _, needle := range []string{
		"libraries[]",
		"must not invent libraries",
		"platform or capability slice",
		"journey debt",
	} {
		if !strings.Contains(inst, needle) {
			t.Fatalf("refine instruction missing %q: %s", needle, jmodules.TypologyRefineModule().GetSignature().Instruction)
		}
	}
}

func TestTypologyClusterModuleLibrariesStance(t *testing.T) {
	t.Parallel()
	inst := strings.ToLower(jmodules.TypologyClusterModule().GetSignature().Instruction)
	for _, needle := range []string{"libraries[]", "platform slice", "operator-facing debt"} {
		if !strings.Contains(inst, needle) {
			t.Fatalf("cluster instruction missing %q: %s", needle, jmodules.TypologyClusterModule().GetSignature().Instruction)
		}
	}
}

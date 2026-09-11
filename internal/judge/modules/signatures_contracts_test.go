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
		foundContracts := false
		foundReadme := false
		foundRoles := false
		foundMechanical := false
		foundConstraints := false
		foundLedger := false
		foundClaimsOut := false
		for _, in := range mod.GetSignature().Inputs {
			switch in.Name {
			case "package_contracts":
				foundContracts = true
			case "readme_snapshot":
				foundReadme = true
			case "package_roles":
				foundRoles = true
			case "mechanical_grouping_md":
				foundMechanical = true
			case "package_capability_constraints":
				foundConstraints = true
			case "slice_objective_ledger_yaml":
				foundLedger = true
			}
		}
		for _, out := range mod.GetSignature().Outputs {
			if out.Name == "objective_claims_yaml" {
				foundClaimsOut = true
			}
		}
		if !foundContracts {
			t.Fatalf("module %s missing package_contracts input", mod.GetDisplayName())
		}
		if !foundReadme {
			t.Fatalf("module %s missing readme_snapshot input", mod.GetDisplayName())
		}
		if !foundRoles {
			t.Fatalf("module %s missing package_roles input", mod.GetDisplayName())
		}
		if !foundConstraints {
			t.Fatalf("module %s missing package_capability_constraints input", mod.GetDisplayName())
		}
		if mod.GetDisplayName() == "Typology Cluster" && !foundMechanical {
			t.Fatalf("module %s missing mechanical_grouping_md input", mod.GetDisplayName())
		}
		if mod.GetDisplayName() == "Typology Refine" && !foundLedger {
			t.Fatalf("module %s missing slice_objective_ledger_yaml input", mod.GetDisplayName())
		}
		if mod.GetDisplayName() == "Typology Refine" && foundClaimsOut {
			t.Fatalf("module %s must not emit objective_claims_yaml; claims come from the ledger in Go", mod.GetDisplayName())
		}
	}
}

func TestTypologyInterventionBriefModuleInputs(t *testing.T) {
	t.Parallel()
	mod := jmodules.TypologyInterventionBriefModule()
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
	outs := map[string]bool{"human_intervention_md": false}
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
	for _, needle := range []string{
		"tutor",
		"libraries",
		"slice-to-library",
		"lean",
		"counsel",
		"see journey_md",
		"must not defer",
		"majordomo",
		"rubber-stamp",
		"library placement",
	} {
		if !strings.Contains(inst, needle) {
			t.Fatalf("instruction missing %q: %s", needle, mod.GetSignature().Instruction)
		}
	}
}

func TestTypologyInterventionPRPriorityAndFindingComment(t *testing.T) {
	t.Parallel()
	pr := jmodules.TypologyInterventionPRPriorityModule()
	inst := strings.ToLower(pr.GetSignature().Instruction)
	for _, needle := range []string{"cold", "tutor", "lean", "context branch"} {
		if !strings.Contains(inst, needle) {
			t.Fatalf("pr priority missing %q", needle)
		}
	}
	comment := jmodules.TypologyFindingCommentModule()
	foundFinding, foundOut := false, false
	for _, in := range comment.GetSignature().Inputs {
		if in.Name == "finding" {
			foundFinding = true
		}
	}
	for _, out := range comment.GetSignature().Outputs {
		if out.Name == "comment_md" {
			foundOut = true
		}
	}
	if !foundFinding || !foundOut {
		t.Fatalf("finding comment module finding=%v out=%v", foundFinding, foundOut)
	}
}

func TestTypologyRefineModuleLibrariesStance(t *testing.T) {
	t.Parallel()
	inst := strings.ToLower(jmodules.TypologyRefineModule().GetSignature().Instruction)
	for _, needle := range []string{
		"libraries[]",
		"must not invent libraries",
		"package_roles",
		"folder names are never evidence",
		"exec_runner",
		"aggregator",
		"lean",
		"counsel",
		"slice_objective_ledger",
		"copy the ledger objective",
	} {
		if !strings.Contains(inst, needle) {
			t.Fatalf("refine instruction missing %q: %s", needle, jmodules.TypologyRefineModule().GetSignature().Instruction)
		}
	}
	if strings.Contains(inst, "packages under cmd/, http/api, ui, dashboard, or server must sit under surfaces") {
		t.Fatal("refine still uses path-token surface mandate")
	}
}

func TestTypologyClusterModuleLibrariesStance(t *testing.T) {
	t.Parallel()
	inst := strings.ToLower(jmodules.TypologyClusterModule().GetSignature().Instruction)
	for _, needle := range []string{
		"package_roles",
		"mechanical_grouping_md",
		"folder and path words",
		"never evidence",
		"fills_dto",
		"uses_runner",
		"exec_runner",
		"aggregator",
		"optional overlay",
		"libraries[]",
		"lean",
		"counsel",
		"approve binding or refactor",
		"authoritative for door-private",
	} {
		if !strings.Contains(inst, needle) {
			t.Fatalf("cluster instruction missing %q: %s", needle, jmodules.TypologyClusterModule().GetSignature().Instruction)
		}
	}
	if strings.Contains(inst, "sole importer: package imported by only one caller -> merge into caller") {
		t.Fatal("cluster instruction still uses sole-importer merge-into-caller heuristic")
	}
}

func TestTypologyInspectModuleForbidsPathNames(t *testing.T) {
	t.Parallel()
	inst := strings.ToLower(jmodules.TypologyInspectModule().GetSignature().Instruction)
	for _, needle := range []string{"must not use the directory", "unknown", "os/exec"} {
		if !strings.Contains(inst, needle) {
			t.Fatalf("inspect instruction missing %q", needle)
		}
	}
}

func TestTypologyModulesShareConsultantCounselContract(t *testing.T) {
	t.Parallel()
	for _, mod := range []core.Module{
		jmodules.TypologyClusterModule(),
		jmodules.TypologyRefineModule(),
		jmodules.TypologyInterventionBriefModule(),
		jmodules.TypologyInterventionPRPriorityModule(),
		jmodules.TypologyFindingCommentModule(),
	} {
		inst := strings.ToLower(mod.GetSignature().Instruction)
		for _, needle := range []string{
			"consultant counsel",
			"recommended lean",
			"must not invent decoy",
			"must not defer",
			"majordomo",
			"typology digest",
			"corporate \"we\"",
			"architecture-grounding proposals",
			"slice-to-library",
			"complete a library classification",
		} {
			if !strings.Contains(inst, needle) {
				t.Fatalf("%s instruction missing %q: %s", mod.GetDisplayName(), needle, mod.GetSignature().Instruction)
			}
		}
	}
}

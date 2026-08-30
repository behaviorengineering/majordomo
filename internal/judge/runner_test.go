package judge_test

import (
	"testing"

	"github.com/behaviorengineering/strop/dspy/registry"
	"github.com/behaviorengineering/strop/evaluation/criteria"

	"github.com/behaviorengineering/majordomo/internal/judge"
	summarypack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/summary"
	techpack "github.com/behaviorengineering/majordomo/internal/judge/evaluation/tech"
)

func TestRegisterPacks(t *testing.T) {
	r := criteria.NewCriterionRegistry()
	judge.RegisterPacks(r)
	for _, id := range summarypack.CriterionIDs {
		if _, err := r.Get(id); err != nil {
			t.Fatalf("summary criterion %s: %v", id, err)
		}
	}
	for _, id := range techpack.CriterionIDs {
		if _, err := r.Get(id); err != nil {
			t.Fatalf("tech criterion %s: %v", id, err)
		}
	}
}

func TestNewJobRunner(t *testing.T) {
	reg := registry.NewModuleRegistry()
	runner := judge.NewJobRunner(reg, nil, nil, nil)
	if runner == nil {
		t.Fatal("nil JobRunner")
	}
}

package agenting

import "testing"

func TestSelectModeAndGlobs(t *testing.T) {
	idx := Index{
		Packs: map[string]Pack{
			"overview": {Modes: []string{ModeFiles, ModeSummary}},
			"auth": {
				Globs: []string{"**/auth/**"},
				Modes: []string{ModeFiles},
			},
		},
		order: []string{"overview", "auth"},
	}
	got := Select(idx, ModeFiles, []string{"internal/auth/jwt.go"})
	want := []string{"overview", "auth"}
	if len(got) != len(want) {
		t.Fatalf("Select = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Select = %v, want %v", got, want)
		}
	}
}

func TestSelectSkipsUnmatchedGlob(t *testing.T) {
	idx := Index{
		Packs: map[string]Pack{
			"auth": {
				Globs: []string{"**/auth/**"},
				Modes: []string{ModeFiles},
			},
		},
		order: []string{"auth"},
	}
	if got := Select(idx, ModeFiles, []string{"internal/db/query.go"}); len(got) != 0 {
		t.Fatalf("Select = %v, want none", got)
	}
}

func TestSelectModeForSkill(t *testing.T) {
	if got := ModeForSkill("pr-review-summary"); got != ModeSummary {
		t.Fatalf("ModeForSkill(summary) = %q", got)
	}
	if got := ModeForSkill("pr-review-technical"); got != ModeTechnical {
		t.Fatalf("ModeForSkill(technical) = %q", got)
	}
	if got := ModeForSkill("pr-review-code"); got != ModeFiles {
		t.Fatalf("ModeForSkill(code) = %q", got)
	}
}

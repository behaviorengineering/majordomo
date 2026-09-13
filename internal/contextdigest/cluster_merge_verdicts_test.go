package contextdigest

import (
	"strings"
	"testing"
)

func TestParseProposedMergesYAMLEmptyList(t *testing.T) {
	merges, err := parseProposedMergesYAML("[]")
	if err != nil {
		t.Fatal(err)
	}
	if len(merges) != 0 {
		t.Fatalf("merges=%v", merges)
	}
}

func TestParseProposedMergesYAMLMissing(t *testing.T) {
	_, err := parseProposedMergesYAML("")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseProposedMergesYAMLRows(t *testing.T) {
	raw := `- id: git
  packages: [internal/remotegit, internal/localgit]
  intent: slice
- id: analysis
  packages: [internal/pruneagent, internal/triage]
  intent: nickname
`
	merges, err := parseProposedMergesYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(merges) != 2 {
		t.Fatalf("len=%d", len(merges))
	}
	if merges[0].ID != "git" || merges[0].Intent != mergeIntentSlice {
		t.Fatalf("row0=%+v", merges[0])
	}
	if got := strings.Join(merges[0].Packages, ","); got != "internal/localgit,internal/remotegit" {
		t.Fatalf("sorted packages=%q", got)
	}
	if merges[1].Intent != mergeIntentNickname {
		t.Fatalf("row1 intent=%q", merges[1].Intent)
	}
}

func TestStickyVerdictMapIgnoresNicknameRename(t *testing.T) {
	sticky := stickyVerdictMap{}
	sticky.put(clusterMergeVerdict{
		ID:       "git",
		Packages: []string{"internal/localgit", "internal/remotegit"},
		Verdict:  verdictOverlay,
		Reason:   "theme only",
	})
	pending, frozen := sticky.applySticky([]proposedMerge{
		{ID: "forge", Packages: []string{"internal/remotegit", "internal/localgit"}, Intent: mergeIntentSlice},
		{ID: "board", Packages: []string{"internal/board"}, Intent: mergeIntentSlice},
	})
	if len(pending) != 1 || pending[0].ID != "board" {
		t.Fatalf("pending=%+v", pending)
	}
	if len(frozen) != 1 || frozen[0].ID != "forge" || frozen[0].Verdict != verdictOverlay {
		t.Fatalf("frozen=%+v", frozen)
	}
}

func TestDemoteRejectedMergesListAndTeachingSync(t *testing.T) {
	merges := []proposedMerge{
		{ID: "git", Packages: []string{"internal/localgit", "internal/remotegit"}, Intent: mergeIntentSlice},
		{ID: "ok", Packages: []string{"internal/a", "internal/b"}, Intent: mergeIntentSlice},
	}
	demoted, nicknames := demoteRejectedMergesList(merges, []clusterMergeVerdict{
		{ID: "git", Packages: []string{"internal/localgit", "internal/remotegit"}, Verdict: verdictOverlay, Reason: "theme"},
		{ID: "ok", Packages: []string{"internal/a", "internal/b"}, Verdict: verdictAccept, Reason: "companions"},
	})
	byID := map[string]proposedMerge{}
	for _, m := range demoted {
		byID[m.ID] = m
	}
	if byID["git"].Intent != mergeIntentNickname {
		t.Fatalf("git intent=%q", byID["git"].Intent)
	}
	if byID["ok"].Intent != mergeIntentSlice {
		t.Fatalf("ok intent=%q", byID["ok"].Intent)
	}
	if len(nicknames) != 1 {
		t.Fatalf("nicknames=%v", nicknames)
	}
	out := syncDemotedMergesIntoTeachingMD("# Cluster\n\nCounsel only.\n", demoted, nicknames)
	if !strings.Contains(out, "## Teaching nicknames") {
		t.Fatalf("missing nicknames section:\n%s", out)
	}
	if !strings.Contains(out, proposedMergesMachineSection) {
		t.Fatalf("missing teaching machine sync:\n%s", out)
	}
}

func TestPackageSetKeyStable(t *testing.T) {
	a := packageSetKey([]string{"internal/b", "internal/a"})
	b := packageSetKey([]string{"internal/a", "internal/b", "internal/a"})
	if a != b || a == "" {
		t.Fatalf("a=%q b=%q", a, b)
	}
}

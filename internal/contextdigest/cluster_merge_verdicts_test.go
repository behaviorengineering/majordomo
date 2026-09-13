package contextdigest

import (
	"strings"
	"testing"
)

func TestParseProposedMergesMachineEmptyList(t *testing.T) {
	md := "# Cluster\n\n## Proposed merges (machine)\n[]\n\n## Capability constraints (is / is-not)\n\n- `internal/demo`\n"
	merges, err := parseProposedMergesMachine(md)
	if err != nil {
		t.Fatal(err)
	}
	if len(merges) != 0 {
		t.Fatalf("merges=%v", merges)
	}
}

func TestParseProposedMergesMachineMissingSection(t *testing.T) {
	_, err := parseProposedMergesMachine("# Cluster\n\nNo machine block.\n")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseProposedMergesMachineRows(t *testing.T) {
	md := `# Cluster

## Proposed merges (machine)
- id: git
  packages: [internal/remotegit, internal/localgit]
  intent: slice
- id: analysis
  packages: [internal/pruneagent, internal/triage]
  intent: nickname

## Capability constraints (is / is-not)
`
	merges, err := parseProposedMergesMachine(md)
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

func TestDemoteRejectedMerges(t *testing.T) {
	md := `# Cluster

## Proposed merges (machine)
- id: git
  packages: [internal/localgit, internal/remotegit]
  intent: slice
- id: ok
  packages: [internal/a, internal/b]
  intent: slice

## Capability constraints (is / is-not)
`
	out := demoteRejectedMerges(md, []clusterMergeVerdict{
		{ID: "git", Packages: []string{"internal/localgit", "internal/remotegit"}, Verdict: verdictOverlay, Reason: "theme"},
		{ID: "ok", Packages: []string{"internal/a", "internal/b"}, Verdict: verdictAccept, Reason: "companions"},
	})
	merges, err := parseProposedMergesMachine(out)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]proposedMerge{}
	for _, m := range merges {
		byID[m.ID] = m
	}
	if byID["git"].Intent != mergeIntentNickname {
		t.Fatalf("git intent=%q", byID["git"].Intent)
	}
	if byID["ok"].Intent != mergeIntentSlice {
		t.Fatalf("ok intent=%q", byID["ok"].Intent)
	}
	if !strings.Contains(out, "## Teaching nicknames") {
		t.Fatalf("missing nicknames section:\n%s", out)
	}
}

func TestPackageSetKeyStable(t *testing.T) {
	a := packageSetKey([]string{"internal/b", "internal/a"})
	b := packageSetKey([]string{"internal/a", "internal/b", "internal/a"})
	if a != b || a == "" {
		t.Fatalf("a=%q b=%q", a, b)
	}
}

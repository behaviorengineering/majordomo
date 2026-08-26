package cache

import "testing"

func TestValidateReviewCacheBranch(t *testing.T) {
	if err := ValidateReviewCacheBranch("majordomo-pr-reviewer-cache/payments-api"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReviewCacheBranch("evil/branch"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidatePollCacheBranch(t *testing.T) {
	if err := ValidatePollCacheBranch("majordomo-poll-cache/payments-api"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePollCacheBranch("majordomo-poll-cache/"); err == nil {
		t.Fatal("expected error for empty id")
	}
	if err := ValidatePollCacheBranch("other/branch"); err == nil {
		t.Fatal("expected error")
	}
}

func TestClusterFilesHashStable(t *testing.T) {
	a := ClusterFilesHash([]string{"b.py", "a.py"})
	b := ClusterFilesHash([]string{"a.py", "b.py"})
	if a != b || len(a) != 64 {
		t.Fatalf("hash mismatch %s %s", a, b)
	}
}

func TestPollCursorShouldReview(t *testing.T) {
	c := &PollCursor{Heads: map[string]string{"1": "abc"}}

	if ShouldReview(c, "1", "abc", true) {
		t.Fatal("continuous: same head should skip")
	}
	if !ShouldReview(c, "1", "def", true) {
		t.Fatal("continuous: new head should review")
	}
	if ShouldReview(c, "1", "def", false) {
		t.Fatal("one-shot: already reviewed PR should skip even on new head")
	}
	if !ShouldReview(c, "2", "zzz", false) {
		t.Fatal("one-shot: unseen PR should review")
	}
}

package contextstore

import "testing"

func TestValidateContextBranch(t *testing.T) {
	if err := ValidateContextBranch("majordomo-context/payments-api"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateContextBranch("majordomo-context/content-pipelines"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateContextBranch("majordomo-context/"); err == nil {
		t.Fatal("expected error for empty id")
	}
	if err := ValidateContextBranch("majordomo-inference-cache/payments-api"); err == nil {
		t.Fatal("expected error for inference-cache branch")
	}
	if err := ValidateContextBranch("majordomo-pr-reviewer-cache/payments-api"); err == nil {
		t.Fatal("expected error for legacy review-cache branch")
	}
	if err := ValidateContextBranch("main"); err == nil {
		t.Fatal("expected error for default branch")
	}
}

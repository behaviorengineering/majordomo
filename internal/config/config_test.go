package config

import "testing"

func TestCacheBranch(t *testing.T) {
	got := CacheBranch("payments-api")
	want := "majordomo-pr-reviewer-cache/payments-api"
	if got != want {
		t.Fatalf("CacheBranch = %q, want %q", got, want)
	}
}

func TestPollCacheBranch(t *testing.T) {
	got := PollCacheBranch("payments-api")
	want := "majordomo-poll-cache/payments-api"
	if got != want {
		t.Fatalf("PollCacheBranch = %q, want %q", got, want)
	}
}

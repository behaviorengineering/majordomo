package config

import "testing"

func TestCacheBranch(t *testing.T) {
	got := CacheBranch("payments-api")
	want := "majordomo-pr-reviewer-cache/payments-api"
	if got != want {
		t.Fatalf("CacheBranch = %q, want %q", got, want)
	}
}

func TestCacheSkipsDefaultOn(t *testing.T) {
	c := Cache{}
	if !c.SkipsEnabled() {
		t.Fatal("skips should be on by default")
	}
	c.DisableSkips = true
	if c.SkipsEnabled() {
		t.Fatal("disableSkips should turn skips off")
	}
}

package config

import "testing"

func TestInferenceCacheBranch(t *testing.T) {
	got := InferenceCacheBranch("payments-api")
	want := "majordomo-inference-cache/payments-api"
	if got != want {
		t.Fatalf("InferenceCacheBranch = %q, want %q", got, want)
	}
	if CacheBranch("payments-api") != got {
		t.Fatalf("CacheBranch alias mismatch")
	}
	if DigestCacheBranch("payments-api") != got {
		t.Fatalf("DigestCacheBranch alias mismatch")
	}
}

func TestContextBranch(t *testing.T) {
	got := ContextBranch("payments-api")
	want := "majordomo-context/payments-api"
	if got != want {
		t.Fatalf("ContextBranch = %q, want %q", got, want)
	}
}

func TestTypologyPromoteBranch(t *testing.T) {
	got := TypologyPromoteBranch("payments-api")
	want := "majordomo-typology/payments-api"
	if got != want {
		t.Fatalf("TypologyPromoteBranch = %q, want %q", got, want)
	}
	upd := TypologyPromoteUpdateBranch("payments-api")
	if upd != want+"-update" {
		t.Fatalf("TypologyPromoteUpdateBranch = %q, want %q", upd, want+"-update")
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

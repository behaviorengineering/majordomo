package contextgate

import "testing"

func TestParseComment(t *testing.T) {
	cases := []struct {
		body   string
		action Action
		payload string
	}{
		{"@majordomo reject bad chronology", ActionReject, "bad chronology"},
		{"@majordomo done", ActionDone, ""},
		{"@majordomo why force-pushed main", ActionWhy, "force-pushed main"},
		{"looks good", ActionIgnore, ""},
	}
	for _, tc := range cases {
		got := ParseComment(tc.body, DefaultPrefix)
		if got.Action != tc.action || got.Payload != tc.payload {
			t.Fatalf("ParseComment(%q) = %+v, want action=%v payload=%q", tc.body, got, tc.action, tc.payload)
		}
	}
}

func TestApplyComments(t *testing.T) {
	comments := []Comment{
		{Body: "@majordomo reject first"},
		{Body: "@majordomo done"},
		{Body: "@majordomo reject final reason"},
	}
	reject, done, why := ApplyComments(comments, DefaultPrefix)
	if reject != "final reason" || done || why != "" {
		t.Fatalf("reject=%q done=%v why=%q", reject, done, why)
	}
}

func TestSidecarRegenRequested(t *testing.T) {
	s := Sidecar{Status: StatusRejected, RejectReason: "fix it"}
	if !s.RegenRequested() {
		t.Fatal("expected regen")
	}
	if s.ReadyToMerge() {
		t.Fatal("rejected should not be ready")
	}
	s = Sidecar{Status: StatusDone}
	if !s.ReadyToMerge() {
		t.Fatal("done should be ready")
	}
}

package agent

import "testing"

func TestParseScore(t *testing.T) {
	n, ok := ParseScore("blah\nSCORE: 17\nmore")
	if !ok || n != 17 {
		t.Fatalf("got %d %v", n, ok)
	}
	_, ok = ParseScore("no score here")
	if ok {
		t.Fatal("expected false")
	}
}

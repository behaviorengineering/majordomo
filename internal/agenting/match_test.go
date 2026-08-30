package agenting

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"**/*.go", "internal/auth/handler.go", true},
		{"**/auth/**", "pkg/auth/token.go", true},
		{"**/auth/**", "internal/db/query.go", false},
		{"*.md", "README.md", true},
		{"internal/*.go", "internal/foo.go", true},
		{"**", "any/path/here", true},
	}
	for _, tc := range cases {
		if got := MatchGlob(tc.pattern, tc.name); got != tc.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

package agent

import (
	"strings"
	"testing"
)

func TestParseRisksByFile(t *testing.T) {
	md := `# Tech

### Risk A

**Does:** ` + "`src/a.py`" + ` something

---

### Risk B

**Does:** ` + "`src/b.py`" + ` other

---

### Not cited

No does line here.
`
	risks, order := ParseRisksByFile(md)
	if len(order) != 2 || order[0] != "src/a.py" || order[1] != "src/b.py" {
		t.Fatalf("order=%v", order)
	}
	if len(risks["src/a.py"]) != 1 || !strings.Contains(risks["src/a.py"][0], "Risk A") {
		t.Fatalf("risks a: %v", risks["src/a.py"])
	}
}

func TestFileSlug(t *testing.T) {
	if got := fileSlug("src/foo_bar.py"); got != "src-foo-bar-py" {
		t.Fatalf("got %q", got)
	}
}

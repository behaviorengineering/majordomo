package llmusage

import (
	"context"
	"strings"
	"testing"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
)

func TestCollectorAggregatesAndFormats(t *testing.T) {
	t.Parallel()
	c := New()
	c.Add("typology_refine", 100, 40, 140)
	c.Add("typology_refine", 50, 10, 60)
	c.Add("file_review", 20, 5, 25)
	c.AddFromTokenUsage("typology_cluster", nil)

	s := c.Snapshot()
	if s.Calls != 4 || s.PromptTokens != 170 || s.CompletionTokens != 55 || s.TotalTokens != 225 {
		t.Fatalf("totals=%+v", s)
	}
	if s.MissingUsageCalls != 1 {
		t.Fatalf("missing=%d", s.MissingUsageCalls)
	}
	if len(s.Tasks) != 3 || s.Tasks[0].Task != "file_review" {
		t.Fatalf("tasks=%+v", s.Tasks)
	}
	text := Format(s)
	if !strings.Contains(text, "LLM usage total:") || !strings.Contains(text, "typology_refine:") {
		t.Fatalf("format=%q", text)
	}
	if !strings.Contains(text, "missing_usage_calls=1") {
		t.Fatalf("expected missing in format: %q", text)
	}
}

func TestFromContextAndPush(t *testing.T) {
	c := New()
	ctx := WithCollector(context.Background(), c)
	FromContext(ctx).AddTokenUsageValue("t1", core.TokenUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3})
	if c.Snapshot().TotalTokens != 3 {
		t.Fatalf("ctx collector not used")
	}

	c2 := New()
	Push(c2)
	defer Pop()
	FromContext(context.Background()).Add("t2", 4, 5, 9)
	if c2.Snapshot().TotalTokens != 9 {
		t.Fatalf("active collector not used: %+v", c2.Snapshot())
	}
}

func TestNilCollectorNoop(t *testing.T) {
	t.Parallel()
	var c *Collector
	c.Add("x", 1, 1, 2)
	c.AddFromTokenUsage("x", &core.TokenUsage{PromptTokens: 1})
	if Format(c.Snapshot()) != "LLM usage: (no LLM calls recorded)" {
		t.Fatalf("unexpected")
	}
}

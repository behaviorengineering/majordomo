// Package llmusage aggregates provider-reported LLM token usage for a Majordomo run.
package llmusage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
)

type ctxKey struct{}

// TaskRow is one task's accumulated usage in a Snapshot.
type TaskRow struct {
	Task              string `json:"task"`
	PromptTokens      int    `json:"prompt_tokens"`
	CompletionTokens  int    `json:"completion_tokens"`
	TotalTokens       int    `json:"total_tokens"`
	Calls             int    `json:"calls"`
	MissingUsageCalls int    `json:"missing_usage_calls,omitempty"`
}

// Summary is a sorted itemization plus totals for one run.
type Summary struct {
	Tasks             []TaskRow `json:"tasks"`
	PromptTokens      int       `json:"prompt_tokens"`
	CompletionTokens  int       `json:"completion_tokens"`
	TotalTokens       int       `json:"total_tokens"`
	Calls             int       `json:"calls"`
	MissingUsageCalls int       `json:"missing_usage_calls,omitempty"`
}

type taskBucket struct {
	prompt, completion, total int
	calls, missing            int
}

// Collector accumulates token usage by task name. Safe for concurrent use.
type Collector struct {
	mu     sync.Mutex
	byTask map[string]*taskBucket
}

// New returns an empty Collector.
func New() *Collector {
	return &Collector{byTask: make(map[string]*taskBucket)}
}

// Add records one call's tokens under task. Empty task becomes "unknown".
func (c *Collector) Add(task string, prompt, completion, total int) {
	if c == nil {
		return
	}
	task = strings.TrimSpace(task)
	if task == "" {
		task = "unknown"
	}
	if total <= 0 && prompt+completion > 0 {
		total = prompt + completion
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	b := c.byTask[task]
	if b == nil {
		b = &taskBucket{}
		c.byTask[task] = b
	}
	b.prompt += prompt
	b.completion += completion
	b.total += total
	b.calls++
	if prompt == 0 && completion == 0 && total == 0 {
		b.missing++
	}
}

// AddFromTokenUsage records provider usage, or a missing-usage call when usage is nil/empty.
func (c *Collector) AddFromTokenUsage(task string, usage *core.TokenUsage) {
	if c == nil {
		return
	}
	if usage == nil {
		c.Add(task, 0, 0, 0)
		return
	}
	c.Add(task, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
}

// AddTokenUsageValue records a non-pointer TokenUsage (e.g. RLM CompletionResult.Usage).
func (c *Collector) AddTokenUsageValue(task string, usage core.TokenUsage) {
	if c == nil {
		return
	}
	c.Add(task, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
}

// Snapshot returns a sorted copy of accumulated usage.
func (c *Collector) Snapshot() Summary {
	if c == nil {
		return Summary{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := Summary{Tasks: make([]TaskRow, 0, len(c.byTask))}
	for task, b := range c.byTask {
		out.Tasks = append(out.Tasks, TaskRow{
			Task:              task,
			PromptTokens:      b.prompt,
			CompletionTokens:  b.completion,
			TotalTokens:       b.total,
			Calls:             b.calls,
			MissingUsageCalls: b.missing,
		})
		out.PromptTokens += b.prompt
		out.CompletionTokens += b.completion
		out.TotalTokens += b.total
		out.Calls += b.calls
		out.MissingUsageCalls += b.missing
	}
	sort.Slice(out.Tasks, func(i, j int) bool { return out.Tasks[i].Task < out.Tasks[j].Task })
	return out
}

// Format renders an itemized list plus totals for operator logs.
func Format(s Summary) string {
	if s.Calls == 0 && len(s.Tasks) == 0 {
		return "LLM usage: (no LLM calls recorded)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "LLM usage total: prompt=%d completion=%d total=%d calls=%d",
		s.PromptTokens, s.CompletionTokens, s.TotalTokens, s.Calls)
	if s.MissingUsageCalls > 0 {
		fmt.Fprintf(&b, " missing_usage_calls=%d", s.MissingUsageCalls)
	}
	b.WriteByte('\n')
	b.WriteString("LLM usage by task:")
	for _, row := range s.Tasks {
		fmt.Fprintf(&b, "\n  %s: prompt=%d completion=%d total=%d calls=%d",
			row.Task, row.PromptTokens, row.CompletionTokens, row.TotalTokens, row.Calls)
		if row.MissingUsageCalls > 0 {
			fmt.Fprintf(&b, " missing_usage_calls=%d", row.MissingUsageCalls)
		}
	}
	return b.String()
}

// WithCollector attaches c to ctx.
func WithCollector(ctx context.Context, c *Collector) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, c)
}

// FromContext returns the collector on ctx, or the run-scoped active collector, or nil.
func FromContext(ctx context.Context) *Collector {
	if ctx != nil {
		if c, ok := ctx.Value(ctxKey{}).(*Collector); ok && c != nil {
			return c
		}
	}
	return Active()
}

// RecordExecutionState copies dspy ExecutionState token usage for task into the collector.
func RecordExecutionState(ctx context.Context, task string) {
	c := FromContext(ctx)
	if c == nil {
		return
	}
	state := core.GetExecutionState(ctx)
	if state == nil {
		c.Add(task, 0, 0, 0)
		return
	}
	c.AddFromTokenUsage(task, state.GetTokenUsage())
}

var (
	activeMu sync.Mutex
	active   []*Collector
)

// Push sets c as the run-scoped collector (for callers that use context.Background).
// Prefer WithCollector when a context is threaded; Push covers digest/review Background paths.
func Push(c *Collector) {
	if c == nil {
		return
	}
	activeMu.Lock()
	defer activeMu.Unlock()
	active = append(active, c)
}

// Pop removes the most recently Pushed collector.
func Pop() {
	activeMu.Lock()
	defer activeMu.Unlock()
	if len(active) == 0 {
		return
	}
	active = active[:len(active)-1]
}

// Active returns the top run-scoped collector, or nil.
func Active() *Collector {
	activeMu.Lock()
	defer activeMu.Unlock()
	if len(active) == 0 {
		return nil
	}
	return active[len(active)-1]
}

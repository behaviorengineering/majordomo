package judge

import (
	"testing"
	"time"

	"github.com/XiaoConstantine/dspy-go/pkg/core"
	"github.com/behaviorengineering/strop/pkg/dspy/factory"
)

func TestRuntimeOptionsWrapLLMWiresFactory(t *testing.T) {
	llmFactory := factory.NewLLMFactory(nil, time.Minute)
	var called bool
	opts := RuntimeOptions{
		WrapLLM: func(llm core.LLM) core.LLM {
			called = true
			return llm
		},
	}
	if opts.WrapLLM != nil {
		llmFactory.SetWrapLLM(opts.WrapLLM)
	}
	stub := struct{ core.LLM }{}
	_ = opts.WrapLLM(stub)
	if !called {
		t.Fatal("expected WrapLLM to run")
	}
}

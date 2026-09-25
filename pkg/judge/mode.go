package judge

import "fmt"

// ErrNotReady is returned when the strop Judge cannot initialize (usually missing LLM keys).
var ErrNotReady = fmt.Errorf("judge: strop not ready (set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY for the embedded gateway)")

// ErrStropJudgeNotReady is an alias kept for existing call sites.
var ErrStropJudgeNotReady = ErrNotReady

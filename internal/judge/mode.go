package judge

import "fmt"

// ErrNotReady is returned when the strop Judge cannot initialize (usually missing LLM keys).
var ErrNotReady = fmt.Errorf("judge: strop not ready (set ANTHROPIC_API_KEY or OPENAI_API_KEY)")

// ErrStropJudgeNotReady is an alias kept for existing call sites.
var ErrStropJudgeNotReady = ErrNotReady

// Package filereview is the Prepare → Judge → Validate → Assemble state machine
// for per-file PR review batches. Structured findings are the machine contract;
// Markdown is a formatter after Validate.
//
// When MAJORDOMO_JUDGE=strop, orchestrate wires Judge to internal/judge.FileReviewBatch
// (strop generator modules) instead of agent-dispatch.sh.
package filereview

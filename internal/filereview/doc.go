// Package filereview is the Prepare → Judge → Validate → Assemble state machine
// for per-file PR review batches. Structured findings are the machine contract;
// Markdown is a formatter after Validate.
//
// Orchestrate wires Judge to internal/judge.FileReviewBatch (strop generators).
package filereview

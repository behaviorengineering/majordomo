// Package contextgate parses @majordomo gate comments and persists gate.json
// sidecar state for context update PRs.
//
// Comment grammar and sidecar shape are the open handshake. Regenerating story
// after reject is factory work; the runner only reads ReadyToMerge / RegenRequested.
package contextgate

// Package cache reads and writes review-cache and poll-cursor data on the served
// repo (branches majordomo-inference-cache/<id> with path prefix review/, plus
// majordomo-poll-cache/<id>).
//
// DigestStore and related fingerprint types under the digest/ prefix are a
// transitional import surface for the private majordomo-context factory. The
// open runner CLI does not operate that store; prefer factory-owned tools once
// the private module hosts it.
package cache

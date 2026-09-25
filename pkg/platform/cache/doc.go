// Package cache reads and writes review-cache and poll-cursor data on the served
// repo (branches majordomo-inference-cache/<id> with path prefix review/, plus
// majordomo-poll-cache/<id>).
//
// Digest inference artifacts use the open path convention DigestCachePrefix
// ("digest") on the inference-cache branch. The store implementation lives in
// the private majordomo-context factory, not in this open runner package.
package cache

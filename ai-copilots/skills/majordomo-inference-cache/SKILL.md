---
name: majordomo-inference-cache
description: >-
  Require keyed skip paths for Majordomo LLM / RLM steps so unchanged
  code+prompt+model+schema do not re-burn tokens. Use when adding generate,
  evaluate, or RLM work to digest, review, or other pipelines.
---

# Majordomo inference cache

**Moral:** Every new LLM step MUST name its invalidation inputs and persist a
keyed artifact. Re-runs with identical fingerprints MUST skip the provider call.

**Related stores:**

| Store | Branch / location | Skips |
|-------|-------------------|-------|
| Inference cache (review + digest) | `majordomo-inference-cache/<repo-id>` with `review/` and `digest/` path prefixes (`digest/inspect`, `ledger`, `cluster`, `refine`, `intervention`, `story`; plus `cluster_audit/`) | Review cluster analysis; digest inspect, ledger, cluster audit, cluster CoT, refine, human intervention, bootstrap story |
| Context cursor | `majordomo-context/<repo-id>` `meta.yaml` | Whole digest job when HEAD already visited |

Legacy branch names `majordomo-pr-reviewer-cache/*` and `majordomo-digest-cache/*` are
cold-start superseded. Poll still excludes them so orphaned refs stay non-product.

---

## When to load

- Adding a generate, evaluate, refine, or RLM step in Majordomo
- Wiring digest or review so reseeds / retries do not repeat identical inferences
- Choosing where to persist skip artifacts under `majordomo-inference-cache/`

---

## Constraints

**CONSTRAINT:** A new generate / evaluate / RLM step MUST declare invalidation
inputs (code or evidence hash, prompt/schema version, model id) before the first
provider call.

- Enforcement: design review + PR checklist; code path has a fingerprint builder
- Violation: STOP, add fingerprint fields, re-verify

**CONSTRAINT:** On a successful grounded / valid result, the step MUST persist a
keyed artifact under an existing Majordomo cache store (`internal/cache`).

- Enforcement: store call after success; unit test that miss stores and hit skips
- Violation: STOP, wire store, re-verify

**CONSTRAINT:** On fingerprint hit with `cache.SkipsEnabled()` true, the step MUST
NOT call the provider. Overclaim, timeout, 502, and validation failure MUST NOT
be stored as hits.

- Enforcement: lookup before LLM; failed verdicts omit Store
- Violation: STOP, fix lookup/store gates

**CONSTRAINT:** MUST NOT invent a new git-branch naming scheme without extending
`internal/cache` and documenting it next to the inference-cache layout.

- Enforcement: branch helpers live in `internal/config` + `internal/cache`
- Violation: STOP, reuse or extend existing prefixes

**CONSTRAINT:** After each successful Store of a keyed digest or review-cache
artifact, the step MUST commit and push the cache branch **on the go** (same
cadence as PR review `majordomo cache store` then `cache push`). MUST NOT wait
for the whole digest or review job to succeed. A later refine, eval, or gate
failure MUST NOT discard already-earned inspect/ledger/cluster/refine/story hits.

- Enforcement: `DigestStore` Flush after Store*; digest run configures push when
  materializing `majordomo-inference-cache/<repo-id>`; final Flush on exit
- Violation: STOP, wire push-on-store (or phase flush), re-verify

**CONSTRAINT:** Digest fingerprint inputs MUST be stable across reseeds of the
same code. Prefer package source hashes (and owned-path / constraint facts) over
ephemeral RLM markdown or LLM cluster proposal prose. Schema bumps (`inspect-v2`,
`ledger-v2`, `cluster-cot-v2`, `refine-v2`) invalidate old keys intentionally.
Cluster/refine MUST hash role identity and verdict decisions (not evidence quotes,
duration_ms, or trace_dir).

- Enforcement: `PackageSourceHash` / `OwnedPackagesSourceHash`; ledger omits cluster hash;
  `rolesIdentitySHA` / `clusterVerdictsIdentitySHA` / `mechanicalIdentitySHA`
- Violation: STOP, remove ephemeral inputs from the fingerprint, bump schema

**CONSTRAINT:** Digest runs MUST log cache hit/miss counts and estimated tokens
saved (`digest cache summary … estimated_tokens_saved=`), using stored per-entry
usage when present.

- Enforcement: `DigestStore` stats + `FormatStatsLine` after inspect/ledger and on exit
- Violation: STOP, wire Record*Hit/Miss and end-of-run summary

**CONSTRAINT:** Digest teaching reseeds MUST NOT delete `majordomo-inference-cache/*`.
Context wipe (`majordomo-context/*`) is allowed; inference reuse must survive it.

- Enforcement: reseed scripts only target context refs; docs list the exclusion
- Violation: STOP, restore inference-cache branch policy

CORRECT:
```go
srcSHA, _ := cache.PackageSourceHash(analysisDir, path)
fp := cache.InspectFingerprint{PackagePath: path, ContextSHA: srcSHA, ModelID: model, SchemaVersion: cache.DigestInspectSchemaV2}
if skips && store != nil {
  if hit, ok, _ := store.LookupInspect(fp); ok {
    store.RecordInspectHit(hit.PromptTokens, hit.CompletionTokens, hit.TotalTokens)
    return hit, nil // no provider call
  }
}
out, err := rlm.Validate(...)
if err == nil && out.Agreement != "" && out.Agreement != "unvalidated" {
  // StoreInspect Flushes the cache branch when push is configured.
  // Persist match, abstain, and disagree so reseeds do not rewrite roles prose.
  _ = store.StoreInspect(fp, out)
}
```

PROHIBITED:
```go
// Fingerprint on ephemeral cluster markdown or regenerated RLM prose
fp.ContextSHA = cache.ContentSHA(clusterProposalMD)

// Persist only in memory until the whole digest finishes successfully
if digestSucceeded {
  _ = store.StoreInspect(fp, out)
  _ = pushDigestCache(...)
}
```

---

## Checklist

- [ ] Invalidation inputs named (code/evidence, prompt/schema, model)
      Method: fingerprint struct fields present
      Pass: all three categories covered
      Fail: STOP, add fields
- [ ] Fingerprint stable across reseeds of the same sources
      Method: PackageSourceHash / OwnedPackagesSourceHash; no cluster prose in key
      Pass: second reseed logs hits for unchanged packages
      Fail: STOP, remove ephemeral hash inputs
- [ ] Lookup before provider when skips enabled
      Method: code path + unit test hit
      Pass: stub LLM not called on hit
      Fail: STOP, wire lookup
- [ ] Store only on successful grounded/valid result
      Method: code path + unit test overclaim/error
      Pass: failures not written
      Fail: STOP, gate Store
- [ ] Push on the go after each successful Store (not only job success)
      Method: Flush after Store*; failed digest still leaves remote hits
      Pass: push configured on digest materialize; test Flush called from Store
      Fail: STOP, wire ConfigurePush / Flush
- [ ] Hit/miss + estimated_tokens_saved logged
      Method: FormatStatsLine in inspect/ledger/exit logs
      Pass: summary line present with counts
      Fail: STOP, wire DigestStore stats
- [ ] Branch / dir uses `internal/cache` helpers
      Method: `InferenceCacheBranch` (aliases `CacheBranch` / `DigestCacheBranch`)
      Pass: no ad-hoc branch string
      Fail: STOP, use helpers

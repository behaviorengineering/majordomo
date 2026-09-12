---
name: majordomo-capability-claims
description: >-
  Keep typology claim codes, is/must_not priors, entailment gates, and ledger
  RLM instructions aligned. Use when editing capability constraints, claim
  entailment, objective ledger prompts, or digest claim loops.
---

# Majordomo capability claims

**Moral:** Deterministic Go gates are the source of truth. The ledger RLM MUST
see those rules in its prompt. Coding agents MUST NOT teach the model a claim
the gate will reject.

**Canonical runtime:**

| Piece | File |
|-------|------|
| Role → is/must_not table | `internal/contextdigest/capability_constraints.go` (`roleCapabilityTable`) |
| Fail-closed post-pass | `buildCapabilityConstraints` + `dropAllowedCapabilityMustNot` |
| Positive entailment | `internal/contextdigest/claim_entailment.go` (`claimEntailed`) |
| Ledger instruction fragment | `claimPolicyPromptRules` (same package; wired into `formatSliceObjectiveLedgerQuery`) |

---

## When to load

- Editing capability codes, role defaults, or fail-closed must_not passes
- Changing claim entailment evidence flags or edges
- Changing typology objective ledger prompts or retry feedback
- Debugging digest failures like "claim X is not entailed" or "intersect must_not"

---

## Constraints

**CONSTRAINT:** Go remains the runtime source of truth. A skill or prompt change
MUST NOT replace `must_not` / `claimEntailed` checks.

- Enforcement: digest still rejects unentailed claims after LLM output
- Violation: STOP, restore gates

**CONSTRAINT:** When adding or narrowing a claim code, MUST update all of:
`roleCapabilityTable`, fail-closed post-pass in `buildCapabilityConstraints`,
`claimEntailed` (if evidence-based), and `claimPolicyPromptRules`.

- Enforcement: PR checklist + unit tests for must_not and prompt fragment
- Violation: STOP, align all four, re-verify

**CONSTRAINT:** Ledger prompts MUST include `claimPolicyPromptRules()` (or an
equivalent generated from the same helpers). A flat claim menu alone is
PROHIBITED.

- Enforcement: `formatSliceObjectiveLedgerQuery` embeds the fragment; unit test
- Violation: STOP, wire fragment

**CONSTRAINT:** Prestige English ("orchestrates", "runs via helper", "builds a
DTO card") MUST NOT become a claim code without matching role, evidence flag,
or edge.

- Enforcement: fail-closed must_not + entailment
- Violation: STOP, forbid the code or add real entailment evidence

CORRECT:
```go
if role != roleExecRunner && !evidenceHasAny(n.Evidence, "imports_os_exec") {
  c.MustNot = uniqueStrings(append(c.MustNot, capExecProcess))
}
if role != roleAggregator {
  c.MustNot = uniqueStrings(append(c.MustNot, capOwnDomainRules))
}
// ... and claimPolicyPromptRules lists the same exceptions
```

PROHIBITED:
```text
Prompt: "you may claim exec_process when the package coordinates CLI work"
// while claimEntailed still requires imports_os_exec
Prompt: "own_domain_rules for entrypoint/http_surface"
// while must_not and claimEntailed only allow aggregator
```

---

## Checklist

- [ ] Table defaults match intended role priors
- [ ] Fail-closed post-pass covers unknown/custom roles
- [ ] Evidence/role exceptions clear table defaults (`dropAllowedCapabilityMustNot`)
- [ ] `claimEntailed` evidence flags match the prompt fragment
- [ ] Unit tests cover must_not allow/deny cases
- [ ] `formatSliceObjectiveLedgerQuery` still embeds constraint rows + policy

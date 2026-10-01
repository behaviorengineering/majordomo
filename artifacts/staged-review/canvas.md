# Staged Review Progress

## Progress
| Stage | Status | Score |
|---|---|---:|
| 1. Automated Tools | done | 9/10 |
| 2. Type Safety | done | 10/10 |
| 3. Error Handling | done | 7/10 |
| 4. Architecture | in progress | |
| 5. Robustness | pending | |
| 6. Testability | pending | |
| 7. Code Clarity | done | 9/10 |
| 8. Generation Gates | done | 6/10 |

Pass A: mechanical findings fixed and verified. Pass B: consultant stages
follow the Pass A commit and draft pull request.

Counts: critical 0, medium 0, low 1, open 1.
Draft PR: https://github.com/behaviorengineering/majordomo/pull/77

## Now
Stage 4 question: `internal/ops/submodule/git.go:37`, should the mixed
submodule manager split its UI, Git adapter, and mutation workflows?
Why this matters: separate seams improve focused tests and error ownership;
one facade is simpler if this is intentionally a small CLI boundary.

## Resolutions
| Finding | Severity | Location | Outcome |
|---|---|---|---|
| Missing golangci-lint | Low | Review environment | Recorded, no product change required |
| Docker tag error discarded | Medium | `internal/ops/satools/satools.go` | Fixed in Pass A |
| CLI writer errors discarded | Low | `internal/ops/cli/root.go` | Fixed in Pass A |
| Test fixture errors discarded | Low | `internal/ops/sa/sa_test.go`, `internal/ops/satools/satools_test.go` | Fixed in Pass A |
| Em dash in user-facing strings | Low | `internal/ops/*` | Fixed in Pass A |
| External process calls bypass resilience | Medium | `internal/ops/{sa,satools,submodule}` | Fixed in Pass A |
| Bare CLI invocation returns an error | Medium | `internal/ops/cli/root.go` | Fixed in Pass A |
| Submodule manager mixes UI, Git, and mutations | Open | `internal/ops/submodule/git.go:37` | Waiting on consultant |

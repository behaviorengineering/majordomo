---
name: majordomo-operator
description: >-
  Guides agents operating the Majordomo CLI safely. Use before running
  repository review, cache, context, publish, static-analysis, or submodule
  commands.
---

# Majordomo operator

Use this order:

1. Read the root `AGENTS.md` and identify the repository and command scope.
2. Run `majordomo version` or `majordomo help` before a workflow.
3. Prefer read-only commands and `--dry-run` before mutations.
4. Keep publishing disabled unless the task explicitly requires it.
5. Confirm paths, repository IDs, and branch names before `push`, `publish`, or submodule reset operations.
6. Preserve command output and return errors instead of treating warnings as success.

Use the root task runner for local checks:

```bash
go tool task format lint vet test build
```

Load the domain skill for inference cache, local seed, capability claims, or operator configuration work.

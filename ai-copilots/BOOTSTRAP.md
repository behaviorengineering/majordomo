# BOOTSTRAP — Majordomo ai-copilots

**Audience:** Any AI agent (Cursor, GitHub Copilot, Claude Code, Codex) in a
workspace that depends on or checks out this module.

**Goal:** Wire host IDE discovery to canonical content under `ai-copilots/`.
Optionally refresh content. **MUST NOT** copy skill bodies unless symlinks or
junctions fail and the user approves copy fallback.

**Module path:** `github.com/behaviorengineering/majordomo`

---

## When to run

| Mode | Phases |
|------|--------|
| **Wire only** | 0 → 2 → 3 → 4 |
| **Refresh content + wire** | 0 → 1 → 2 → 3 → 4 |

---

## Phase 0 — Resolve module root

From a Go module that requires `github.com/behaviorengineering/majordomo` (or this checkout):

```bash
MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/majordomo)"
test -d "$MOD/ai-copilots" || { echo "missing ai-copilots under $MOD"; exit 1; }
echo "Module Dir: $MOD"
```

If `go list` is unavailable, use a known nested checkout path only when it
clearly contains `ai-copilots/`. MUST NOT invent a path.

Re-run wire after module version bumps (cache Dir can change).

---

## Phase 1 — Refresh content (optional)

Edit only files under `$MOD/ai-copilots/`. Load author-ai-copilots + agent-smith
when authoring skills.

---

## Phase 2 — Ask IDE and OS if unknown

1. IDE: Cursor, GitHub Copilot, Claude Code, Codex
2. OS: macOS/Linux symlink vs Windows junction/copy
3. Workspace: library alone vs nested under a parent monorepo vs dependency-only

---

## Phase 3 — Wire discovery

| Host skill name | Path under `$MOD` |
|-----------------|-------------------|
| `majordomo-inference-cache` | `ai-copilots/skills/majordomo-inference-cache/` |
| `majordomo-capability-claims` | `ai-copilots/skills/majordomo-capability-claims/` |

| IDE | Skills |
|-----|--------|
| Cursor | `.cursor/skills/**/SKILL.md` |
| GitHub Copilot | `.github/skills/**/SKILL.md` |
| Claude Code | `.claude/skills/**/SKILL.md` |
| Codex | `.codex/skills/**/SKILL.md` |

**Cursor (macOS/Linux)** from host workspace root:

```bash
MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/majordomo)"
mkdir -p .cursor/skills
ln -snf "$MOD/ai-copilots/skills/majordomo-inference-cache" .cursor/skills/majordomo-inference-cache
ln -snf "$MOD/ai-copilots/skills/majordomo-capability-claims" .cursor/skills/majordomo-capability-claims
```

When the workspace root is this module:

```bash
mkdir -p .cursor/skills
ln -snf ../ai-copilots/skills/majordomo-inference-cache .cursor/skills/majordomo-inference-cache
ln -snf ../ai-copilots/skills/majordomo-capability-claims .cursor/skills/majordomo-capability-claims
```

**Windows:** junction or developer-mode symlink; copy only with user approval.

---

## Phase 4 — Verify

```bash
MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/majordomo)"
ls -la .cursor/skills/majordomo-inference-cache
ls -la .cursor/skills/majordomo-capability-claims
test -f .cursor/skills/majordomo-inference-cache/SKILL.md
test -f .cursor/skills/majordomo-capability-claims/SKILL.md
test -f "$MOD/ai-copilots/BOOTSTRAP.md"
```

Ask before committing host wiring.

# Architecture catalog (typology)

Majordomo uses [typology](https://github.com/behaviorengineering/typology) v0.0.4 (pinned in `go.mod`).

**Source of truth:** `architecture/typology.yaml` (from `discover`; edit by hand before emit).

## Install

```bash
go tool typology version
```

To bump after a typology release:

```bash
go get -tool github.com/behaviorengineering/typology/cmd/typology@vX.Y.Z
go mod tidy
```

## Workflow

1. **Discover** (refresh from import graph):

   ```bash
   go tool typology discover . --out architecture/typology.yaml
   ```

2. **Review** slice names, merges, bindings, and fix `id:` if needed (`majordomo`).

3. **Validate**:

   ```bash
   go tool typology validate . --catalog architecture/typology.yaml
   ```

4. **Emit** doc skeletons when ready:

   ```bash
   go tool typology emit . --catalog architecture/typology.yaml
   ```

5. **Remediate** one slice at a time:

   ```bash
   go tool typology remediate . orchestrate --catalog architecture/typology.yaml
   ```

## Current state

- **3 bounded contexts (`review`, `context`, `operations`)**, **33 packages**, **6** slice bindings
- Draft: `architecture/typology.draft.yaml` (mirrored to `typology.yaml`)
- **Review**: staging, clustering, static analysis, wave orchestration, strop judge, workspace port, cache, report, publish
- **Context**: context branch store, digest catch-up, human gate, grounding packs
- **Operations**: multi-SCM poll, outbound HTTP, git HTTPS auth, central config, loopback AI gateway, OpenTelemetry, CLI surface (`cmd/majordomo`, `internal/cli`)
- Docs tree: **7** DocPages in total (`overview` + `components` for each slice, plus `cli` for `operations`)
- First-map session: `architecture/typology-journey.md`
- Upstream backlog: `architecture/typology-upstream-notes.md`

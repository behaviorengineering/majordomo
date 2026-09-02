# Architecture catalog (typology)

Majordomo uses [typology](https://github.com/behaviorengineering/typology) v0.0.5 (pinned in `go.mod`).

**Human system map:** [`architecture.md`](architecture.md) (how the control plane is put together).  
**Machine inventory:** [`typology.yaml`](typology.yaml) (slices, packages, bindings).  
**Upstream gaps:** [`typology-upstream-notes.md`](typology-upstream-notes.md).

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
   go tool typology discover . --out architecture/typology.draft.yaml
   ```

2. **Cluster-pass / review** slice names, merges, bindings; keep `id: majordomo`.

3. **Validate**:

   ```bash
   go tool typology validate . --catalog architecture/typology.yaml
   ```

4. **Emit** doc skeletons when ready:

   ```bash
   go tool typology emit . --catalog architecture/typology.yaml
   ```

5. **Fill** develop DocPages and keep [`architecture.md`](architecture.md) aligned with the catalog.

6. **Remediate** one slice at a time:

   ```bash
   go tool typology remediate . review --catalog architecture/typology.yaml
   ```

## Current state

- **3 bounded contexts** (`review`, `context`, `operations`), **33 packages**, **6** slice bindings
- Narrative: [`architecture.md`](architecture.md)
- Develop DocPages under `docs/develop/{review,context,operations}/`
- Journey: [`typology-journey.md`](typology-journey.md) (phase `done`)
- Upstream / LLM auto-doc gaps: [`typology-upstream-notes.md`](typology-upstream-notes.md)

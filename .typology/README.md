# Typology

This directory holds the Typology catalog for this repository. Use it to guide architecture decisions and code structure: slices are bounded contexts, components are packages, and bindings are the allowed couplings. One repository owns one catalog and its architecture docs.

`typology.yaml` is the source of truth for scope, slices, components, bindings, subprograms, actuators, and opRuns. In a multi-module workspace, `scope.modules` declares which repository-local modules Typology inspects; `go.work` does not widen that scope.

## For Agents

Before you change this catalog or the code it describes, load these skills:

- `typology-journey`: first map, resume `typology-journey.md`
- `typology-catalog`: model, YAML shape, subprograms, actuators, bindings
- `typology-cli`: discover, emit, validate, remediate
- `typology-docs`: fill and evaluate develop DocPages

If your host does not have these skills, install the Typology module and symlink the skills from `$TYPOLOGY_ROOT/skills/` into your host skills directory (see the Typology module `AGENTS.md`).

## Consumer bootstrap

Before running Typology commands in a consumer, register the CLI as a Go tool:

```bash
go run github.com/behaviorengineering/typology/cmd/typology@v0.0.6 init .
go tool typology version
```

The bootstrap updates the selected module's `go.mod` and `go.sum`, but it does not add the CLI to application imports or binaries. If `go.work` covers more than one module, pass `--module PATH` explicitly.

## Typical commands

- `typology init REPO [--module PATH] [--version VERSION]` — registers the CLI as a Go tool in the consumer module
- `typology discover REPO [--module PATH]`: writes a draft to `tmp/typology/typology.yaml`; use `--module` when the repository has multiple Go modules
- `typology emit REPO`: writes `.typology/typology.yaml` and DocPages
- `typology architecture REPO [--module PATH]`: writes `docs/architecture/typology.md` for human review within `scope.modules`
- `typology validate REPO [--module PATH]`: checks the catalog and scoped modules against each other
- `typology remediate REPO SLICE [--module PATH]`: agent-scoped violations for one slice and module scope

## Workflow

Catalog first, code second, validation last:

1. Update `typology.yaml` to declare the intended architecture, components, bindings, and programs before writing the code.
2. Implement the code to match the catalog.
3. In a multi-module repository, set `scope.modules` to the modules owned by this catalog. Do not rely on `go.work` as Typology scope.
4. Run `typology architecture REPO` to give humans a readable comparison of the catalog and the observed Go topology.
5. Have an agent or architect fix each finding or record the boundary debt in `.typology/typology-journey.md`.
6. Run `typology validate REPO` and fix every issue before considering the change done.

A green catalog means the code matches the declared slices, components, and bindings.

## Files

- `typology.yaml`: confirmed catalog
- `docs/architecture/typology.md`: generated architecture brief; remove its marker after human acceptance
- `tools.yaml`: generated CLI tool index from catalog `opRuns`
- `typology-journey.md`: first-map session file (created during a journey)
- `README.md`: this file
- `../docs/architecture/architecture.md`: human system architecture narrative
- `typology-upstream-notes.md`: tracked upstream product gaps

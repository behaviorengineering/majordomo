# Overview

Context is the bounded context for the repository context branch, and operators run its digest and gate flow through Majordomo. It keeps repo memory, grounding packs, and branch history in one place so review jobs can read a consistent story.

See [README.md](README.md) for the tree hub and [components.md](components.md) for the package table. For how context hangs together with review and operations end to end, read [architecture.md](../../../architecture/architecture.md).

`context` keeps `contextstore`, `contextdigest`, `contextgate`, and `agenting` together because they all support the same long-lived branch lifecycle.

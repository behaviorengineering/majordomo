# Overview

Operations is the bounded context for Majordomo's control plane, and operators run it through the root CLI and shared runtime. It keeps polling, config, SCM adapters, telemetry, and the CLI surface together as the host boundary for the rest of the system.

See [README.md](README.md) for the tree hub and [components.md](components.md) for the package table. For how the control plane hangs together with review and context end to end, read [architecture.md](../../architecture/architecture.md).

`operations` owns the platform-level pieces that the review and context slices depend on, rather than splitting them into a separate technical layer.

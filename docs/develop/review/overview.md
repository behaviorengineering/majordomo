# Overview

Review is the bounded context for PR review automation, and operators run it through the Majordomo control plane. It turns a change into staged review work, judge runs, and published results without making the caller track each package boundary.

See [README.md](README.md) for the tree hub and [components.md](components.md) for the package table. For how review hangs together with context and operations end to end, read [architecture.md](../../architecture/architecture.md).

`review` keeps the workflow together so staging, orchestration, judge, cache, report, publish, and status stay in one context instead of becoming separate slices.

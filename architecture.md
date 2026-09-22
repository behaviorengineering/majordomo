# Architecture

<!-- majordomo-reading-nav:start -->
**Reading path:** [Prev: mission.md](mission.md) · [Next: conventions.md](conventions.md) · [TOC](README.md)
<!-- majordomo-reading-nav:end -->

> Teaching story. Living project architecture for humans and review grounding. Not Typology seed evidence (see `evidence/typology/architecture_brief.md`).
The majordomo system is organized around the lifecycle of repository operations and context management. The architecture is driven by the objective to manage context loading, validation, and observability through specialized agenting, digest, gate, and store packages.
The system's shape is defined by several core functional areas:
Agentic Execution: Utilizing specialized agenting to drive repository workflows and operations.
Context Management: A pipeline involving digest and gate mechanisms to ensure validated context is available for tasks.
Persistence and State: A dedicated store package to maintain durable configuration and cached state across evolving software environments.
This structure ensures that repository operations—from polling changes to running structured PR reviews—are grounded in a consistent, observable context.

## Grounded slice objectives

- **review**: The review slice manages the lifecycle of code changes through staging, clustering, static analysis, orchestration of evaluation modules, and final reporting.
- **context**: Manage context loading, validation, and observability through specialized agenting, digest, gate, and store packages.
- **operations**: The operations slice manages CLI entrypoints, configuration, HTTP serving, external adapter interactions, observability, and process execution.

# Chronology

<!-- majordomo-reading-nav:start -->
**Reading path:** [Prev: weaknesses.md](weaknesses.md) · [Next: README.md](evidence/typology/README.md) · [TOC](README.md)
<!-- majordomo-reading-nav:end -->

Newest first.
## Current State: Control Tower Runtime
The system operates as a Go-based CLI (`majordomo`) integrated into a GitHub Actions control tower. This architecture provides the foundation for SCM-agnostic operations and durable organizational configuration.
## Capability Evolution: From File-Review to Synthesis
The product's operational logic has evolved from basic file-level classification to complex synthesis:
File-Review Layer: Automated routing of changed files to specialized skills (code, docs, and configuration) via `majordomo prep`.
Synthesis Layer: The addition of high-level orchestration, producing `summary.md` (high-level summaries), `tech-review.md` (deep-dive analysis), and `blast-radius.md` (impact mapping) following file-level inspection.

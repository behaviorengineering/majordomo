# Overview
High-level project grounding for review. Digest expands this from mission.md and architecture.md.
Majordomo serves as a control plane for software repositories undergoing continuous evolution. It provides the durable configuration, state, and orchestration required to run automated jobs—most notably structured pull request reviews—against a served repository. The system operates as a Go-based CLI integrated with a GitHub Actions control tower, utilizing OpenCode for agentic workflows and specialized Docker images for security analysis and forge operations.
### Core Objectives
The system maintains a ledger of repository state and operational objectives, focusing on:
Automated Review Orchestration: Transforming raw git changes into structured intelligence through a pipeline of file-review skills (code, docs, and configuration) and synthesis skills (technical deep-dives and impact mapping).
Durable Context Management: Maintaining a persistent, versioned understanding of repository history and organizational standards through dedicated context stores and digests.
Repository Operations: Providing SCM-agnostic tools for managing submodules, executing portable pipelines, and coordinating agentic tasks within a controlled environment.
### Functional Typology
The product's capabilities are categorized by their role in the repository lifecycle:
Review Workflows: Automated detection, classification, and synthesis of pull request changes.
Context Services: Management of durable repository state and agentic grounding data.
Orchestration & Execution: Coordination of jobs, agentic tasks via OpenCode, and deployment of specialized toolchains.
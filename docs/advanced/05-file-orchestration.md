# File Orchestration and Batching

*Majordomo — repository operations for evolving software.*

This page is the index for the file orchestration section. It maps the three orchestration phases and links to each sub-page.

## 🧭 What You'll Learn

**Concepts:**
- [Staging and Classification](05.1-staging-and-classification.md) - How changed files become structured review tasks
- [Dependency Clustering](05.2-dependency-clustering.md) - How related files are grouped to preserve context across batches
- [Skill Dispatch and Orchestration](05.3-skill-dispatch-and-orchestration.md) - How batches run in waves and results aggregate

**Reference:**
- [Example Python PR Walkthrough](05.4-example-python-pr.md) - End-to-end trace from diff to final artifacts

---

## 💡 Quick Model

File orchestration has three phases. Staging prepares reviewable inputs, clustering preserves file relationships, and dispatch executes skill-specific batches and merges results.

---

## 🔗 Related Docs

- [Pipeline Stages Reference](04.1-pipeline-stages-reference.md) - Review phases and where orchestration runs
- [PR Summary Flow](06-pr-summary-flow.md) - How summary and technical scoring loops work

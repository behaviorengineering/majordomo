# Conventions

<!-- majordomo-reading-nav:start -->
**Reading path:** [Prev: architecture.md](architecture.md) · [Next: weaknesses.md](weaknesses.md) · [TOC](README.md)
<!-- majordomo-reading-nav:end -->

Contributors interact with Majordomo primarily through the Go CLI. Development and automation workflows rely on the portable pipeline pattern to ensure consistency across different environments.
### Environment Configuration
To ensure the CLI and associated scripts operate correctly, contributors configure the following:
`MAJORDOMO_SCRIPTS`: Set this if `pipelines/scripts` is not discoverable from the current working directory.
`MAJORDOMO_BIN`: Set this when dispatch must locate the CLI outside the standard system PATH.
### Workflow Integration
The repository follows a structured review process. Contributors engage with the product through established workflows, moving from initial setup and submodule management into the deep-dive review cycles. Instead of managing raw inventories, the system operates via a refined Typology catalog and a slice objective ledger to maintain order across evolving software repositories.
For detailed operational steps, follow the documented progression:
1. Implement the Portable Pipeline Pattern.
2. Execute setup and local development via the Setup guide.
3. Manage dependencies through the Submodule workflow.
4. Engage with the Review workflow, including file orchestration and PR summary flows.

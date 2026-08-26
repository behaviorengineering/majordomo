# Customising the Review

*Majordomo — repository operations for evolving software.*

Configuration lives in the **control-tower repo** (`majordomo-central-config/<repo-slug>.yaml` merged with `_defaults.yaml`). Per-repo overrides replace org defaults; omit keys to inherit.

The pipeline ships **eight built-in skills** across three categories. Only `pr-review-code` is **routed by default** — all other skills require explicit routing configuration.

---

## 🏷️ Available Skills

**File-review skills** (routed by file type, produce per-file reports):

| Skill | Default routing | Blast radius |
|---|---|---|
| `pr-review-code` | Source code extensions (Python, JS, Java, Go, etc.) | Yes (mandatory) |
| `pr-review-conf` | Not routed by default. Configure explicitly. | No |
| `pr-review-docs` | Not routed by default. Configure explicitly. | No |
| `pr-review-tests` | Not routed by default. Configure explicitly. | No |

**Synthesis skills** (run automatically after file-review, not routed by file type):

| Skill | What it produces |
|---|---|
| `pr-review-summary` | `summary.md` — high-level PR summary written for developer and reviewer |
| `pr-review-technical` | `tech-review.md` — deep-dive: control flow, concurrency, test coverage gaps |
| `pr-review-blast-radius` | `blast-radius.md` — impact map across the changeset |

**Scoring skills** (used internally by iteration loops, not invoked directly):

| Skill | What it does |
|---|---|
| `pr-review-summary-score` | Scores `summary.md` against a rubric; drives the write/score iteration loop |
| `pr-review-technical-score` | Scores `tech-review.md` against a rubric; drives the tech-review iteration loop |

---

## ⚙️ Configuration Overrides

### Credentials and registry (control tower)

Store secrets in the control-tower repo (GitHub Actions secrets / org secrets). Reference them from per-repo YAML — never commit tokens.

```yaml
# majordomo-central-config/payments-api.yaml (example shape — see PLAN doc)
registry:
  pullDomain: docker-pull.example.com
  pushDomain: docker-push.example.com

packageRegistry:
  host: packages.example.com
  caCertUrl: https://packages.example.com/generic/security/certificates/corp-ca.pem
  debianRepoPath: debian-repo
  pipIndexPath: api/pypi/pypi-virtual/simple
  npmVirtualPath: api/npm/npm-virtual/

secrets:
  llmProviderKey: OPENAI_API_KEY           # or ANTHROPIC_API_KEY / OPENCODE_PROVIDER_API_KEY
  scmToken: SCM_TOKEN
  registryUser: REGISTRY_USER
  registryToken: REGISTRY_TOKEN
```

Corp Docker builds use `packageRegistry` + registry credentials. Open/GitHub-hosted runners omit `packageRegistry` and build with `--target public`.

---

### Override which files each skill receives

`majordomo prep` classifies each changed file using glob patterns. **First matching glob wins.**

```yaml
pipelines:
  pr-review:
    routing:
      pr-review-docs:
        - "**/*.md"
        - "**/*.rst"
      pr-review-conf:
        - "**/*.yml"
        - "**/*.yaml"
        - "docs/**"
      pr-review-code:
        - "**"   # catch-all — must be last
```

Pass a routing JSON file to prep with `--routing` when the orchestrator materialises config at runtime.

---

### Inject team or domain context into the reviewer

```yaml
pipelines:
  pr-review:
    agentContext:
      global:
        customRules:
          - "No hardcoded credentials."
      scoped:
        "services/payments-api/**":
          techStack: [python, fastapi, openapi]
          reviewFocus: [openapi-contract, auth]
          customRules:
            - file: .majordomo/rules/mesh-api-contract.md
            - "FastAPI must use exception_handlers, not Flask-style decorators."
```

---

### Override a skill's review rules

Point to your own skill directory (must contain a `SKILL.md`):

```yaml
pipelines:
  pr-review:
    skills:
      pr-review-docs: agents/skills/my-docs   # path relative to app repo checkout
      pr-review-code: null                    # null = use submodule default
```

---

### Override the orchestrator agent

```yaml
pipelines:
  pr-review:
    agent: agents/my-pr-review.agent.md
```

---

### Models and cache (org defaults + per-repo overrides)

```yaml
pipelines:
  pr-review:
    model: claude-sonnet-4.5
    scoreModel: gpt-5.4-mini

cache:
  cacheRepo: project          # project | central
  enableSkips: false
  retentionDays: 120
```

See [PLAN — Control Tower, GitHub Actions, and Go](../PLAN-control-tower-github-go.md) for the full YAML schema as it lands in the Go loader.

---

### Static analysis tools

```yaml
staticAnalysis:
  - dockerfile: dockerfiles/sa-tools/ruff.Dockerfile
    command: check --output-format=concise
    glob: "**/*.py"
  - image: ghcr.io/org/sa-custom:1.0.0    # BYO — no build step
    command: lint
    glob: "**/*.go"
```

---

## 🔗 Related

- [05.1 Staging and Classification](05.1-staging-and-classification.md)
- [09 — Example routing in README](../README.md#-customising-the-review)
- [PLAN — Control Tower](../PLAN-control-tower-github-go.md)

---
name: majordomo-operator-config
description: >-
  Majordomo XDG operator overlay (~/.config/majordomo/config.yaml), majordomo init,
  MAJORDOMO_CONFIG / MAJORDOMO_CONFIG_DIR, and secrets via operatorconfig. Use when
  wiring local dev credentials, default config_dir, or export-env deploy scripts.
  Control tower YAML stays under majordomo-central-config.
---

# Majordomo operator config

**Moral:** Two layers. The **control tower** is multi-file YAML under `--config-dir` (`_defaults.yaml` + `<repo-id>.yaml`). The **operator overlay** is optional single-file XDG config for local defaults and declared secrets.

Library: `github.com/behaviorengineering/operatorconfig@v0.1.1` via `pkg/platform/operatorcfg`.

## Paths

| Input | Resolves |
|-------|----------|
| `--config` / `MAJORDOMO_CONFIG` | Operator file (must exist when set) |
| (default) | `$XDG_CONFIG_HOME/majordomo/config.yaml` |
| `--config-dir` / `MAJORDOMO_CONFIG_DIR` / operator `config_dir:` | Tower tree |
| Fallback | `majordomo-central-config` |

## Commands

- `majordomo init` — writes `~/.config/majordomo/config.yaml` (0600) when missing; refreshes `config.yaml.example`.
- Other commands (except `init`, `version`, `context`) call `operatorcfg.Apply` before tower load when an operator file exists.

## Secrets

Declare env names under top-level `secrets:` in the operator file. Resolution order: process env, macOS Keychain / OS store (service `majordomo`), optional `~/.config/majordomo/secrets.enc.yaml` (SOPS).

SCM tokens for poll/review still follow tower rules: `MAJORDOMO_CREDENTIAL_<repo_id>` or `GH_TOKEN_<OWNER>` / `GITLAB_TOKEN_<OWNER>` (see `pkg/platform/config`).

Example:

```yaml
config_dir: /path/to/checkout/majordomo-central-config
secrets:
  - GH_TOKEN_YOUR_ORG
  - env: PHOENIX_API_KEY
    required: false
```

**CI and GitHub Actions:** inject secrets via env and keep using `--config-dir majordomo-central-config`. Do not rely on a keyring inside the runner. Operator file is optional in CI.

## Host towers

Product hosts (for example majordomo-tower) keep live tower YAML in `majordomo-central-config/`. Document host paths in host docs only, not in portable majordomo examples.

## Related

- Shared pack skill: host `.cursor/skills/operator-config/SKILL.md`
- Library skill: `operatorconfig` module `ai-copilots/skills/operatorconfig/SKILL.md`
- Go package: `pkg/platform/operatorcfg`

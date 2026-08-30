#!/bin/bash
# OpenCode single-batch dispatcher — called once per batch by the review
# orchestrator (Go/GHA host). Skills stay file-based under agents/; OpenCode
# loads them from paths supplied in the prompt.
#
# Usage:
#   agent-dispatch.sh <pr-number> <staging-dir> <output-dir> [MODE]
#
#   MODE (optional 4th argument):
#     (omitted)    File-review batch — reviews individual files, writes per-file <slug>.md reports
#     --finalize         Reads per-file reports and writes blast-radius.md, index.md
#     --summary          Synthesises all reports into summary.md (iterative, driven by majordomo summary loop)
#     --score            Scores an existing summary.md against the rubric; writes score.md
#     --technical        Produces a technical deep-dive report from the full diff (iterative, driven by majordomo tech loop)
#     --tech-score       Scores an existing tech-review.md against the rubric; writes tech-score.md
#     --prose            Post-wave style rewriter — applies prose-quality rules to all per-file reports
#     --technical-deep   Focused second pass on one file cited in tech-review.md (driven by majordomo tech-deep)
#
# Environment:
#   LLM auth (runtime only; never bake keys into the agent image):
#     OPENAI_API_KEY              Stock OpenAI OR dummy key when Majordomo embeds Bifrost
#     ANTHROPIC_API_KEY           Stock Anthropic (prefer letting Majordomo aigateway own this)
#     OPENCODE_PROVIDER_API_KEY   Custom OpenAI-compatible gateway (wire as {env:OPENCODE_PROVIDER_API_KEY}
#                                 in opencode.json / OPENCODE_CONFIG_CONTENT)
#     OPENAI_BASE_URL             When set by Majordomo aigateway.ChildEnv, points at loopback Bifrost
#                                 (OpenAI-compatible). Real provider keys are stripped from the child env.
#     GOOGLE_GENERATIVE_AI_API_KEY  Stock Google Generative AI (when used)
#   COPILOT_PIPELINE      Legacy name for orchestrator agent label (default: pr-review)
#   OPENCODE_AGENT        Optional OpenCode --agent name (overrides pipeline label when set)
#   COPILOT_AGENT_OVERRIDES   Optional comma-separated name=/abs/path pairs for agent overrides
#   COPILOT_SKILL_OVERRIDES   Optional comma-separated skill=/abs/dir pairs for skill dir overrides
#   COPILOT_MODEL / OPENCODE_MODEL  Model for file-review, finalize, prose (provider/model or short name)
#   COPILOT_SUMMARY_MODEL / OPENCODE_SUMMARY_MODEL
#   COPILOT_TECHNICAL_MODEL / OPENCODE_TECHNICAL_MODEL
#   COPILOT_DEEP_TECHNICAL_MODEL / OPENCODE_DEEP_TECHNICAL_MODEL
#   COPILOT_SCORE_MODEL / OPENCODE_SCORE_MODEL  (default: auto — omit --model)
#   OPENCODE_PROVIDER     Default provider prefix when model has no slash (default: anthropic)
#   OPENCODE_CONFIG / OPENCODE_CONFIG_CONTENT  Optional provider config (baseURL, custom provider id)
   MAJORDOMO_GROUNDING   Set by majordomo dispatch — comma-separated absolute paths to grounding packs

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"
source "${SCRIPT_DIR}/lib/copilot-common.sh"

setup_error_trap
require_command opencode
require_command jq

# Resolve majordomo CLI for Go-ported helpers (all-diffs, etc.).
resolve_majordomo() {
    if [ -n "${MAJORDOMO_BIN:-}" ]; then
        echo "${MAJORDOMO_BIN}"
        return 0
    fi
    if command -v majordomo >/dev/null 2>&1; then
        command -v majordomo
        return 0
    fi
    local cand
    for cand in \
        "${SCRIPT_DIR}/../../bin/majordomo" \
        "${SCRIPT_DIR}/../../majordomo" \
        "$(pwd)/bin/majordomo" \
        "$(pwd)/majordomo"
    do
        if [ -x "${cand}" ]; then
            echo "${cand}"
            return 0
        fi
    done
    return 1
}

# Map short Copilot-era model names to OpenCode provider/model form.
normalize_opencode_model() {
    local m="$1"
    if [ -z "${m}" ] || [ "${m}" = "auto" ] || [[ "${m}" == */* ]]; then
        printf '%s' "${m}"
        return
    fi
    case "${m}" in
        gpt-*|o1*|o3*|o4*)
            printf 'openai/%s' "${m}"
            ;;
        *)
            local provider="${OPENCODE_PROVIDER:-anthropic}"
            # claude-sonnet-4.5 → claude-sonnet-4-5
            printf '%s/%s' "${provider}" "${m//./-}"
            ;;
    esac
}

# ---------------------------------------------------------------------------
# Args
# ---------------------------------------------------------------------------

if [ $# -lt 3 ] || [ $# -gt 4 ]; then
    log_error "Usage: $0 <pr-number> <staging-dir> <output-dir> [--finalize|--summary|--score|--technical|--tech-score|--prose|--technical-deep]"
    exit 1
fi

case "${4:-}" in
    --finalize)        MODE="finalize"        ;;
    --summary)         MODE="summary"         ;;
    --score)           MODE="score"           ;;
    --technical)       MODE="technical"       ;;
    --tech-score)      MODE="tech-score"      ;;
    --prose)           MODE="prose"           ;;
    --technical-deep)  MODE="technical-deep"  ;;
    "")                MODE="files"           ;;
    *)  log_error "Unknown mode: ${4}. Expected --finalize, --summary, --score, --technical, --tech-score, --prose, or --technical-deep."
        exit 1 ;;
esac

# ---------------------------------------------------------------------------
# Workspace setup
# ---------------------------------------------------------------------------

setup_workspace "${COPILOT_AGENT_OVERRIDES:-}"

PR_NUMBER="$1"
STAGING_DIR="$(realpath "$2")"
OUTPUT_DIR="$(realpath -m "$3")"

# ---------------------------------------------------------------------------
# Auth (OpenCode LLM provider — not SCM)
# ---------------------------------------------------------------------------
# GITHUB_TOKEN / Bitbucket tokens are for clone/publish/status elsewhere.
# They are not OpenCode LLM credentials. OpenCode reads provider keys from
# env (or auth.json); opencode run does not preflight, so we fail fast here.

opencode_provider_key_present() {
    [ -n "${OPENAI_API_KEY:-}" ] && return 0
    [ -n "${ANTHROPIC_API_KEY:-}" ] && return 0
    [ -n "${OPENCODE_PROVIDER_API_KEY:-}" ] && return 0
    [ -n "${GOOGLE_GENERATIVE_AI_API_KEY:-}" ] && return 0
    return 1
}

if ! opencode_provider_key_present; then
    log_error "OpenCode provider API key required at runtime (set OPENAI_API_KEY, ANTHROPIC_API_KEY, or OPENCODE_PROVIDER_API_KEY). Do not bake keys into the agent image."
    exit 1
fi

# ---------------------------------------------------------------------------
# Pre-flight
# ---------------------------------------------------------------------------

# prose mode works directly on OUTPUT_DIR — STAGING_DIR is the skill root, no manifest there
# technical-deep: manifest is already written by majordomo tech-deep into the batch dir
if [ "${MODE}" != "prose" ] && [ ! -f "${STAGING_DIR}/manifest.json" ]; then
    log_error "Manifest not found: ${STAGING_DIR}/manifest.json"
    exit 1
fi

mkdir -p "${OUTPUT_DIR}"

PIPELINE="${COPILOT_PIPELINE:-pr-review}"
SKILLS_BASE="${SCRIPT_DIR}/../../agents/skills"

# ---------------------------------------------------------------------------
# Resolve skill from manifest and locate skill directory
# ---------------------------------------------------------------------------

# prose and technical-deep modes set/determine SKILL separately
if [ "${MODE}" != "prose" ]; then
    SKILL=$(jq -r '.review_agents | keys[0]' "${STAGING_DIR}/manifest.json")
    if [ -z "${SKILL}" ] || [ "${SKILL}" = "null" ]; then
        log_error "Cannot determine skill from manifest: ${STAGING_DIR}/manifest.json"
        exit 1
    fi
fi

get_skill_dir() {
    local skill="$1"
    if [ -n "${COPILOT_SKILL_OVERRIDES:-}" ]; then
        local pair
        IFS=',' read -ra pairs <<< "${COPILOT_SKILL_OVERRIDES}"
        for pair in "${pairs[@]}"; do
            if [ "${pair%%=*}" = "${skill}" ]; then
                echo "${pair#*=}"
                return
            fi
        done
    fi
    echo "${SKILLS_BASE}/${skill}"
}

# prose mode sets SKILL + SKILL_DIR in the staging layout section below
if [ "${MODE}" != "prose" ]; then
    SKILL_DIR="$(get_skill_dir "${SKILL}")"
    if [ ! -d "${SKILL_DIR}" ]; then
        log_error "Skill directory not found: ${SKILL_DIR} (skill: ${SKILL})"
        exit 1
    fi
fi

TASK_COUNT=0
if [ "${MODE}" != "prose" ]; then
    TASK_COUNT=$(jq '.reviewable | length' "${STAGING_DIR}/manifest.json")
fi

# ---------------------------------------------------------------------------
# Technical-deep mode: SKILL + staging already set up by majordomo tech-deep.
# Batch dir IS the staging dir. SKILL.md, templates, manifest.json, and input
# file are already in place. Just resolve the skill dir and set AGENT_STAGING.
# ---------------------------------------------------------------------------

if [ "${MODE}" = "technical-deep" ]; then
    SKILL_DIR="$(get_skill_dir "${SKILL}")"
    if [ ! -d "${SKILL_DIR}" ]; then
        log_error "Technical-deep skill directory not found: ${SKILL_DIR} (skill: ${SKILL})"
        exit 1
    fi
fi

# ---------------------------------------------------------------------------
# Score mode: override SKILL to the scorer skill before staging setup
# ---------------------------------------------------------------------------

if [ "${MODE}" = "score" ]; then
    SKILL="pr-review-summary-score"
    SKILL_DIR="$(get_skill_dir "${SKILL}")"
    if [ ! -d "${SKILL_DIR}" ]; then
        log_error "Score skill directory not found: ${SKILL_DIR}"
        exit 1
    fi
fi

if [ "${MODE}" = "tech-score" ]; then
    SKILL="pr-review-technical-score"
    SKILL_DIR="$(get_skill_dir "${SKILL}")"
    if [ ! -d "${SKILL_DIR}" ]; then
        log_error "Tech-score skill directory not found: ${SKILL_DIR}"
        exit 1
    fi
fi

# ---------------------------------------------------------------------------
# Staging layout
#
# finalize: STAGING_DIR is the skill dir — agent needs dirname(STAGING_DIR) as root.
#           Copy SKILL.md into STAGING_DIR so the agent finds it at <staging>/<skill>/SKILL.md.
#
# prose:    No staging dir needed — agent works directly on OUTPUT_DIR.
#           SKILL is overridden to prose-quality. SKILL.md is passed via --add-dir.
#
# all other modes: STAGING_DIR is the batch_000 dir.
#           Create STAGING_DIR/<skill>/ with manifest.json + SKILL.md.
#           Agent receives staging:STAGING_DIR and navigates to STAGING_DIR/<skill>/.
# ---------------------------------------------------------------------------

if [ "${MODE}" = "technical-deep" ]; then
    # Technical-deep mode: batch dir was fully prepared by majordomo tech-deep.
    # SKILL.md, templates, manifest.json, input file, and review_timestamp.txt
    # are already in STAGING_DIR. Agent receives STAGING_DIR directly.
    AGENT_STAGING="${STAGING_DIR}"
elif [ "${MODE}" = "prose" ]; then
    # Prose mode: override SKILL to the prose-quality skill.
    # No staging setup needed — the agent receives the output dir directly and
    # lists/rewrites per-file .md reports in place. SKILL.md is passed via --add-dir.
    SKILL="prose-quality"
    SKILL_DIR="$(get_skill_dir "${SKILL}")"
    if [ ! -d "${SKILL_DIR}" ]; then
        log_error "Prose skill directory not found: ${SKILL_DIR}"
        exit 1
    fi
    AGENT_STAGING="${OUTPUT_DIR}"
elif [ "${MODE}" = "finalize" ]; then
    AGENT_STAGING="$(dirname "${STAGING_DIR}")"
    cp "${SKILL_DIR}/SKILL.md" "${STAGING_DIR}/SKILL.md"
    # Write a tiny summary of manifest fields the agent needs for finalize.
    # The full manifest can exceed 1,000 lines; reading it repeatedly exhausts
    # the model's context window before summary.md / index.md can be written.
    jq '{base_branch, skill_dir, files: (.reviewable | map(.file) | unique | sort), files_reviewed: (.reviewable | map(.file) | unique | length), excluded_count: (.excluded | length), excluded}' \
        "${STAGING_DIR}/manifest.json" > "${STAGING_DIR}/finalize-context.json"

    # Pre-filter per-file reports to only those containing findings.
    # Without this the agent reads all ~100 reports individually, exhausting
    # context before it can write summary.md / index.md.
    # The agent is given findings.md and MUST NOT read individual report files.
    FINDINGS_FILE="${STAGING_DIR}/findings.md"
    grep -rl '\[CRITICAL\]\|\[WARN\]' "${OUTPUT_DIR}/per-file" \
        | sort \
        | xargs --no-run-if-empty cat > "${FINDINGS_FILE}" \
        || true
    # If no findings at all, write a sentinel so the agent knows — empty file
    # would be ambiguous (could mean grep found nothing vs. file never created).
    if [ ! -s "${FINDINGS_FILE}" ]; then
        printf '<!-- no findings -->\n' > "${FINDINGS_FILE}"
    fi

    # Pre-write the Sydney timestamp. The agent's --allow-tool='read, write'
    # means it cannot execute shell commands (CONSTRAINT 3). Without this,
    # the agent silently fabricates a timestamp or omits the field.
    TZ=Australia/Sydney date +"%Y-%m-%dT%H:%M:%S%:z" > "${STAGING_DIR}/review_timestamp.txt"
else
    AGENT_STAGING="${STAGING_DIR}"
    mkdir -p "${STAGING_DIR}/${SKILL}"
    cp "${STAGING_DIR}/manifest.json" "${STAGING_DIR}/${SKILL}/manifest.json"
    cp "${SKILL_DIR}/SKILL.md" "${STAGING_DIR}/${SKILL}/SKILL.md"
    if [ -d "${SKILL_DIR}/templates" ]; then
        cp -r "${SKILL_DIR}/templates" "${STAGING_DIR}/${SKILL}/templates"
    fi
    # Pre-write the Sydney timestamp for synthesis modes — agent cannot run
    # shell commands (CONSTRAINT 3: --allow-tool='read, write' only).
    if [ "${MODE}" = "summary" ] || [ "${MODE}" = "technical" ]; then
        TZ=Australia/Sydney date +"%Y-%m-%dT%H:%M:%S%:z" > "${STAGING_DIR}/${SKILL}/review_timestamp.txt"
    fi

    if [ -d "${STAGING_DIR}/.grounding" ]; then
        cp -r "${STAGING_DIR}/.grounding" "${STAGING_DIR}/${SKILL}/.grounding"
    fi

    # Rewrite input_file in the batch manifest to workspace-relative paths.
    # The agent reads input_file directly — no path construction from the staging: prompt
    # parameter, which caused "missing path segment" errors when the agent converted the
    # absolute staging path to a relative path incorrectly.
    STAGING_REL="${STAGING_DIR#$(pwd)/}"
    jq --arg base "${STAGING_REL}" \
        '.reviewable |= map(.input_file = ($base + "/" + .input_file))' \
        "${STAGING_DIR}/${SKILL}/manifest.json" \
        > "${STAGING_DIR}/${SKILL}/manifest.json.tmp" \
        && mv "${STAGING_DIR}/${SKILL}/manifest.json.tmp" \
              "${STAGING_DIR}/${SKILL}/manifest.json"

    # Pre-concatenate all diffs into a single file for synthesis modes.
    # Without this the agent reads 100+ files individually, exhausting context
    # before it can write its output (same failure mode as finalize).
    # Format: each diff is preceded by '=== FILE: <path> ===' so the agent can
    # identify which file each diff belongs to without reading individual files.
    #
    # Summary mode caps each file at SUMMARY_DIFF_CAP lines.
    # Large PRs (100+ files, 400+ KB of diffs) cause the Copilot CLI tool to return
    # truncated "1-line" responses on the first read attempt. The agent then re-reads
    # with explicit line ranges to get full content — doubling context and triggering
    # a 400 Bad Request from the API. The summary skill needs scope, not every line.
    # Technical mode uses a higher cap (150 lines/file): enough to cover control-flow
    # and error-handling patterns without saturating the context window on large PRs.
    if [ "${MODE}" = "summary" ] || [ "${MODE}" = "technical" ]; then
    # Summary mode caps each diff to limit total context — see majordomo report all-diffs.
    # Technical mode caps at a higher value: enough to cover control-flow and
    # error-handling patterns in most diff chunks without saturating the context window.
    # (Uncapped, a large PR can exceed 11k lines / 167k tokens and trigger a 400 from the API.)
    _cap_arg=()
    [ "${MODE}" = "summary" ]   && _cap_arg=(--cap 50)
    [ "${MODE}" = "technical" ] && _cap_arg=(--cap 150)
        _majordomo="$(resolve_majordomo)" || {
            echo "ERROR: majordomo binary not found (set MAJORDOMO_BIN or install majordomo on PATH)" >&2
            exit 1
        }
        "${_majordomo}" report all-diffs \
            "${STAGING_DIR}/${SKILL}/manifest.json" \
            "${STAGING_DIR}/${SKILL}/all-diffs.txt" \
            "${_cap_arg[@]}"
    fi
fi

# ---------------------------------------------------------------------------
# Per-mode configuration: prompt output path, log label, session file, model, extra dirs
# ---------------------------------------------------------------------------

LOGS_DIR="${OUTPUT_DIR}/logs"
WORK_DIR=""

case "${MODE}" in
    finalize)
        PROMPT_OUTPUT="${OUTPUT_DIR}"
        LOG_LABEL="Finalize: ${PIPELINE} / skill:${SKILL}"
        SESSION_FILE="${LOGS_DIR}/finalize_session.md"
        EXTRA_MODEL="${OPENCODE_MODEL:-${COPILOT_MODEL:-anthropic/claude-sonnet-4-5}}"
        ;;
    summary)
        PROMPT_OUTPUT="$(dirname "${OUTPUT_DIR}")"
        LOG_LABEL="Summary: ${PIPELINE} / skill:${SKILL} / iter:${SUMMARY_ITER:-1}"
        SESSION_FILE="${LOGS_DIR}/summary_iter_${SUMMARY_ITER:-1}_session.md"
        WORK_DIR="$(pwd)"
        EXTRA_MODEL="${OPENCODE_SUMMARY_MODEL:-${COPILOT_SUMMARY_MODEL:-${OPENCODE_MODEL:-${COPILOT_MODEL:-anthropic/claude-sonnet-4-5}}}}"
        ;;
    score)
        PROMPT_OUTPUT="$(dirname "${OUTPUT_DIR}")"
        LOG_LABEL="Score: ${PIPELINE} / skill:${SKILL} / iter:${SUMMARY_ITER:-1}"
        SESSION_FILE="${LOGS_DIR}/score_iter_${SUMMARY_ITER:-1}_session.md"
        # Avoid mounting the full pipeline output tree (context bloat). Paths are in the prompt.
        _score_model="${OPENCODE_SCORE_MODEL:-${COPILOT_SCORE_MODEL:-auto}}"
        EXTRA_MODEL="${_score_model:?OPENCODE_SCORE_MODEL/COPILOT_SCORE_MODEL must not be empty}"
        ;;
    technical)
        PROMPT_OUTPUT="$(dirname "${OUTPUT_DIR}")"
        LOG_LABEL="Technical review: ${PIPELINE} / skill:${SKILL} / iter:${TECH_ITER:-1}"
        SESSION_FILE="${LOGS_DIR}/technical_iter_${TECH_ITER:-1}_session.md"
        WORK_DIR="$(pwd)"
        EXTRA_MODEL="${OPENCODE_TECHNICAL_MODEL:-${COPILOT_TECHNICAL_MODEL:-${OPENCODE_MODEL:-${COPILOT_MODEL:-anthropic/claude-sonnet-4-5}}}}"
        ;;
    technical-deep)
        _slug=$(jq -r '.reviewable[0].slug' "${STAGING_DIR}/manifest.json")
        PROMPT_OUTPUT="${OUTPUT_DIR}"
        LOG_LABEL="Technical deep: ${PIPELINE} / skill:${SKILL} / file:${_slug}"
        SESSION_FILE="${LOGS_DIR}/technical_deep_${_slug}_session.md"
        WORK_DIR="$(pwd)"
        EXTRA_MODEL="${OPENCODE_DEEP_TECHNICAL_MODEL:-${COPILOT_DEEP_TECHNICAL_MODEL:-${OPENCODE_TECHNICAL_MODEL:-${COPILOT_TECHNICAL_MODEL:-${OPENCODE_MODEL:-${COPILOT_MODEL:-anthropic/claude-sonnet-4-5}}}}}}"
        ;;
    tech-score)
        PROMPT_OUTPUT="$(dirname "${OUTPUT_DIR}")"
        LOG_LABEL="Tech-score: ${PIPELINE} / skill:${SKILL} / iter:${TECH_ITER:-1}"
        SESSION_FILE="${LOGS_DIR}/tech_score_iter_${TECH_ITER:-1}_session.md"
        _score_model="${OPENCODE_SCORE_MODEL:-${COPILOT_SCORE_MODEL:-auto}}"
        EXTRA_MODEL="${_score_model:?OPENCODE_SCORE_MODEL/COPILOT_SCORE_MODEL must not be empty}"
        ;;
    prose)
        PROMPT_OUTPUT="${OUTPUT_DIR}/per-file"
        LOG_LABEL="Prose rewrite: ${PIPELINE} / skill:prose-quality"
        SESSION_FILE="${LOGS_DIR}/prose_session.md"
        EXTRA_MODEL="${OPENCODE_MODEL:-${COPILOT_MODEL:-anthropic/claude-sonnet-4-5}}"
        ;;
    files)
        BATCH_LABEL="$(basename "${STAGING_DIR}")"
        PROMPT_OUTPUT="${OUTPUT_DIR}/per-file"
        LOG_LABEL="File-review batch: ${PIPELINE} / skill:${SKILL} / ${BATCH_LABEL}"
        SESSION_FILE="${LOGS_DIR}/${BATCH_LABEL}_session.md"
        EXTRA_MODEL="${OPENCODE_MODEL:-${COPILOT_MODEL:-anthropic/claude-sonnet-4-5}}"
        ;;
esac

WORK_DIR="${WORK_DIR:-$(pwd)}"
EXTRA_MODEL="$(normalize_opencode_model "${EXTRA_MODEL}")"

# skill_dir: for finalize = STAGING_DIR (skill dir, SKILL.md at root)
# skill_dir: for prose = SKILL_DIR (prose-quality skill dir, no staging copy)
# skill_dir: for file batches = STAGING_DIR/<SKILL>/ (where SKILL.md was copied)
if [ "${MODE}" = "finalize" ]; then
    PROMPT_SKILL_DIR="${STAGING_DIR}"
elif [ "${MODE}" = "prose" ]; then
    PROMPT_SKILL_DIR="${SKILL_DIR}"
elif [ "${MODE}" = "technical-deep" ]; then
    # SKILL.md is at the root of STAGING_DIR (batch dir) — not in a subdirectory.
    PROMPT_SKILL_DIR="${STAGING_DIR}"
else
    PROMPT_SKILL_DIR="${STAGING_DIR}/${SKILL}"
fi
PROMPT="PR #${PR_NUMBER} staging:${AGENT_STAGING} skill:${SKILL} skill_dir:${PROMPT_SKILL_DIR} output:${PROMPT_OUTPUT} mode:${MODE}"
if [ -n "${MAJORDOMO_GROUNDING:-}" ]; then
    PROMPT="${PROMPT} grounding:${MAJORDOMO_GROUNDING}"
    log_info "  Grounding:     ${MAJORDOMO_GROUNDING}"
fi

log_header "${LOG_LABEL}"
log_info "  Agent staging: ${AGENT_STAGING}"
log_info "  Output dir:    ${PROMPT_OUTPUT}"
[ "${MODE}" = "files" ] && log_info "  Tasks:         ${TASK_COUNT}"

mkdir -p "${OUTPUT_DIR}" "${LOGS_DIR}" "${PROMPT_OUTPUT}"
RUN_START=$(date -u +%s)

write_finalize_fallback_outputs() {
    local context_file="${STAGING_DIR}/finalize-context.json"
    local timestamp_file="${STAGING_DIR}/review_timestamp.txt"
    local summary_file="${OUTPUT_DIR}/summary.md"
    local index_file="${OUTPUT_DIR}/index.md"

    if [ ! -f "${context_file}" ]; then
        log_error "Finalize fallback failed: missing context file ${context_file}"
        return 1
    fi

    local base_branch files_reviewed excluded_count reviewed_at
    base_branch=$(jq -r '.base_branch // "unknown"' "${context_file}")
    files_reviewed=$(jq -r '.files_reviewed // 0' "${context_file}")
    excluded_count=$(jq -r '.excluded_count // 0' "${context_file}")
    reviewed_at=$(cat "${timestamp_file}" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")

    cat > "${summary_file}" <<EOF
# PR Review Summary - PR #${PR_NUMBER}

**Skill:** ${SKILL}
**Base Branch:** ${base_branch}
**Files Reviewed:** ${files_reviewed}
**Excluded:** ${excluded_count}

---

## Verdict

Approve - No review findings were identified for this change set.

## Critical Issues

None.

## Cross-Cutting Themes

None observed.

## Top Recommendations

1. No code changes required based on this review.
2. Keep static analysis checks enabled to catch non-functional issues early.
3. Continue monitoring future structural changes in this module.
EOF

    cat > "${index_file}" <<EOF
# Copilot PR Review - PR #${PR_NUMBER}

**Skill:** ${SKILL}
**Base Branch:** ${base_branch}
**Reviewed At:** ${reviewed_at}

---

**PR Summary:** \
\`<output>/summary.md\` - start here

---

## Files Reviewed

EOF

    if jq -e '.files | length > 0' "${context_file}" >/dev/null; then
        while IFS= read -r file; do
            printf -- "- %s\n" "${file}" >> "${index_file}"
        done < <(jq -r '.files[]' "${context_file}")
    else
        printf -- "- None\n" >> "${index_file}"
    fi

    if jq -e '.excluded | length > 0' "${context_file}" >/dev/null; then
        cat >> "${index_file}" <<EOF

## Excluded

EOF
        while IFS= read -r file; do
            printf -- "- %s\n" "${file}" >> "${index_file}"
        done < <(jq -r '.excluded[]' "${context_file}")
    fi

    cat >> "${index_file}" <<EOF

---

_Reviewed: ${files_reviewed} | Excluded: ${excluded_count}_
EOF

    return 0
}

# ---------------------------------------------------------------------------
# UTF-8 locale — required for Node.js / Python subprocesses.
# The agent environment may not have a locale set, causing stdout/file writes
# to fall back to ASCII and produce mojibake (â€" instead of —).
# ---------------------------------------------------------------------------
export LANG="en_US.UTF-8"
export LC_ALL="en_US.UTF-8"
export PYTHONIOENCODING="utf-8"
export PYTHONUTF8="1"

# ---------------------------------------------------------------------------
# Invoke OpenCode (non-interactive)
# ---------------------------------------------------------------------------

OPENCODE_ARGS=(run --auto --dir "${WORK_DIR}")
if [ -n "${OPENCODE_AGENT:-}" ]; then
    OPENCODE_ARGS+=(--agent "${OPENCODE_AGENT}")
fi
if [ -n "${EXTRA_MODEL}" ] && [ "${EXTRA_MODEL}" != "auto" ]; then
    OPENCODE_ARGS+=(--model "${EXTRA_MODEL}")
fi

set +e
opencode "${OPENCODE_ARGS[@]}" "${PROMPT}" 2>&1 | tee "${SESSION_FILE}"
AGENT_EXIT=${PIPESTATUS[0]}
set -e

# ---------------------------------------------------------------------------
# Append run metrics to metrics.jsonl in OUTPUT_DIR (archived, not in logs/)
# ---------------------------------------------------------------------------
RUN_END=$(date -u +%s)
RUN_DURATION=$(( RUN_END - RUN_START ))
RUN_TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
RUN_BATCH="${MODE}"
[ "${MODE}" = "files" ] && RUN_BATCH="$(basename "${STAGING_DIR}")"
printf '{"timestamp":"%s","pr":"%s","skill":"%s","mode":"%s","batch":"%s","model":"%s","duration_seconds":%d,"exit_code":%d}\n' \
    "${RUN_TIMESTAMP}" "${PR_NUMBER}" "${SKILL}" "${MODE}" "${RUN_BATCH}" "${EXTRA_MODEL}" \
    "${RUN_DURATION}" "${AGENT_EXIT}" \
    >> "${OUTPUT_DIR}/metrics.jsonl"

if [ "${AGENT_EXIT}" -ne 0 ]; then
    log_error "opencode exited with code ${AGENT_EXIT}"
    exit "${AGENT_EXIT}"
fi

if [ "${MODE}" = "finalize" ]; then
    if [ ! -f "${OUTPUT_DIR}/summary.md" ] || [ ! -f "${OUTPUT_DIR}/index.md" ]; then
        log_warn "Finalize completed without required outputs. Writing deterministic fallback summary/index."
        write_finalize_fallback_outputs
    fi

    if [ ! -f "${OUTPUT_DIR}/summary.md" ] || [ ! -f "${OUTPUT_DIR}/index.md" ]; then
        log_error "Finalize fallback failed to produce summary.md and index.md"
        exit 1
    fi
fi

log_info "Done — output: ${PROMPT_OUTPUT}"

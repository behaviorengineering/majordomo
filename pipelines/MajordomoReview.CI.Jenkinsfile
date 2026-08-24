// .majordomo/pipelines/MajordomoReview.CI.Jenkinsfile
// Triggered by Bitbucket Server webhooks via Generic Webhook Trigger plugin.
// Stage 1: Ensures the Copilot CLI Docker image exists in the registry (builds and caches if not).
//          Image tag is derived from the SHA of the Dockerfile — rebuild only when the image changes.
// Stage 2: Runs the Copilot PR review — skipped automatically on plain branch pushes.
//
// Jenkins job configuration:
//   Script Path:  .majordomo/pipelines/MajordomoReview.CI.Jenkinsfile
//   Trigger:      Generic Webhook Trigger — configure ALL four variable extractions below.
//                 They resolve to empty string when the path is absent (no errors).
//
//   Variable         Expression                              Set by
//   CHANGE_ID        $.pullRequest.id                        pr:opened, pr:from_ref_updated
//   CHANGE_TARGET    $.pullRequest.toRef.displayId           pr:opened, pr:from_ref_updated
//   CHANGE_BRANCH    $.pullRequest.fromRef.displayId         pr:opened, pr:from_ref_updated
//   CHANGE_COMMIT    $.pullRequest.fromRef.latestCommit      pr:opened, pr:from_ref_updated
//   PUSH_BRANCH      $.changes[0].ref.displayId              repo:refs_changed (branch push)
//   PR_EVENT_TYPE    $.eventKey                              all events (pr:opened, pr:from_ref_updated, etc.)
//
// Required credentials:
//   example-docker-creds        - Username/password for package registry Docker registry
//   example-registry-token   - REGISTRY_USER + REGISTRY_TOKEN for BuildKit secrets (apt/npm)
//   github-copilot-token        - GitHub personal access token with Copilot access

// Expected runtime behavior (AI and operator guardrails):
// 1) Jenkins job SCM points to the target app repository. Webhook events come from that repo.
// 2) This Jenkinsfile is loaded from the .majordomo submodule path within that app repo.
// 3) Safe Checkout runs checkout scm first for workspace correctness and config availability.
// 4) For PR events, stages that inspect diffs must run on CHANGE_BRANCH (webhook source branch).
// 5) Static Analysis and PR Review enforce source-branch checkout explicitly to avoid drift.
// 6) .majordomo-config.groovy is read from app-repo root; submodule provides templates/code only.

// Fallback defaults — used only when .majordomo-config.groovy is absent from the app repo.
// All values here are placeholders; populate .majordomo-config.groovy at the repo root instead.
// jenkinsAgent is fixed infrastructure — not overridable via config or parameters.
@groovy.transform.Field
final String REVIEW_OUTPUT_PREFIX = 'copilot-review-pr-'

@groovy.transform.Field
final Map PIPELINE_CONFIG = [
    registry: [
        pullDomain:    'your-pull-registry.example.com',
        pullUrl:       'https://your-pull-registry.example.com',
        pushDomain:    'your-push-registry.example.com',
        credentialsId: 'your-docker-registry-creds'
    ],
    jenkinsAgent: [
        label:      'linux-shared-agent',
        dockerArgs: '-u root -e HOME=/root'
    ],
    credentials: [
        package-registryCredentialsId:    'your-package-registry-token-credential-id',
        githubCopilotCredentialsId:   'your-github-copilot-token-credential-id',
        bitbucketTokenCredentialsId: 'bitbucket-access-token',
        gwtTokenCredentialsId:       ''  // Optional: GWT token credential ID — enables webhook re-fire on submodule drift (no upstream parent link)
    ]
]

pipeline {
    // Single top-level agent — all stages run on the same node / workspace.
    // All Docker usage in this pipeline is via docker.image().inside() (scripted), not
    // declarative Docker agent blocks, so there is no Docker-in-Docker risk.
    // A per-stage agent (agent none + stage-level label) would let Jenkins dispatch
    // each stage to a different node, each with its own workspace; checkout scm on
    // node A leaves every other node with a stale workspace, causing loadConfig()
    // to read the old 'def config()' format and return placeholder defaults.
    agent { label PIPELINE_CONFIG.jenkinsAgent.label }

    options {
        timeout(time: 60, unit: 'MINUTES')
        skipDefaultCheckout(true)  // Explicit Checkout stage fixes permissions before git runs
    }

    // Populated by Generic Webhook Trigger plugin from Bitbucket webhook payloads.
    // PR events (pr:opened, pr:from_ref_updated) set CHANGE_ID/CHANGE_TARGET/CHANGE_BRANCH.
    // Push events (repo:refs_changed) leave those empty and set PUSH_BRANCH instead.
    // CHANGE_ID being empty is the signal that skips the PR Review stage.
    parameters {
        string(name: 'CHANGE_ID',      defaultValue: '', description: 'PR number — set by pr:opened / pr:from_ref_updated ($.pullRequest.id)')
        string(name: 'CHANGE_TARGET',  defaultValue: 'master', description: 'PR target branch ($.pullRequest.toRef.displayId)')
        string(name: 'CHANGE_BRANCH',  defaultValue: '', description: 'PR source branch ($.pullRequest.fromRef.displayId)')
        string(name: 'CHANGE_COMMIT',  defaultValue: '', description: 'PR source branch HEAD commit ($.pullRequest.fromRef.latestCommit) — used for Bitbucket build status notifications')
        string(name: 'PUSH_BRANCH',    defaultValue: '', description: 'Pushed branch — set by repo:refs_changed ($.changes[0].ref.displayId)')
        string(name: 'PR_EVENT_TYPE',  defaultValue: '', description: 'Bitbucket event key ($.eventKey) — pr:opened triggers auto-review; pr:from_ref_updated requires manual approval')
        booleanParam(name: 'ENABLE_CONTINUOUS_RUNS', defaultValue: false, description: 'When true, pr:from_ref_updated webhook events run automatically instead of requiring a manual rebuild')
        choice(name: 'SUMMARY_PUBLISH_MODE', choices: ['auto', 'comment', 'description', 'off'], description: 'How to publish the summary to Bitbucket: auto = claim description if empty/owned, else post a link comment; comment = always post full content as comment; description = always replace description; off = skip')
        string(name: 'REPLAY_OF_BUILD', defaultValue: '', description: 'Internal marker: source replay build number for auto-queued fresh reruns')
    }

    environment {
        IMAGE_NAME = 'copilot-cli'
        DOCKERFILE = '.majordomo/dockerfiles/copilot-cli.Dockerfile'
    }

    stages {
        stage('Pipeline Guard') {
            // Per-branch lock guard only. PR update policy is evaluated after config load
            // so enableContinuousRuns can be sourced from configuration settings.
            when {
                expression {
                    return true
                }
            }
            // Per-branch concurrency control: allows different branches to run concurrently
            // while deduplicating the duplicate events Bitbucket fires on a push to an open PR
            // (repo:refs_changed + pr:from_ref_updated arrive simultaneously for the same branch).
            // skipIfLocked: the second build skips its inner stages instead of queuing — builds
            // for other branches have their own lock resource and are unaffected.
            options {
                lock(
                    resource: "${env.JOB_NAME}-${params.CHANGE_BRANCH ?: params.PUSH_BRANCH ?: 'default'}",
                    skipIfLocked: true   // Second duplicate event for the same branch (repo:refs_changed + pr:from_ref_updated) skips instead of queuing — different branches are unaffected
                )
            }
            stages {
                stage('Safe Checkout') {
                    // Explicit checkout so we can fix root-owned files BEFORE git runs.
                    // Docker stages use '-u root'; NFS root_squash means the Jenkins host user
                    // cannot unlink root-owned files — git checkout exits 1 and the pipeline
                    // enters a permanent broken loop. chmod -R a+rw via alpine fixes this.
                    // alpine is pulled via package registry pull-through cache (no Docker Hub access).
                    // Guarded to avoid noisy first-run failures when config is not yet in workspace.
                    options { timeout(time: 3, unit: 'MINUTES') }
                    steps {
                        script {
                            if (currentBuild.getBuildCauses('org.jenkinsci.plugins.workflow.cps.replay.ReplayCause').size() > 0) {
                                echo "[WARN] This build was started via Jenkins Replay. The pipeline Groovy script is frozen at the original build — it may not match the current repo state. Run a fresh build if the pipeline or config has changed since then."
                            }
                            preCheckoutWorkspaceCleanup(PIPELINE_CONFIG)
                        }
                        checkout scm
                    }
                }

                stage('Pipeline Snapshot Guard') {
                    // Compare checked-out .majordomo submodule SHA against remote
                    // updates branch and offer a fresh-build handoff only on drift.
                    options { timeout(time: 3, unit: 'MINUTES') }
                    steps {
                        script {
                            verifySubmoduleIsCurrent()
                        }
                    }
                }

                stage('Validate Config') {
                    options { timeout(time: 2, unit: 'MINUTES') }
                    steps {
                        script {
                            if (!fileExists('.majordomo-config.groovy')) {
                                error """.majordomo-config.groovy not found at the repository root.
Copy the template and configure it:
  cp .majordomo/example.majordomo-config.groovy .majordomo-config.groovy
  git add .majordomo-config.groovy && git commit -m 'Add Majordomo config'"""
                            }

                            // Cache config from the pipeline branch checkout before any later
                            // stage switches the workspace to the PR source branch.
                            cacheRuntimeConfig(loadConfig(PIPELINE_CONFIG))
                        }
                    }
                }

                stage('PR Update Policy') {
                    options { timeout(time: 1, unit: 'MINUTES') }
                    steps {
                        script {
                            env.COPILOT_SKIP_PR_RUN = 'false'
                            if (!isPrBuild()) {
                                return
                            }

                            def cfg = getRuntimeConfig(PIPELINE_CONFIG)
                            def triggeredByUser = currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause').size() > 0
                            def isRetrigger = params.REPLAY_OF_BUILD?.trim() as Boolean
                            def isPrUpdate = params.PR_EVENT_TYPE == 'pr:from_ref_updated'
                            def configContinuousRunsEnabled = (cfg?.cache?.enableContinuousRuns ?: false) as Boolean
                            def continuousRunsEnabled = (
                                (params.ENABLE_CONTINUOUS_RUNS as Boolean)
                                || ((env.COPILOT_ENABLE_CONTINUOUS_RUNS ?: 'false').toBoolean())
                                || configContinuousRunsEnabled
                            )

                            if (isPrUpdate && !triggeredByUser && !isRetrigger && !continuousRunsEnabled) {
                                env.COPILOT_SKIP_PR_RUN = 'true'
                                echo "[INFO] PR update auto-run skipped by policy. Set cache.enableContinuousRuns=true (or ENABLE_CONTINUOUS_RUNS=true) to enable continuous runs."
                            }
                        }
                    }
                }

                stage('Ensure Images') {
                    // NOTE: script { parallel(...) } used instead of declarative parallel
                    // because Jenkins forbids 'agent' and 'parallel' on the same stage.
                    when {
                        expression { env.COPILOT_SKIP_PR_RUN != 'true' }
                    }
                    options { timeout(time: 20, unit: 'MINUTES') }
                    steps {
                        script {
                            def cfg  = getRuntimeConfig(PIPELINE_CONFIG)
                            def deps = loadDependencies()
                            ensureImages(cfg, deps)
                        }
                    }
                }

                stage('Notify: Build In Progress') {
                    // Notify Bitbucket only for PR builds when token credentials are configured.
                    when {
                        expression {
                            def cfg = getRuntimeConfig(PIPELINE_CONFIG)
                            return env.COPILOT_SKIP_PR_RUN != 'true' && isPrBuild() && (cfg.credentials?.bitbucketTokenCredentialsId as Boolean)
                        }
                    }
                    options { timeout(time: 3, unit: 'MINUTES') }
                    steps {
                        script {
                            def cfg         = getRuntimeConfig(PIPELINE_CONFIG)
                            def deps        = loadDependencies()
                            if (!deps.notify) {
                                echo '[WARN] notify.groovy not found; skipping Bitbucket INPROGRESS notification.'
                                return
                            }
                            def tokenCredId = cfg.credentials?.bitbucketTokenCredentialsId
                            def commitSha   = deps.notify.resolveGitCommitSha()

                            deps.notify.notifyBitbucketBuildStatus(
                                deps.logger,
                                deps.executor,
                                'INPROGRESS',
                                'Copilot review pipeline running',
                                tokenCredId,
                                env.COPILOT_FULL_IMAGE,
                                commitSha
                            )
                        }
                    }
                }

                stage('Static Analysis') {
                    // Skipped when staticAnalysis is absent or empty in config, or on plain branch pushes.
                    when {
                        expression {
                            def cfg      = getRuntimeConfig(PIPELINE_CONFIG)
                            def hasTools = cfg.staticAnalysis as Boolean
                            def isPr     = isPrBuild()
                            return env.COPILOT_SKIP_PR_RUN != 'true' && hasTools && isPr
                        }
                    }
                    options { timeout(time: 20, unit: 'MINUTES') }
                    steps {
                        script {
                            // Guard: ensure analysis runs on webhook PR source branch, not only
                            // on the initial checkout scm revision.
                            ensurePrSourceBranchCheckedOut('Static Analysis')

                            def cfg       = getRuntimeConfig(PIPELINE_CONFIG)
                            def deps      = loadDependencies()
                            def module    = load '.majordomo/stages/static-analysis.groovy'
                            def saTools   = cfg.staticAnalysis as List ?: []
                            def imageMap  = new groovy.json.JsonSlurperClassic().parseText(env.SA_IMAGE_MAP ?: '{}') as Map
                            def baseBranch = params.CHANGE_TARGET ?: env.CHANGE_TARGET ?: 'master'
                            module.run(deps.logger, deps.executor, saTools, imageMap, baseBranch)
                        }
                    }
                }

                stage('PR Review') {
                    // Always runs for PR events (CHANGE_ID present).
                    // The script inside decides whether to perform the review:
                    //   - pr:opened        → full review
                    //   - pr:from_ref_updated (webhook) → skips with a message; Rebuild the job to run
                    //   - Rebuild / manual user trigger  → full review (UserIdCause detected)
                    // Skipped entirely on plain branch pushes (no CHANGE_ID).
                    when {
                        expression {
                            return env.COPILOT_SKIP_PR_RUN != 'true' && isPrBuild()
                        }
                    }
                    // unstash must run on the host (NFS root_squash prevents unstash inside Docker).
                    // docker.image().inside() is used in steps so unstash and the container
                    // share the same node and workspace.
                    // Proxy required: copilot must reach api.github.com via corporate proxy.
                    // NO_PROXY excludes internal hosts so git/npm don't route through the proxy.
                    // CA cert is baked into the image (NODE_EXTRA_CA_CERTS already set there).
                    // GITHUB_TOKEN is injected in steps via withCredentials so the credential ID
                    // can be overridden by .majordomo-config.groovy (credentials.githubCopilotCredentialsId).
                    //
                    // NOTE: Proxy values are intentionally hardcoded for the corporate environment.
                    // This pipeline is designed for internal corporate infrastructure where these
                    // endpoints are stable. Non-corporate deployments should override HTTP_PROXY,
                    // HTTPS_PROXY, and NO_PROXY via Jenkins global environment variables or a
                    // custom .majordomo-config.groovy. Accepted portability limitation — intentional.
                    environment {
                        HTTP_PROXY   = 'http://proxy.example.com:8080'
                        HTTPS_PROXY  = 'http://proxy.example.com:8080'
                        NO_PROXY     = 'localhost,127.0.0.1,.example.com,packages.example.com'
                    }
                    options { timeout(time: 45, unit: 'MINUTES') }
                    steps {
                        script {
                            // Load config and credentials BEFORE checkoutBranch.
                            // Jenkins already checked out the correct commit via checkout scm,
                            // so .majordomo-config.groovy is present here.
                            // checkoutBranch runs git clean -ffd which deletes untracked/ignored
                            // files — including .majordomo-config.groovy if it is not
                            // committed to the PR branch.
                            def cfg        = getRuntimeConfig(PIPELINE_CONFIG)
                            def deps       = loadDependencies()
                            def module     = load '.majordomo/stages/copilot-review.groovy'
                            def prNumber   = getPrNumber()
                            def baseBranch = params.CHANGE_TARGET ?: env.CHANGE_TARGET ?: 'master'
                            def githubCopilotTokenCredId = cfg.credentials.githubCopilotCredentialsId

                            // Guard: ensure review always executes on webhook PR source branch.
                            // Config is loaded first because checkoutBranch performs git clean -ffd.
                            ensurePrSourceBranchCheckedOut('PR Review')

                            // Restore .sa/ findings AFTER checkoutBranch — git clean -ffd (run inside
                            // checkoutBranch) would delete the untracked .sa/ directory if unstashed earlier.
                            // Must happen here on the label agent — unstash inside a Docker container
                            // fails due to NFS root_squash rejecting chown() calls.
                            deps.executor.unstashIfPresent('sa-findings')

                            // Run the review inside the Copilot CLI image.
                            // docker.image().inside() mounts this node's workspace, so .sa/ files
                            // restored by unstash above are visible to git-diff-prep.py.
                            // LANG/LC_ALL/PYTHONIOENCODING are set explicitly — docker.image().inside()
                            // does not forward ENV vars from the image; the Jenkins agent environment
                            // takes precedence, leaving locale unset and causing UTF-8 mojibake in output.
                            // NOTE: copilotInsideArgs() MUST be called before docker.withRegistry() —
                            // withRegistry sets $DOCKER_CONFIG as an env var which shadows same-named
                            // script variables inside the closure.
                            env.COPILOT_HAS_REVIEWABLE_FILES = 'false'
                            def reviewInsideArgs = copilotInsideArgs()
                            docker.withRegistry(env.COPILOT_REGISTRY, env.COPILOT_REGISTRY_CREDS) {
                                docker.image(env.COPILOT_FULL_IMAGE).inside(reviewInsideArgs) {
                                    withCredentials([string(credentialsId: githubCopilotTokenCredId, variable: 'GITHUB_TOKEN')]) {
                                        def hasReviewableFiles = module.review(
                                            deps.logger, deps.executor,
                                            prNumber,
                                            baseBranch,
                                            cfg.pipelines ?: [:],
                                            cfg.cache ?: [:],
                                            cfg.credentials ?: [:]
                                        )
                                        env.COPILOT_HAS_REVIEWABLE_FILES = hasReviewableFiles ? 'true' : 'false'
                                    }
                                }
                            }
                        }
                    }
                }

                stage('Convert Reports to HTML') {
                    // Converts summary.md and tech-review.md to self-contained HTML pages and
                    // archives them as build artifacts for direct browser linking.
                    // Skipped on plain branch pushes (no CHANGE_ID).
                    when {
                        expression {
                            return env.COPILOT_SKIP_PR_RUN != 'true' && isPrBuild() && env.COPILOT_HAS_REVIEWABLE_FILES == 'true'
                        }
                    }
                    options { timeout(time: 5, unit: 'MINUTES') }
                    steps {
                        script {
                            def cfg       = getRuntimeConfig(PIPELINE_CONFIG)
                            def deps      = loadDependencies()
                            def module    = load '.majordomo/stages/convert-reports.groovy'
                            def prNumber  = getPrNumber()
                            def outputDir = "${REVIEW_OUTPUT_PREFIX}${prNumber}/pr-review"
                            def convertInsideArgs = copilotInsideArgs()
                            docker.withRegistry(env.COPILOT_REGISTRY, env.COPILOT_REGISTRY_CREDS) {
                                docker.image(env.COPILOT_FULL_IMAGE).inside(convertInsideArgs) {
                                    module.convert.call(deps.logger, deps.executor, outputDir)
                                }
                            }
                        }
                    }
                }

                stage('Publish PR Summary') {
                    // Posts the Copilot summary.md to the Bitbucket PR as a comment or replaces the PR description.
                    // Skipped on plain branch pushes (no CHANGE_ID) or when SUMMARY_PUBLISH_MODE is 'off'.
                    when {
                        expression {
                            def cfg = getRuntimeConfig(PIPELINE_CONFIG)
                            return env.COPILOT_SKIP_PR_RUN != 'true' && isPrBuild() &&
                                   env.COPILOT_HAS_REVIEWABLE_FILES == 'true' &&
                                   params.SUMMARY_PUBLISH_MODE != 'off' &&
                                   cfg.credentials?.bitbucketTokenCredentialsId as Boolean
                        }
                    }
                    options { timeout(time: 5, unit: 'MINUTES') }
                    steps {
                        script {
                            def cfg         = getRuntimeConfig(PIPELINE_CONFIG)
                            def deps        = loadDependencies()
                            def module      = load '.majordomo/stages/publish-pr-summary.groovy'
                            def prNumber    = getPrNumber()
                            def outputDir   = "${REVIEW_OUTPUT_PREFIX}${prNumber}"
                            def pipeName    = 'pr-review'
                            def tokenCredId = cfg.credentials?.bitbucketTokenCredentialsId
                            module.publishSummary(
                                deps.logger, deps.executor,
                                prNumber,
                                outputDir,
                                pipeName,
                                params.SUMMARY_PUBLISH_MODE,
                                tokenCredId,
                                env.COPILOT_FULL_IMAGE,
                                env.COPILOT_REGISTRY,
                                env.COPILOT_REGISTRY_CREDS
                            )
                        }
                    }
                }
            }
        }
    }

    post {
        always {
            script {
                // Post runs on the pipeline's single allocated agent — same node and workspace
                // as all stages. No node() wrapper needed.
                logPipelineStatus('INFO', 'completed')
                archiveReviewOutputs()
                cleanupRootOwnedFiles()
            }
        }
        success {
            script {
                notifyBitbucketBuildStatus('SUCCESSFUL', 'Copilot review pipeline passed')
            }
        }
        failure {
            script {
                notifyBitbucketBuildStatus('FAILED', 'Copilot review pipeline failed')
                logPipelineStatus('ERROR', 'failed')
            }
        }
        unstable {
            script {
                notifyBitbucketBuildStatus('SUCCESSFUL', 'Copilot review pipeline completed with warnings (unstable)')
            }
        }
        aborted {
            script {
                notifyBitbucketBuildStatus('FAILED', 'Copilot review pipeline aborted')
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Dispatcher helpers
// ---------------------------------------------------------------------------

// Returns Docker inside() args with root user, HOME, and UTF-8 locale.
// LANG/LC_ALL/PYTHONIOENCODING must be set explicitly — docker.image().inside() does not
// forward ENV vars from the image; the Jenkins agent environment takes precedence.
def copilotInsideArgs() {
    return "${PIPELINE_CONFIG.jenkinsAgent.dockerArgs} -e LANG=en_US.UTF-8 -e LC_ALL=en_US.UTF-8 -e PYTHONIOENCODING=utf-8 -e PYTHONUTF8=1"
}

// Builds and caches all required Docker images in parallel.
// Copilot CLI image is always built; SA tool images are added only when staticAnalysis is configured.
// Sets env.COPILOT_FULL_IMAGE, env.COPILOT_REGISTRY, env.COPILOT_REGISTRY_CREDS, env.SA_IMAGE_MAP.
def ensureImages(Map cfg, Map deps) {
    withCredentials([
        usernamePassword(
            credentialsId: cfg.registry.credentialsId,
            usernameVariable: 'REGISTRY_USR',
            passwordVariable: 'REGISTRY_PSW'
        ),
        usernamePassword(
            credentialsId: cfg.credentials.package-registryCredentialsId,
            usernameVariable: 'REGISTRY_USER',
            passwordVariable: 'REGISTRY_TOKEN'
        )
    ]) {
        def branches = [:]

        branches['Ensure Copilot CLI Image'] = {
            def tags   = load '.majordomo/stages/calculate-image-tags.groovy'
            def module = load '.majordomo/stages/copilot-image.groovy'
            def result = module.ensure(
                deps.logger, deps.executor, tags,
                cfg.registry.pushDomain, env.IMAGE_NAME, env.DOCKERFILE
            )
            env.COPILOT_IMAGE_TAG      = result.imageTag
            env.COPILOT_FULL_IMAGE     = result.fullImage
            env.COPILOT_REGISTRY       = "https://${cfg.registry.pushDomain}"
            env.COPILOT_REGISTRY_CREDS = cfg.registry.credentialsId
        }

        // Only add SA branch when staticAnalysis tools are configured
        def saTools = cfg.staticAnalysis as List ?: []
        if (saTools) {
            branches['Ensure SA Tool Images'] = {
                def tags   = load '.majordomo/stages/calculate-image-tags.groovy'
                def module = load '.majordomo/stages/sa-image.groovy'
                def imageMap = module.ensure(
                    deps.logger, deps.executor, tags,
                    cfg.registry.pushDomain, saTools
                )
                // Serialise imageMap to JSON so it survives the env var boundary
                env.SA_IMAGE_MAP = groovy.json.JsonOutput.toJson(imageMap)
            }
        }

        parallel(branches)
    }
}

def logPipelineStatus(String level, String verb) {
    def branch = params.CHANGE_BRANCH ?: params.PUSH_BRANCH ?: env.BRANCH_NAME ?: env.CHANGE_BRANCH ?: 'unknown'
    def status = (level == 'INFO') ? (currentBuild.result ?: 'IN_PROGRESS') : null
    def suffix = status ? ", Status: ${status}" : ''
    echo "[${new Date().format('yyyy-MM-dd HH:mm:ss')}] [${level}] Pipeline ${verb} — Branch: ${branch}, PR: ${getPrNumber() ?: 'n/a'}, Build: ${env.BUILD_NUMBER}${suffix}"
}

// Archives review outputs unconditionally so failed builds still produce artifacts.
// On success, review() already archived the full set before dropping per-file dirs;
// this call picks up anything left (e.g. partial outputs after a prose-synthesis failure).
// allowEmptyArchive suppresses errors when no output was produced at all.
def archiveReviewOutputs() {
    def prNum = getPrNumber()
    if (!prNum?.trim()) { return }
    def reviewOutput = "${REVIEW_OUTPUT_PREFIX}${prNum}"
    if (!fileExists(reviewOutput)) { return }
    archiveArtifacts(
        artifacts: "${reviewOutput}/**",
        excludes:  "${reviewOutput}/**/logs/**",
        allowEmptyArchive: true
    )
}

// Removes root-owned workspace files left by Docker stages (-u root).
// Uses COPILOT_FULL_IMAGE — already pulled by Ensure Images, no registry lookup needed.
// If the image is not set (Ensure Images failed), logs a warning and skips.
def cleanupRootOwnedFiles() {
    def cleanupImage = env.COPILOT_FULL_IMAGE
    if (cleanupImage) {
        sh "docker run --rm -v \"\$(pwd):/workspace\" ${cleanupImage} sh -c 'rm -rf /workspace/${REVIEW_OUTPUT_PREFIX}* && chmod -R a+rw /workspace 2>/dev/null || true' || true"
    } else {
        echo "[WARN] COPILOT_FULL_IMAGE not set — skipping root-owned file cleanup (Ensure Images may have failed)"
    }
}

def isPrBuild() {
    return getPrNumber()?.trim() as Boolean
}

def verifySubmoduleIsCurrent() {
    def pipelineLib   = load '.majordomo/lib/pipeline.groovy'
    def submodulePath = '.majordomo'
    def trackedBranch = 'updates'

    // Skip drift check on guard-triggered retrigers to prevent infinite blocking loops.
    // Two paths:
    //   build() fallback — REPLAY_OF_BUILD is set explicitly by replayHandoffParameters()
    //   GWT webhook re-fire — fires a raw webhook with no actor; ACTOR_NAME is empty
    // In both cases the guard already ran and the user approved the handoff on the previous
    // build. Re-running the check here would block again on the same (still-stale) pin.
    def isRetrigger = (params.REPLAY_OF_BUILD?.trim() as Boolean) ||
                      (!params.ACTOR_NAME?.trim() && params.PR_EVENT_TYPE?.trim() as Boolean)
    if (isRetrigger) {
        echo "[INFO] Submodule drift check skipped — this is a guard-triggered retrigger (REPLAY_OF_BUILD=${params.REPLAY_OF_BUILD ?: ''}, ACTOR_NAME=${params.ACTOR_NAME ?: '<empty>'})."
        return
    }

    if (!fileExists(submodulePath)) {
        echo "[WARN] Submodule drift check skipped — ${submodulePath} not found in workspace."
        return
    }

    def localSha = sh(
        script: "git -C '${submodulePath}' rev-parse HEAD",
        returnStdout: true
    ).trim()

    def cfg = loadConfig(PIPELINE_CONFIG)

    // ls-remote requires SSH credentials — use the configured Bitbucket SSH credential if available.
    // Falls back to warn-and-skip if the credential is not configured or the connection fails.
    def lsRemoteOutput = ''
    def sshCredId = cfg.credentials?.bitbucketSshCredentialsId?.trim()
    try {
        if (sshCredId) {
            withCredentials([sshUserPrivateKey(credentialsId: sshCredId, keyFileVariable: 'SUBMODULE_SSH_KEY')]) {
                lsRemoteOutput = sh(
                    script: "GIT_SSH_COMMAND='ssh -i \$SUBMODULE_SSH_KEY -o StrictHostKeyChecking=no -o BatchMode=yes' git -C '${submodulePath}' ls-remote origin refs/heads/${trackedBranch}",
                    returnStdout: true
                ).trim()
            }
        } else {
            echo '[WARN] bitbucketSshCredentialsId not configured — attempting ls-remote with agent default key.'
            lsRemoteOutput = sh(
                script: "git -C '${submodulePath}' ls-remote origin refs/heads/${trackedBranch}",
                returnStdout: true
            ).trim()
        }
    } catch (Exception e) {
        echo "[WARN] Could not reach submodule remote (${e.message?.readLines()?.first() ?: 'connection failed'}) — skipping drift check."
        return
    }

    if (!lsRemoteOutput) {
        echo "[WARN] Could not resolve remote SHA for ${submodulePath}/${trackedBranch} — skipping drift check."
        return
    }

    def remoteSha = lsRemoteOutput.split(/\s+/)[0]
    if (localSha == remoteSha) {
        echo "[INFO] Submodule ${submodulePath} is current (${localSha.take(8)})."
        return
    }

    def approvalUrl = pipelineLib.buildInputApprovalUrl()
    echo "[WARN] Submodule drift detected: local=${localSha.take(8)} remote=${remoteSha.take(8)} — this build uses stale pipeline code."
    echo "[WARN] Approve fresh-build handoff here: ${approvalUrl}"

    def driftTimeoutMinutes = (cfg.submoduleDriftTimeoutMinutes ?: 60) as Integer

    try {
        timeout(time: driftTimeoutMinutes, unit: 'MINUTES') {
            input(
                message: "Submodule ${submodulePath} is behind '${trackedBranch}' (local: ${localSha.take(8)}, latest: ${remoteSha.take(8)}). Trigger a fresh build with updated pipeline code instead?\nApproval link: ${approvalUrl}",
                ok: 'Trigger Fresh Build'
            )
        }
    } catch (org.jenkinsci.plugins.workflow.steps.FlowInterruptedException e) {
        error 'Build blocked — no handoff approval received. Trigger a fresh build/rebuild instead.'
    }

    // Prefer GWT webhook re-fire — triggered build has no upstream parent link in the Jenkins UI.
    // Falls back to build() when gwtTokenCredentialsId is not configured or the POST fails.
    if (pipelineLib.triggerFreshBuildViaGwt(cfg)) {
        error 'Build aborted — fresh build triggered via webhook with updated pipeline code.'
    }

    def freshBuildQueued = false
    try {
        build(
            job: env.JOB_NAME,
            parameters: pipelineLib.replayHandoffParameters(),
            wait: false
        )
        freshBuildQueued = true
    } catch (Throwable e) {
        echo "[WARN] Could not queue fresh build (${e.class.simpleName}): ${e.message}. Proceeding with replay."
    }

    if (freshBuildQueued) {
        echo '[INFO] Fresh build queued. Aborting this replay.'
        error 'Build aborted — fresh build with updated pipeline code has been queued.'
    }
}

def notifyBitbucketBuildStatus(String state, String description) {
    if (!isPrBuild()) {
        return
    }

    def cfg         = getRuntimeConfig(PIPELINE_CONFIG)
    def tokenCredId = cfg.credentials?.bitbucketTokenCredentialsId
    if (!tokenCredId) {
        return
    }

    if (!env.COPILOT_FULL_IMAGE?.trim()) {
        echo '[WARN] COPILOT_FULL_IMAGE is empty; skipping Bitbucket build-status notification.'
        return
    }

    def deps = loadDependencies()
    if (!deps.notify) {
        echo '[WARN] notify.groovy not found; skipping Bitbucket build-status notification.'
        return
    }

    deps.notify.notifyBitbucketBuildStatus(
        deps.logger,
        deps.executor,
        state,
        description,
        tokenCredId,
        env.COPILOT_FULL_IMAGE
    )
}

def getPrNumber() {
    return params.CHANGE_ID ?: env.CHANGE_ID
}

def cacheRuntimeConfig(Map cfg) {
    env.RUNTIME_CONFIG_JSON = groovy.json.JsonOutput.toJson(cfg)
}

def getRuntimeConfig(Map defaults) {
    def cached = env.RUNTIME_CONFIG_JSON?.trim()
    if (!cached) {
        return loadConfig(defaults)
    }
    return new groovy.json.JsonSlurperClassic().parseText(cached) as Map
}

// Ensures diff-sensitive stages run on the webhook source branch for PR events.
// No-op on non-PR builds.
def ensurePrSourceBranchCheckedOut(String stageName) {
    if (!isPrBuild()) { return }

    def sourceBranch = params.CHANGE_BRANCH?.trim()
    if (!sourceBranch) {
        error "${stageName}: CHANGE_BRANCH is empty for a PR build; cannot guarantee webhook source-branch context."
    }

    checkoutBranch(sourceBranch)
}

// Best-effort cleanup before checkout scm to recover reused workspaces.
// Safe to skip on fresh workspaces where config is not present yet.
def preCheckoutWorkspaceCleanup(Map defaults) {
    if (!fileExists('.majordomo-config.groovy')) {
        echo '[INFO] Pre-checkout workspace cleanup skipped — config not present yet (fresh workspace expected).'
        return
    }

    def cfg = loadConfig(defaults)
    def pullDomain = cfg.registry?.pullDomain?.toString()?.trim()
    if (!pullDomain || pullDomain == 'your-pull-registry.example.com') {
        echo '[INFO] Pre-checkout workspace cleanup skipped — registry pullDomain is placeholder or empty.'
        return
    }

    def cleanupImage = "${pullDomain}/alpine:latest"
    def rc = sh(
        script: "docker run --rm -v \"\$(pwd):/workspace\" ${cleanupImage} sh -c 'chmod -R a+rw /workspace 2>/dev/null || true'",
        returnStatus: true
    )
    if (rc != 0) {
        echo "[INFO] Pre-checkout workspace cleanup skipped — ${cleanupImage} unavailable (exit ${rc})."
    }
}

def loadDependencies() {
    def logger = null
    if (fileExists('.majordomo/lib/logger.groovy')) {
        logger = load '.majordomo/lib/logger.groovy'
    } else if (fileExists('lib/logger.groovy')) {
        logger = load 'lib/logger.groovy'
    } else {
        error 'logger.groovy not found at .majordomo/lib/ or lib/ — cannot continue.'
    }

    def executor = null
    if (fileExists('.majordomo/lib/executor.groovy')) {
        executor = load '.majordomo/lib/executor.groovy'
    } else if (fileExists('lib/executor.groovy')) {
        executor = load 'lib/executor.groovy'
    } else {
        error 'executor.groovy not found at .majordomo/lib/ or lib/ — cannot continue.'
    }

    def notify = null
    if (fileExists('.majordomo/lib/notify.groovy')) {
        notify = load '.majordomo/lib/notify.groovy'
    } else if (fileExists('lib/notify.groovy')) {
        notify = load 'lib/notify.groovy'
    } else {
        echo '[WARN] notify.groovy not found; build-status notifications will be skipped.'
    }

    return [logger: logger, executor: executor, notify: notify]
}

// Merges defaults with overrides from .majordomo-config.groovy at the app repo root.
// Must be called inside steps — requires a node context for fileExists/load.
// Config file MUST end with `return [...]` so that load() returns the Map directly.
// Calling methods on loaded scripts goes through the Jenkins sandbox DSL dispatcher
// and throws NoSuchMethodError — method calls on loaded scripts are not sandbox-safe.
def loadConfig(Map defaults) {
    if (!fileExists('.majordomo-config.groovy')) { return defaults }
    def overrides = load '.majordomo-config.groovy'
    if (!(overrides instanceof Map)) {
        echo "[WARN] .majordomo-config.groovy did not return a Map — using pipeline defaults. Ensure the file ends with: return [registry: [...], ...]"
        return defaults
    }
    return deepMerge(defaults, overrides)
}

def deepMerge(Map base, Map overrides) {
    def result = base.clone()
    overrides.each { k, v ->
        result[k] = (result[k] instanceof Map && v instanceof Map) ? deepMerge(result[k], v) : v
    }
    return result
}

// Marks the workspace as git-safe and checks out the given branch.
// safe.directory is required when running inside Docker as root: the NFS workspace
// is owned by a different UID and git refuses to operate without the exception.
// Root-owned staging dirs from a prior PR Review run are deleted via docker run before
// git clean — the Jenkins host user cannot remove root-owned files directly (NFS root_squash).
// git clean removes any remaining untracked files left by previous Docker builds.
// -ffd: force (twice to handle nested git dirs), remove untracked directories.
// IMPORTANT: Always reset local branch state to origin/<branch> to avoid stale divergence
// from prior builds inflating git diff scope (e.g. Static Analysis scanning unrelated files).
// Do not fetch here: this shell context does not have SCM credentials attached.
// checkout scm already fetched remote refs using Jenkins-managed credentials.
def checkoutBranch(String branch) {
    // COPILOT_FULL_IMAGE is set by Ensure Images (which always runs before checkoutBranch).
    // Use it directly — already on the host, no registry lookup required.
    def cleanupImage = env.COPILOT_FULL_IMAGE
    sh "docker run --rm -v \"\$(pwd):/workspace\" ${cleanupImage} sh -c 'rm -rf /workspace/${REVIEW_OUTPUT_PREFIX}* && chmod -R a+rw /workspace 2>/dev/null || true' || true"
    sh """
        git config --global --add safe.directory '${env.WORKSPACE}'
        git clean -ffd -e .majordomo/ -e .jenkins-python-ci/
        git checkout -B '${branch}' 'origin/${branch}'
        git reset --hard 'origin/${branch}'
    """
}

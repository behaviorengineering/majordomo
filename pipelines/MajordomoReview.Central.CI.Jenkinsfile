// .majordomo/pipelines/MajordomoReview.Central.CI.Jenkinsfile
// Central execution mode — one Jenkins job serves ALL onboarded repos.
// Triggered by Generic Webhook Trigger from any managed app repo.
// Per-repo config loaded from majordomo-central-config/<repo-slug>.groovy, merged over majordomo-central-config/_defaults.groovy.
//
// Jenkins job configuration:
//   Script Path:  .majordomo/pipelines/MajordomoReview.Central.CI.Jenkinsfile
//   SCM:          This pipeline repo (NOT the app repos)
//   Trigger:      Generic Webhook Trigger — configure ALL variable extractions below.
//
//   Variable         Expression                              Set by
//   REPO_SLUG        $.pullRequest.fromRef.repository.slug   PR events
//   PROJECT_KEY      $.pullRequest.fromRef.repository.project.key PR events
//   CHANGE_ID        $.pullRequest.id                        pr:opened, pr:from_ref_updated
//   CHANGE_TARGET    $.pullRequest.toRef.displayId           pr:opened, pr:from_ref_updated
//   CHANGE_BRANCH    $.pullRequest.fromRef.displayId         pr:opened, pr:from_ref_updated
//   CHANGE_COMMIT    $.pullRequest.fromRef.latestCommit      pr:opened, pr:from_ref_updated
//   PUSH_BRANCH      $.changes[0].ref.displayId              repo:refs_changed
//   PR_EVENT_TYPE    $.eventKey                              all events
//
// Required credentials (set once in majordomo-central-config/_defaults.groovy):
//   <docker-registry-creds>          - Username/password for Artifactory Docker registry
//   <artifactory-token-creds>        - SALARY_ID + JFROG_TOKEN for BuildKit secrets
//   <github-copilot-token-creds>     - GitHub personal access token with Copilot access
//   <bitbucket-service-account-token>- Service account token with PR-write access across all repos
//   <bitbucket-ssh-creds>            - SSH key for cloning app repos

@groovy.transform.Field
final String REVIEW_OUTPUT_PREFIX = 'copilot-review-pr-'

@groovy.transform.Field
final String APP_REPO_WORKSPACE = 'app-repo'

// Agent label and Docker args are fixed here — cannot be read from config before agent allocation.
// Update these to match your org infrastructure if they differ from the per-repo pipeline defaults.
@groovy.transform.Field
final String JENKINS_AGENT_LABEL = 'edp_obm_lnx_shared'

@groovy.transform.Field
final String JENKINS_DOCKER_ARGS = '-u root -e HOME=/root'

pipeline {
    agent { label JENKINS_AGENT_LABEL }

    options {
        timeout(time: 60, unit: 'MINUTES')
        skipDefaultCheckout(true)
    }

    parameters {
        // Repo identity — injected by GWT from Bitbucket webhook payload
        string(name: 'REPO_SLUG',    defaultValue: '', description: 'App repo slug ($.pullRequest.fromRef.repository.slug) — used to load majordomo-central-config/<slug>.groovy')
        string(name: 'PROJECT_KEY',  defaultValue: '', description: 'Bitbucket project key ($.pullRequest.fromRef.repository.project.key)')
        // PR / push identity
        string(name: 'CHANGE_ID',     defaultValue: '', description: 'PR number — set by pr:opened / pr:from_ref_updated ($.pullRequest.id)')
        string(name: 'CHANGE_TARGET', defaultValue: 'master', description: 'PR target branch ($.pullRequest.toRef.displayId)')
        string(name: 'CHANGE_BRANCH', defaultValue: '', description: 'PR source branch ($.pullRequest.fromRef.displayId)')
        string(name: 'CHANGE_COMMIT', defaultValue: '', description: 'PR source branch HEAD commit ($.pullRequest.fromRef.latestCommit)')
        string(name: 'PUSH_BRANCH',   defaultValue: '', description: 'Pushed branch — set by repo:refs_changed ($.changes[0].ref.displayId)')
        string(name: 'PR_EVENT_TYPE', defaultValue: '', description: 'Bitbucket event key ($.eventKey)')
        booleanParam(name: 'ENABLE_CONTINUOUS_RUNS', defaultValue: false, description: 'When true, pr:from_ref_updated webhook events run automatically instead of requiring a manual rebuild')
        // Optional — sent by Parameterized Builds plugin for profile/flag overrides
        string(name: 'REVIEW_PROFILE',       defaultValue: '', description: 'Named pipeline profile from repo config (optional)')
        choice(name: 'SUMMARY_PUBLISH_MODE', choices: ['auto', 'comment', 'description', 'off'], description: 'How to publish the summary to Bitbucket')
        string(name: 'REPLAY_OF_BUILD',      defaultValue: '', description: 'Internal marker: source replay build number for auto-queued fresh reruns')
    }

    environment {
        // Paths relative to workspace root after checkout scm.
        IMAGE_NAME = 'copilot-cli'
        DOCKERFILE = '.majordomo/dockerfiles/copilot-cli.Dockerfile'
    }

    stages {
        stage('Pipeline Guard') {
            // Per-repo/per-branch lock guard only. PR update policy is evaluated
            // after config load so enableContinuousRuns can be sourced from settings.
            when {
                expression {
                    return true
                }
            }
            // Lock keyed on repo + branch so multiple repos can run concurrently,
            // but duplicate Bitbucket events for the same repo+branch are deduplicated.
            options {
                lock(
                    resource: "${env.JOB_NAME}-${params.REPO_SLUG ?: 'unknown'}-${params.CHANGE_BRANCH ?: params.PUSH_BRANCH ?: 'default'}",
                    skipIfLocked: true
                )
            }
            stages {
                stage('Safe Checkout') {
                    // Checks out this pipeline repo — stages, scripts, configs, and Dockerfiles
                    // are all here. The app repo is checked out separately in 'Checkout App Repo'.
                    options { timeout(time: 3, unit: 'MINUTES') }
                    steps {
                        script {
                            if (currentBuild.getBuildCauses('org.jenkinsci.plugins.workflow.cps.replay.ReplayCause').size() > 0) {
                                echo "[WARN] This build was started via Jenkins Replay. The pipeline Groovy script is frozen at the original build — run a fresh build if pipeline code has changed."
                            }
                            centralPreCheckoutWorkspaceCleanup()
                        }
                        checkout scm
                    }
                }

                stage('Validate Config') {
                    // Verifies REPO_SLUG is present, the per-repo config file exists, and
                    // loads + caches the merged config (_defaults.groovy + <repo-slug>.groovy).
                    options { timeout(time: 2, unit: 'MINUTES') }
                    steps {
                        script {
                            def repoSlug = params.REPO_SLUG?.trim()
                            if (!repoSlug) {
                                error "REPO_SLUG parameter is empty — ensure the GWT variable extraction for '\$.pullRequest.fromRef.repository.slug' is configured on this job."
                            }
                            def repoConfigPath = "majordomo-central-config/${repoSlug}.groovy"
                            if (!fileExists(repoConfigPath)) {
                                error "${repoConfigPath} not found.\nOnboard this repo by creating majordomo-central-config/${repoSlug}.groovy based on example.majordomo-central-config/example.repo-config.groovy."
                            }
                            cacheRuntimeConfig(loadCentralConfig(repoSlug))
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

                            def cfg = getRuntimeConfig([:])
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

                stage('Checkout App Repo') {
                    // Clones the triggering app repo into APP_REPO_WORKSPACE subdirectory.
                    // Sets APP_REPO_DIR so copilot-review.groovy and static-analysis.groovy
                    // run git commands from the app repo root instead of the pipeline repo root.
                    // Sets BB_PROJECT_KEY + BB_REPO_SLUG so notify-build-status.groovy resolves
                    // the Bitbucket callback URL without needing git remote on the app repo.
                    when {
                        expression { env.COPILOT_SKIP_PR_RUN != 'true' }
                    }
                    options { timeout(time: 5, unit: 'MINUTES') }
                    steps {
                        script {
                            def cfg      = getRuntimeConfig([:])
                            def repoSlug = params.REPO_SLUG.trim()
                            def cloneUrl = cfg.bitbucket?.cloneSshUrl?.trim()
                            if (!cloneUrl) {
                                error "majordomo-central-config/${repoSlug}.groovy is missing bitbucket.cloneSshUrl — required for central checkout."
                            }
                            def sshCredId = cfg.credentials?.bitbucketSshCredentialsId?.trim()
                            if (!sshCredId) {
                                error "credentials.bitbucketSshCredentialsId not configured in majordomo-central-config/_defaults.groovy."
                            }

                            // Clean any prior app-repo checkout so git clone starts fresh
                            sh "rm -rf '${APP_REPO_WORKSPACE}'"

                            def sourceBranch = params.CHANGE_BRANCH?.trim() ?: params.PUSH_BRANCH?.trim() ?: params.CHANGE_TARGET?.trim() ?: 'master'
                            def baseBranch   = params.CHANGE_TARGET?.trim() ?: 'master'
                            withCredentials([sshUserPrivateKey(credentialsId: sshCredId, keyFileVariable: 'APP_REPO_SSH_KEY')]) {
                                sh """
                                    GIT_SSH_COMMAND='ssh -i \$APP_REPO_SSH_KEY -o StrictHostKeyChecking=no -o BatchMode=yes' \\
                                        git clone --depth=50 --branch '${sourceBranch}' '${cloneUrl}' '${APP_REPO_WORKSPACE}'
                                """
                                // Also fetch the base branch so git-diff-prep.py can compute
                                // origin/<baseBranch>...HEAD — clone above only pulls sourceBranch.
                                if (sourceBranch != baseBranch) {
                                    sh """
                                        GIT_SSH_COMMAND='ssh -i \$APP_REPO_SSH_KEY -o StrictHostKeyChecking=no -o BatchMode=yes' \\
                                            git -C '${APP_REPO_WORKSPACE}' fetch --depth=50 origin \
                                                '+refs/heads/${baseBranch}:refs/remotes/origin/${baseBranch}'
                                    """
                                }
                            }

                            env.APP_REPO_DIR  = "${env.WORKSPACE}/${APP_REPO_WORKSPACE}"
                            env.BB_PROJECT_KEY = cfg.bitbucket?.projectKey?.trim() ?: params.PROJECT_KEY?.trim() ?: ''
                            env.BB_REPO_SLUG   = repoSlug

                            echo "[INFO] App repo checked out: ${cloneUrl} @ ${sourceBranch}"
                            echo "[INFO] APP_REPO_DIR=${env.APP_REPO_DIR}"
                        }
                    }
                }

                stage('Ensure Images') {
                    // Builds and caches Copilot CLI image (and SA tool images when configured).
                    // Image paths are relative to this pipeline repo root — no .majordomo/ prefix.
                    when {
                        expression { env.COPILOT_SKIP_PR_RUN != 'true' }
                    }
                    options { timeout(time: 20, unit: 'MINUTES') }
                    steps {
                        script {
                            def cfg  = getRuntimeConfig([:])
                            def deps = loadDependencies()
                            ensureImages(cfg, deps)
                        }
                    }
                }

                stage('Notify: Build In Progress') {
                    when {
                        expression {
                            def cfg = getRuntimeConfig([:])
                            return env.COPILOT_SKIP_PR_RUN != 'true' && isPrBuild() && (cfg.credentials?.bitbucketTokenCredentialsId as Boolean)
                        }
                    }
                    options { timeout(time: 3, unit: 'MINUTES') }
                    steps {
                        script {
                            def cfg         = getRuntimeConfig([:])
                            def deps        = loadDependencies()
                            if (!deps.notify) {
                                echo '[WARN] notify.groovy not found; skipping Bitbucket INPROGRESS notification.'
                                return
                            }
                            def tokenCredId = cfg.credentials?.bitbucketTokenCredentialsId
                            def commitSha   = deps.notify.resolveGitCommitSha()
                            deps.notify.notifyBitbucketBuildStatus(
                                deps.logger, deps.executor,
                                'INPROGRESS', 'Copilot review pipeline running',
                                tokenCredId, env.COPILOT_FULL_IMAGE, commitSha
                            )
                        }
                    }
                }

                stage('Static Analysis') {
                    when {
                        expression {
                            def cfg      = getRuntimeConfig([:])
                            def hasTools = cfg.staticAnalysis as Boolean
                            return env.COPILOT_SKIP_PR_RUN != 'true' && hasTools && isPrBuild()
                        }
                    }
                    options { timeout(time: 20, unit: 'MINUTES') }
                    steps {
                        script {
                            // Ensure SA runs on the PR source branch in the app repo checkout.
                            ensureCentralPrSourceBranchCheckedOut('Static Analysis')

                            def cfg       = getRuntimeConfig([:])
                            def deps      = loadDependencies()
                            def module    = loadCentralStage('static-analysis.groovy')
                            def saTools   = cfg.staticAnalysis as List ?: []
                            def imageMap  = new groovy.json.JsonSlurperClassic().parseText(env.SA_IMAGE_MAP ?: '{}') as Map
                            def baseBranch = params.CHANGE_TARGET ?: 'master'
                            module.run(deps.logger, deps.executor, saTools, imageMap, baseBranch)
                        }
                    }
                }

                stage('PR Review') {
                    when {
                        expression { return env.COPILOT_SKIP_PR_RUN != 'true' && isPrBuild() }
                    }
                    environment {
                        HTTP_PROXY  = 'http://wbcorp-pr-proxy0.westpac.com.au:8080'
                        HTTPS_PROXY = 'http://wbcorp-pr-proxy0.westpac.com.au:8080'
                        NO_PROXY    = 'localhost,127.0.0.1,.westpac.com.au,artifactory.srv.westpac.com.au'
                    }
                    options { timeout(time: 45, unit: 'MINUTES') }
                    steps {
                        script {
                            def cfg        = getRuntimeConfig([:])
                            def deps       = loadDependencies()
                            def module     = loadCentralStage('copilot-review.groovy')
                            def prNumber   = getPrNumber()
                            def baseBranch = params.CHANGE_TARGET ?: 'master'
                            def githubCopilotTokenCredId = cfg.credentials.githubCopilotCredentialsId

                            // Ensure review runs on the PR source branch in the app repo.
                            ensureCentralPrSourceBranchCheckedOut('PR Review')

                            // Restore SA findings after branch checkout — git clean inside
                            // ensureCentralPrSourceBranchCheckedOut would delete untracked .sa/ dir.
                            deps.executor.unstashIfPresent('sa-findings')

                            // Resolve active pipeline config — REVIEW_PROFILE selects a named
                            // profile from the repo config if defined; falls back to cfg.pipelines.
                            def pipelines = resolveReviewPipelines(cfg)

                            env.COPILOT_HAS_REVIEWABLE_FILES = 'false'
                            def reviewInsideArgs = centralCopilotInsideArgs()
                            docker.withRegistry(env.COPILOT_REGISTRY, env.COPILOT_REGISTRY_CREDS) {
                                docker.image(env.COPILOT_FULL_IMAGE).inside(reviewInsideArgs) {
                                    withCredentials([string(credentialsId: githubCopilotTokenCredId, variable: 'GITHUB_TOKEN')]) {
                                        def hasReviewableFiles = module.review(
                                            deps.logger, deps.executor,
                                            prNumber, baseBranch, pipelines, cfg.cache ?: [:], cfg.credentials ?: [:]
                                        )
                                        env.COPILOT_HAS_REVIEWABLE_FILES = hasReviewableFiles ? 'true' : 'false'
                                    }
                                }
                            }
                        }
                    }
                }

                stage('Convert Reports to HTML') {
                    when {
                        expression {
                            return env.COPILOT_SKIP_PR_RUN != 'true' && isPrBuild() && env.COPILOT_HAS_REVIEWABLE_FILES == 'true'
                        }
                    }
                    options { timeout(time: 5, unit: 'MINUTES') }
                    steps {
                        script {
                            def deps      = loadDependencies()
                            def module    = loadCentralStage('convert-reports.groovy')
                            def prNumber  = getPrNumber()
                            def outputDir = "${REVIEW_OUTPUT_PREFIX}${prNumber}/pr-review"
                            def insideArgs = centralCopilotInsideArgs()
                            docker.withRegistry(env.COPILOT_REGISTRY, env.COPILOT_REGISTRY_CREDS) {
                                docker.image(env.COPILOT_FULL_IMAGE).inside(insideArgs) {
                                    module.convert.call(deps.logger, deps.executor, outputDir)
                                }
                            }
                        }
                    }
                }

                stage('Publish PR Summary') {
                    when {
                        expression {
                            def cfg = getRuntimeConfig([:])
                            return env.COPILOT_SKIP_PR_RUN != 'true' && isPrBuild() &&
                                   env.COPILOT_HAS_REVIEWABLE_FILES == 'true' &&
                                   params.SUMMARY_PUBLISH_MODE != 'off' &&
                                   cfg.credentials?.bitbucketTokenCredentialsId as Boolean
                        }
                    }
                    options { timeout(time: 5, unit: 'MINUTES') }
                    steps {
                        script {
                            def cfg         = getRuntimeConfig([:])
                            def deps        = loadDependencies()
                            def module      = loadCentralStage('publish-pr-summary.groovy')
                            def prNumber    = getPrNumber()
                            def outputDir   = "${REVIEW_OUTPUT_PREFIX}${prNumber}"
                            def tokenCredId = cfg.credentials?.bitbucketTokenCredentialsId
                            module.publishSummary(
                                deps.logger, deps.executor,
                                prNumber, outputDir, 'pr-review',
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
                logPipelineStatus('INFO', 'completed')
                archiveReviewOutputs()
                cleanupRootOwnedFiles()
            }
        }
        success {
            script { notifyBitbucketBuildStatus('SUCCESSFUL', 'Copilot review pipeline passed') }
        }
        failure {
            script {
                notifyBitbucketBuildStatus('FAILED', 'Copilot review pipeline failed')
                logPipelineStatus('ERROR', 'failed')
            }
        }
        unstable {
            script { notifyBitbucketBuildStatus('SUCCESSFUL', 'Copilot review pipeline completed with warnings (unstable)') }
        }
        aborted {
            script { notifyBitbucketBuildStatus('FAILED', 'Copilot review pipeline aborted') }
        }
    }
}

// ---------------------------------------------------------------------------
// Dispatcher helpers — central mode
// ---------------------------------------------------------------------------

// Loads org defaults then deep-merges per-repo config on top.
def loadCentralConfig(String repoSlug) {
    def defaults  = fileExists('majordomo-central-config/_defaults.groovy') ? load('majordomo-central-config/_defaults.groovy') : [:]
    if (!(defaults instanceof Map)) {
        error "majordomo-central-config/_defaults.groovy did not return a Map — ensure the file ends with: return [...]"
    }
    def repoConfig = load "majordomo-central-config/${repoSlug}.groovy"
    if (!(repoConfig instanceof Map)) {
        error "majordomo-central-config/${repoSlug}.groovy did not return a Map — ensure the file ends with: return [...]"
    }
    return deepMerge(defaults as Map, repoConfig as Map)
}

// Selects the active pipeline config map.
// If REVIEW_PROFILE param is set, looks for cfg.profiles.<profile> first.
// Falls back to cfg.pipelines, then to the built-in pr-review default.
def resolveReviewPipelines(Map cfg) {
    def profile = params.REVIEW_PROFILE?.trim()
    if (profile && cfg.profiles instanceof Map && cfg.profiles[profile] instanceof Map) {
        echo "[INFO] Using named review profile: ${profile}"
        return [(profile): cfg.profiles[profile]]
    }
    return cfg.pipelines ?: ['pr-review': [:]]
}

// Returns Docker inside() args with root user, HOME, and UTF-8 locale.
def centralCopilotInsideArgs() {
    return "${JENKINS_DOCKER_ARGS} -e LANG=en_US.UTF-8 -e LC_ALL=en_US.UTF-8 -e PYTHONIOENCODING=utf-8 -e PYTHONUTF8=1"
}

// Ensures the app repo checkout is on the PR source branch before diff-sensitive stages.
// No-op on non-PR builds.
def ensureCentralPrSourceBranchCheckedOut(String stageName) {
    if (!isPrBuild()) { return }
    def sourceBranch = params.CHANGE_BRANCH?.trim()
    if (!sourceBranch) {
        error "${stageName}: CHANGE_BRANCH is empty for a PR build — cannot guarantee source-branch context."
    }
    def appRepoDir = env.APP_REPO_DIR?.trim()
    if (!appRepoDir) {
        error "${stageName}: APP_REPO_DIR is not set — Checkout App Repo stage must run first."
    }
    sh """
        git -C '${appRepoDir}' config --global --add safe.directory '${appRepoDir}'
        git -C '${appRepoDir}' clean -ffd
        git -C '${appRepoDir}' checkout -B '${sourceBranch}' 'origin/${sourceBranch}'
        git -C '${appRepoDir}' reset --hard 'origin/${sourceBranch}'
    """
}

// Best-effort workspace cleanup before checkout scm.
// In central mode this is the pipeline repo — much simpler than the per-repo case
// (no app-repo root-owned files at this point; app-repo dir cleaned in Checkout App Repo).
def centralPreCheckoutWorkspaceCleanup() {
    if (fileExists('majordomo-central-config/_defaults.groovy')) {
        echo '[INFO] Pre-checkout workspace cleanup: pipeline repo workspace reused — no cleanup needed.'
    }
}

// Builds and caches all required Docker images in parallel.
// Paths are relative to this pipeline repo root (no .majordomo/ prefix).
def ensureImages(Map cfg, Map deps) {
    def copilotDockerfile = resolveExistingPath(
        ['.majordomo/dockerfiles/copilot-cli.Dockerfile', 'dockerfiles/copilot-cli.Dockerfile'],
        'Copilot CLI Dockerfile'
    )
    withCredentials([
        usernamePassword(credentialsId: cfg.registry.credentialsId,
                         usernameVariable: 'REGISTRY_USR', passwordVariable: 'REGISTRY_PSW'),
        usernamePassword(credentialsId: cfg.credentials.artifactoryCredentialsId,
                         usernameVariable: 'SALARY_ID', passwordVariable: 'JFROG_TOKEN')
    ]) {
        def branches = [:]

        branches['Ensure Copilot CLI Image'] = {
            def tags   = loadCentralStage('calculate-image-tags.groovy')
            def module = loadCentralStage('copilot-image.groovy')
            def result = module.ensure(
                deps.logger, deps.executor, tags,
                cfg.registry.pushDomain, env.IMAGE_NAME, copilotDockerfile
            )
            env.COPILOT_IMAGE_TAG      = result.imageTag
            env.COPILOT_FULL_IMAGE     = result.fullImage
            env.COPILOT_REGISTRY       = "https://${cfg.registry.pushDomain}"
            env.COPILOT_REGISTRY_CREDS = cfg.registry.credentialsId
        }

        def saTools = cfg.staticAnalysis as List ?: []
        if (saTools) {
            branches['Ensure SA Tool Images'] = {
                def tags   = loadCentralStage('calculate-image-tags.groovy')
                def module = loadCentralStage('sa-image.groovy')
                def imageMap = module.ensure(
                    deps.logger, deps.executor, tags,
                    cfg.registry.pushDomain, saTools
                )
                env.SA_IMAGE_MAP = groovy.json.JsonOutput.toJson(imageMap)
            }
        }

        parallel(branches)
    }
}

def loadCentralStage(String stageFileName) {
    def stagePath = resolveExistingPath(
        [".majordomo/stages/${stageFileName}", "stages/${stageFileName}"],
        "stage module ${stageFileName}"
    )
    return load(stagePath)
}

def resolveExistingPath(List<String> candidates, String label) {
    for (def candidate in candidates) {
        if (fileExists(candidate)) {
            return candidate
        }
    }
    def candidateList = candidates.join(', ')
    error "Could not resolve ${label}. Checked: ${candidateList}"
}

// Loads lib dependencies from this pipeline repo root (no .majordomo/ prefix).
def loadDependencies() {
    def logger = fileExists('lib/logger.groovy') ? load('lib/logger.groovy') :
                 (fileExists('.majordomo/lib/logger.groovy') ? load('.majordomo/lib/logger.groovy') : null)
    if (!logger) { error 'logger.groovy not found at lib/ — cannot continue.' }

    def executor = fileExists('lib/executor.groovy') ? load('lib/executor.groovy') :
                   (fileExists('.majordomo/lib/executor.groovy') ? load('.majordomo/lib/executor.groovy') : null)
    if (!executor) { error 'executor.groovy not found at lib/ — cannot continue.' }

    def notify = null
    if (fileExists('lib/notify.groovy')) {
        notify = load 'lib/notify.groovy'
    } else if (fileExists('.majordomo/lib/notify.groovy')) {
        notify = load '.majordomo/lib/notify.groovy'
    } else {
        echo '[WARN] notify.groovy not found; build-status notifications will be skipped.'
    }

    return [logger: logger, executor: executor, notify: notify]
}

def isPrBuild() {
    return getPrNumber()?.trim() as Boolean
}

def getPrNumber() {
    return params.CHANGE_ID ?: env.CHANGE_ID
}

def cacheRuntimeConfig(Map cfg) {
    env.RUNTIME_CONFIG_JSON = groovy.json.JsonOutput.toJson(cfg)
}

// getRuntimeConfig in central mode takes no defaults arg — config is always loaded from files.
def getRuntimeConfig(Map ignored = [:]) {
    def cached = env.RUNTIME_CONFIG_JSON?.trim()
    if (!cached) {
        def repoSlug = params.REPO_SLUG?.trim()
        if (!repoSlug) { error 'REPO_SLUG is empty — cannot load config.' }
        return loadCentralConfig(repoSlug)
    }
    return new groovy.json.JsonSlurperClassic().parseText(cached) as Map
}

def deepMerge(Map base, Map overrides) {
    def result = base.clone()
    overrides.each { k, v ->
        result[k] = (result[k] instanceof Map && v instanceof Map) ? deepMerge(result[k], v) : v
    }
    return result
}

def logPipelineStatus(String level, String verb) {
    def branch = params.CHANGE_BRANCH ?: params.PUSH_BRANCH ?: 'unknown'
    def repoSlug = params.REPO_SLUG ?: 'unknown'
    def status = (level == 'INFO') ? (currentBuild.result ?: 'IN_PROGRESS') : null
    def suffix = status ? ", Status: ${status}" : ''
    echo "[${new Date().format('yyyy-MM-dd HH:mm:ss')}] [${level}] Pipeline ${verb} — Repo: ${repoSlug}, Branch: ${branch}, PR: ${getPrNumber() ?: 'n/a'}, Build: ${env.BUILD_NUMBER}${suffix}"
}

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

def cleanupRootOwnedFiles() {
    def cleanupImage = env.COPILOT_FULL_IMAGE
    if (cleanupImage) {
        sh "docker run --rm -v \"\$(pwd):/workspace\" ${cleanupImage} sh -c 'rm -rf /workspace/${REVIEW_OUTPUT_PREFIX}* && chmod -R a+rw /workspace 2>/dev/null || true' || true"
    } else {
        echo '[WARN] COPILOT_FULL_IMAGE not set — skipping root-owned file cleanup.'
    }
}

def notifyBitbucketBuildStatus(String state, String description) {
    if (!isPrBuild()) { return }
    def cachedConfig = env.RUNTIME_CONFIG_JSON?.trim()
    if (!cachedConfig) {
        echo '[INFO] Runtime config not cached; skipping Bitbucket build-status notification.'
        return
    }
    def cfg         = getRuntimeConfig()
    def tokenCredId = cfg.credentials?.bitbucketTokenCredentialsId
    if (!tokenCredId) { return }
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
        deps.logger, deps.executor,
        state, description, tokenCredId, env.COPILOT_FULL_IMAGE
    )
}

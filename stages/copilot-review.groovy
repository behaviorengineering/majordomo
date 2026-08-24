// .majordomo/stages/copilot-review.groovy
// Runs Copilot CLI PR review per pipeline inside the copilot-cli Docker container
// and archives the results as a directory of markdown artifacts.
// Receives logger and executor via dependency injection (IoC).

@groovy.transform.Field
final String REVIEW_OUTPUT_PREFIX = 'copilot-review-pr-'
//
// Wave orchestration:
//   git-diff-prep.py writes per-skill/per-batch staging dirs and batch-plan.json.
//   This stage reads batch-plan.json and runs batches in waves of COPILOT_CONCURRENCY
//   (default 3) using script { parallel(...) } — all batches within a wave run inside
//   the same Docker container allocated for the PR Review stage.
//   After all waves, one finalize session per skill writes blast-radius, summary, index.
//
//   Checkpoint files written by this stage:
//     <output>/<skill>/logs/batch_NNN.done.txt   — written after each successful batch
//     <output>/<skill>/logs/finalize.done.txt    — written after each successful finalize
//   Restarting the PR Review stage re-runs all waves; checkpoints skip completed work.

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

def buildEnvVars(String pipelineName, Map pipeConfig) {
    def envVars = ["COPILOT_PIPELINE=${pipelineName}"]
    if (pipeConfig?.model) {
        envVars << "COPILOT_MODEL=${pipeConfig.model}"
    }
    if (pipeConfig?.summaryModel) {
        envVars << "COPILOT_SUMMARY_MODEL=${pipeConfig.summaryModel}"
    }
    if (pipeConfig?.technicalModel) {
        envVars << "COPILOT_TECHNICAL_MODEL=${pipeConfig.technicalModel}"
    }
    if (pipeConfig?.deepTechnicalModel) {
        envVars << "COPILOT_DEEP_TECHNICAL_MODEL=${pipeConfig.deepTechnicalModel}"
    }
    if (pipeConfig?.scoreModel) {
        envVars << "COPILOT_SCORE_MODEL=${pipeConfig.scoreModel}"
    }
    if (pipeConfig?.agent) {
        envVars << "COPILOT_AGENT_OVERRIDES=${pipelineName}=${env.WORKSPACE}/${pipeConfig.agent}"
    }
    if (pipeConfig?.skills) {
        def skillOverrides = pipeConfig.skills
            .findAll { skill, path -> path != null }
            .collect { skill, path -> "${skill}=${env.WORKSPACE}/${path}" }
            .join(',')
        if (skillOverrides) {
            envVars << "COPILOT_SKILL_OVERRIDES=${skillOverrides}"
        }
    }
    return envVars
}

def writeRoutingFile(String stagingDir, def routing) {
    if (!routing) { return '' }
    def routingJson = groovy.json.JsonOutput.toJson(routing)
    def routingFile = "${stagingDir}-routing.json"
    writeFile file: routingFile, text: routingJson
    return "--routing '${routingFile}'"
}

def writeAgentContextFile(String stagingDir, def agentContext) {
    if (!agentContext) { return '' }
    def contextJson = groovy.json.JsonOutput.toJson(agentContext)
    def contextFile = "${stagingDir}-agent-context.json"
    writeFile file: contextFile, text: contextJson
    return "--agent-context '${contextFile}'"
}

def writeSummaryConfigFile(String stagingDir, def summaryConfig) {
    if (!summaryConfig) { return '' }
    def configJson = groovy.json.JsonOutput.toJson(summaryConfig)
    def configFile = "${stagingDir}-summary-config.json"
    writeFile file: configFile, text: configJson
    return "--summary-config '${configFile}'"
}

def readBatchPlan(String batchPlanFile) {
    // readJSON (Pipeline Utility Steps) returns serializable data compatible with Jenkins CPS.
    def parsed = readJSON(file: batchPlanFile)
    // Eagerly convert to plain serializable types to avoid LazyMap serialization issues.
    def batches = []
    for (def b in parsed.batches) {
        batches << [
            skill:       b.skill       as String,
            batch_num:   b.batch_num   as String,
            task_count:  b.task_count  as int,
            staging_dir: b.staging_dir as String,
        ]
    }
    def skills = []
    for (def s in parsed.skills) {
        skills << (s as String)
    }
    return [batches: batches, skills: skills, total_batches: batches.size()]
}

def resolveScriptPath(String scriptFileName) {
    def workspace = env.WORKSPACE?.trim() ?: '.'
    def candidates = [
        "${workspace}/.majordomo/pipelines/scripts/${scriptFileName}",
        "${workspace}/pipelines/scripts/${scriptFileName}",
        "${workspace}/.majordomo/scripts/${scriptFileName}",
        "${workspace}/scripts/${scriptFileName}",
        "./.majordomo/pipelines/scripts/${scriptFileName}",
        "./pipelines/scripts/${scriptFileName}",
        "./.majordomo/scripts/${scriptFileName}",
        "./scripts/${scriptFileName}",
    ]
    for (def candidate in candidates) {
        if (sh(script: "[ -f '${candidate}' ]", returnStatus: true) == 0) {
            return candidate
        }
    }
    error "${scriptFileName} not found. Checked: ${candidates.join(', ')}"
}

def resolveDiffPrepPath() {
    return resolveScriptPath('git-diff-prep.py')
}

/** Prefer majordomo prep binary; MAJORDOMO_PREP=python forces Python fallback. */
def resolveDiffPrepCommand(String baseBranch, String stagingDir, String routingArg, String agentContextArg, String summaryConfigArg) {
    def args = ["'${baseBranch}'", "'${stagingDir}'", routingArg, agentContextArg, summaryConfigArg]
        .findAll { it }
        .join(' ')
    def forcePython = (env.MAJORDOMO_PREP ?: '').trim().equalsIgnoreCase('python')
    def majordomoBin = ''
    if (!forcePython) {
        def candidates = [
            './majordomo',
            './.majordomo/majordomo',
            'majordomo',
        ]
        for (def c in candidates) {
            if (c == 'majordomo') {
                if (sh(script: "command -v majordomo >/dev/null 2>&1", returnStatus: true) == 0) {
                    majordomoBin = 'majordomo'
                    break
                }
            } else if (sh(script: "[ -x '${c}' ]", returnStatus: true) == 0) {
                majordomoBin = c
                break
            }
        }
    }
    if (majordomoBin) {
        return "${majordomoBin} prep ${args}"
    }
    def diffPrepPath = resolveDiffPrepPath()
    return "python3 '${diffPrepPath}' ${args}"
}

def sha256Hex(String input) {
    def tempFile = sh(script: 'mktemp', returnStdout: true).trim()
    if (!tempFile) {
        error 'Unable to allocate temporary file for SHA-256 computation'
    }

    writeFile(file: tempFile, text: input)
    try {
        return sh(
            script: "python3 - <<'PY'\nimport hashlib\nfrom pathlib import Path\nprint(hashlib.sha256(Path('${tempFile}').read_bytes()).hexdigest())\nPY",
            returnStdout: true
        ).trim()
    } finally {
        sh(script: "rm -f '${tempFile}'", returnStatus: true)
    }
}

def collectBatchFiles(String batchStagingDir) {
    def manifestPath = "${batchStagingDir}/manifest.json"
    if (!fileExists(manifestPath)) {
        return []
    }
    def manifest = readJSON(file: manifestPath)
    def tasks = (manifest?.reviewable ?: []) as List
    return tasks
        .collect { task ->
            def path = task?.file?.toString() ?: ''
            return path.replace('\\\\', '/')
        }
        .findAll { it }
        .unique()
        .sort()
}

def shellQuote(String value) {
    return "'${value.replace("'", "'\\\"'\\\"'")}'"
}

def buildClusterFileArgs(List clusterFiles) {
    return (clusterFiles ?: [])
        .collect { filePath -> "--cluster-file ${shellQuote(filePath.toString())}" }
}

def buildArtifactFileArgs(List artifactFiles) {
    return (artifactFiles ?: [])
        .collect { fileName -> "--artifact-file ${shellQuote(fileName.toString())}" }
}

def synthesisArtifactFiles(String skillName) {
    if (skillName == 'pr-review-summary') {
        return ['summary.md', 'score.md']
    }
    if (skillName == 'pr-review-technical') {
        return ['tech-review.md', 'tech-score.md']
    }
    if (skillName == 'pr-review-blast-radius') {
        return ['summary.md']
    }
    return []
}

def resolveAppRepoDir() {
    return (env.APP_REPO_DIR?.trim() ?: env.WORKSPACE?.trim() ?: '.').toString()
}

def resolveBlobShaForPath(String filePath) {
    def repoDir = resolveAppRepoDir()
    def escapedPath = filePath.replace("'", "'\"'\"'")
    return sh(
        script: "git -C '${repoDir}' rev-parse 'HEAD:${escapedPath}'",
        returnStdout: true
    ).trim()
}

def resolveModelIdForSkill(String skillName, Map pipeConfig) {
    if (skillName == 'pr-review-summary' || skillName == 'pr-review-blast-radius') {
        return (
            pipeConfig?.summaryModel
            ?: pipeConfig?.model
            ?: env.COPILOT_SUMMARY_MODEL
            ?: env.COPILOT_MODEL
            ?: 'unknown-model'
        ).toString()
    }

    if (skillName == 'pr-review-technical') {
        return (
            pipeConfig?.technicalModel
            ?: pipeConfig?.model
            ?: env.COPILOT_TECHNICAL_MODEL
            ?: env.COPILOT_MODEL
            ?: 'unknown-model'
        ).toString()
    }

    return (pipeConfig?.model ?: env.COPILOT_MODEL ?: 'unknown-model').toString()
}

def buildBatchCacheContext(String skillName, String batchStagingDir, Map pipeConfig) {
    def batchFiles = collectBatchFiles(batchStagingDir)
    if (!batchFiles) {
        return [valid: false, reason: 'batch-files-missing']
    }

    def fingerprintVersion = (env.COPILOT_FINGERPRINT_VERSION ?: 'v1').toString()
    def clusterFilesHash = sha256Hex(batchFiles.join('\n'))
    def fingerprintInputs = batchFiles.collect { filePath ->
        def blobSha = resolveBlobShaForPath(filePath)
        return "${filePath}:${blobSha}"
    }.sort()
    def clusterSha = sha256Hex(fingerprintInputs.join('\n'))

    def modelId = resolveModelIdForSkill(skillName, pipeConfig)
    def modelRevision = (env.COPILOT_MODEL_REVISION ?: '').toString()
    def instructionHash = (env.COPILOT_INSTRUCTION_BUNDLE_HASH ?: 'unknown').toString()
    def promptHash = (env.COPILOT_PROMPT_TEMPLATE_HASH ?: 'unknown').toString()
    def rubricHash = (env.COPILOT_SCORING_RUBRIC_HASH ?: 'unknown').toString()
    def outputSchemaVersion = (env.COPILOT_OUTPUT_SCHEMA_VERSION ?: 'v1').toString()

    return [
        valid: true,
        clusterFiles: batchFiles,
        clusterFilesHash: clusterFilesHash,
        fingerprintVersion: fingerprintVersion,
        clusterSha: clusterSha,
        modelId: modelId,
        modelRevision: modelRevision,
        instructionHash: instructionHash,
        promptHash: promptHash,
        rubricHash: rubricHash,
        outputSchemaVersion: outputSchemaVersion,
    ]
}

def runCachePrecheck(
    logger,
    String reviewCacheScriptPath,
    String projectId,
    String cacheDir,
    String indexFile,
    Map cacheConfig
) {
    def projectRetention = env.COPILOT_CACHE_PROJECT_RETENTION_DAYS?.trim()
    if (!projectRetention && cacheConfig?.retentionDays != null) {
        projectRetention = cacheConfig.retentionDays.toString()
    }

    def centralRetention = env.COPILOT_CACHE_CENTRAL_RETENTION_DAYS?.trim()
    if (!centralRetention && cacheConfig?.centralRetentionDays != null) {
        centralRetention = cacheConfig.centralRetentionDays.toString()
    }

    def globalRetention = env.COPILOT_CACHE_RETENTION_DAYS?.trim()
    if (!globalRetention && cacheConfig?.globalRetentionDays != null) {
        globalRetention = cacheConfig.globalRetentionDays.toString()
    }
    if (!globalRetention) {
        globalRetention = '180'
    }

    def minRetention = env.COPILOT_CACHE_MIN_RETENTION_DAYS?.trim()
    if (!minRetention && cacheConfig?.minRetentionDays != null) {
        minRetention = cacheConfig.minRetentionDays.toString()
    }
    if (!minRetention) {
        minRetention = '30'
    }

    def commandParts = [
        "python3 '${reviewCacheScriptPath}' precheck",
        "--project-id '${projectId}'",
        "--cache-dir '${cacheDir}'",
        "--global-retention-days '${globalRetention}'",
        "--min-retention-days '${minRetention}'",
        "--index-out '${indexFile}'",
    ]

    if (projectRetention) {
        commandParts << "--project-retention-days '${projectRetention}'"
    }
    if (centralRetention) {
        commandParts << "--central-retention-days '${centralRetention}'"
    }

    def command = commandParts.join(' ')
    def status = sh(script: command, returnStatus: true)
    if (status != 0) {
        logger.warn("Cache precheck failed (exit ${status}) — continuing without cache decisions")
        return false
    }
    logger.info("Cache precheck complete: ${indexFile}")
    return true
}

def evaluateBatchCache(logger, String reviewCacheScriptPath, String indexFile, String skillName, String stagingDir, Map pipeConfig) {
    def cacheContext = buildBatchCacheContext(skillName, stagingDir, pipeConfig)
    if (!cacheContext.valid) {
        return [hit: false, reason: cacheContext.reason]
    }

    if (!fileExists(indexFile)) {
        return [hit: false, reason: 'index-missing', context: cacheContext]
    }

    def batchFiles = cacheContext.clusterFiles as List
    def clusterFileArgs = buildClusterFileArgs(batchFiles)

    def lookupParts = [
        "python3 '${reviewCacheScriptPath}' lookup",
        "--index-file '${indexFile}'",
        "--cluster-sha '${cacheContext.clusterSha}'",
        "--skill-name '${skillName}'",
        "--fingerprint-version '${cacheContext.fingerprintVersion}'",
        "--model-id '${cacheContext.modelId}'",
        "--instruction-bundle-hash '${cacheContext.instructionHash}'",
        "--prompt-template-hash '${cacheContext.promptHash}'",
        "--scoring-rubric-hash '${cacheContext.rubricHash}'",
        "--output-schema-version '${cacheContext.outputSchemaVersion}'",
    ]
    lookupParts.addAll(clusterFileArgs)
    if (cacheContext.modelRevision) {
        lookupParts << "--model-revision '${cacheContext.modelRevision}'"
    }

    def lookupJson = sh(script: "${lookupParts.join(' ')} || true", returnStdout: true).trim()
    if (!lookupJson) {
        return [hit: false, reason: 'lookup-empty', context: cacheContext]
    }

    try {
        def parsed = new groovy.json.JsonSlurperClassic().parseText(lookupJson) as Map
        return [
            hit: parsed.hit as Boolean,
            clusterSha: cacheContext.clusterSha,
            file: parsed.file ?: '',
            reason: parsed.reason ?: '',
            context: cacheContext,
        ]
    } catch (Exception ex) {
        logger.warn("Cache lookup parse failed for ${stagingDir}: ${ex.message}")
        return [hit: false, reason: 'lookup-parse-error', context: cacheContext]
    }
}

def restoreBatchCacheArtifacts(
    logger,
    String reviewCacheScriptPath,
    String cacheDir,
    String outputDir,
    String cacheEntryFile,
    String batchLabel
) {
    if (!cacheEntryFile?.trim()) {
        logger.warn("${batchLabel}: cache entry file missing; cannot restore artifacts")
        return false
    }

    def restoreCommand = [
        "python3 '${reviewCacheScriptPath}' restore",
        "--cache-dir '${cacheDir}'",
        "--entry-file '${cacheEntryFile}'",
        "--output-dir '${outputDir}'",
    ].join(' ')

    def restoreOutput = sh(script: "${restoreCommand} || true", returnStdout: true).trim()
    if (!restoreOutput) {
        logger.warn("${batchLabel}: cache restore returned empty output")
        return false
    }

    try {
        def parsed = new groovy.json.JsonSlurperClassic().parseText(restoreOutput) as Map
        def restored = parsed.restored as Boolean
        def restoredCount = parsed.restored_count ?: 0
        if (restored) {
            logger.info("${batchLabel}: restored ${restoredCount} cached markdown report(s)")
        } else {
            logger.warn("${batchLabel}: no cached markdown artifacts restored (${parsed.reason ?: 'unknown-reason'})")
        }
        return restored
    } catch (Exception ex) {
        logger.warn("${batchLabel}: cache restore parse failed: ${ex.message}")
        return false
    }
}

def normalizeProjectId(String rawProjectId) {
    return rawProjectId
        .toLowerCase()
        .replace('_', '-')
        .replace('/', '-')
        .replaceAll('[^a-z0-9-]', '-')
        .replaceAll('-+', '-')
        .replaceAll('^-|-$', '')
}

def resolveCacheBranchName(String projectId) {
    def normalized = normalizeProjectId(projectId)
    return "majordomo-pr-reviewer-cache/${normalized}"
}

def validateCacheBranchName(String branchName) {
    if (!(branchName ==~ /^majordomo-pr-reviewer-cache\/[a-z0-9][a-z0-9-]*$/)) {
        error "Invalid cache branch name '${branchName}'"
    }
}

def resolveGitRemote(String repoDir) {
    return sh(script: "git -C '${repoDir}' remote get-url origin", returnStdout: true).trim()
}

def toHttpsBitbucketRemote(String remoteUrl) {
    if (remoteUrl.startsWith('http://') || remoteUrl.startsWith('https://')) {
        return remoteUrl
    }

    def sshSchemeMatcher = (remoteUrl =~ /^ssh:\/\/[^@]+@([^:\/]+)(?::\d+)?\/(.+)$/)
    if (sshSchemeMatcher.matches()) {
        def host = sshSchemeMatcher[0][1]
        def path = sshSchemeMatcher[0][2]
        if (path.startsWith('scm/')) {
            return "https://${host}/${path}"
        }
        return "https://${host}/scm/${path}"
    }

    def scpMatcher = (remoteUrl =~ /^[^@]+@([^:]+):(.+)$/)
    if (scpMatcher.matches()) {
        def host = scpMatcher[0][1]
        def path = scpMatcher[0][2]
        if (path.startsWith('scm/')) {
            return "https://${host}/${path}"
        }
        return "https://${host}/scm/${path}"
    }

    return ''
}

def resolveCacheRemoteUrl(String cacheRepoMode, Map cacheConfig) {
    def appRepoDir = resolveAppRepoDir()
    def workspaceDir = env.WORKSPACE?.trim() ?: '.'

    if (cacheRepoMode == 'project') {
        def configured = cacheConfig?.projectRepoHttpUrl?.toString()?.trim()
        if (configured) {
            return configured
        }
        def originUrl = resolveGitRemote(appRepoDir)
        return toHttpsBitbucketRemote(originUrl)
    }

    def configuredCentral = cacheConfig?.centralRepoHttpUrl?.toString()?.trim()
    if (configuredCentral) {
        return configuredCentral
    }

    def centralOrigin = resolveGitRemote(workspaceDir)
    return toHttpsBitbucketRemote(centralOrigin)
}

def initializeCacheWorktree(String cacheDir, String remoteUrl, String branchName) {
    sh "mkdir -p '${cacheDir}'"
    if (sh(script: "[ -d '${cacheDir}/.git' ]", returnStatus: true) != 0) {
        sh "git -C '${cacheDir}' init"
    }

    if (sh(script: "git -C '${cacheDir}' remote get-url origin >/dev/null 2>&1", returnStatus: true) != 0) {
        sh "git -C '${cacheDir}' remote add origin '${remoteUrl}'"
    } else {
        sh "git -C '${cacheDir}' remote set-url origin '${remoteUrl}'"
    }

    def fetchStatus = sh(
        script: "git -C '${cacheDir}' fetch --depth=1 origin '+refs/heads/${branchName}:refs/remotes/origin/${branchName}'",
        returnStatus: true
    )
    if (fetchStatus == 0) {
        sh "git -C '${cacheDir}' checkout -B '${branchName}' 'refs/remotes/origin/${branchName}'"
        return
    }

    sh "git -C '${cacheDir}' checkout -B '${branchName}'"
}

def persistBatchCacheArtifact(
    logger,
    String reviewCacheScriptPath,
    String pushToCacheScriptPath,
    String cacheDir,
    String stagingDir,
    String skillName,
    Map cacheContext,
    String cacheBranch,
    String cacheRemoteUrl,
    String cacheTokenCredentialsId,
    String cacheUsername
) {
    final int cacheBranchValidationExitCode = 42

    def clusterFileArgs = buildClusterFileArgs(cacheContext.clusterFiles as List)

    def payloadFilePath = "${stagingDir}/manifest.json"
    def storeParts = [
        "python3 '${reviewCacheScriptPath}' store",
        "--cache-dir '${cacheDir}'",
        "--skill-name '${skillName}'",
        "--cluster-sha '${cacheContext.clusterSha}'",
        "--fingerprint-version '${cacheContext.fingerprintVersion}'",
        "--model-id '${cacheContext.modelId}'",
        "--instruction-bundle-hash '${cacheContext.instructionHash}'",
        "--prompt-template-hash '${cacheContext.promptHash}'",
        "--scoring-rubric-hash '${cacheContext.rubricHash}'",
        "--output-schema-version '${cacheContext.outputSchemaVersion}'",
        "--analysis-file '${payloadFilePath}'",
    ]
    storeParts.addAll(clusterFileArgs)
    if (cacheContext.modelRevision) {
        storeParts << "--model-revision '${cacheContext.modelRevision}'"
    }

    def storeJson = sh(script: storeParts.join(' '), returnStdout: true).trim()
    if (!storeJson) {
        logger.warn("Cache store returned empty output for ${skillName}/${cacheContext.clusterSha}")
        return
    }

    def storeResult = new groovy.json.JsonSlurperClassic().parseText(storeJson) as Map
    def artifactWritten = storeResult.written as Boolean
    if (!artifactWritten) {
        logger.info("Cache artifact unchanged for ${skillName}/${cacheContext.clusterSha}; will still attempt branch push")
    }

    def relativeCacheFile = storeResult.file?.toString()?.trim()
    if (!relativeCacheFile) {
        logger.warn("Cache store result missing file path for ${skillName}/${cacheContext.clusterSha}")
        return
    }

    if (!cacheTokenCredentialsId) {
        logger.error("Cache token credential is not configured; cache push cannot proceed")
        return [securityViolation: false, commitFailure: false, pushFailure: true, reason: "missing-cache-token-credential"]
    }

    def persistOutcome = [securityViolation: false, commitFailure: false, pushFailure: false, reason: "ok"]

    withCredentials([string(credentialsId: cacheTokenCredentialsId, variable: 'BITBUCKET_TOKEN')]) {
        initializeCacheWorktree(cacheDir, cacheRemoteUrl, cacheBranch)

        def cacheCommitName = (env.COPILOT_CACHE_COMMIT_NAME ?: 'majordomo-pr-reviewer-cache-bot').toString().trim()
        def cacheCommitEmail = (env.COPILOT_CACHE_COMMIT_EMAIL ?: 'majordomo-pr-reviewer-cache-bot@example.com').toString().trim()
        sh "git -C '${cacheDir}' config user.name ${shellQuote(cacheCommitName)}"
        sh "git -C '${cacheDir}' config user.email ${shellQuote(cacheCommitEmail)}"

        sh "git -C '${cacheDir}' add '${relativeCacheFile}'"

        def hasStagedChanges = sh(
            script: "git -C '${cacheDir}' diff --cached --quiet -- '${relativeCacheFile}'",
            returnStatus: true
        ) != 0
        if (!hasStagedChanges) {
            logger.info("No cache commit needed for ${skillName}/${cacheContext.clusterSha}")
        } else {
            def commitMessage = "cache(${skillName}): ${cacheContext.clusterSha}"
            def commitStatus = sh(
                script: "git -C '${cacheDir}' commit -m '${commitMessage}'",
                returnStatus: true
            )
            if (commitStatus != 0) {
                logger.error("Cache commit failed for ${skillName}/${cacheContext.clusterSha} (exit ${commitStatus})")
                persistOutcome = [securityViolation: false, commitFailure: true, pushFailure: false, reason: "cache-commit-failed"]
                return
            }
        }

        if (sh(script: "git -C '${cacheDir}' rev-parse --verify HEAD >/dev/null 2>&1", returnStatus: true) != 0) {
            logger.warn("Cache worktree has no commits yet for ${skillName}/${cacheContext.clusterSha}; skipping cache push")
            persistOutcome = [securityViolation: false, commitFailure: false, pushFailure: false, reason: "no-commit-to-push"]
            return
        }

        def pushEnv = []
        if (cacheUsername) {
            pushEnv << "BITBUCKET_USERNAME=${cacheUsername}"
        }
        def pushStatus = withEnv(pushEnv) {
            sh(
                script: "python3 '${pushToCacheScriptPath}' --remote '${cacheRemoteUrl}' --branch '${cacheBranch}' --worktree '${cacheDir}'",
                returnStatus: true
            )
        }
        if (pushStatus == cacheBranchValidationExitCode) {
            persistOutcome = [securityViolation: true, commitFailure: false, pushFailure: false, reason: "cache-branch-pattern-validation-failed"]
            return
        }
        if (pushStatus != 0) {
            logger.error("Cache push failed for ${skillName}/${cacheContext.clusterSha} (exit ${pushStatus})")
            persistOutcome = [securityViolation: false, commitFailure: false, pushFailure: true, reason: "cache-push-failed"]
            return
        }

        persistOutcome = [securityViolation: false, commitFailure: false, pushFailure: false, reason: "ok"]
    }

    return persistOutcome
}

def writePendingCachePersistRequest(
    String requestFilePath,
    String batchLabel,
    String skillName,
    String stagingDir,
    Map cacheContext,
    String cacheBranch,
    String cacheRemoteUrl,
    String cacheTokenCredentialsId,
    String cacheUsername
) {
    def request = [
        batchLabel: batchLabel,
        skillName: skillName,
        stagingDir: stagingDir,
        cacheContext: cacheContext,
        cacheBranch: cacheBranch,
        cacheRemoteUrl: cacheRemoteUrl,
        cacheTokenCredentialsId: cacheTokenCredentialsId,
        cacheUsername: cacheUsername,
    ]
    def requestDirPath = requestFilePath.contains('/')
        ? requestFilePath.substring(0, requestFilePath.lastIndexOf('/'))
        : '.'
    def requestJson = groovy.json.JsonOutput.toJson(request)
    sh(
        script: """
            mkdir -p '${requestDirPath}'
            REQUEST_FILE='${requestFilePath}' REQUEST_JSON='${requestJson}' python3 - <<'PY'
import os
from pathlib import Path

Path(os.environ['REQUEST_FILE']).write_text(os.environ['REQUEST_JSON'], encoding='utf-8')
PY
        """.stripIndent()
    )
}

def resolveCacheFlushLockResource(Map cacheConfig, String projectId, String cacheBranch) {
    def configuredTemplate = cacheConfig?.lockResource?.toString()?.trim()
    if (configuredTemplate) {
        def rendered = configuredTemplate
            .replace('{projectId}', projectId)
            .replace('{cacheBranch}', cacheBranch)
        return rendered.replaceAll(/[^A-Za-z0-9._-]/, '-')
    }

    def sanitizedProjectId = normalizeProjectId(projectId)
    return "copilot-cache-${sanitizedProjectId}"
}

def flushPendingCachePersistRequests(
    logger,
    String reviewCacheScriptPath,
    String pushToCacheScriptPath,
    String cacheDir,
    String outputDir,
    String projectId,
    Map cacheConfig
) {
    // Collect all pending requests written by batch closures.
    def requestListStr = sh(
        script: "find '${outputDir}' -type f -path '*/logs/cache-persist-*.json' | sort",
        returnStdout: true
    ).trim()
    if (!requestListStr) {
        return
    }

    def requests = []
    for (def requestFilePath in (requestListStr.split('\n') as List)) {
        def request = readJSON(file: requestFilePath) as Map
        def cacheContext = request.cacheContext as Map
        if (!(cacheContext?.valid as Boolean)) {
            logger.warn("Skipping malformed cache persist request: ${requestFilePath}")
            continue
        }
        requests << [path: requestFilePath, data: request]
    }
    if (!requests) { return }

    // All requests in a single pipeline share the same cache branch and credential.
    // Use the first request's settings for a single worktree setup + single push.
    def firstData  = requests[0].data as Map
    def cacheBranch             = firstData.cacheBranch as String
    def cacheRemoteUrl          = firstData.cacheRemoteUrl as String
    def cacheTokenCredentialsId = firstData.cacheTokenCredentialsId as String

    if (!cacheTokenCredentialsId) {
        logger.warn("Cache token credential not configured — skipping cache flush")
        return
    }

    final int cacheBranchValidationExitCode = 42
    def cacheLockResource = resolveCacheFlushLockResource(cacheConfig, projectId, cacheBranch)

    lock(resource: cacheLockResource) {
        logger.info("Acquired cache flush lock: ${cacheLockResource}")

        withCredentials([string(credentialsId: cacheTokenCredentialsId, variable: 'BITBUCKET_TOKEN')]) {
            // ONE fetch + checkout — all commits land on the same tip.
            initializeCacheWorktree(cacheDir, cacheRemoteUrl, cacheBranch)

            def cacheCommitName  = (env.COPILOT_CACHE_COMMIT_NAME  ?: 'majordomo-pr-reviewer-cache-bot').toString().trim()
            def cacheCommitEmail = (env.COPILOT_CACHE_COMMIT_EMAIL ?: 'majordomo-pr-reviewer-cache-bot@example.com').toString().trim()
            sh "git -C '${cacheDir}' config user.name ${shellQuote(cacheCommitName)}"
            sh "git -C '${cacheDir}' config user.email ${shellQuote(cacheCommitEmail)}"

            for (def entry in requests) {
                def request     = entry.data as Map
                def cacheContext = request.cacheContext as Map
                def skillName   = request.skillName as String
                def stagingDir  = request.stagingDir as String
                def summaryOnlySkills = ['pr-review-summary', 'pr-review-blast-radius', 'pr-review-technical']
                def reportDir = summaryOnlySkills.contains(skillName)
                    ? outputDir
                    : "${outputDir}/${skillName}"
                def artifactFileArgs = buildArtifactFileArgs(synthesisArtifactFiles(skillName))

                def clusterFileArgs = buildClusterFileArgs(cacheContext.clusterFiles as List)
                def storeParts = [
                    "python3 '${reviewCacheScriptPath}' store",
                    "--cache-dir '${cacheDir}'",
                    "--skill-name '${skillName}'",
                    "--cluster-sha '${cacheContext.clusterSha}'",
                    "--fingerprint-version '${cacheContext.fingerprintVersion}'",
                    "--model-id '${cacheContext.modelId}'",
                    "--instruction-bundle-hash '${cacheContext.instructionHash}'",
                    "--prompt-template-hash '${cacheContext.promptHash}'",
                    "--scoring-rubric-hash '${cacheContext.rubricHash}'",
                    "--output-schema-version '${cacheContext.outputSchemaVersion}'",
                    "--analysis-file '${stagingDir}/manifest.json'",
                    "--reports-dir '${reportDir}'",
                ]
                storeParts.addAll(clusterFileArgs)
                storeParts.addAll(artifactFileArgs)
                if (cacheContext.modelRevision) {
                    storeParts << "--model-revision '${cacheContext.modelRevision}'"
                }

                def storeJson = sh(script: storeParts.join(' '), returnStdout: true).trim()
                if (!storeJson) {
                    logger.warn("Cache store returned empty output for ${skillName}/${cacheContext.clusterSha}")
                    continue
                }

                def storeResult      = new groovy.json.JsonSlurperClassic().parseText(storeJson) as Map
                def relativeCacheFile = storeResult.file?.toString()?.trim()
                if (!relativeCacheFile) {
                    logger.warn("Cache store result missing file path for ${skillName}/${cacheContext.clusterSha}")
                    continue
                }

                sh "git -C '${cacheDir}' add '${relativeCacheFile}'"
                def hasStagedChanges = sh(
                    script: "git -C '${cacheDir}' diff --cached --quiet -- '${relativeCacheFile}'",
                    returnStatus: true
                ) != 0

                if (!hasStagedChanges) {
                    logger.info("No cache commit needed for ${skillName}/${cacheContext.clusterSha}")
                } else {
                    def commitStatus = sh(
                        script: "git -C '${cacheDir}' commit -m 'cache(${skillName}): ${cacheContext.clusterSha}'",
                        returnStatus: true
                    )
                    if (commitStatus != 0) {
                        logger.warn("Cache commit failed for ${skillName}/${cacheContext.clusterSha} (exit ${commitStatus})")
                    }
                }

                sh "rm -f '${entry.path}'"
            }

            // ONE push for the entire flush — eliminates stale-info races from sequential pushes.
            if (sh(script: "git -C '${cacheDir}' rev-parse --verify HEAD >/dev/null 2>&1", returnStatus: true) != 0) {
                logger.warn("Cache worktree has no commits — skipping push")
                return
            }
            def pushStatus = sh(
                script: "python3 '${pushToCacheScriptPath}' --remote '${cacheRemoteUrl}' --branch '${cacheBranch}' --worktree '${cacheDir}'",
                returnStatus: true
            )
            if (pushStatus == cacheBranchValidationExitCode) {
                logger.warn("Cache push branch pattern validation failed — cache write-back skipped")
            } else if (pushStatus != 0) {
                logger.warn("Cache push failed (exit ${pushStatus}) — cache write-back incomplete for this build")
            }
        }
    }
}

// ---------------------------------------------------------------------------
// runPipelineWithBatches: wave-based batch orchestration
// ---------------------------------------------------------------------------

def runPipelineWithBatches(logger, executor, String prNumber, String baseBranch,
                           String outputBaseDir, String pipelineName, Map pipeConfig,
                           Map cacheConfig, Map credentialsConfig) {
    def workspaceDir = env.WORKSPACE?.trim() ?: '.'
    def stagingDir = "${workspaceDir}/${outputBaseDir}-staging-${pipelineName}"
    def outputDir  = "${outputBaseDir}/${pipelineName}"
    def envVars    = buildEnvVars(pipelineName, pipeConfig)
    def routingArg = writeRoutingFile(stagingDir, pipeConfig?.routing)
    def agentContextArg = writeAgentContextFile(stagingDir, pipeConfig?.agentContext)
    def summaryConfigArg = writeSummaryConfigFile(stagingDir, pipeConfig?.summary)
    def dispatchScriptPath = resolveScriptPath('copilot-dispatch.sh')
    def summaryLoopScriptPath = resolveScriptPath('summary-loop.py')
    def techReviewLoopScriptPath = resolveScriptPath('tech-review-loop.py')
    def techReviewDeepScriptPath = resolveScriptPath('tech-review-deep.py')
    def reviewCacheScriptPath = resolveScriptPath('review-cache.py')
    def pushToCacheScriptPath = resolveScriptPath('push-to-cache.py')

    // Run majordomo prep (or git-diff-prep.py fallback) — writes staging dirs + batch-plan.json
    // APP_REPO_DIR: set only by the central pipeline — git commands run from that subdirectory.
    // Unset (per-repo submodule mode): behaviour unchanged, cwd is the workspace root.
    // MAJORDOMO_PREP=python forces the Python script for one-release rollback.
    def appRepoDir   = env.APP_REPO_DIR?.trim()
    def diffPrepInner = resolveDiffPrepCommand(baseBranch, stagingDir, routingArg, agentContextArg, summaryConfigArg)
    def diffPrepCmd  = appRepoDir ?
        "cd '${appRepoDir}' && ${diffPrepInner}" :
        diffPrepInner
    def rc = sh(
        script: diffPrepCmd,
        returnStatus: true
    )
    if (rc == 2) {
        logger.info("[${pipelineName}] prep: nothing to review — skipping")
        return
    }
    if (rc != 0) { error "prep failed with exit code ${rc}" }

    def batchPlanFile = "${stagingDir}/batch-plan.json"
    if (!fileExists(batchPlanFile)) {
        error "batch-plan.json not found after prep — expected: ${batchPlanFile}"
    }

    def batchPlan    = readBatchPlan(batchPlanFile)
    def allBatches   = batchPlan.batches   as List
    def skills       = batchPlan.skills    as List
    def concurrency  = (env.COPILOT_CONCURRENCY ?: '6').toInteger()

    // Pre-analysis cache gate: retention resolution + cache metadata validation + index build.
    def projectId = (env.BB_REPO_SLUG ?: env.JOB_BASE_NAME ?: 'unknown-project').toString()
    def cacheBranch = resolveCacheBranchName(projectId)
    validateCacheBranchName(cacheBranch)

    def cacheRepoMode = (env.COPILOT_CACHE_REPO ?: cacheConfig?.cacheRepo ?: 'project').toString().trim().toLowerCase()
    if (!(cacheRepoMode in ['project', 'central'])) {
        logger.warn("Invalid cacheRepo value '${cacheRepoMode}' — defaulting to 'project'")
        cacheRepoMode = 'project'
    }

    def cacheRemoteUrl = resolveCacheRemoteUrl(cacheRepoMode, cacheConfig ?: [:])
    if (!cacheRemoteUrl) {
        logger.warn("Cache remote URL could not be resolved for mode '${cacheRepoMode}' — cache push disabled")
    }

    def cacheSkipsEnabled = false
    if (env.COPILOT_ENABLE_CACHE_SKIPS?.trim()) {
        cacheSkipsEnabled = env.COPILOT_ENABLE_CACHE_SKIPS.toBoolean()
    } else if (cacheConfig?.enableSkips != null) {
        cacheSkipsEnabled = cacheConfig.enableSkips as Boolean
    }

    def cacheTokenCredentialsId = (
        credentialsConfig?.bitbucketTokenCredentialsId
        ?: cacheConfig?.cacheTokenCredentialsId
        ?: credentialsConfig?.cacheTokenCredentialsId
        ?: ''
    ).toString().trim()
    def cacheUsername = (
        env.COPILOT_CACHE_USERNAME
        ?: cacheConfig?.cacheUsername
        ?: credentialsConfig?.cacheUsername
        ?: ''
    ).toString().trim()

    def configuredCacheDir = cacheConfig?.dir?.toString()?.trim()
    def cacheDir = (env.COPILOT_REVIEW_CACHE_DIR ?: configuredCacheDir ?: "${env.WORKSPACE}/.majordomo-review-cache/${projectId}").toString()
    def cacheIndexFile = "${stagingDir}/cache-index.json"

    if (cacheRemoteUrl && cacheTokenCredentialsId) {
        withCredentials([string(credentialsId: cacheTokenCredentialsId, variable: 'BITBUCKET_TOKEN')]) {
            try {
                initializeCacheWorktree(cacheDir, cacheRemoteUrl, cacheBranch)
            } catch (Exception ex) {
                logger.warn("Cache worktree initialization failed: ${ex.message}")
            }
        }
    }

    def cacheReady = runCachePrecheck(
        logger,
        reviewCacheScriptPath,
        projectId,
        cacheDir,
        cacheIndexFile,
        cacheConfig ?: [:]
    )

    // Split batches into two phases:
    //   Phase 1 — file-review skills: produce per-file <slug>.md reports
    //   Phase 2 — synthesis skills: run AFTER all per-file reports and indexes exist
    // Synthesis skills (summary, technical, blast-radius) read the finalized output
    // from Phase 1. Running them concurrently with Phase 1 gives them an incomplete
    // picture — they must start only after finalize has written index.md for every
    // per-file skill.
    def summaryOnlySkills = ['pr-review-summary', 'pr-review-blast-radius', 'pr-review-technical']
    def fileBatches      = allBatches.findAll { !summaryOnlySkills.contains(it.skill) }
    def synthesisBatches = allBatches.findAll {  summaryOnlySkills.contains(it.skill) }
    def totalFileBatches = fileBatches.size()

    logger.info("[${pipelineName}] ${allBatches.size()} batch(es) across skill(s): ${skills.join(', ')}")
    logger.info("[${pipelineName}] Phase 1 — file-review batches: ${totalFileBatches}")
    logger.info("[${pipelineName}] Phase 2 — synthesis batches:   ${synthesisBatches.size()}")

    // ── Phase 1: File-review waves ───────────────────────────────────────
    // Each wave runs up to COPILOT_CONCURRENCY batches in parallel inside this
    // stage's Docker container.  script { parallel(...) } is used instead of
    // declarative parallel because Jenkins forbids 'agent' and 'parallel' on
    // the same stage — all closures run on the same allocated Docker node.
    // Batches only write per-file <slug>.md reports; summary/index/blast-radius
    // are deferred to the finalize step below.

    int waveIdx = 0
    int waveNum = 1
    while (waveIdx < totalFileBatches) {
        def end         = Math.min(waveIdx + concurrency, totalFileBatches)
        def waveBatches = fileBatches[waveIdx..<end]
        logger.info("[${pipelineName}] Wave ${waveNum}: batches ${waveIdx + 1}–${end} of ${totalFileBatches}")

        def parallelMap = [:]
        for (def batch in waveBatches) {
            def b              = batch  // capture for closure
            def skillOutputDir = "${outputDir}/${b.skill}"
            def checkpointFile = "${skillOutputDir}/logs/batch_${b.batch_num}.done.txt"
            def batchLabel     = "${b.skill} / batch_${b.batch_num}"

            parallelMap[batchLabel] = {
                sh "mkdir -p '${skillOutputDir}/logs'"
                if (fileExists(checkpointFile)) {
                    logger.info("[${pipelineName}] ${batchLabel}: checkpoint — skipping")
                    return
                }

                def cacheDecision = evaluateBatchCache(
                    logger,
                    reviewCacheScriptPath,
                    cacheIndexFile,
                    b.skill as String,
                    b.staging_dir as String,
                    pipeConfig
                )
                if (cacheReady) {
                    if (cacheDecision.hit) {
                        logger.info("[${pipelineName}] ${batchLabel}: cache-hit for ${cacheDecision.clusterSha}")
                        if (cacheSkipsEnabled) {
                            def restored = restoreBatchCacheArtifacts(
                                logger,
                                reviewCacheScriptPath,
                                cacheDir,
                                skillOutputDir,
                                cacheDecision.file?.toString() ?: '',
                                "[${pipelineName}] ${batchLabel}"
                            )
                            if (restored) {
                                logger.info("[${pipelineName}] ${batchLabel}: skipping execution due to cache-hit (artifacts restored)")
                                sh "touch '${checkpointFile}'"
                                return
                            }
                            logger.warn("[${pipelineName}] ${batchLabel}: cache-hit found but restore failed; continuing execution")
                        } else {
                            logger.info("[${pipelineName}] ${batchLabel}: cache skip disabled — continuing execution")
                        }
                    }
                }

                Throwable batchFailure = null
                try {
                    withEnv(envVars) {
                        // Phase 1 only contains file-review skills — all use default mode.
                        // Synthesis skills (summary, technical, blast-radius) are handled in Phase 2.
                        // Each batch is capped at COPILOT_BATCH_TIMEOUT_MINUTES (default 8).
                        // If it exceeds that, we pause 60s and retry once before failing.
                        def batchTimeoutMin = (env.COPILOT_BATCH_TIMEOUT_MINUTES ?: '8').toInteger()
                        def batchRc = -1
                        def batchScript = "bash '${dispatchScriptPath}' '${prNumber}' '${b.staging_dir}' '${skillOutputDir}'"
                        def attempts = 0
                        def timedOut = false
                        while (attempts < 2) {
                            attempts++
                            timedOut = false
                            try {
                                timeout(time: batchTimeoutMin, unit: 'MINUTES') {
                                    batchRc = sh(script: batchScript, returnStatus: true)
                                }
                                break  // completed within timeout — exit retry loop
                            } catch (org.jenkinsci.plugins.workflow.steps.FlowInterruptedException e) {
                                timedOut = true
                                if (attempts < 2) {
                                    logger.warn("[${pipelineName}] ${batchLabel}: timed out after ${batchTimeoutMin}min — pausing 60s then retrying once")
                                    sleep(time: 60, unit: 'SECONDS')
                                } else {
                                    error "${batchLabel} timed out twice (>${batchTimeoutMin}min each) — batch is too expensive to run"
                                }
                            }
                        }
                        if (!timedOut && batchRc != 0) { error "${batchLabel} failed (exit ${batchRc})" }
                    }
                } catch (Throwable err) {
                    batchFailure = err
                } finally {
                    def cacheContext = cacheDecision?.context
                    if (cacheContext?.valid && cacheRemoteUrl) {
                        def requestFilePath = "${skillOutputDir}/logs/cache-persist-${b.batch_num}.json"
                        writePendingCachePersistRequest(
                            requestFilePath,
                            batchLabel,
                            b.skill as String,
                            b.staging_dir as String,
                            cacheContext,
                            cacheBranch,
                            cacheRemoteUrl,
                            cacheTokenCredentialsId,
                            cacheUsername
                        )
                    }
                    sh "touch '${checkpointFile}'"
                }
                if (batchFailure) { throw batchFailure }
            }
        }

        parallel parallelMap

        waveIdx += concurrency
        waveNum++
    }

    // ── Finalize per file-review skill ───────────────────────────────────
    // One copilot session per skill: reads all per-file reports, runs blast-radius
    // analysis, then writes summary.md and index.md.
    // Synthesis skills (pr-review-summary, pr-review-blast-radius, pr-review-technical)
    // are excluded — they have no per-file reports to finalize.
    // Phase 2 (synthesis) runs after this loop so it can read the completed indexes.
    for (def skill in skills) {
        def s = skill  // capture
        if (summaryOnlySkills.contains(s)) {
            logger.info("[${pipelineName}] ${s}: summary-mode skill — no finalize step")
            continue
        }
        def skillStagingDir    = "${stagingDir}/${s}"
        def skillOutputDir     = "${outputDir}/${s}"
        def finalizeCheckpoint = "${skillOutputDir}/logs/finalize.done.txt"

        sh "mkdir -p '${skillOutputDir}/logs'"

        executor.withOperationLogging(logger, "Finalize ${pipelineName}/${s}", "PR-${prNumber}") {
            if (fileExists(finalizeCheckpoint)) {
                logger.info("[${pipelineName}] ${s} finalize: checkpoint — skipping")
            } else {
                withEnv(envVars) {
                    def finalizeRc = sh(
                        script: "bash '${dispatchScriptPath}' '${prNumber}' '${skillStagingDir}' '${skillOutputDir}' --finalize",
                        returnStatus: true
                    )
                    if (finalizeRc != 0) { error "Finalize failed for skill ${s} (exit ${finalizeRc})" }
                }
                // Validate required finalize outputs — copilot may exit 0 but write a
                // fallback meta-report instead of the real summary/index outputs.
                def missingSummary = !fileExists("${skillOutputDir}/summary.md")
                def missingIndex   = !fileExists("${skillOutputDir}/index.md")
                if (missingSummary || missingIndex) {
                    def missing = [missingSummary ? 'summary.md' : null, missingIndex ? 'index.md' : null].findAll { it }.join(', ')
                    error "Finalize for skill ${s} exited 0 but required output(s) missing: ${missing}"
                }
                // Copy findings.md (WARN/CRITICAL concat built during finalize staging) into the
                // skill output dir so it survives as an artifact after per-file/ is dropped.
                if (fileExists("${skillStagingDir}/findings.md")) {
                    sh "cp '${skillStagingDir}/findings.md' '${skillOutputDir}/findings.md'"
                }
                sh "touch '${finalizeCheckpoint}'"
            }
        }
    }

    // ── Prose rewrite pass ───────────────────────────────────────────────
    // Runs once per file-review skill after finalize, before Phase 2 synthesis.
    // Applies prose-quality rules (em dashes, label prefixes, fluff, hedging, etc.)
    // to all per-file reports in place. Synthesis skills then read prose-cleaned inputs.
    for (def skill in skills) {
        def s = skill  // capture
        if (summaryOnlySkills.contains(s)) { continue }
        def skillOutputDir    = "${outputDir}/${s}"
        def proseCheckpoint   = "${skillOutputDir}/logs/prose.done.txt"
        def proseStagingDir   = "${stagingDir}/${s}"

        executor.withOperationLogging(logger, "Prose rewrite ${pipelineName}/${s}", "PR-${prNumber}") {
            if (fileExists(proseCheckpoint)) {
                logger.info("[${pipelineName}] ${s} prose: checkpoint — skipping")
            } else {
                withEnv(envVars) {
                    def proseRc = sh(
                        script: "bash '${dispatchScriptPath}' '${prNumber}' '${proseStagingDir}' '${skillOutputDir}' --prose",
                        returnStatus: true
                    )
                    if (proseRc != 0) { error "Prose rewrite failed for skill ${s} (exit ${proseRc})" }
                }
                sh "touch '${proseCheckpoint}'"
            }
        }
    }

    // ── Phase 2: Synthesis skills ────────────────────────────────────────
    // Runs AFTER all per-file batches and finalize are complete.
    // At this point every per-file skill has written its index.md and per-file reports,
    // giving synthesis skills (summary, technical, blast-radius) a complete picture.
    if (synthesisBatches) {
        logger.info("[${pipelineName}] Phase 2 — running ${synthesisBatches.size()} synthesis batch(es)")
        def synthesisMap = [:]
        for (def batch in synthesisBatches) {
            def b              = batch  // capture for closure
            def skillOutputDir = "${outputDir}/${b.skill}"
            def checkpointFile = "${skillOutputDir}/logs/batch_${b.batch_num}.done.txt"
            def batchLabel     = "${b.skill} / batch_${b.batch_num}"

            synthesisMap[batchLabel] = {
                sh "mkdir -p '${skillOutputDir}/logs'"
                if (fileExists(checkpointFile)) {
                    logger.info("[${pipelineName}] ${batchLabel}: checkpoint — skipping")
                    return
                }

                def cacheDecision = evaluateBatchCache(
                    logger,
                    reviewCacheScriptPath,
                    cacheIndexFile,
                    b.skill as String,
                    b.staging_dir as String,
                    pipeConfig
                )
                if (cacheReady) {
                    if (cacheDecision.hit) {
                        logger.info("[${pipelineName}] ${batchLabel}: cache-hit for ${cacheDecision.clusterSha}")
                        if (cacheSkipsEnabled) {
                            def restored = restoreBatchCacheArtifacts(
                                logger,
                                reviewCacheScriptPath,
                                cacheDir,
                                outputDir,
                                cacheDecision.file?.toString() ?: '',
                                "[${pipelineName}] ${batchLabel}"
                            )
                            if (restored) {
                                logger.info("[${pipelineName}] ${batchLabel}: skipping synthesis due to cache-hit (artifacts restored)")
                                sh "touch '${checkpointFile}'"
                                return
                            }
                            logger.warn("[${pipelineName}] ${batchLabel}: cache-hit found but synthesis restore failed; continuing execution")
                        } else {
                            logger.info("[${pipelineName}] ${batchLabel}: cache skip disabled — continuing execution")
                        }
                    }
                }

                Throwable batchFailure = null
                try {
                    withEnv(envVars) {
                        int batchRc
                        if (b.skill == 'pr-review-summary') {
                            batchRc = sh(
                                script: "python3 '${summaryLoopScriptPath}' '${prNumber}' '${b.staging_dir}' '${skillOutputDir}'",
                                returnStatus: true
                            )
                        } else if (b.skill == 'pr-review-blast-radius') {
                            batchRc = sh(
                                script: "bash '${dispatchScriptPath}' '${prNumber}' '${b.staging_dir}' '${skillOutputDir}' --summary",
                                returnStatus: true
                            )
                        } else if (b.skill == 'pr-review-technical') {
                            batchRc = sh(
                                script: "python3 '${techReviewLoopScriptPath}' '${prNumber}' '${b.staging_dir}' '${skillOutputDir}'",
                                returnStatus: true
                            )
                        } else {
                            batchRc = sh(
                                script: "bash '${dispatchScriptPath}' '${prNumber}' '${b.staging_dir}' '${skillOutputDir}'",
                                returnStatus: true
                            )
                        }
                        if (batchRc != 0) { error "${batchLabel} failed (exit ${batchRc})" }
                    }
                } catch (Throwable err) {
                    batchFailure = err
                } finally {
                    def cacheContext = cacheDecision?.context
                    if (cacheContext?.valid && cacheRemoteUrl) {
                        def requestFilePath = "${skillOutputDir}/logs/cache-persist-${b.batch_num}.json"
                        writePendingCachePersistRequest(
                            requestFilePath,
                            batchLabel,
                            b.skill as String,
                            b.staging_dir as String,
                            cacheContext,
                            cacheBranch,
                            cacheRemoteUrl,
                            cacheTokenCredentialsId,
                            cacheUsername
                        )
                    }
                    sh "touch '${checkpointFile}'"
                }
                if (batchFailure) { throw batchFailure }
            }
        }
        parallel synthesisMap
    }

    if (cacheRemoteUrl) {
        flushPendingCachePersistRequests(
            logger,
            reviewCacheScriptPath,
            pushToCacheScriptPath,
            cacheDir,
            outputDir,
            projectId,
            cacheConfig ?: [:]
        )
    }

    // ── Prose rewrite pass (synthesis) ──────────────────────────────────
    // Runs after Phase 2. Applies prose-quality rules to synthesis outputs
    // (tech-review.md, summary.md, blast-radius.md) in each synthesis skill
    // output dir. Mirrors the per-file prose pass above.
    for (def batch in synthesisBatches) {
        def b              = batch  // capture
        def skillOutputDir = "${outputDir}/${b.skill}"
        def proseStagingDir   = "${stagingDir}/${b.skill}"
        def proseCheckpoint   = "${skillOutputDir}/logs/prose-synthesis.done.txt"

        executor.withOperationLogging(logger, "Prose rewrite (synthesis) ${pipelineName}/${b.skill}", "PR-${prNumber}") {
            if (fileExists(proseCheckpoint)) {
                logger.info("[${pipelineName}] ${b.skill} prose-synthesis: checkpoint — skipping")
            } else {
                withEnv(envVars) {
                    def proseRc = sh(
                        script: "bash '${dispatchScriptPath}' '${prNumber}' '${proseStagingDir}' '${skillOutputDir}' --prose",
                        returnStatus: true
                    )
                    if (proseRc != 0) { error "Prose rewrite (synthesis) failed for skill ${b.skill} (exit ${proseRc})" }
                }
                sh "touch '${proseCheckpoint}'"
            }
        }
    }

    // ── Technical deep review ────────────────────────────────────────────
    // Runs AFTER Phase 2 synthesis (tech-review.md must be finalised first).
    // Parses tech-review.md, extracts cited files, runs a focused deep-dive
    // on each file, and writes tech-review-deep.md.
    def techReviewFile      = "${outputDir}/tech-review.md"
    def deepCheckpointFile  = "${outputDir}/pr-review-technical/logs/deep.done.txt"
    def deepOutputDir       = "${outputDir}/pr-review-technical-deep"
    def workspaceDirDeep    = env.APP_REPO_DIR?.trim() ?: (env.WORKSPACE?.trim() ?: '.')

    if (fileExists(techReviewFile)) {
        executor.withOperationLogging(logger, "Tech deep review ${pipelineName}", "PR-${prNumber}") {
            if (fileExists(deepCheckpointFile)) {
                logger.info("[${pipelineName}] tech-review-deep: checkpoint \u2014 skipping")
            } else {
                sh "mkdir -p '${deepOutputDir}'"
                withEnv(envVars) {
                    def deepRc = sh(
                        script: "python3 '${techReviewDeepScriptPath}' '${prNumber}' '${techReviewFile}' '${workspaceDirDeep}' '${stagingDir}' '${deepOutputDir}'",
                        returnStatus: true
                    )
                    if (deepRc != 0) { error "tech-review-deep failed (exit ${deepRc})" }
                }
                sh "touch '${deepCheckpointFile}'"
            }
        }
    } else {
        logger.info("[${pipelineName}] tech-review-deep: tech-review.md not found \u2014 skipping")
    }

    // Copy staging manifest into output dir for archiving
    def manifest = "${stagingDir}/manifest.json"
    if (fileExists(manifest)) {
        sh "cp '${manifest}' '${outputDir}/review-manifest.json'"
    }}

// ---------------------------------------------------------------------------
// Archive and JUnit conversion
// ---------------------------------------------------------------------------

def dropPerFileReports(String outputBaseDir) {
    // Delete all per-file/ subdirectories — per-file reports have been converted to JUnit.
    // Synthesis outputs (summary.md, index.md, blast-radius.md) live at the skill root and are preserved.
    sh "find '${outputBaseDir}' -type d -name 'per-file' | xargs --no-run-if-empty rm -rf"
}

def archiveReport(String outputBaseDir) {
    archiveArtifacts(
        artifacts: "${outputBaseDir}/**",
        excludes: "${outputBaseDir}/**/logs/**",
        allowEmptyArchive: true
    )
}

def convertToJUnit(String outputDir) {
    def junitDir = "${outputDir}/junit"
    def reportToJunitPath = resolveScriptPath('review-to-junit.py')
    sh "python3 '${reportToJunitPath}' '${outputDir}' '${junitDir}'"
    junit testResults: "${junitDir}/*.xml", allowEmptyResults: true
}

// ---------------------------------------------------------------------------
// Public API — receives injected dependencies
// ---------------------------------------------------------------------------

// pipelines: map of pipeline-name → config (agent, skills, routing).
// Omit pipelines to run the built-in pr-review pipeline with default routing.
def review(logger, executor, String prNumber, String baseBranch, Map pipelines = [:], Map cache = [:], Map credentials = [:]) {
    if (!prNumber)   error "Missing required parameter: prNumber (CHANGE_ID not set — is this a PR build?)"
    if (!baseBranch) error "Missing required parameter: baseBranch (CHANGE_TARGET not set)"

    def outputDir          = "${REVIEW_OUTPUT_PREFIX}${prNumber}"
    def effectivePipelines = pipelines ?: ['pr-review': [:]]

    logger.info("Review configuration:")
    logger.info("  PR number:   ${prNumber}")
    logger.info("  Base branch: ${baseBranch}")
    logger.info("  Output dir:  ${outputDir}")
    logger.info("  Pipelines:   ${effectivePipelines.keySet().join(', ')}")

    effectivePipelines.each { pipelineName, pipeConfig ->
        executor.withOperationLogging(logger, "Copilot ${pipelineName}", "PR-${prNumber}") {
            runPipelineWithBatches(
                logger,
                executor,
                prNumber,
                baseBranch,
                outputDir,
                pipelineName,
                pipeConfig ?: [:],
                cache ?: [:],
                credentials ?: [:]
            )
        }
    }

    if (!fileExists(outputDir)) {
        logger.info("Nothing to review — no output produced, skipping archive and JUnit steps")
        return false
    }

    convertToJUnit(outputDir)
    dropPerFileReports(outputDir)
    archiveReport(outputDir)
    return true
}

return [review: this.&review]

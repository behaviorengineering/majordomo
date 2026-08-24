// .majordomo/stages/notify-build-status.groovy
// Notifies Bitbucket Server of build status updates via the commit-builds REST API.
// Receives logger and executor via dependency injection (IoC).

// Corporate CA cert URL — same source used by Dockerfiles to bootstrap SSL trust.
// Downloaded with --insecure (no prior trust) then used for subsequent HTTPS calls.
// Update this value when the certificate bundle is rotated.
// @NonCPS: private static final fields on loaded Scripts are not in the CPS binding and
// cannot be resolved via property lookup. A @NonCPS method returning a literal is the
// correct pattern for constants in Jenkins loaded scripts.
@NonCPS
private static String _getCorpCaCertUrl() {
    'https://packages.example.com/package-registry/example-generic/example/security/certificates/20241001/cacert-20241001.pem'
}

@NonCPS
def _parseBitbucketBaseUrl(String remoteUrl) {
    // SSH: ssh://git@host:port/PROJECT/repo.git or git@host:PROJECT/repo.git
    def sshMatcher = remoteUrl =~ /(?:ssh:\/\/)?git@([^:\/ ]+)/
    if (sshMatcher.find()) {
        return "https://${sshMatcher.group(1)}"
    }

    // HTTPS: https://host/path
    def httpsMatcher = remoteUrl =~ /https?:\/\/([^\/]+)/
    if (httpsMatcher.find()) {
        return "https://${httpsMatcher.group(1)}"
    }

    return null
}

// Extracts Bitbucket project key and repo slug from a git remote URL.
// Supports SSH (with/without port) and HTTPS remote formats.
// Returns a map with keys projectKey and repoSlug, or null values if parsing fails.
// Examples:
//   ssh://git@host:7999/PROJECT/repo.git  → [projectKey: 'PROJECT', repoSlug: 'repo']
//   git@host:~user/repo.git               → [projectKey: '~user', repoSlug: 'repo']
//   https://host/scm/PROJECT/repo.git     → [projectKey: 'PROJECT', repoSlug: 'repo']
@NonCPS
def _parseRemoteParts(String remoteUrl) {
    def url = remoteUrl.trim().replaceAll(/\.git$/, '').replaceAll(/\/$/, '')
    def m = url =~ /[:\/]([^\/:]*)[\/]([^\/]+)$/
    if (m.find()) {
        return [projectKey: m.group(1), repoSlug: m.group(2)]
    }
    return [projectKey: null, repoSlug: null]
}

@NonCPS
def _toBlueOceanUrl(String classicUrl) {
    if (!classicUrl?.trim()) {
        return classicUrl
    }

    def baseMatcher = classicUrl =~ /^(https?:\/\/[^\/]+)/
    if (!baseMatcher.find()) {
        return classicUrl
    }
    def base = baseMatcher.group(1)

    def segments = []
    def segMatcher = classicUrl =~ /\/job\/([^\/]+)/
    while (segMatcher.find()) {
        segments << segMatcher.group(1)
    }
    if (!segments) {
        return classicUrl
    }

    def numMatcher = classicUrl =~ /\/(\d+)\/?$/
    if (!numMatcher.find()) {
        return classicUrl
    }
    def buildNum = numMatcher.group(1)

    def encodedPath = segments.join('%2F')
    def leafJob = segments[-1]
    return "${base}/blue/organizations/jenkins/${encodedPath}/detail/${leafJob}/${buildNum}/pipeline"
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

def notifyBuildStatus(logger, executor, String commitSha, String state, String description, String bitbucketTokenCredentialsId, String pythonImage) {
    if (!commitSha?.trim()) {
        logger.warn('No commit SHA available; skipping build-status notification')
        return
    }

    if (!pythonImage?.trim()) {
        logger.warn('No runtime image available; skipping build-status notification')
        return
    }

    executor.withOperationLogging(logger, 'Notify Build Status', state) {
        def baseUrl    = env.BITBUCKET_BASE_URL?.trim()
        def projectKey = env.BB_PROJECT_KEY?.trim()
        def repoSlug   = env.BB_REPO_SLUG?.trim()

        if (!baseUrl || !projectKey || !repoSlug) {
            def remoteUrl = sh(script: 'git remote get-url origin', returnStdout: true).trim()
            logger.info("Remote URL: ${remoteUrl}")
            if (!baseUrl)    baseUrl    = _parseBitbucketBaseUrl(remoteUrl)
            def parts = _parseRemoteParts(remoteUrl)
            if (!projectKey) projectKey = parts.projectKey
            if (!repoSlug)   repoSlug   = parts.repoSlug
        }

        if (!baseUrl || !projectKey || !repoSlug) {
            logger.warn('Could not resolve Bitbucket base URL / project key / repo slug — skipping')
            return
        }

        def buildRef = env.CHANGE_BRANCH?.trim() ? "refs/heads/${env.CHANGE_BRANCH}" :
                       (env.PUSH_BRANCH?.trim()  ? "refs/heads/${env.PUSH_BRANCH}"   : '')

        logger.info("Bitbucket URL: ${baseUrl}")
        logger.info("Project:       ${projectKey}")
        logger.info("Repo:          ${repoSlug}")
        logger.info("Commit:        ${commitSha}")
        logger.info("State:         ${state}")
        logger.info("Ref:           ${buildRef ?: '(not set)'}")

        withCredentials([string(credentialsId: bitbucketTokenCredentialsId, variable: 'BITBUCKET_TOKEN')]) {
            def blueOceanUrl = _toBlueOceanUrl(env.BUILD_URL ?: '')
            logger.info("Build URL:     ${blueOceanUrl}")

            // Capture @NonCPS constant into a local variable — CPS GString interpolation
            // cannot resolve static fields on loaded Script classes directly.
            def corpCaCertUrl = _getCorpCaCertUrl()
            // resolveScriptPath returns an absolute host path; docker run mounts the
            // workspace at /workspace so the path must be container-relative.
            def notifyScriptHostPath = resolveScriptPath('notify-build-status.py')
            def notifyScriptPath = '/workspace' + notifyScriptHostPath.substring(env.WORKSPACE.length())

            withEnv([
                "BB_BASE_URL=${baseUrl}",
                "BB_PROJECT_KEY=${projectKey}",
                "BB_REPO_SLUG=${repoSlug}",
                "BB_BUILD_KEY=${env.JOB_NAME}",
                "BB_BUILD_NAME=${env.JOB_NAME} #${env.BUILD_NUMBER}",
                "BB_BUILD_DESCRIPTION=${description}",
                "BB_BUILD_NUMBER=${env.BUILD_NUMBER ?: ''}",
                "BB_BUILD_REF=${buildRef}",
                "BUILD_URL=${blueOceanUrl}",
            ]) {
                sh """
                    # Bootstrap corporate CA cert — same approach as Dockerfiles (curl --insecure).
                    # Non-fatal: if package registry is unreachable the cert won't be mounted and
                    # Python falls back to the system trust store (which will likely fail with SSL).
                    curl -fsSL --insecure \\
                        -o /tmp/corp-ca-notify.pem \\
                        '${corpCaCertUrl}' \\
                        2>/dev/null || true
                    docker run --rm \\
                        -v "\$(pwd):/workspace" -w /workspace \\
                        \$([ -f /tmp/corp-ca-notify.pem ] && echo "-v /tmp/corp-ca-notify.pem:/tmp/corp-ca.pem:ro -e REQUESTS_CA_BUNDLE=/tmp/corp-ca.pem -e SSL_CERT_FILE=/tmp/corp-ca.pem") \\
                        -e BB_BASE_URL -e BB_PROJECT_KEY -e BB_REPO_SLUG \\
                        -e BB_BUILD_KEY -e BB_BUILD_NAME -e BB_BUILD_DESCRIPTION \\
                        -e BB_BUILD_NUMBER -e BB_BUILD_REF \\
                        -e BITBUCKET_TOKEN -e BUILD_URL \\
                        ${pythonImage} \\
                        python3 '${notifyScriptPath}' '${commitSha}' '${state}'
                """
            }
        }
    }
}

return [notifyBuildStatus: this.&notifyBuildStatus]

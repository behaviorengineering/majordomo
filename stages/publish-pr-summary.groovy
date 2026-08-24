// .majordomo/stages/publish-pr-summary.groovy
// Publishes the Copilot PR summary.md to Bitbucket Server via REST API.
// In central mode, prefers the app repo identity injected into env by the pipeline.
// Falls back to deriving Bitbucket coordinates from `git remote get-url origin`.
// Receives logger and executor via dependency injection (IoC).

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

def parseBitbucketOrigin(String remoteUrl) {
    // Supports SSH:   ssh://git@bitbucket.example.com:7999/PROJ/repo.git
    //                 git@bitbucket.example.com:PROJ/repo.git
    // Supports HTTPS: https://bitbucket.example.com/scm/PROJ/repo.git
    //                 https://bitbucket.example.com/projects/PROJ/repos/repo    //
    // NOTE: All origins are normalised to https:// because Bitbucket Server in
    // this environment is always served on standard HTTPS (port 443). SSH remotes
    // carry no scheme, so https:// is the only safe default. If your deployment
    // uses a non-standard port or HTTP, override bitbucketBaseUrl in
    // .majordomo-config.groovy instead of changing this parser.
    // Accepted portability limitation — intentional.
    def result = [baseUrl: null, project: null, repo: null]

    // SSH: ssh://git@host:port/PROJECT/repo.git  or  git@host:PROJECT/repo.git
    def sshMatch = remoteUrl =~ /(?:ssh:\/\/)?git@([^:\/]+)(?::[0-9]+)?[:\/]([^\/]+)\/([^\/]+?)(?:\.git)?$/
    if (sshMatch) {
        result.baseUrl = "https://${sshMatch[0][1]}"
        result.project = sshMatch[0][2].toUpperCase()
        result.repo    = sshMatch[0][3]
        return result
    }

    // HTTPS with /scm/ prefix: https://host/scm/PROJECT/repo.git
    def httpsScmMatch = remoteUrl =~ /https?:\/\/([^\/]+)\/scm\/([^\/]+)\/([^\/]+?)(?:\.git)?$/
    if (httpsScmMatch) {
        result.baseUrl = "https://${httpsScmMatch[0][1]}"
        result.project = httpsScmMatch[0][2].toUpperCase()
        result.repo    = httpsScmMatch[0][3]
        return result
    }

    // HTTPS with /projects/ prefix: https://host/projects/PROJECT/repos/repo
    def httpsProjectsMatch = remoteUrl =~ /https?:\/\/([^\/]+)\/projects\/([^\/]+)\/repos\/([^\/]+?)(?:\.git)?$/
    if (httpsProjectsMatch) {
        result.baseUrl = "https://${httpsProjectsMatch[0][1]}"
        result.project = httpsProjectsMatch[0][2].toUpperCase()
        result.repo    = httpsProjectsMatch[0][3]
        return result
    }

    return result
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

// ---------------------------------------------------------------------------
// Public API — receives injected dependencies
// ---------------------------------------------------------------------------

// Publishes the pr-review-summary/summary.md from the review output dir to Bitbucket.
// prNumber:          Bitbucket PR ID
// outputBaseDir:     review output root (e.g. copilot-review-pr-42)
// pipelineName:         pipeline name whose summary to publish (e.g. pr-review)
// mode:                 'auto', 'comment', or 'description'
// bitbucketTokenCredId: Jenkins credential ID for the Bitbucket access token (secret text)
// copilotImage:         full image ref for the copilot-cli container (e.g. registry/copilot-cli:abc123)
// registryUrl:          Docker registry URL for pulling copilotImage (e.g. https://my-registry.example.com)
// registryCredId:       Jenkins credential ID for the Docker registry
def publishSummary(logger, executor, String prNumber, String outputBaseDir, String pipelineName, String mode, String bitbucketTokenCredId, String copilotImage, String registryUrl, String registryCredId) {
    def summaryFile         = "${outputBaseDir}/${pipelineName}/summary.md"
    def summaryHtmlFile     = "${outputBaseDir}/${pipelineName}/summary.html"
    def techReviewHtmlFile  = "${outputBaseDir}/${pipelineName}/tech-review.html"
    def techDeepHtmlFile    = "${outputBaseDir}/${pipelineName}/pr-review-technical-deep/tech-review-deep.html"

    // Construct the Jenkins artifact URLs so the script can use them in messages.
    // BUILD_URL is always set by Jenkins (e.g. https://jenkins.example.com/job/MyJob/42/).
    def artifactUrl            = "${env.BUILD_URL}artifact/${summaryFile}"
    def summaryHtmlArtifactUrl = "${env.BUILD_URL}artifact/${summaryHtmlFile}"
    def techReviewArtifactUrl  = "${env.BUILD_URL}artifact/${techReviewHtmlFile}"
    def techDeepArtifactUrl    = "${env.BUILD_URL}artifact/${techDeepHtmlFile}"

    executor.withOperationLogging(logger, 'Publish PR Summary', "PR-${prNumber}") {
        def continuousRunsEnabled = (
            (params.ENABLE_CONTINUOUS_RUNS as Boolean)
            || ((env.COPILOT_ENABLE_CONTINUOUS_RUNS ?: 'false').toBoolean())
        )
        def effectiveMode = mode
        if (continuousRunsEnabled && mode == 'auto') {
            // In continuous mode, force description updates so later runs mutate one canonical surface.
            effectiveMode = 'description'
        }

        if (!fileExists(summaryFile)) {
            logger.warn("summary.md not found at ${summaryFile} — skipping publish")
            return
        }

        // Discover any SA tool output files archived during the Static Analysis stage.
        // SA findings are restored into .sa/ via unstash in the PR Review stage and
        // persist in the workspace through to here.
        def saArtifactUrls = []
        if (fileExists('.sa')) {
            findFiles(glob: '.sa/*.txt').each { f ->
                def slug = f.name.replaceAll(/\.txt$/, '')
                saArtifactUrls << [slug: slug, url: "${env.BUILD_URL}artifact/${f.path}"]
            }
        }

        logger.info("Summary source:       ${summaryFile}")
        logger.info("Publish mode:         ${mode}")
        logger.info("Effective mode:       ${effectiveMode}")
        logger.info("Continuous runs:      ${continuousRunsEnabled}")
        logger.info("Artifact URL:         ${artifactUrl}")
        logger.info("Summary HTML URL:     ${summaryHtmlArtifactUrl}")
        logger.info("Tech review URL:      ${techReviewArtifactUrl}")
        logger.info("Tech deep URL:        ${techDeepArtifactUrl}")
        logger.info("SA tools found:       ${saArtifactUrls.collect { it.slug }.join(', ') ?: 'none'}")

        // Central mode sets BB_PROJECT_KEY / BB_REPO_SLUG during Checkout App Repo.
        // When unavailable, fall back to parsing the appropriate git remote.
        def appRepoDir = env.APP_REPO_DIR?.trim()
        def remoteCmd = appRepoDir ? "git -C '${appRepoDir}' remote get-url origin" : 'git remote get-url origin'
        def remoteUrl = sh(script: remoteCmd, returnStdout: true).trim()
        logger.info("Remote URL: ${remoteUrl}")

        def parsed = parseBitbucketOrigin(remoteUrl)
        parsed.project = env.BB_PROJECT_KEY?.trim() ?: parsed.project
        parsed.repo = env.BB_REPO_SLUG?.trim() ?: parsed.repo
        if (!parsed.baseUrl || !parsed.project || !parsed.repo) {
            error "Could not resolve Bitbucket base URL, project, or repo from remote/env: ${remoteUrl}"
        }

        logger.info("Bitbucket URL: ${parsed.baseUrl}")
        logger.info("Project:       ${parsed.project}")
        logger.info("Repo:          ${parsed.repo}")
        def publishScriptPath = resolveScriptPath('publish-pr-summary.py')

        withCredentials([string(credentialsId: bitbucketTokenCredId, variable: 'BITBUCKET_TOKEN')]) {
            // Only pass HTML artifact URLs when the files were actually produced.
            // An always-present URL pointing at a missing artifact creates a dead link.
            def summaryHtmlEnv = fileExists(summaryHtmlFile)
                ? ["SUMMARY_HTML_ARTIFACT_URL=${summaryHtmlArtifactUrl}"]
                : []
            def techEnv = fileExists(techReviewHtmlFile)
                ? ["TECH_REVIEW_ARTIFACT_URL=${techReviewArtifactUrl}"]
                : []
            def techDeepEnv = fileExists(techDeepHtmlFile)
                ? ["TECH_DEEP_ARTIFACT_URL=${techDeepArtifactUrl}"]
                : []
            def saEnv = saArtifactUrls
                ? ["SA_ARTIFACT_URLS=${groovy.json.JsonOutput.toJson(saArtifactUrls)}"]
                : []
            withEnv([
                "BITBUCKET_URL=${parsed.baseUrl}",
                "BB_PROJECT=${parsed.project}",
                "BB_REPO=${parsed.repo}",
                "SUMMARY_ARTIFACT_URL=${artifactUrl}",
            ] + summaryHtmlEnv + techEnv + techDeepEnv + saEnv) {
                docker.withRegistry(registryUrl, registryCredId) {
                    docker.image(copilotImage).inside('-u root -e HOME=/root') {
                        def rc = sh(
                            script: "python3 '${publishScriptPath}' '${prNumber}' '${summaryFile}' '${effectiveMode}'",
                            returnStatus: true
                        )
                        if (rc != 0) { error "publish-pr-summary.py failed (exit ${rc})" }
                    }
                }
            }
        }
    }
}

return [publishSummary: this.&publishSummary]

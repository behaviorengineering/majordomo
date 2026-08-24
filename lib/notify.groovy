// .majordomo/lib/notify.groovy
// Shared helpers for Bitbucket build status notifications.

// Resolve commit SHA from git, with env fallback for post-state execution paths.
def resolveGitCommitSha() {
    // Prefer the webhook-injected PR commit (params.CHANGE_COMMIT) — set by GWT from
    // $.pullRequest.fromRef.latestCommit. This is the correct PR source branch HEAD.
    // git rev-parse HEAD reflects the pipeline default branch checkout (checkout scm),
    // not the PR branch, so it is only used as a last resort.
    def paramSha = params?.CHANGE_COMMIT?.trim()
    if (paramSha) {
        return paramSha
    }
    def rc = sh(script: "git rev-parse HEAD > .git-pr-sha 2>/dev/null", returnStatus: true)
    if (rc == 0 && fileExists('.git-pr-sha')) {
        def sha = readFile('.git-pr-sha').trim()
        sh "rm -f .git-pr-sha"
        if (sha) {
            return sha
        }
    }
    return env.GIT_COMMIT ?: ''
}

def notifyBitbucketBuildStatus(logger, executor, String state, String description, String bitbucketTokenCredentialsId, String pythonImage, String commitSha = '') {
    if (!bitbucketTokenCredentialsId?.trim()) {
        logger.warn('No Bitbucket token credential configured; skipping build-status notification')
        return
    }

    if (!pythonImage?.trim()) {
        logger.warn('No runtime image available; skipping build-status notification')
        return
    }

    def resolvedCommitSha = commitSha?.trim() ?: resolveGitCommitSha()
    if (!resolvedCommitSha?.trim()) {
        logger.warn('No commit SHA available; skipping build-status notification')
        return
    }

    def notifyBuildStatusPath = fileExists('.majordomo/stages/notify-build-status.groovy')
        ? '.majordomo/stages/notify-build-status.groovy'
        : 'stages/notify-build-status.groovy'
    def module = load(notifyBuildStatusPath)
    module.notifyBuildStatus(
        logger,
        executor,
        resolvedCommitSha,
        state,
        description,
        bitbucketTokenCredentialsId,
        pythonImage
    )
}

return this

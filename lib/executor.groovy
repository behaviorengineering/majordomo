// .majordomo/lib/executor.groovy
// Execution wrappers — injected into stages via dependency injection (IoC)

def withOperationLogging(logger, String operation, String context, Closure closure) {
    def success = false
    logger.header("<<<< Starting ${operation} for ${context}")
    try {
        def result = closure()
        success = true
        return result
    } finally {
        def status = success ? 'SUCCESS' : 'FAILED'
        logger.header(">>>> ${operation} completed for ${context}: ${status}")
    }
}

// Whitelist of keywords that indicate transient, recoverable failures.
// Anything NOT matching is treated as a hard failure and not retried.
@groovy.transform.Field
final List<String> RECOVERABLE_PATTERNS = [
    'timeout', 'timed out',
    'connection refused', 'connection reset', 'connection error',
    'socket', 'network',
    'rate limit', 'too many requests',
    '502', '503', '504',
    'pull access denied', 'toomanyrequests',
    'registry',
]

// Default recoverability check — matches exception message against RECOVERABLE_PATTERNS.
// A null/empty message is treated as non-recoverable so hard build errors are never silently retried.
boolean defaultIsRecoverable(Exception e) {
    def msg = e?.message?.toLowerCase()
    if (!msg) return false
    return RECOVERABLE_PATTERNS.any { msg.contains(it) }
}

def retry(logger, int maxAttempts = 3, int initialDelaySeconds = 5, Closure isRecoverable = null, Closure closure) {
    def recoverableFn = isRecoverable ?: this.&defaultIsRecoverable
    def attempt = 1
    def lastException = null

    while (attempt <= maxAttempts) {
        try {
            if (attempt > 1) {
                logger.info("Retry attempt ${attempt}/${maxAttempts}")
            }
            return closure()
        } catch (Exception e) {
            lastException = e

            if (!recoverableFn(e)) {
                logger.error("Non-recoverable failure — not retrying: ${e.message ?: '(no message)'}")
                throw e
            }

            if (attempt == maxAttempts) {
                logger.error("All ${maxAttempts} attempts failed: ${e.message}")
                throw e
            }

            def delaySeconds = initialDelaySeconds * Math.pow(2d, (attempt - 1) as double)
            logger.warn("Attempt ${attempt} failed (recoverable): ${e.message}")
            logger.info("Retrying in ${delaySeconds}s (exponential backoff)...")

            sleep(time: delaySeconds as long, unit: 'SECONDS')
            attempt++
        }
    }

    throw lastException
}

// Remove a root-owned directory (or glob) from the workspace by running rm -rf inside a
// Docker container as root, bypassing the NFS root_squash ownership restriction.
// The label agent must have Docker available.
def cleanRootOwnedDir(String image, String dir) {
    sh "docker run --rm -v \"\$(pwd):/workspace\" ${image} sh -c 'rm -rf /workspace/${dir}'"
}

// Fix permissions on all workspace files left root-owned by Docker stages running as root.
// Runs chmod -R a+rw inside the container so the host Jenkins user (subject to NFS root_squash)
// can read, write, and delete those files on the next build — most critically for git checkout.
// Must be called in post { always } after any stage that uses Docker with '-u root'.
def fixWorkspacePermissions(String image) {
    sh "docker run --rm -v \"\$(pwd):/workspace\" ${image} sh -c 'chmod -R a+rw /workspace'"
}

// Best-effort workspace permission fix that never fails the stage.
// Useful for pre-checkout cleanup paths where image availability is optional.
def chmodWorkspaceBestEffort(logger, String image) {
    if (!image?.trim()) {
        logger.info('Pre-checkout workspace cleanup skipped — cleanup image is empty.')
        return
    }

    def rc = sh(
        script: "docker run --rm -v \"\$(pwd):/workspace\" ${image} sh -c 'chmod -R a+rw /workspace 2>/dev/null || true'",
        returnStatus: true
    )
    if (rc != 0) {
        logger.info("Pre-checkout workspace cleanup skipped — ${image} unavailable (exit ${rc}).")
    }
}

// Optional stash restore for stages that may be skipped by when-conditions.
def unstashIfPresent(String stashName) {
    try {
        unstash stashName
    } catch (ignored) {
        // Optional stash may be absent when upstream stage is skipped.
    }
}

return this

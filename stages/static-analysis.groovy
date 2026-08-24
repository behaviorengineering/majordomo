// .majordomo/stages/static-analysis.groovy
// Runs configured SA tools against changed files in the PR, writing output to
// .sa/<tool-slug>.txt in the workspace for git-diff-prep.py to embed in staging slugs.
// Receives logger, executor via dependency injection (IoC).
//
// saTools:  list of staticAnalysis config entries (from .majordomo-config.groovy)
// imageMap: map of toolSlug → fullImage produced by sa-image.groovy
// baseBranch: git base branch (used to compute changed files list)

def changedFiles(String baseBranch) {
    // APP_REPO_DIR: set only by the central pipeline — git diff runs from the checked-out app repo.
    // Unset (per-repo submodule mode): cwd is the workspace root — behaviour unchanged.
    def appRepoDir = env.APP_REPO_DIR?.trim()
    def gitCmd = appRepoDir ?
        "git -C '${appRepoDir}' diff --name-only --diff-filter=d 'origin/${baseBranch}...HEAD'" :
        "git diff --name-only --diff-filter=d 'origin/${baseBranch}...HEAD'"
    def output = sh(
        script: gitCmd,
        returnStdout: true
    ).trim()
    return output ? output.split('\n') as List : []
}

def filesForGlob(List allFiles, String glob) {
    // Convert a glob pattern to a match against each file path.
    // Supports ** prefix wildcard (e.g. **/*.py) and exact suffix matching.
    // **/ must be protected before the single-* replacement runs, otherwise the
    // .* inserted for **/ gets its * replaced by [^/]*, breaking cross-dir matching.
    def pattern = glob
        .replace('.', '\\.')          // escape dots
        .replace('**/', '__DS__')     // protect double-star-slash from single-* replacement
        .replace('*', '[^/]*')        // single * → match within path segment only
        .replace('__DS__', '.*')      // double-star-slash → match any path prefix (incl. /)
    return allFiles.findAll { f -> f ==~ /(?i)${pattern}/ }
}

// Inline version of sa-image.groovy#toolSlug — avoids calling load() inside a closure.
// Must stay in sync with sa-image.groovy.
def toolSlugLocal(String dockerfilePath) {
    return dockerfilePath.tokenize('/')[-1].replace('.Dockerfile', '').toLowerCase()
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

// Public API — receives injected dependencies
def run(logger, executor, List saTools, Map imageMap, String baseBranch) {
    def workspace = env.WORKSPACE
    def runSaToolScriptPath = resolveScriptPath('run-sa-tool.sh')

    def allChanged = []
    executor.withOperationLogging(logger, 'Static Analysis', 'changed-files') {
        allChanged = changedFiles(baseBranch)
        logger.info("Changed files: ${allChanged.size()}")
    }

    if (!allChanged) {
        logger.info("No changed files — skipping static analysis")
        return
    }

    for (def tool in saTools) {
        def t    = tool  // capture
        def slug = t.dockerfile
            ? toolSlugLocal(t.dockerfile as String)
            : (t.slug ?: (t.image as String).tokenize('/')[-1].tokenize(':')[0])
        def image   = imageMap[slug]
        def command = t.command as String
        def glob    = t.glob   as String ?: '**'

        if (!image) {
            logger.warn("SA tool '${slug}': no image resolved — skipping")
            continue
        }

        def matchedFiles = filesForGlob(allChanged, glob)
        if (!matchedFiles) {
            logger.info("SA tool '${slug}': no changed files match glob '${glob}' — skipping")
            continue
        }

        executor.withOperationLogging(logger, "SA: ${slug}", "${matchedFiles.size()} file(s)") {
            def fileArgs   = matchedFiles.collect { "'${it}'" }.join(' ')
            // APP_REPO_DIR: central mode — SA tool mounts and operates on the app repo checkout.
            // Unset: workspace root is the app repo — behaviour unchanged.
            def appRepoDir = env.APP_REPO_DIR?.trim() ?: workspace
            sh "bash '${runSaToolScriptPath}' '${slug}' '${image}' '${command}' '${appRepoDir}' ${fileArgs}"
        }
    }

    // Archive .sa/ output as Jenkins artifacts so findings are inspectable
    // in the build UI independently of whether PR Review runs.
    // Stash also keeps them available for the PR Review stage on any node.
    if (fileExists('.sa')) {
        archiveArtifacts artifacts: '.sa/**', allowEmptyArchive: true
        stash name: 'sa-findings', includes: '.sa/**', allowEmpty: true
        logger.info('SA findings archived and stashed for PR Review stage')
    }
}

return [run: this.&run]

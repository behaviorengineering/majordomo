// .majordomo/stages/convert-reports.groovy
// Converts Markdown review reports to self-contained HTML files and archives them.
// Called inside docker.image().inside() so Python 3 and md-to-html.py are available.
// Receives logger and executor via dependency injection (IoC).

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// Markdown files produced by the review pipelines that should be converted.
// Keys are the relative path inside <outputDir>; values are the output filename.
def reportFiles(String outputDir) {
    return [
        "${outputDir}/summary.md":                                     "${outputDir}/summary.html",
        "${outputDir}/tech-review.md":                                  "${outputDir}/tech-review.html",
        "${outputDir}/pr-review-technical-deep/tech-review-deep.md":   "${outputDir}/pr-review-technical-deep/tech-review-deep.html",
    ]
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

def convertOne(logger, String mdFile, String htmlFile) {
    def converterPath = resolveScriptPath('md-to-html.py')
    def rc = sh(
        script: "python3 '${converterPath}' '${mdFile}' '${htmlFile}'",
        returnStatus: true
    )
    if (rc != 0) {
        logger.warn("md-to-html.py failed for ${mdFile} (exit ${rc}) — skipping")
        return false
    }
    logger.info("Converted: ${mdFile} → ${htmlFile}")
    return true
}

// ---------------------------------------------------------------------------
// Public API — receives injected dependencies
// ---------------------------------------------------------------------------

// Converts all known Markdown reports under outputDir to HTML and archives them.
// outputDir: review output directory for the pipeline (e.g. copilot-review-pr-42/pr-review)
def convert(logger, executor, String outputDir) {
    executor.withOperationLogging(logger, 'Convert Reports to HTML', outputDir) {
        def reports = reportFiles(outputDir)
        def converted = []

        reports.each { mdFile, htmlFile ->
            if (!fileExists(mdFile)) {
                logger.info("${mdFile} not found — skipping")
                return
            }
            if (convertOne(logger, mdFile, htmlFile)) {
                converted << htmlFile
            }
        }

        if (!converted) {
            logger.warn("No reports were converted — nothing to archive")
            return
        }

        def pattern = converted.collect { it }.join(',')
        logger.info("Archiving HTML reports: ${pattern}")
        archiveArtifacts(artifacts: pattern, allowEmptyArchive: false)
    }
}

return [convert: this.&convert]

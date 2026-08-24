// .majordomo/stages/copilot-image.groovy
// Ensures the Copilot CLI Docker image exists in the registry, building it if not cached.
// Receives logger, executor, and tags module via dependency injection (IoC)

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

def buildImage(String registry, String imageName, String imageTag, String dockerfilePath) {
    def buildScriptPath = resolveScriptPath('build-copilot-image.sh')
    sh "bash '${buildScriptPath}' '${registry}' '${imageName}' '${imageTag}' '${dockerfilePath}'"
}

// Public API — receives injected dependencies
// tags: calculate-image-tags module (injected by dispatcher)
// Credentials (REGISTRY_USR, REGISTRY_PSW, SALARY_ID, JFROG_TOKEN) are in scope
// via withCredentials at the dispatcher — used by build-copilot-image.sh at runtime.
def ensure(logger, executor, tags, String pushDomain, String imageName, String dockerfile) {
    def imageTag  = executor.retry(logger, 3, 5) {
        tags.getDockerfileHash(dockerfile)
    }
    def fullImage = "${pushDomain}/${imageName}:${imageTag}"
    logger.info("Image tag (Dockerfile SHA): ${imageTag}")

    def exists = false
    executor.retry(logger, 3, 10) {
        exists = tags.imageExists(fullImage)
    }

    if (exists) {
        logger.info("Cached image found — skipping build: ${fullImage}")
    } else {
        logger.info("Image not cached — building: ${fullImage}")
        try {
            executor.retry(logger, 3, 20) {
                buildImage(pushDomain, imageName, imageTag, dockerfile)
            }
        } catch (Exception buildErr) {
            // Artifactory may block tag overwrite when a concurrent build pushed the same tag.
            // Check once more — if the image now exists, treat it as a cache hit and continue.
            logger.warn("Build/push failed — checking if tag was pushed by a concurrent build: ${fullImage}")
            def pushedConcurrently = false
            executor.retry(logger, 3, 10) {
                pushedConcurrently = tags.imageExists(fullImage)
            }
            if (pushedConcurrently) {
                logger.info("Image already exists in registry (pushed by concurrent build) — continuing: ${fullImage}")
            } else {
                throw buildErr
            }
        }
    }

    return [imageTag: imageTag, fullImage: fullImage]
}

return [ensure: this.&ensure]

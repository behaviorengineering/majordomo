// .majordomo/stages/sa-image.groovy
// Ensures SA tool Docker images exist in the registry, building them if not cached.
// Mirrors the copilot-image.groovy pattern: Dockerfile SHA → tag → check → build if missing.
// Receives logger, executor, and tags module via dependency injection (IoC).
//
// saTools: list of staticAnalysis config entries that have a 'dockerfile' key.
// Returns a map of toolSlug → fullImage for use by static-analysis.groovy.
// BYO entries (image: key only, no dockerfile:) are passed through unchanged.

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

def toolSlug(String dockerfilePath) {
    // Derive a short slug from the Dockerfile filename: sa-tools/ruff.Dockerfile → ruff
    return dockerfilePath.tokenize('/')[-1].replace('.Dockerfile', '').toLowerCase()
}

def buildImage(String registry, String imageName, String imageTag, String dockerfilePath) {
    def buildScriptPath = resolveScriptPath('build-copilot-image.sh')
    sh "bash '${buildScriptPath}' '${registry}' '${imageName}' '${imageTag}' '${dockerfilePath}'"
}

// Public API — receives injected dependencies
// credentials (REGISTRY_USR, REGISTRY_PSW, REGISTRY_USER, REGISTRY_TOKEN) must be in scope
// via withCredentials at the dispatcher before calling this method.
def ensure(logger, executor, tags, String pushDomain, List saTools) {
    def imageMap = [:]   // toolSlug → fullImage

    for (def tool in saTools) {
        def t = tool  // capture for closure

        if (t.image && !t.dockerfile) {
            // BYO image — no build needed, pass through as-is
            def slug = t.slug ?: t.image.tokenize('/')[-1].tokenize(':')[0]
            logger.info("SA tool '${slug}': BYO image — ${t.image}")
            imageMap[slug] = t.image
            continue
        }

        def dockerfile = t.dockerfile as String
        def slug       = toolSlug(dockerfile)
        def imageName  = "sa-${slug}"

        // Include shared setup and username-sanitizer scripts in the hash so edits
        // to either also bump this image.
        def dockerfileDir   = dockerfile.contains('/') ? dockerfile.substring(0, dockerfile.lastIndexOf('/') + 1) : ''
        def sharedScript    = "${dockerfileDir}scripts/setup-corp-apt.sh"
        def userScript      = "${dockerfileDir}scripts/registry-user.sh"
        def hashInputs      = [sharedScript, userScript]
        if (slug == 'mypy') {
            hashInputs << "${dockerfileDir}mypy/mypy-entrypoint.sh"
            hashInputs << "${dockerfileDir}mypy/mypy-default.ini"
        }
        def imageTag = executor.retry(logger, 3, 5) {
            tags.getDockerfileHash(dockerfile, hashInputs)
        }
        def fullImage = "${pushDomain}/${imageName}:${imageTag}"
        logger.info("SA tool '${slug}': tag ${imageTag}")

        def exists = false
        executor.retry(logger, 3, 10) {
            exists = tags.imageExists(fullImage)
        }

        if (exists) {
            logger.info("SA tool '${slug}': cached — skipping build: ${fullImage}")
        } else {
            logger.info("SA tool '${slug}': not cached — building: ${fullImage}")
            try {
                executor.retry(logger, 3, 20) {
                    buildImage(pushDomain, imageName, imageTag, dockerfile)
                }
            } catch (Exception buildErr) {
                // package registry may block tag overwrite when a concurrent build pushed the same tag.
                // Check once more — if the image now exists, treat it as a cache hit and continue.
                logger.warn("SA tool '${slug}': build/push failed — checking if tag was pushed by a concurrent build: ${fullImage}")
                def pushedConcurrently = false
                executor.retry(logger, 3, 10) {
                    pushedConcurrently = tags.imageExists(fullImage)
                }
                if (pushedConcurrently) {
                    logger.info("SA tool '${slug}': image already exists in registry (pushed by concurrent build) — continuing: ${fullImage}")
                } else {
                    throw buildErr
                }
            }
        }

        imageMap[slug] = fullImage
    }

    return imageMap
}

return [ensure: this.&ensure, toolSlug: this.&toolSlug]

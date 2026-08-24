// .jenkins-local/pipelines/SATools.CI.Jenkinsfile
// Internal CI for this repo — validates and pre-warms the canonical SA tool Dockerfiles.
// Triggered by changes to dockerfiles/sa-tools/** on any branch.
// Builds and pushes all four SA tool images (eslint, hadolint, ruff, shellcheck) in parallel.
// Each image is tagged by the SHA of its Dockerfile — already-cached images are skipped.
//
// Jenkins job configuration:
//   Script Path:  .jenkins-local/pipelines/SATools.CI.Jenkinsfile
//   Path Filter:  dockerfiles/sa-tools/**
//
// Required credentials:
//   example-docker-creds        - Username/password for package registry Docker registry
//   example-registry-token   - REGISTRY_USER + REGISTRY_TOKEN for BuildKit secrets (apt/npm)

def DOCKER_CONFIG = [
    registry: [
        pullDomain:    'example-docker-snapshot-dependencies.packages.example.com',
        pullUrl:       'https://example-docker-snapshot-dependencies.packages.example.com',
        pushDomain:    'example-docker-snapshot-local.packages.example.com',
        credentialsId: 'example-docker-creds',
    ],
    jenkinsAgent: [
        label:      'linux-shared-agent',
        dockerArgs: '-u root -e HOME=/root',
    ],
    credentials: [
        package-registryCredentialsId: 'example-registry-token',
    ],
]

// SA tool Dockerfiles in this repository — one entry per tool.
def SA_TOOLS = [
    [slug: 'eslint',     dockerfile: 'dockerfiles/sa-tools/eslint.Dockerfile'],
    [slug: 'hadolint',   dockerfile: 'dockerfiles/sa-tools/hadolint.Dockerfile'],
    [slug: 'ruff',       dockerfile: 'dockerfiles/sa-tools/ruff.Dockerfile'],
    [slug: 'shellcheck', dockerfile: 'dockerfiles/sa-tools/shellcheck.Dockerfile'],
]

pipeline {
    // Single top-level agent — all stages run on the same node and workspace.
    // Docker images are built via `docker build` inside bash scripts (not declarative Docker
    // stage agents), so there is no Docker-in-Docker risk with a top-level label agent.
    agent { label DOCKER_CONFIG.jenkinsAgent.label }

    options {
        timeout(time: 30, unit: 'MINUTES')
        disableConcurrentBuilds(abortPrevious: true)
        skipDefaultCheckout(true)  // Explicit Checkout stage fixes root-owned files before git runs
    }

    stages {
        stage('Checkout') {
            // Docker stages run as root; NFS root_squash means the Jenkins host user
            // cannot unlink root-owned files from a prior build. chmod via alpine fixes
            // this before git checkout to prevent permanent broken-loop failures.
            options { timeout(time: 3, unit: 'MINUTES') }
            steps {
                script {
                    def cleanupImage = "${DOCKER_CONFIG.registry.pullDomain}/alpine:latest"
                    sh "docker run --rm -v \"\$(pwd):/workspace\" ${cleanupImage} sh -c 'chmod -R a+rw /workspace 2>/dev/null || true' || true"
                }
                checkout scm
            }
        }

        stage('Build SA Tool Images') {
            // NOTE: script { parallel(...) } used instead of declarative parallel
            // because Jenkins forbids 'agent' and 'parallel' on the same stage.
            // All four builds run on the same allocated node (same Docker host and credentials).
            options { timeout(time: 25, unit: 'MINUTES') }
            steps {
                script {
                    def deps = loadDependencies()
                    def tags = load 'stages/calculate-image-tags.groovy'

                    withCredentials([
                        usernamePassword(
                            credentialsId: DOCKER_CONFIG.registry.credentialsId,
                            usernameVariable: 'REGISTRY_USR',
                            passwordVariable: 'REGISTRY_PSW'
                        ),
                        usernamePassword(
                            credentialsId: DOCKER_CONFIG.credentials.package-registryCredentialsId,
                            usernameVariable: 'REGISTRY_USER',
                            passwordVariable: 'REGISTRY_TOKEN'
                        ),
                    ]) {
                        def branches = [:]

                        for (def tool in SA_TOOLS) {
                            def t = tool  // capture for closure
                            branches["Build ${t.slug}"] = {
                                ensureImage(deps, tags, DOCKER_CONFIG, t.slug, t.dockerfile)
                            }
                        }

                        parallel(branches)
                    }
                }
            }
        }
    }

    post {
        always {
            script {
                echo "[${new Date().format('yyyy-MM-dd HH:mm:ss')}] [INFO] Pipeline completed — Branch: ${env.BRANCH_NAME ?: 'unknown'}, Build: ${env.BUILD_NUMBER}, Status: ${currentBuild.result ?: 'IN_PROGRESS'}"
                // Fix root-owned workspace files left by docker build stages.
                def cleanupImage = "${DOCKER_CONFIG.registry.pullDomain}/alpine:latest"
                sh "docker run --rm -v \"\$(pwd):/workspace\" ${cleanupImage} sh -c 'chmod -R a+rw /workspace 2>/dev/null || true' || true"
            }
        }
        failure {
            script {
                echo "[${new Date().format('yyyy-MM-dd HH:mm:ss')}] [ERROR] Pipeline failed — Branch: ${env.BRANCH_NAME ?: 'unknown'}, Build: ${env.BUILD_NUMBER}"
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Dispatcher helpers
// ---------------------------------------------------------------------------

def loadDependencies() {
    def logger   = load 'lib/logger.groovy'
    def executor = load 'lib/executor.groovy'
    return [logger: logger, executor: executor]
}

// Check if the image is cached; build and push if not.
// Credentials (REGISTRY_USR, REGISTRY_PSW, REGISTRY_USER, REGISTRY_TOKEN) must be in scope
// via withCredentials at the caller — they are consumed by build-copilot-image.sh.
def ensureImage(deps, tags, Map dockerConfig, String slug, String dockerfile) {
    def pushDomain = dockerConfig.registry.pushDomain
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
    def imageTag = deps.executor.retry(deps.logger, 3, 5) {
        tags.getDockerfileHash(dockerfile, hashInputs)
    }
    def fullImage = "${pushDomain}/${imageName}:${imageTag}"
    deps.logger.info("SA tool '${slug}': tag ${imageTag}")

    def exists = false
    deps.executor.retry(deps.logger, 3, 10) {
        exists = tags.imageExists(fullImage)
    }

    if (exists) {
        deps.logger.info("SA tool '${slug}': cached — skipping build: ${fullImage}")
    } else {
        deps.logger.info("SA tool '${slug}': not cached — building: ${fullImage}")
        deps.executor.retry(deps.logger, 3, 20) {
            sh "bash ./pipelines/scripts/build-copilot-image.sh '${pushDomain}' '${imageName}' '${imageTag}' '${dockerfile}'"
        }
    }
}

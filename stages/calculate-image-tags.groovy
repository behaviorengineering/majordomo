// .majordomo/stages/calculate-image-tags.groovy
// Computes image tags from file content hashes and checks registry existence

def getDockerfileHash(String dockerfilePath, List<String> additionalFiles = []) {
    def allFiles = ([dockerfilePath] + additionalFiles)
        .findAll { f -> fileExists(f) }
        .join(' ')
    return sh(
        script: "cat ${allFiles} | sha256sum | cut -c1-12",
        returnStdout: true
    ).trim()
}

def imageExists(String image) {
    return sh(
        script: "docker manifest inspect ${image} > /dev/null 2>&1",
        returnStatus: true
    ) == 0
}

return [
    getDockerfileHash: this.&getDockerfileHash,
    imageExists: this.&imageExists
]

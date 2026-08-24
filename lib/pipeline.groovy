// .majordomo/lib/pipeline.groovy
// Loaded via: def pipelineLib = load '.majordomo/lib/pipeline.groovy'

// ---------------------------------------------------------------------------
// GWT webhook re-fire
// ---------------------------------------------------------------------------

// Triggers a fresh build via Generic Webhook Trigger re-fire.
// Returns true when the webhook POST succeeded (HTTP 2xx), false otherwise.
// Only fires for PR builds — non-PR calls return false immediately.
// Requires cfg.credentials.gwtTokenCredentialsId to be configured.
// Payload uses eventKey 'pr:opened' so the triggered build bypasses the
// pr:from_ref_updated guard and runs as a full review — with no upstream
// parent link in the Jenkins UI (unlike build()).
def triggerFreshBuildViaGwt(Map cfg) {
    def gwtCredId = cfg.credentials?.gwtTokenCredentialsId?.trim()
    if (!gwtCredId) { return false }

    def prId = (params.CHANGE_ID ?: env.CHANGE_ID)?.trim()
    if (!prId) { return false }  // not a PR build — GWT re-fire only makes sense for PRs

    def jenkinsBase  = (env.JENKINS_URL ?: '').replaceAll('/+$', '')
    def payloadFile  = "${env.WORKSPACE}/.gwt-payload-${env.BUILD_NUMBER}.json"
    def payload = groovy.json.JsonOutput.toJson([
        eventKey: 'pr:opened',
        pullRequest: [
            id:      prId.isInteger() ? prId.toInteger() : 0,
            toRef:   [displayId: params.CHANGE_TARGET ?: 'master'],
            fromRef: [displayId: params.CHANGE_BRANCH ?: '']
        ]
    ])
    writeFile file: payloadFile, text: payload

    def gwtFired = false
    try {
        withCredentials([string(credentialsId: gwtCredId, variable: 'GWT_TOKEN')]) {
            def httpCode = sh(
                script: """curl -s -o /dev/null -w '%{http_code}' -x '' -X POST -H 'Content-Type: application/json' --data-binary @'${payloadFile}' "${jenkinsBase}/generic-webhook-trigger/invoke?token=\${GWT_TOKEN}" """,
                returnStdout: true
            ).trim()
            if (httpCode ==~ /^2\d\d$/) {
                echo "[INFO] Fresh build triggered via GWT webhook (HTTP ${httpCode}) — no upstream parent link."
                gwtFired = true
            } else {
                echo "[WARN] GWT webhook returned HTTP ${httpCode} — falling back to build() trigger."
            }
        }
    } finally {
        sh "rm -f '${payloadFile}'"
    }
    return gwtFired
}

// ---------------------------------------------------------------------------
// Submodule drift helpers
// ---------------------------------------------------------------------------

def buildInputApprovalUrl() {
    def jenkinsRoot = env.JENKINS_URL ?: ''
    def jobPath = env.JOB_NAME?.split('/')?.collect { "job/${it}" }?.join('/') ?: ''
    return "${jenkinsRoot}${jobPath}/${env.BUILD_NUMBER}/input/"
}

def replayHandoffParameters() {
    return [
        string(name: 'CHANGE_ID',           value: params.CHANGE_ID ?: ''),
        string(name: 'CHANGE_TARGET',        value: params.CHANGE_TARGET ?: 'master'),
        string(name: 'CHANGE_BRANCH',        value: params.CHANGE_BRANCH ?: ''),
        string(name: 'PUSH_BRANCH',          value: params.PUSH_BRANCH ?: ''),
        string(name: 'PR_EVENT_TYPE',        value: params.PR_EVENT_TYPE ?: ''),
        booleanParam(name: 'ENABLE_CONTINUOUS_RUNS', value: params.ENABLE_CONTINUOUS_RUNS ?: false),
        string(name: 'SUMMARY_PUBLISH_MODE', value: params.SUMMARY_PUBLISH_MODE ?: 'auto'),
        string(name: 'REPLAY_OF_BUILD',      value: env.BUILD_NUMBER ?: '')
    ]
}

return this

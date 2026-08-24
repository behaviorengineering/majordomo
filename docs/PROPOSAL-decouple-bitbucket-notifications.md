# Proposal: Decouple Early-Stage Bitbucket Notifications from Image Availability

*Majordomo — repository operations for evolving software.*

> **Historical — Jenkins retired.** This document predates removal of Jenkinsfiles, Groovy stages, and `.majordomo-config.groovy`. Behaviour below is a porting spec for the Go/GHA control tower — see [PLAN — Control Tower, GitHub Actions, and Go](../PLAN-control-tower-github-go.md).


**Status:** Exploratory  
**Date:** 2026-05-07  
**Problem:** Pipeline failures before `Ensure Images` completes leave Bitbucket status notifications undelivered.

---

## Problem Statement

Current behavior:
- All post-block notifications depend on `env.COPILOT_FULL_IMAGE` being set
- `COPILOT_FULL_IMAGE` is set only if `Ensure Images` completes successfully
- If registry auth fails, image build fails, or registry is unavailable, `COPILOT_FULL_IMAGE` remains empty
- Post-block notifiers skip silently when `COPILOT_FULL_IMAGE` is empty
- Result: Bitbucket sees no build status, operators have no visibility into early-stage failures

This creates an operational blind spot: the most critical failures (infrastructure/registry) are invisible.

---

## Root Cause Analysis

The current design uses Docker to execute Bitbucket notifications because:
1. **Certificate validation:** Corporate proxy and Bitbucket may require internal CA certificates
2. **Consistency:** All external API calls run inside the Copilot image, which has certs baked in (via `NODE_EXTRA_CA_CERTS`)
3. **Policy:** Avoid direct outbound connections from the Jenkins host; route through the image

Evidence in code:
- [PR Review stage environment](../pipelines/MajordomoReview.CI.Jenkinsfile#L260):
  ```groovy
  HTTP_PROXY   = 'http://proxy.example.com:8080'
  HTTPS_PROXY  = 'http://proxy.example.com:8080'
  ```
- [notify.groovy loads stages/notify-build-status.groovy](../lib/notify.groovy#L30), which runs Python inside Docker
- [publish-pr-summary.py](../pipelines/scripts/publish-pr-summary.py) uses urllib inside the container

**Question:** Is this a cert validation issue, or a proxy routing issue, or both?

---

## Proposed Solution (Conditional on Cert Availability)

### Option A: Early Commit SHA Capture + Conditional Fallback (Recommended for exploration)

**If certs are available on the Jenkins host:**

1. Capture commit SHA in Safe Checkout (always succeeds):
```groovy
stage('Safe Checkout') {
    steps {
        script {
            preCheckoutWorkspaceCleanup(PIPELINE_CONFIG)
        }
        checkout scm
        // Always available; no Docker dependency
        env.GIT_COMMIT_SHA = sh(script: "git rev-parse HEAD", returnStdout: true).trim()
    }
}
```

2. Add a lightweight direct-curl notifier in post blocks:
```groovy
def notifyBitbucketDirectly(String state, String description) {
    if (!isPrBuild() || !env.GIT_COMMIT_SHA?.trim()) return
    
    def cfg = getRuntimeConfig(PIPELINE_CONFIG)
    def tokenCredId = cfg.credentials?.bitbucketTokenCredentialsId
    if (!tokenCredId?.trim()) return
    
    // Try direct curl first; fall back gracefully if certs fail
    withCredentials([string(credentialsId: tokenCredId, variable: 'BB_TOKEN')]) {
        def rc = sh(
            script: """
            curl -s -f -X POST \\
              -H "Content-Type: application/json" \\
              -H "Authorization: Bearer ${BB_TOKEN}" \\
              -d '{"state":"${state}","description":"${description}","url":"${env.BUILD_URL}"}' \\
              "https://bitbucket.example.com/rest/build-status/1.0/commits/${env.GIT_COMMIT_SHA}" \\
              2>/dev/null || true
            """,
            returnStatus: true
        )
        // Non-zero is OK — we tried; don't fail the post block
    }
}
```

3. Update post blocks:
```groovy
post {
    always {
        script {
            // Try lightweight direct notification first
            notifyBitbucketDirectly('INPROGRESS', 'Pipeline completed')
            // Then try Docker-based notification if image is available (richer, more reliable)
            if (env.COPILOT_FULL_IMAGE?.trim()) {
                notifyBitbucketBuildStatus('SUCCESSFUL', 'Copilot review passed')
            }
        }
    }
}
```

**Pros:**
- Provides visibility even on early-stage failures
- Graceful fallback (curl fails, but post block doesn't fail)
- Minimal code change

**Cons:**
- Requires certs on the Jenkins host OR disabling cert validation (risky)
- Duplicates Bitbucket credential references
- Curl errors are silent (intentional, but may hide real issues)

---

### Option B: Pre-built Curl-Safe Image (If certs are the blocker)

If the Jenkins host lacks internal CA certs, create a lightweight image with certs baked in:

```dockerfile
# .majordomo/dockerfiles/bitbucket-notify.Dockerfile
FROM alpine:latest
RUN apk add --no-cache curl ca-certificates
# Copy corporate CA cert if available
COPY corporate-ca.crt /etc/ssl/certs/
RUN update-ca-certificates
ENTRYPOINT ["curl"]
```

Then use it in post blocks:
```groovy
docker.image("my-registry/bitbucket-notify:latest").inside() {
    sh 'curl ...'
}
```

**Pros:**
- Explicit cert handling; no assumptions about host
- Still decoupled from `COPILOT_FULL_IMAGE`

**Cons:**
- Adds another image to manage
- Still depends on registry being available for *this* image (but likely more reliable than full Copilot image)

---

### Option C: Accept Current Design, Document It Explicitly

If certificate validation is truly blocking and the Docker dependency is unavoidable:

**Action:** Document the assumption and add pre-flight validation:
```groovy
stage('Validate Infrastructure') {
    steps {
        script {
            // Fail early and loudly if Bitbucket is unreachable or certs are invalid
            withCredentials([string(credentialsId: cfg.credentials.bitbucketTokenCredentialsId, variable: 'BB_TOKEN')]) {
                sh '''
                curl -s -f -I \
                  -H "Authorization: Bearer ${BB_TOKEN}" \
                  "https://bitbucket.example.com/rest/api/1.0" \
                  || error "Bitbucket is unreachable; cannot proceed"
                '''
            }
        }
    }
}
```

**Pros:**
- No code changes to notification logic
- Fails fast with visibility

**Cons:**
- Doesn't solve the visibility problem on early-stage failures
- Pre-flight adds latency

---

## Recommendation for Investigation

**Before implementing any option:**

1. **Test cert availability on the Jenkins host:**
   ```bash
   echo | openssl s_client -connect bitbucket.example.com:443 2>/dev/null | grep -i "verify ok"
   ```

2. **Check what certs are in the Copilot image:**
   ```bash
   docker run --rm my-registry/copilot-cli:latest sh -c "cat /etc/ssl/certs/ca-certificates.crt | wc -l"
   ```

3. **Attempt a curl from the host vs. from inside the image:**
   ```bash
   # From host
   curl -v https://bitbucket.example.com/rest/api/1.0 2>&1 | grep -i "certificate"
   
   # From image
   docker run --rm my-registry/copilot-cli:latest curl -v https://bitbucket.example.com/rest/api/1.0 2>&1 | grep -i "certificate"
   ```

4. **If host curl fails with cert error:**
   - Determine how the cert should be provisioned (Jenkins agent config? /etc/ssl/certs/?)
   - Consider Option B (lightweight notify image)
   - If not feasible, document as a known limitation and accept Option C

---

## Implementation Path (if Option A is viable)

1. Add GIT_COMMIT_SHA capture in Safe Checkout
2. Implement `notifyBitbucketDirectly()` as a best-effort helper
3. Update all post blocks to call it first
4. Test with a simulated `Ensure Images` failure
5. Document cert assumptions in README

---

## Questions for the Team

1. **Why was the Docker dependency chosen?** Was it specifically due to cert validation, or for other reasons?
2. **Is corporate CA cert provisioning possible on the Jenkins host?**
3. **How often does registry auth or image build fail in production?** (severity estimate)
4. **What is the current visibility when early-stage failures happen?** (manual log inspection only?)

---

## References

- [Bitbucket REST API 1.0](https://docs.atlassian.com/bitbucket-server/latest/com.atlassian.bitbucket.server.docs:bitbucket-rest-api-docs/html/index.html)
- [Current notifier implementation](../lib/notify.groovy)
- [Pipeline proxy config](../pipelines/MajordomoReview.CI.Jenkinsfile#L260)

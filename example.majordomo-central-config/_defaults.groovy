// Majordomo — repository operations for evolving software.
// configs/_defaults.groovy — org-wide defaults for the central pipeline.
// Loaded first by MajordomoReview.Central.CI.Jenkinsfile; per-repo config is deep-merged on top.
// Only specify values that apply across ALL onboarded repos.
// Per-repo config (configs/<repo-slug>.groovy) overrides any key here.
//
// IMPORTANT: This file must end with `return [...]` so Jenkins load() receives the Map directly.

return [
    registry: [
        pullDomain:    '<artifactory-pull-domain>',
        pullUrl:       'https://<artifactory-pull-domain>',
        pushDomain:    '<artifactory-push-domain>',
        credentialsId: '<jenkins-docker-username-password-credential-id>',
    ],
    credentials: [
        // Org-wide credentials — same for all onboarded repos.
        artifactoryCredentialsId:    '<jenkins-artifactory-username-password-credential-id>',
        githubCopilotCredentialsId:  '<jenkins-github-copilot-secret-text-credential-id>',
        // Service account with PR-comment write access across all managed repos.
        // Teams do NOT set this in their per-repo config — it is set once here.
        bitbucketTokenCredentialsId: '<jenkins-bitbucket-service-account-token-credential-id>',
        // SSH credential used by the central job to check out app repos.
        bitbucketSshCredentialsId:   '<jenkins-bitbucket-ssh-credential-id>',
    ],
    jenkinsAgent: [
        label:      'edp_obm_lnx_shared',
        dockerArgs: '-u root -e HOME=/root',
    ],
    // cache: org-wide defaults for review cache gate.
    // Repos can override these in configs/<repo-slug>.groovy.
    cache: [
        // Default cache repo host mode.
        // Repos can override to 'central' in their repo config.
        cacheRepo: 'project',

        // Default behavior for cache-hit execution skipping.
        // Repos can override this in their repo config.
        enableSkips: false,

        // Optional explicit central cache remote (HTTPS).
        // Used when cacheRepo == 'central' and repo-specific override is absent.
        // centralRepoHttpUrl: 'https://bitbucket.example.com/scm/tooling/majordomo-central-config.git',

        // Global default retention (days) when repo override is absent.
        globalRetentionDays: 180,
        // Minimum retention floor (days) applied to resolved value.
        minRetentionDays: 30,
    ],
    // pipelines: org-wide per-pipeline model defaults.
    // Repos can override individual keys in their own config.
    pipelines: [
        'pr-review': [
            model:      'claude-sonnet-4.5',  // used for review, summary, and technical modes
            scoreModel: 'gpt-5.4-mini',        // used for score mode only
        ],
    ],
]

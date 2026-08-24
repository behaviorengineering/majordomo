// Majordomo — repository operations for evolving software.
// .majordomo-config.groovy — app-repo overrides for the shared .majordomo pipeline.
// Place this file at the repository root (alongside the .majordomo/ directory).
// Only specify the values you want to change — omit keys to keep the pipeline defaults.
//
// IMPORTANT: This file must end with `return [...]` so Jenkins load() receives the Map directly.
// Calling methods on loaded scripts goes through the Jenkins sandbox DSL dispatcher and
// throws NoSuchMethodError — method calls on loaded scripts are not sandbox-safe.

return [
        registry: [
            pullDomain:    '<artifactory-pull-domain>',
            pullUrl:       'https://<artifactory-pull-domain>',
            pushDomain:    '<artifactory-push-domain>',
            credentialsId: '<jenkins-docker-username-password-credential-id>',  // Username with password credential — used to authenticate with the Docker registry
        ],
        credentials: [
            artifactoryCredentialsId:    '<jenkins-artifactory-username-password-credential-id>',
            githubCopilotCredentialsId: '<jenkins-github-copilot-secret-text-credential-id>',  // Secret text credential — GitHub Copilot API token
            bitbucketTokenCredentialsId: '<jenkins-bitbucket-token-secret-text-credential-id>',  // secret text credential — personal access token with repo write permission
            bitbucketSshCredentialsId:   '<jenkins-bitbucket-ssh-credential-id>',  // SSH username with private key credential — same one used in Jenkins job SCM; used by snapshot guard ls-remote
            gwtTokenCredentialsId:       '<jenkins-gwt-token-secret-text-credential-id>',   // secret text credential — GWT token for webhook re-fire on submodule drift (avoids upstream parent link in Jenkins UI); omit or leave empty to fall back to build()
            prmTokenCredentialsId:       '<jenkins-prm-token-secret-text-credential-id>',   // secret text credential — optional token used for parameterized remote trigger flows (/buildWithParameters)
            // cacheTokenCredentialsId:   '<jenkins-cache-token-secret-text-credential-id>', // optional: dedicated cache push token; falls back to bitbucketTokenCredentialsId
        ],
        submoduleDriftTimeoutMinutes: 60,  // minutes to wait for drift-handoff approval before aborting; default 60
        // cache: optional review-cache settings for this repo.
        // Keys map directly to pre-analysis cache gate inputs.
        // cache: [
        //     // cacheRepo controls where cache branch is hosted.
        //     // 'project' = app repo, 'central' = central pipeline repo.
        //     cacheRepo: 'project',
        //
        //     // Enable skipping execution on cache hits.
        //     // Env COPILOT_ENABLE_CACHE_SKIPS overrides this value.
        //     enableSkips: false,
        //
        //     // dir: cache file location inside workspace.
        //     // If omitted, defaults to: ${WORKSPACE}/.majordomo-review-cache/<project-id>
        //     dir: '.majordomo-review-cache/my-repo',
        //
        //     // Optional explicit HTTPS remotes for cache push target selection.
        //     // projectRepoHttpUrl: 'https://bitbucket.example.com/scm/proj/my-repo.git',
        //     // centralRepoHttpUrl: 'https://bitbucket.example.com/scm/tooling/majordomo-central-config.git',
        //
        //     // Per-project retention in days (highest precedence from config).
        //     retentionDays: 120,
        //
        //     // Optional central and global defaults at repo level.
        //     // Usually configured in central defaults, shown here for completeness.
        //     centralRetentionDays: 180,
        //     globalRetentionDays: 180,
        //
        //     // Safety floor for retention.
        //     minRetentionDays: 30,
        // ],
        // staticAnalysis: [
        //     // Each entry runs one SA tool against matching changed files before the PR review.
        //     // Output is embedded in each slug file so the LLM sees SA findings alongside the diff.
        //     //
        //     // Two modes per entry — use one or the other:
        //     //
        //     //   dockerfile: path to a Dockerfile in this submodule (relative to repo root).
        //     //               Image is built and cached by SHA automatically (same as copilot-cli).
        //     //               Use this to get the submodule-provided tool images.
        //     //
        //     //   image:      fully-qualified image reference to use directly (BYO image).
        //     //               No build step — the image must already exist in a reachable registry.
        //     //               Use this to bring your own pre-built SA tool image.
        //     //
        //     // Required for both modes:
        //     //   command:    command string passed to the container entrypoint (file paths appended by the pipeline).
        //     //   glob:       glob pattern to select which changed files this tool runs against.
        //
        //     // --- Submodule-provided tools (built and cached automatically) ---
        //     [
        //         dockerfile: '.majordomo/dockerfiles/sa-tools/ruff.Dockerfile',
        //         command:    'check --output-format=concise',
        //         glob:       '**/*.py',
        //     ],
        //     [
        //         dockerfile: '.majordomo/dockerfiles/sa-tools/shellcheck.Dockerfile',
        //         command:    '-S warning -f gcc',
        //         glob:       '**/*.sh',
        //     ],
        //     [
        //         dockerfile: '.majordomo/dockerfiles/sa-tools/hadolint.Dockerfile',
        //         command:    '--no-color',
        //         glob:       '**/Dockerfile*',
        //     ],
        //     [
        //         dockerfile: '.majordomo/dockerfiles/sa-tools/eslint.Dockerfile',
        //         command:    '--format unix',
        //         glob:       '**/*.{js,ts,jsx,tsx}',
        //     ],
        //
        //     // --- BYO image (pre-built, no dockerfile required) ---
        //     [
        //         image:   '<registry-domain>/<image-name>:<tag>',
        //         command: '--format text',
        //         glob:    'src/**/*.groovy',
        //     ],
        // ],

        // pipelines: [
        //     // Each key is a pipeline name that maps to an orchestrator agent + skills + routing.
        //     // Omit a pipeline entirely to keep the submodule defaults.
        //     // Omit pipelines entirely to run the built-in pr-review pipeline with default routing.
        //     'pr-review': [
        //         // agent: path to orchestrator agent file, relative to app repo root.
        //         // Omit to use the submodule default (agents/pr-review.agent.md).
        //         agent: 'agents/my-pr-review.agent.md',
        //
        //         // skills: override the directory containing SKILL.md for a specific skill.
        //         // Omit a skill to use the submodule default (agents/skills/<skill>/).
        //         // File-review skills (routed by file type):
        //         //   pr-review-code   source code (default routing — active by default)
        //         //   pr-review-conf   config files: .yml .yaml .toml .json (opt-in via routing below)
        //         //   pr-review-docs   markdown docs — corpus-aware cross-doc consistency (opt-in via routing below)
        //         //   pr-review-tests  test files (opt-in via routing below)
        //         // Synthesis skills (run automatically after file-review, not routed by file type):
        //         //   pr-review-summary         writes summary.md
        //         //   pr-review-technical       writes tech-review.md
        //         //   pr-review-blast-radius    writes blast-radius.md
        //         // Scoring skill (used internally by summary loop):
        //         //   pr-review-summary-score   scores summary.md; drives write/score loop
        //         // Prose-quality runs as a pipeline phase (not a user-configurable skill).
        //         skills: [
        //             'pr-review-code':  null,                      // null = use submodule default
        //             'pr-review-conf':  'agents/skills/my-conf',   // path relative to app repo root
        //             'pr-review-docs':  null,
        //             'pr-review-tests': null,
        //         ],
        //
        //         // routing: control which files each file-review skill receives.
        //         // Synthesis skills (summary, technical, blast-radius) are not listed here —
        //         // they run automatically and do not receive individual files.
        //         // First matching glob wins. Files with no match are excluded.
        //         routing: [
        //             'pr-review-docs': ['**/*.md', '**/*.rst'],                                             // markdown — corpus-aware consistency check
        //             'pr-review-conf': ['**/*.yml', '**/*.yaml', '**/*.toml', '**/*.json', '**/*.env', 'docs/**'],  // config files — isolated review + blast radius
        //             'pr-review-tests':           ['**/test_*.py', '**/*_test.py', '**/*.spec.ts', '**/*.spec.js'],
        //             'pr-review-code':            ['**'],  // catch-all — must be last
        //         ],
        //     ],
        // ],
]

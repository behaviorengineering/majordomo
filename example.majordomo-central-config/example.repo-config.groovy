// Majordomo — repository operations for evolving software.
// configs/example-repo.groovy — per-repo config template for the central pipeline.
// Copy to configs/<repo-slug>.groovy and fill in the values.
// Only specify keys that differ from configs/_defaults.groovy.
// Omit a key entirely to keep the org-wide default.
//
// IMPORTANT: This file must end with `return [...]` so Jenkins load() receives the Map directly.

return [
    // bitbucket: identifies the app repo — required for central checkout and build-status callbacks.
    bitbucket: [
        projectKey: '<BITBUCKET-PROJECT-KEY>',   // e.g. 'PAYMENTS'
        repoSlug:   '<repo-slug>',               // must match the filename (configs/<repo-slug>.groovy)
        cloneSshUrl: 'ssh://git@<bitbucket-host>:<port>/<PROJECT>/<repo-slug>.git',
    ],

    // credentials: override org-wide credentials for this repo only.
    // Omit any key to inherit the org-wide value from _defaults.groovy.
    // credentials: [
    //     githubCopilotCredentialsId: '<repo-specific-github-copilot-secret-text-credential-id>',
    //     bitbucketSshCredentialsId:  '<repo-specific-ssh-credential-id>',  // needed if repo is in a different Bitbucket project
    //     artifactoryCredentialsId:   '<repo-specific-artifactory-credential-id>',
    // ],

    // staticAnalysis: SA tools to run on changed files before the PR review.
    // Omit to disable SA for this repo.
    // staticAnalysis: [
    //     [
    //         dockerfile: '.majordomo/dockerfiles/sa-tools/ruff.Dockerfile',
    //         command:    'check --output-format=concise',
    //         glob:       '**/*.py',
    //     ],
    // ],

    // cache: per-repo review cache settings.
    // Omit keys to inherit from configs/_defaults.groovy.
    // cache: [
    //     // Cache host selection: 'project' (default) or 'central'.
    //     cacheRepo: 'project',
    //
    //     // Enable skipping execution on cache hits for this repo.
    //     // Env COPILOT_ENABLE_CACHE_SKIPS overrides this value.
    //     enableSkips: false,
    //
    //     // Optional explicit HTTPS remotes for push target selection.
    //     // projectRepoHttpUrl: 'https://bitbucket.example.com/scm/proj/<repo-slug>.git',
    //     // centralRepoHttpUrl: 'https://bitbucket.example.com/scm/tooling/majordomo-central-config.git',
    //
    //     // Optional cache directory path used by pre-analysis cache gate.
    //     // Defaults to ${WORKSPACE}/.majordomo-review-cache/<project-id> when omitted.
    //     dir: '.majordomo-review-cache/<repo-slug>',
    //
    //     // Per-project retention in days.
    //     // This is the highest-precedence config value (unless env override is set).
    //     retentionDays: 120,
    //
    //     // Optional central default for this repo profile.
    //     centralRetentionDays: 180,
    // ],

    // pipelines: routing, skill overrides, model, and agent context for this repo.
    // Omit to use org-wide defaults.
    // pipelines: [
    //     'pr-review': [
    //         // model: base model for file-review batches, finalize, and prose modes.
    //         // model: 'gpt-5-mini',
    //
    //         // summaryModel: model for summary synthesis (falls back to model if omitted).
    //         // summaryModel: 'claude-sonnet-4.5',
    //
    //         // technicalModel: model for technical + technical-deep modes (falls back to model if omitted).
    //         // technicalModel: 'claude-sonnet-4.6',
    //
    //         // scoreModel: model for score + tech-score modes.
    //         // scoreModel: 'auto',
    //
    //         // routing: controls which changed files go to which review skill.
    //         // First matching glob wins. Files with no match are excluded.
    //         // Simple form — globs only:
    //         routing: [
    //             'pr-review-docs':  ['**/*.md', '**/*.rst'],
    //             'pr-review-conf':  ['**/*.yml', '**/*.yaml', '**/*.toml', '**/*.json'],
    //             'pr-review-tests': ['**/test_*.py', '**/*_test.py'],
    //             'pr-review-code':  ['**'],  // catch-all — must be last
    //         ],
    //         // Extended form — globs + optional per-skill persona:
    //         // routing: [
    //         //     'pr-review-docs':  [globs: ['**/*.md', '**/*.rst'],                          persona: '.majordomo/agents/personas/pr-review-docs.persona.md'],
    //         //     'pr-review-conf':  [globs: ['**/*.yml', '**/*.yaml', '**/*.toml', '**/*.json'], persona: '.majordomo/agents/personas/pr-review-conf.persona.md'],
    //         //     'pr-review-tests': [globs: ['**/test_*.py', '**/*_test.py'],                 persona: '.majordomo/agents/personas/pr-review-tests.persona.md'],
    //         //     'pr-review-code':  [globs: ['**'],                                           persona: '.majordomo/agents/personas/pr-review-code.persona.md'],
    //         // ],
    //
    //         // summary: controls which sections appear in summary.md and how they are filled.
    //         // All sections are enabled by default. Omit to use skill defaults.
    //         // summary: [
    //         //     sections: [
    //         //         'why':        [enabled: true,  instructions: 'Focus on API contract decisions only.'],
    //         //         'tldr':       [enabled: true],
    //         //         'what-built': [enabled: true],
    //         //         'low-risk':   [enabled: false],   // suppress the Low-Risk section entirely
    //         //         'judgment':   [enabled: true,  instructions: 'Flag migration risks and deployment prerequisites only.'],
    //         //         'focus':      [enabled: true],
    //         //     ],
    //         // ],
    //
    //         // agentContext: repo-specific review context, merged by file path.
    //         // Format:
    //         //   global: baseline context for all files
    //         //   scoped: first matching glob wins; merged on top of global for that file
    //         // customRules supports inline rules and shared-file references:
    //         //   'inline rule text'
    //         //   [file: '.majordomo/rules/<ruleset>.md']
    //         agentContext: [
    //             global: [
    //                 customRules: [
    //                     [file: '.majordomo/rules/shared/security-baseline.md'],
    //                     'No credentials or tokens hardcoded in source or docs.',
    //                 ],
    //             ],
    //             scoped: [
    //                 'services/payments-api/**': [
    //                     techStack:   ['python', 'fastapi', 'mesh', 'openapi'],
    //                     reviewFocus: ['openapi-contract', 'error-handling', 'logging', 'auth'],
    //                     customRules: [
    //                         [file: '.majordomo/rules/mesh/mesh-api-contract.md'],
    //                         [file: '.majordomo/rules/mesh/mesh-logging.md'],
    //                         'FastAPI error handling must use exception_handlers, not Flask-style decorators.',
    //                     ],
    //                 ],
    //                 'src/mesh_repos/contracts/**': [
    //                     techStack:   ['openapi', 'json-schema', 'mesh'],
    //                     reviewFocus: ['contract-compatibility', 'schema-quality'],
    //                     customRules: [
    //                         [file: '.majordomo/rules/mesh/mesh-contracts.md'],
    //                         'No breaking contract field changes without version bump and migration note.',
    //                     ],
    //                 ],
    //                 'sops/**': [
    //                     techStack:   ['docs', 'runbooks'],
    //                     reviewFocus: ['procedural-accuracy', 'rollback-completeness'],
    //                     customRules: [
    //                         [file: '.majordomo/rules/docs/doc-quality.md'],
    //                         'Every operational procedure must include rollback or recovery guidance.',
    //                     ],
    //                 ],
    //             ],
    //         ],
    //     ],
    // ],
]

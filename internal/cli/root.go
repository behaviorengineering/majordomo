// Package cli wires majordomo subcommands.
package cli

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/behaviorengineering/majordomo/internal/agent"
	"github.com/behaviorengineering/majordomo/internal/cache"
	"github.com/behaviorengineering/majordomo/internal/config"
	diffpkg "github.com/behaviorengineering/majordomo/internal/diff"
	"github.com/behaviorengineering/majordomo/internal/orchestrate"
	"github.com/behaviorengineering/majordomo/internal/poll"
	"github.com/behaviorengineering/majordomo/internal/publish"
	"github.com/behaviorengineering/majordomo/internal/report"
	"github.com/behaviorengineering/majordomo/internal/sa"
	"github.com/behaviorengineering/majordomo/internal/satools"
	"github.com/behaviorengineering/majordomo/internal/staging"
	"github.com/behaviorengineering/majordomo/internal/status"
	"github.com/behaviorengineering/majordomo/internal/submodule"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// NewRoot returns the root majordomo command.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "majordomo",
		Short: "Repository operations for evolving software",
		Long: `Majordomo — repository operations for evolving software.

Control-plane CLI for PR/MR review: poll, prep, orchestrate, publish, and cache.
See docs/PLAN-control-tower-github-go.md.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newPollCmd())
	root.AddCommand(newPrepCmd())
	root.AddCommand(newDispatchCmd())
	root.AddCommand(newOrchestrateCmd())
	root.AddCommand(newPublishCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newCacheCmd())
	root.AddCommand(newReportCmd())
	root.AddCommand(newBuildSAToolsCmd())
	root.AddCommand(newSACmd())
	root.AddCommand(newSubmoduleCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print majordomo version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
		},
	}
}

func stub(name, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("%s: not implemented yet (see docs/PLAN-control-tower-github-go.md)", name)
		},
	}
}

func newPollCmd() *cobra.Command {
	var configDir, cursorDir, outPath string
	cmd := &cobra.Command{
		Use:   "poll",
		Short: "Poll SCM APIs for open PRs/MRs that need review (reconciliation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := outPath
			if out == "-" {
				out = ""
			}
			return poll.Run(poll.Options{
				ConfigDir: configDir,
				CursorDir: cursorDir,
				OutPath:   out,
			})
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "majordomo-central-config", "path to majordomo-central-config")
	cmd.Flags().StringVar(&cursorDir, "cursor-dir", ".poll-cache", "local poll-cursor store (use Actions cache)")
	cmd.Flags().StringVar(&outPath, "out", "pending-reviews.json", "write pending reviews JSON (\"-\" for stdout)")
	return cmd
}

func newPrepCmd() *cobra.Command {
	var routing, agentContext, summaryConfig, configDir, repoID, pipeline string
	cmd := &cobra.Command{
		Use:   "prep <base-branch> <staging-dir>",
		Short: "Classify diffs, cluster files, write staging manifest",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			matDir := config.MaterializeDirForStaging(args[1])
			routingPath, agentContextPath, _, err := config.ResolvePrepPaths(
				configDir, repoID, pipeline, matDir, routing, agentContext,
			)
			if err != nil {
				return err
			}
			opts := staging.Options{
				BaseBranch:        args[0],
				StagingDir:        args[1],
				RoutingPath:       routingPath,
				AgentContextPath:  agentContextPath,
				SummaryConfigPath: summaryConfig,
			}
			return staging.Run(opts)
		},
	}
	cmd.Flags().StringVar(&routing, "routing", "", "path to routing JSON")
	cmd.Flags().StringVar(&agentContext, "agent-context", "", "path to agent context JSON")
	cmd.Flags().StringVar(&summaryConfig, "summary-config", "", "path to summary config JSON")
	cmd.Flags().StringVar(&configDir, "config-dir", "", "majordomo-central-config dir (materialize routing/agentContext)")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "served repo id under config-dir")
	cmd.Flags().StringVar(&pipeline, "pipeline", "pr-review", "pipelines.<name> key when using --config-dir")
	return cmd
}

func newDispatchCmd() *cobra.Command {
	var (
		scriptsDir string
		finalize   bool
		summary    bool
		score      bool
		technical  bool
		techScore  bool
		prose      bool
		techDeep   bool
	)
	cmd := &cobra.Command{
		Use:   "dispatch <pr-number> <staging-dir> <output-dir>",
		Short: "Run one agent batch (wraps agent-dispatch.sh / OpenCode)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := agent.ModeFiles
			switch {
			case finalize:
				mode = agent.ModeFinalize
			case summary:
				mode = agent.ModeSummary
			case score:
				mode = agent.ModeScore
			case technical:
				mode = agent.ModeTechnical
			case techScore:
				mode = agent.ModeTechScore
			case prose:
				mode = agent.ModeProse
			case techDeep:
				mode = agent.ModeTechnicalDeep
			}
			return agent.Dispatch(agent.DispatchOptions{
				PRNumber:   args[0],
				StagingDir: args[1],
				OutputDir:  args[2],
				Mode:       mode,
				ScriptsDir: scriptsDir,
			})
		},
	}
	cmd.Flags().StringVar(&scriptsDir, "scripts-dir", "", "pipelines/scripts directory")
	cmd.Flags().BoolVar(&finalize, "finalize", false, "finalize mode")
	cmd.Flags().BoolVar(&summary, "summary", false, "summary mode")
	cmd.Flags().BoolVar(&score, "score", false, "score mode")
	cmd.Flags().BoolVar(&technical, "technical", false, "technical mode")
	cmd.Flags().BoolVar(&techScore, "tech-score", false, "tech-score mode")
	cmd.Flags().BoolVar(&prose, "prose", false, "prose mode")
	cmd.Flags().BoolVar(&techDeep, "technical-deep", false, "technical-deep mode")
	return cmd
}

func newOrchestrateCmd() *cobra.Command {
	var (
		pr, stagingDir, outputDir, baseBranch, pipeline, scriptsDir, repoRoot string
		routing, agentContext, summaryConfig, configDir, repoID               string
		concurrency                                                           int
		skipPrep, skipDeep, skipReport                                        bool
		timeoutMin                                                            int
	)
	cmd := &cobra.Command{
		Use:   "orchestrate",
		Short: "Run review waves, checkpoints, finalize, and synthesis loops",
		RunE: func(cmd *cobra.Command, args []string) error {
			if concurrency <= 0 {
				if v := os.Getenv("COPILOT_CONCURRENCY"); v != "" {
					if n, err := strconv.Atoi(v); err == nil {
						concurrency = n
					}
				}
			}
			var timeout time.Duration
			if timeoutMin > 0 {
				timeout = time.Duration(timeoutMin) * time.Minute
			}
			return orchestrate.Run(orchestrate.Options{
				PRNumber:          pr,
				BaseBranch:        baseBranch,
				StagingDir:        stagingDir,
				OutputDir:         outputDir,
				Pipeline:          pipeline,
				Concurrency:       concurrency,
				ScriptsDir:        scriptsDir,
				SkipPrep:          skipPrep,
				SkipDeep:          skipDeep,
				SkipReport:        skipReport,
				RepoRoot:          repoRoot,
				RoutingPath:       routing,
				AgentContextPath:  agentContext,
				SummaryConfigPath: summaryConfig,
				ConfigDir:         configDir,
				RepoID:            repoID,
				BatchTimeout:      timeout,
			})
		},
	}
	cmd.Flags().StringVar(&pr, "pr", "", "PR/MR number (required)")
	cmd.Flags().StringVar(&stagingDir, "staging-dir", "", "prep staging directory (required)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "pipeline output directory (required)")
	cmd.Flags().StringVar(&baseBranch, "base-branch", "", "base branch for prep (required unless --skip-prep)")
	cmd.Flags().StringVar(&pipeline, "pipeline", "pr-review", "pipeline name label")
	cmd.Flags().StringVar(&scriptsDir, "scripts-dir", "", "pipelines/scripts directory")
	cmd.Flags().StringVar(&repoRoot, "repo-root", "", "app repo root for prep/deep (default: cwd)")
	cmd.Flags().StringVar(&routing, "routing", "", "routing JSON for prep")
	cmd.Flags().StringVar(&agentContext, "agent-context", "", "agent context JSON for prep")
	cmd.Flags().StringVar(&summaryConfig, "summary-config", "", "summary config JSON for prep")
	cmd.Flags().StringVar(&configDir, "config-dir", "", "majordomo-central-config dir")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "served repo id under config-dir")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "max parallel batches (default COPILOT_CONCURRENCY or 6)")
	cmd.Flags().IntVar(&timeoutMin, "batch-timeout-minutes", 0, "per-batch timeout (default 8)")
	cmd.Flags().BoolVar(&skipPrep, "skip-prep", false, "assume staging already prepared")
	cmd.Flags().BoolVar(&skipDeep, "skip-deep", false, "skip technical deep review pass")
	cmd.Flags().BoolVar(&skipReport, "skip-report", false, "skip JUnit conversion")
	_ = cmd.MarkFlagRequired("pr")
	_ = cmd.MarkFlagRequired("staging-dir")
	_ = cmd.MarkFlagRequired("output-dir")
	return cmd
}

func newSACmd() *cobra.Command {
	var configDir, repoID, repoRoot, baseBranch, scriptsDir, imagePrefix string
	cmd := &cobra.Command{
		Use:   "sa",
		Short: "Run staticAnalysis tools from central config into .sa/",
		RunE: func(cmd *cobra.Command, args []string) error {
			return sa.Run(sa.Options{
				ConfigDir:   configDir,
				RepoID:      repoID,
				RepoRoot:    repoRoot,
				BaseBranch:  baseBranch,
				ScriptsDir:  scriptsDir,
				ImagePrefix: imagePrefix,
			})
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "majordomo-central-config", "path to majordomo-central-config")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "served repo id (required)")
	cmd.Flags().StringVar(&repoRoot, "repo-root", "", "served repo checkout (default: cwd)")
	cmd.Flags().StringVar(&baseBranch, "base-branch", "", "base branch for changed-file list (required)")
	cmd.Flags().StringVar(&scriptsDir, "scripts-dir", "", "pipelines/scripts with run-sa-tool.sh")
	cmd.Flags().StringVar(&imagePrefix, "image-prefix", "", "registry prefix when tool has no image (or MAJORDOMO_SA_IMAGE_PREFIX)")
	_ = cmd.MarkFlagRequired("repo-id")
	_ = cmd.MarkFlagRequired("base-branch")
	return cmd
}

func newPublishCmd() *cobra.Command {
	var scm, owner, repo, repoID, gitlabHost, gitlabProjectID string
	cmd := &cobra.Command{
		Use:   "publish <pr-number> <summary-file> <mode>",
		Short: "Publish summary to PR/MR (github|gitlab via gh/glab; bitbucket HTTP)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return publish.Run(publish.Options{
				SCM:             scm,
				PRNumber:        args[0],
				SummaryFile:     args[1],
				Mode:            publish.Mode(args[2]),
				RepoID:          repoID,
				GitHubOwner:     owner,
				GitHubRepo:      repo,
				GitLabHost:      gitlabHost,
				GitLabProjectID: gitlabProjectID,
			})
		},
	}
	cmd.Flags().StringVar(&scm, "scm", "github", "scm forge: github|gitlab|bitbucket")
	cmd.Flags().StringVar(&owner, "owner", "", "GitHub/GitLab owner or group path")
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub/GitLab project name")
	cmd.Flags().StringVar(&repoID, "repo-id", "", "central-config id (MAJORDOMO_CREDENTIAL_ override; or MAJORDOMO_REPO_ID)")
	cmd.Flags().StringVar(&gitlabHost, "gitlab-host", "", "GitLab host (default gitlab.com; or GITLAB_HOST)")
	cmd.Flags().StringVar(&gitlabProjectID, "gitlab-project-id", "", "GitLab numeric project id (or GITLAB_PROJECT_ID)")
	return cmd
}

func newStatusCmd() *cobra.Command {
	var scm, contextName string
	cmd := &cobra.Command{
		Use:   "status <commit-sha> <state>",
		Short: "Post commit/check status (INPROGRESS|SUCCESSFUL|FAILED)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return status.Run(status.Options{
				SCM:       scm,
				CommitSHA: args[0],
				State:     status.State(args[1]),
				Context:   contextName,
			})
		},
	}
	cmd.Flags().StringVar(&scm, "scm", "github", "scm forge: github|bitbucket")
	cmd.Flags().StringVar(&contextName, "context", "", "GitHub status context (default majordomo)")
	return cmd
}

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Review and poll cache on served repo",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "validate-branch <branch>",
		Short: "Validate majordomo-pr-reviewer-cache branch name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cache.ValidateReviewCacheBranch(args[0])
		},
	})
	var remote, branch, worktree string
	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push review-cache branch with constrained auth",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cache.Push(cache.PushOptions{Remote: remote, Branch: branch, Worktree: worktree})
		},
	}
	pushCmd.Flags().StringVar(&remote, "remote", "", "https remote URL")
	pushCmd.Flags().StringVar(&branch, "branch", "", "cache branch name")
	pushCmd.Flags().StringVar(&worktree, "worktree", "", "cache worktree path")
	_ = pushCmd.MarkFlagRequired("remote")
	_ = pushCmd.MarkFlagRequired("branch")
	_ = pushCmd.MarkFlagRequired("worktree")
	cmd.AddCommand(pushCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "poll-get <cursor-file>",
		Short: "Print poll-cursor.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cache.ReadPollCursor(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%v\n", c.Heads)
			return nil
		},
	})
	var pr, sha string
	setCmd := &cobra.Command{
		Use:   "poll-set <cursor-file>",
		Short: "Record PR head SHA in poll-cursor.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cache.ReadPollCursor(args[0])
			if err != nil {
				return err
			}
			cache.RecordHead(c, pr, sha)
			return cache.WritePollCursor(args[0], c)
		},
	}
	setCmd.Flags().StringVar(&pr, "pr", "", "PR number")
	setCmd.Flags().StringVar(&sha, "sha", "", "head commit SHA")
	_ = setCmd.MarkFlagRequired("pr")
	_ = setCmd.MarkFlagRequired("sha")
	cmd.AddCommand(setCmd)

	var (
		projectID            string
		cacheDir             string
		projectRetentionDays int
		centralRetentionDays int
		globalRetentionDays  int
		minRetentionDays     int
		indexOut             string
		haveProjectRet       bool
		haveCentralRet       bool
	)
	precheckCmd := &cobra.Command{
		Use:   "precheck",
		Short: "Prune expired cluster cache entries and build index",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := cache.PrecheckOptions{
				ProjectID:           projectID,
				CacheDir:            cacheDir,
				GlobalRetentionDays: globalRetentionDays,
				MinRetentionDays:    minRetentionDays,
				IndexOut:            indexOut,
			}
			if haveProjectRet {
				v := projectRetentionDays
				opts.ProjectRetentionDays = &v
			}
			if haveCentralRet {
				v := centralRetentionDays
				opts.CentralRetentionDays = &v
			}
			result, err := cache.Precheck(opts)
			if err != nil {
				return err
			}
			return cache.PrintJSONPretty(result)
		},
	}
	precheckCmd.Flags().StringVar(&projectID, "project-id", "", "project id")
	precheckCmd.Flags().StringVar(&cacheDir, "cache-dir", "", "cache directory")
	precheckCmd.Flags().IntVar(&projectRetentionDays, "project-retention-days", 0, "project retention days")
	precheckCmd.Flags().IntVar(&centralRetentionDays, "central-retention-days", 0, "central retention days")
	precheckCmd.Flags().IntVar(&globalRetentionDays, "global-retention-days", 180, "global retention days")
	precheckCmd.Flags().IntVar(&minRetentionDays, "min-retention-days", 30, "minimum retention days")
	precheckCmd.Flags().StringVar(&indexOut, "index-out", "", "optional path to write index JSON")
	_ = precheckCmd.MarkFlagRequired("project-id")
	_ = precheckCmd.MarkFlagRequired("cache-dir")
	precheckCmd.PreRun = func(cmd *cobra.Command, args []string) {
		haveProjectRet = cmd.Flags().Changed("project-retention-days")
		haveCentralRet = cmd.Flags().Changed("central-retention-days")
	}
	cmd.AddCommand(precheckCmd)

	var (
		indexFile             string
		clusterSHA            string
		skillName             string
		fingerprintVersion    string
		clusterFiles          []string
		clusterFilesFile      string
		modelID               string
		modelRevision         string
		instructionBundleHash string
		promptTemplateHash    string
		scoringRubricHash     string
		outputSchemaVersion   string
	)
	lookupCmd := &cobra.Command{
		Use:   "lookup",
		Short: "Evaluate cluster cache hit for current run context",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := cache.Lookup(cache.LookupOptions{
				IndexFile:             indexFile,
				ClusterSHA:            clusterSHA,
				SkillName:             skillName,
				FingerprintVersion:    fingerprintVersion,
				ClusterFiles:          clusterFiles,
				ClusterFilesFile:      clusterFilesFile,
				ModelID:               modelID,
				ModelRevision:         modelRevision,
				InstructionBundleHash: instructionBundleHash,
				PromptTemplateHash:    promptTemplateHash,
				ScoringRubricHash:     scoringRubricHash,
				OutputSchemaVersion:   outputSchemaVersion,
			})
			if err != nil {
				return err
			}
			return cache.PrintJSON(result)
		},
	}
	lookupCmd.Flags().StringVar(&indexFile, "index-file", "", "precheck index JSON")
	lookupCmd.Flags().StringVar(&clusterSHA, "cluster-sha", "", "cluster sha256")
	lookupCmd.Flags().StringVar(&skillName, "skill-name", "", "skill name")
	lookupCmd.Flags().StringVar(&fingerprintVersion, "fingerprint-version", "", "fingerprint version")
	lookupCmd.Flags().StringArrayVar(&clusterFiles, "cluster-file", nil, "cluster file path (repeatable)")
	lookupCmd.Flags().StringVar(&clusterFilesFile, "cluster-files-file", "", "file with cluster paths")
	lookupCmd.Flags().StringVar(&modelID, "model-id", "", "model id")
	lookupCmd.Flags().StringVar(&modelRevision, "model-revision", "", "model revision")
	lookupCmd.Flags().StringVar(&instructionBundleHash, "instruction-bundle-hash", "", "instruction bundle hash")
	lookupCmd.Flags().StringVar(&promptTemplateHash, "prompt-template-hash", "", "prompt template hash")
	lookupCmd.Flags().StringVar(&scoringRubricHash, "scoring-rubric-hash", "", "scoring rubric hash")
	lookupCmd.Flags().StringVar(&outputSchemaVersion, "output-schema-version", "", "output schema version")
	_ = lookupCmd.MarkFlagRequired("index-file")
	_ = lookupCmd.MarkFlagRequired("cluster-sha")
	_ = lookupCmd.MarkFlagRequired("fingerprint-version")
	_ = lookupCmd.MarkFlagRequired("model-id")
	_ = lookupCmd.MarkFlagRequired("instruction-bundle-hash")
	_ = lookupCmd.MarkFlagRequired("prompt-template-hash")
	_ = lookupCmd.MarkFlagRequired("scoring-rubric-hash")
	_ = lookupCmd.MarkFlagRequired("output-schema-version")
	cmd.AddCommand(lookupCmd)

	var (
		storeCacheDir      string
		storeSkill         string
		storeSHA           string
		storeFP            string
		storeClusterFiles  []string
		storeClusterFile   string
		storeModelID       string
		storeModelRev      string
		storeInstrHash     string
		storePromptHash    string
		storeRubricHash    string
		storeSchemaVer     string
		storeAnalysisFile  string
		storeReportsDir    string
		storeArtifactFiles []string
	)
	storeCmd := &cobra.Command{
		Use:   "store",
		Short: "Write or update cluster cache artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := cache.Store(cache.StoreOptions{
				CacheDir:              storeCacheDir,
				SkillName:             storeSkill,
				ClusterSHA:            storeSHA,
				FingerprintVersion:    storeFP,
				ClusterFiles:          storeClusterFiles,
				ClusterFilesFile:      storeClusterFile,
				ModelID:               storeModelID,
				ModelRevision:         storeModelRev,
				InstructionBundleHash: storeInstrHash,
				PromptTemplateHash:    storePromptHash,
				ScoringRubricHash:     storeRubricHash,
				OutputSchemaVersion:   storeSchemaVer,
				AnalysisFile:          storeAnalysisFile,
				ReportsDir:            storeReportsDir,
				ArtifactFiles:         storeArtifactFiles,
			})
			if err != nil {
				return err
			}
			return cache.PrintJSON(result)
		},
	}
	storeCmd.Flags().StringVar(&storeCacheDir, "cache-dir", "", "cache directory")
	storeCmd.Flags().StringVar(&storeSkill, "skill-name", "", "skill name")
	storeCmd.Flags().StringVar(&storeSHA, "cluster-sha", "", "cluster sha256")
	storeCmd.Flags().StringVar(&storeFP, "fingerprint-version", "", "fingerprint version")
	storeCmd.Flags().StringArrayVar(&storeClusterFiles, "cluster-file", nil, "cluster file path (repeatable)")
	storeCmd.Flags().StringVar(&storeClusterFile, "cluster-files-file", "", "file with cluster paths")
	storeCmd.Flags().StringVar(&storeModelID, "model-id", "", "model id")
	storeCmd.Flags().StringVar(&storeModelRev, "model-revision", "", "model revision")
	storeCmd.Flags().StringVar(&storeInstrHash, "instruction-bundle-hash", "", "instruction bundle hash")
	storeCmd.Flags().StringVar(&storePromptHash, "prompt-template-hash", "", "prompt template hash")
	storeCmd.Flags().StringVar(&storeRubricHash, "scoring-rubric-hash", "", "scoring rubric hash")
	storeCmd.Flags().StringVar(&storeSchemaVer, "output-schema-version", "", "output schema version")
	storeCmd.Flags().StringVar(&storeAnalysisFile, "analysis-file", "", "analysis payload file")
	storeCmd.Flags().StringVar(&storeReportsDir, "reports-dir", "", "optional reports directory")
	storeCmd.Flags().StringArrayVar(&storeArtifactFiles, "artifact-file", nil, "extra markdown artifact names")
	_ = storeCmd.MarkFlagRequired("cache-dir")
	_ = storeCmd.MarkFlagRequired("skill-name")
	_ = storeCmd.MarkFlagRequired("cluster-sha")
	_ = storeCmd.MarkFlagRequired("fingerprint-version")
	_ = storeCmd.MarkFlagRequired("model-id")
	_ = storeCmd.MarkFlagRequired("instruction-bundle-hash")
	_ = storeCmd.MarkFlagRequired("prompt-template-hash")
	_ = storeCmd.MarkFlagRequired("scoring-rubric-hash")
	_ = storeCmd.MarkFlagRequired("output-schema-version")
	_ = storeCmd.MarkFlagRequired("analysis-file")
	cmd.AddCommand(storeCmd)

	var restoreCacheDir, restoreEntry, restoreOut string
	restoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore cached markdown artifacts for a cache hit",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := cache.Restore(cache.RestoreOptions{
				CacheDir:  restoreCacheDir,
				EntryFile: restoreEntry,
				OutputDir: restoreOut,
			})
			if err != nil {
				return err
			}
			return cache.PrintJSON(result)
		},
	}
	restoreCmd.Flags().StringVar(&restoreCacheDir, "cache-dir", "", "cache directory")
	restoreCmd.Flags().StringVar(&restoreEntry, "entry-file", "", "relative cache entry path")
	restoreCmd.Flags().StringVar(&restoreOut, "output-dir", "", "directory for restored markdown")
	_ = restoreCmd.MarkFlagRequired("cache-dir")
	_ = restoreCmd.MarkFlagRequired("entry-file")
	_ = restoreCmd.MarkFlagRequired("output-dir")
	cmd.AddCommand(restoreCmd)

	return cmd
}

func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Convert review reports (junit, html, all-diffs)",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "junit <review-output-dir> <junit-output-dir>",
		Short: "Convert findings to JUnit XML",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return report.ConvertToJUnit(args[0], args[1])
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "html <input.md> <output.html>",
		Short: "Convert markdown reports to HTML",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return report.ConvertMarkdownToHTMLCLI(args[0], args[1])
		},
	})
	var capLines int
	allDiffsCmd := &cobra.Command{
		Use:   "all-diffs <manifest.json> <output-file>",
		Short: "Concatenate per-file staging diffs into all-diffs.txt",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := diffpkg.BuildAllOptions{Manifest: args[0], Output: args[1]}
			if cmd.Flags().Changed("cap") {
				c := capLines
				opts.Cap = &c
			}
			return diffpkg.BuildAll(opts)
		},
	}
	allDiffsCmd.Flags().IntVar(&capLines, "cap", 0, "truncate each file diff to N lines")
	cmd.AddCommand(allDiffsCmd)
	return cmd
}

func newBuildSAToolsCmd() *cobra.Command {
	var dryRun, verbose, corp bool
	cmd := &cobra.Command{
		Use:   "build-sa-tools",
		Short: "Build local SA tool Docker images to validate Dockerfiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return satools.Run(satools.Options{DryRun: dryRun, Verbose: verbose, Corp: corp})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list tools without building")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print full build output on success")
	cmd.Flags().BoolVar(&corp, "corp", false, "corporate registry mode (PACKAGE_REGISTRY_* + credentials)")
	return cmd
}

func newSubmoduleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "submodule",
		Short: "Interactive manager for a vendored .majordomo submodule",
		RunE: func(cmd *cobra.Command, args []string) error {
			return submodule.Run(submodule.Options{})
		},
	}
}

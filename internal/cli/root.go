// Package cli wires majordomo subcommands.
package cli

import (
	"fmt"

	"github.com/behaviorengineering/majordomo/internal/staging"
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
	root.AddCommand(newOrchestrateCmd())
	root.AddCommand(newPublishCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newCacheCmd())
	root.AddCommand(newReportCmd())

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
	return stub("poll", "Poll SCM APIs for open PRs/MRs that need review (reconciliation)")
}

func newPrepCmd() *cobra.Command {
	var routing, agentContext, summaryConfig string
	cmd := &cobra.Command{
		Use:   "prep <base-branch> <staging-dir>",
		Short: "Classify diffs, cluster files, write staging manifest",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := staging.Options{
				BaseBranch:        args[0],
				StagingDir:        args[1],
				RoutingPath:       routing,
				AgentContextPath:  agentContext,
				SummaryConfigPath: summaryConfig,
			}
			return staging.Run(opts)
		},
	}
	cmd.Flags().StringVar(&routing, "routing", "", "path to routing JSON")
	cmd.Flags().StringVar(&agentContext, "agent-context", "", "path to agent context JSON")
	cmd.Flags().StringVar(&summaryConfig, "summary-config", "", "path to summary config JSON")
	return cmd
}

func newOrchestrateCmd() *cobra.Command {
	return stub("orchestrate", "Run review waves, checkpoints, finalize")
}

func newPublishCmd() *cobra.Command {
	return stub("publish", "Publish summary to PR/MR (github|gitlab|bitbucket)")
}

func newStatusCmd() *cobra.Command {
	return stub("status", "Post commit/check status to SCM")
}

func newCacheCmd() *cobra.Command {
	return stub("cache", "Review and poll cache read/write on served repo")
}

func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Convert review reports (junit, html)",
	}
	cmd.AddCommand(stub("junit", "Convert findings to JUnit XML"))
	cmd.AddCommand(stub("html", "Convert markdown reports to HTML"))
	return cmd
}

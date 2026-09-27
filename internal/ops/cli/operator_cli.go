package cli

import (
	"fmt"
	"path/filepath"

	"github.com/behaviorengineering/majordomo/pkg/platform/operatorcfg"
	"github.com/spf13/cobra"
)

var operatorConfigPath string

func addOperatorFlags(root *cobra.Command) {
	root.PersistentFlags().StringVar(&operatorConfigPath, "config", "", "path to operator config (~/.config/majordomo/config.yaml; or MAJORDOMO_CONFIG)")
	root.PersistentPreRunE = persistentOperatorBootstrap
	root.AddCommand(newInitCmd())
}

func persistentOperatorBootstrap(cmd *cobra.Command, _ []string) error {
	if skipOperatorBootstrap(cmd) {
		return nil
	}
	_, err := operatorcfg.Apply(operatorcfg.Options{
		ConfigFlagPath: operatorConfigPath,
	})
	return err
}

func skipOperatorBootstrap(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "init", "version", "context":
			return true
		}
	}
	return false
}

func towerConfigDir(flagValue string) string {
	return operatorcfg.EffectiveConfigDir(flagValue)
}

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create ~/.config/majordomo/config.yaml from the embedded example",
		Long: `Create the XDG operator config directory and seed config.yaml when missing.

Refreshes config.yaml.example on every run. Use --force to overwrite an existing live config.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			written, path, err := operatorcfg.InitUserConfig(force)
			if err != nil {
				return err
			}
			if written {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Config already exists at %s (use --force to overwrite)\n", path)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", filepath.Join(filepath.Dir(path), "config.yaml.example"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config.yaml")
	return cmd
}

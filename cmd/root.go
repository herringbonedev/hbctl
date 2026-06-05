package cmd

import (
	"os"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

var (
	projectName        = "herringbone"
	secretsDirOverride = ""
	rootCmd            = &cobra.Command{
		Use:           "hbctl",
		Short:         "Control and manage a Herringbone deployment",
		Long:          "hbctl manages Herringbone services, units, receivers, secrets, and local Docker Compose workflows.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		ui.FError(os.Stderr, "%v", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(func() {
		secrets.SetBaseDir(secretsDirOverride)
	})

	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.PersistentFlags().StringVar(&projectName, "project", "herringbone", "Compose project name")
	rootCmd.PersistentFlags().StringVar(&secretsDirOverride, "secrets", "", "Use an alternate hbctl secrets directory instead of the default")

	rootCmd.AddCommand(versionCommand())
	rootCmd.AddCommand(elementsCommand())
	rootCmd.AddCommand(unitsCommand())
	rootCmd.AddCommand(startCommand())
	rootCmd.AddCommand(stopCommand())
	rootCmd.AddCommand(restartCommand())
	rootCmd.AddCommand(upgradeCommand())
	rootCmd.AddCommand(statusCommand())
	rootCmd.AddCommand(pruneCommand())
	rootCmd.AddCommand(logsCommand())
	rootCmd.AddCommand(loginCommand())
	rootCmd.AddCommand(receiverCommand())
	rootCmd.AddCommand(releasesCommand())
}

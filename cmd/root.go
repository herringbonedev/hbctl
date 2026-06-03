package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	projectName = "herringbone"
	rootCmd     = &cobra.Command{
		Use:           "hbctl",
		Short:         "Control and manage a Herringbone deployment",
		Long:          "hbctl manages Herringbone services, units, receivers, secrets, and local Docker Compose workflows.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.PersistentFlags().StringVar(&projectName, "project", "herringbone", "Compose project name")

	rootCmd.AddCommand(versionCommand())
	rootCmd.AddCommand(elementsCommand())
	rootCmd.AddCommand(unitsCommand())
	rootCmd.AddCommand(startCommand())
	rootCmd.AddCommand(stopCommand())
	rootCmd.AddCommand(restartCommand())
	rootCmd.AddCommand(upgradeCommand())
	rootCmd.AddCommand(statusCommand())
	rootCmd.AddCommand(logsCommand())
	rootCmd.AddCommand(loginCommand())
	rootCmd.AddCommand(receiverCommand())
}

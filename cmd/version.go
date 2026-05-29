package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "alpha-0.5.0"

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show hbctl version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "hbctl version %s\n", Version)
		},
	}
}

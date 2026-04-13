package cmd

import (
	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func logsCommand() *cobra.Command {
	var unit string
	var follow bool
	var tail int

	cmd := &cobra.Command{
		Use:   "logs [element ...]",
		Short: "Show logs for elements or a unit",
		RunE: func(cmd *cobra.Command, args []string) error {
			return local.Logs(local.LogsOptions{
				Project:  projectName,
				Unit:     unit,
				Follow:   follow,
				Tail:     tail,
				Elements: args,
			})
		},
	}

	cmd.Flags().StringVar(&unit, "unit", "", "Unit to show logs for")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVar(&tail, "tail", 0, "Number of lines from the end of logs")
	return cmd
}

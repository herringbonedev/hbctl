package cmd

import (
	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func statusCommand() *cobra.Command {
	var unit string
	var asJSON bool
	var showAll bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status for the active stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return local.Status(local.StatusOptions{
				Project: projectName,
				Unit:    unit,
				JSON:    asJSON,
				All:     showAll,
			})
		},
	}

	cmd.Flags().StringVar(&unit, "unit", "", "Unit to filter by")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&showAll, "all", false, "Include stopped/exited containers in the status table")
	return cmd
}

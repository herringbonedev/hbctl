package cmd

import (
	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func pruneCommand() *cobra.Command {
	var core bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove stopped Herringbone containers without removing volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return local.Prune(local.PruneOptions{
				Project: projectName,
				Core:    core,
			})
		},
	}

	cmd.Flags().BoolVar(&core, "core", false, "Also prune stopped MongoDB, proxy, and auth containers; volumes are still preserved")
	return cmd
}

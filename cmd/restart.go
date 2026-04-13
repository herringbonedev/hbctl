package cmd

import (
	"strings"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func restartCommand() *cobra.Command {
	var element string
	var all bool

	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart an element or the full stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				element = ""
			}
			return local.Restart(local.RestartOptions{Project: projectName, Element: strings.TrimSpace(element)})
		},
	}

	cmd.Flags().StringVar(&element, "element", "", "Element to restart")
	cmd.Flags().BoolVar(&all, "all", false, "Restart the full stack")
	return cmd
}

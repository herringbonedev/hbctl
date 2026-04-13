package cmd

import (
	"strings"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func stopCommand() *cobra.Command {
	var element string
	var all bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop an element or the full stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				element = ""
			}
			return local.Stop(local.StopOptions{Project: projectName, Element: strings.TrimSpace(element)})
		},
	}

	cmd.Flags().StringVar(&element, "element", "", "Element to stop")
	cmd.Flags().BoolVar(&all, "all", false, "Stop the full stack")
	return cmd
}

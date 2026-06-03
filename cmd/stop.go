package cmd

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func stopCommand() *cobra.Command {
	var element string
	var unit string
	var all bool
	var down bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop an element, a unit, or the full stack without destroying containers by default",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && strings.TrimSpace(element) == "" && strings.TrimSpace(unit) == "" {
				return fmt.Errorf("specify --element, --unit, or --all. Full-stack stop is no longer implicit")
			}
			return local.Stop(local.StopOptions{
				Project: projectName,
				Element: strings.TrimSpace(element),
				Unit:    strings.TrimSpace(unit),
				All:     all,
				Down:    down,
			})
		},
	}

	cmd.Flags().StringVar(&element, "element", "", "Element to stop")
	cmd.Flags().StringVar(&unit, "unit", "", "Unit to stop")
	cmd.Flags().BoolVar(&all, "all", false, "Stop the full stack")
	cmd.Flags().BoolVar(&down, "down", false, "Use docker compose down instead of stop. This removes containers/networks and must be explicit")
	return cmd
}

package cmd

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func restartCommand() *cobra.Command {
	var element string
	var unit string
	var all bool
	var enterprise bool

	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart an element, a unit, or the full stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && strings.TrimSpace(element) == "" && strings.TrimSpace(unit) == "" {
				return fmt.Errorf("specify --element, --unit, or --all. Full-stack restart is no longer implicit")
			}
			return local.Restart(local.RestartOptions{
				Project:    projectName,
				Element:    strings.TrimSpace(element),
				Unit:       strings.TrimSpace(unit),
				All:        all,
				Enterprise: enterprise,
			})
		},
	}

	cmd.Flags().StringVar(&element, "element", "", "Element to restart")
	cmd.Flags().StringVar(&unit, "unit", "", "Unit to restart")
	cmd.Flags().BoolVar(&all, "all", false, "Restart the full stack")
	cmd.Flags().BoolVar(&enterprise, "enterprise", false, "Restart enterprise services and set HB_ENTERPRISE=true")
	return cmd
}

package cmd

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func upgradeCommand() *cobra.Command {
	var element string
	var unit string
	var all bool
	var noPull bool
	var forceRecreate bool
	var enterprise bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Pull and recreate platform services without tearing down the deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && strings.TrimSpace(element) == "" && strings.TrimSpace(unit) == "" {
				return fmt.Errorf("specify --element, --unit, or --all")
			}
			return local.Upgrade(local.UpgradeOptions{
				Project:       projectName,
				Element:       strings.TrimSpace(element),
				Unit:          strings.TrimSpace(unit),
				All:           all,
				Pull:          !noPull,
				ForceRecreate: forceRecreate,
				Enterprise:    enterprise,
			})
		},
	}

	cmd.Flags().StringVar(&element, "element", "", "Element to upgrade")
	cmd.Flags().StringVar(&unit, "unit", "", "Unit to upgrade")
	cmd.Flags().BoolVar(&all, "all", false, "Upgrade the full stack")
	cmd.Flags().BoolVar(&noPull, "no-pull", false, "Skip docker compose pull before recreating services")
	cmd.Flags().BoolVar(&forceRecreate, "force-recreate", true, "Force container recreation during upgrade")
	cmd.Flags().BoolVar(&enterprise, "enterprise", true, "Use enterprise environment defaults")
	return cmd
}

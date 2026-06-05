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
	var proxy bool
	var mongo bool
	var auth bool
	var keepContainers bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop application services by default while protecting MongoDB, proxy, and auth",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && strings.TrimSpace(element) == "" && strings.TrimSpace(unit) == "" && !proxy && !mongo && !auth {
				return fmt.Errorf("specify --element, --unit, --all, --proxy, --mongo, or --auth. Full-stack stop is no longer implicit")
			}
			return local.Stop(local.StopOptions{
				Project:        projectName,
				Element:        strings.TrimSpace(element),
				Unit:           strings.TrimSpace(unit),
				All:            all,
				Down:           down,
				Proxy:          proxy,
				Mongo:          mongo,
				Auth:           auth,
				KeepContainers: keepContainers,
			})
		},
	}

	cmd.Flags().StringVar(&element, "element", "", "Element to stop")
	cmd.Flags().StringVar(&unit, "unit", "", "Unit to stop")
	cmd.Flags().BoolVar(&all, "all", false, "Stop application services while leaving protected core running")
	cmd.Flags().BoolVar(&proxy, "proxy", false, "Explicitly stop the protected proxy service")
	cmd.Flags().BoolVar(&mongo, "mongo", false, "Explicitly stop the protected MongoDB service without removing volumes")
	cmd.Flags().BoolVar(&auth, "auth", false, "Explicitly stop the protected auth service")
	cmd.Flags().BoolVar(&down, "down", false, "Deprecated for --all; hbctl now stops and prunes application containers without removing volumes")
	cmd.Flags().BoolVar(&keepContainers, "keep-containers", false, "Stop containers but leave stopped containers present instead of pruning them")
	return cmd
}

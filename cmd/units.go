package cmd

import (
	"encoding/json"
	"sort"

	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/herringbonedev/hbctl/internal/units"
	"github.com/spf13/cobra"
)

func unitsCommand() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "units",
		Short: "List available units",
		RunE: func(cmd *cobra.Command, args []string) error {
			var names []string
			for name := range units.ServiceUnits {
				names = append(names, name)
			}
			sort.Strings(names)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(names)
			}

			ui.FHeader(cmd.OutOrStdout(), "Herringbone units")
			rows := make([][]string, 0, len(names))
			for _, name := range names {
				rows = append(rows, []string{name})
			}
			ui.FTable(cmd.OutOrStdout(), []string{"UNIT"}, rows)
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

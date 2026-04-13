package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/herringbonedev/hbctl/internal/units"
	"github.com/spf13/cobra"
)

func elementsCommand() *cobra.Command {
	var asJSON bool
	var namesOnly bool
	var filter string
	var wide bool

	cmd := &cobra.Command{
		Use:   "elements",
		Short: "List available elements",
		RunE: func(cmd *cobra.Command, args []string) error {
			var out []units.ElementInfo
			if strings.TrimSpace(filter) == "" {
				out = append(out, units.AllElements...)
			} else {
				needle := strings.ToLower(strings.TrimSpace(filter))
				for _, element := range units.AllElements {
					if strings.Contains(strings.ToLower(element.Name), needle) ||
						strings.Contains(strings.ToLower(element.Description), needle) ||
						strings.Contains(strings.ToLower(element.Unit), needle) {
						out = append(out, element)
					}
				}
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			if namesOnly {
				for _, element := range out {
					fmt.Fprintln(cmd.OutOrStdout(), element.Name)
				}
				return nil
			}

			if wide {
				fmt.Fprintf(cmd.OutOrStdout(), "%-3s %-28s %-18s %s\n", "#", "NAME", "UNIT", "DESCRIPTION")
				for index, element := range out {
					fmt.Fprintf(cmd.OutOrStdout(), "%-3d %-28s %-18s %s\n", index+1, element.Name, element.Unit, element.Description)
				}
				return nil
			}

			grouped := map[string][]units.ElementInfo{}
			for _, element := range out {
				grouped[element.Unit] = append(grouped[element.Unit], element)
			}

			var unitNames []string
			for unitName := range grouped {
				unitNames = append(unitNames, unitName)
			}
			sort.Strings(unitNames)

			for _, unitName := range unitNames {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s]\n", unitName)
				for _, element := range grouped[unitName] {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-28s %s\n", element.Name, element.Description)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&namesOnly, "names", false, "Output only names")
	cmd.Flags().StringVar(&filter, "filter", "", "Filter by name, description, or unit")
	cmd.Flags().BoolVar(&wide, "wide", false, "Show a wide table")
	return cmd
}

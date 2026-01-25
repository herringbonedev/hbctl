package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/herringbonedev/hbctl/internal/units"
)

type elementInfo = units.ElementInfo

func init() {
	Register("elements", elementsCmd)
}

func elementsCmd(args []string) {
	fs := flag.NewFlagSet("elements", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "Output elements as JSON")
	namesOnly := fs.Bool("names", false, "Output only element names")
	filter := fs.String("filter", "", "Filter elements by name, description, or unit")
	wide := fs.Bool("wide", false, "Show wide table with units")
	_ = fs.Parse(args)

	var out []elementInfo
	if *filter == "" {
		out = units.AllElements
	} else {
		f := strings.ToLower(*filter)
		for _, e := range units.AllElements {
			if strings.Contains(strings.ToLower(e.Name), f) ||
				strings.Contains(strings.ToLower(e.Description), f) ||
				strings.Contains(strings.ToLower(e.Unit), f) {
				out = append(out, e)
			}
		}
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return
	}

	if *namesOnly {
		for _, e := range out {
			fmt.Println(e.Name)
		}
		return
	}

	if *wide {
		fmt.Printf("%-3s %-26s %-12s %s\n", "#", "NAME", "UNIT", "DESCRIPTION")
		for i, e := range out {
			fmt.Printf("%-3d %-26s %-12s %s\n", i+1, e.Name, e.Unit, e.Description)
		}
		return
	}

	grouped := map[string][]elementInfo{}
	for _, e := range out {
		grouped[e.Unit] = append(grouped[e.Unit], e)
	}

	var units []string
	for u := range grouped {
		units = append(units, u)
	}
	sort.Strings(units)

	fmt.Println("Elements grouped by unit:")

	for _, u := range units {
		fmt.Printf("\n[%s]\n", strings.ToUpper(u[:1])+u[1:])
		for _, e := range grouped[u] {
			fmt.Printf("  %-26s %s\n", e.Name, e.Description)
		}
	}
}


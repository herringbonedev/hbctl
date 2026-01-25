package cmd

import (
	"flag"
	"fmt"
	"sort"

	"github.com/herringbonedev/hbctl/internal/units"
)

func init() {
	Register("units", unitsCmd)
}

func unitsCmd(args []string) {
	fs := flag.NewFlagSet("units", flag.ExitOnError)
	fs.Parse(args)

	var names []string
	for u := range units.ServiceUnits {
		names = append(names, u)
	}

	sort.Strings(names)

	for _, u := range names {
		fmt.Println(u)
	}
}

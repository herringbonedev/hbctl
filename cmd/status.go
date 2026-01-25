package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/local"
)

func init() {
	Register("status", statusCmd)
}

func statusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	unit := fs.String("unit", "", "Unit (subsystem) to show status for (e.g. parser, detection, incidents)")
	asJSON := fs.Bool("json", false, "Output status as JSON")
	fs.Parse(args)

	if err := local.Status(local.StatusOptions{
		Project: composeProject,
		Unit:    *unit,
		JSON:    *asJSON,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

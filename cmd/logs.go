package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/local"
)

func init() {
	Register("logs", logsCmd)
}

func logsCmd(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)

	unit := fs.String("unit", "", "Unit (subsystem) to show logs for (e.g. parser, detection, incidents)")
	follow := fs.Bool("follow", false, "Follow log output")
	tail := fs.Int("tail", 0, "Number of lines to show from the end of logs")

	fs.Parse(args)

	elements := fs.Args()

	if err := local.Logs(local.LogsOptions{
		Project:  composeProject,
		Unit:     *unit,
		Follow:   *follow,
		Tail:     *tail,
		Elements: elements,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

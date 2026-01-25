package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/local"
)

func init() {
	Register("restart", restartCmd)
}

func restartCmd(args []string) {
	fs := flag.NewFlagSet("restart", flag.ExitOnError)
	element := fs.String("element", "", "Element (service) to restart (e.g. parser-extractor, herringbone-search)")
	fs.Parse(args)

	if err := local.Restart(local.RestartOptions{Project: composeProject, Element: *element}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

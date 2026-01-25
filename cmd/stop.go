package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/local"
)

func init() {
	Register("stop", stopCmd)
}

func stopCmd(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	element := fs.String("element", "", "Element (service) to stop (e.g. mongodb, parser-extractor, herringbone-search)")
	fs.Parse(args)

	if err := local.Stop(local.StopOptions{Project: composeProject, Element: *element}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

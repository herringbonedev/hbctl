package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/local"
)

func init() {
	Register("start", startCmd)
}

func startCmd(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)

	element := fs.String("element", "", "Element (service) to start")
	unit := fs.String("unit", "", "Unit (subsystem) to start")
	all := fs.Bool("all", false, "Start full Herringbone stack")
	recvType := fs.String("type", "", "Receiver type (UDP, TCP, HTTP)")

	fs.Parse(args)

	opts := local.StartOptions{
		Project:     composeProject,
		Element:     *element,
		Unit:        *unit,
		All:         *all,
		RecvType:    *recvType,
	}

	if err := local.Start(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

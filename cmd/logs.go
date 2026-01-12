package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/docker"
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

	// Remaining args are element names
	elements := fs.Args()

	env := map[string]string{
		"MONGO_USER":    "",
		"MONGO_PASS":    "",
		"DB_NAME":       "",
		"AUTH_DB":       "",
		"RECEIVER_TYPE": "",
	}

	var composeArgs []string
	composeArgs = append(composeArgs, "-p", composeProject)

	if *unit != "" {
		fmt.Println("[hbctl] Using unit:", *unit)
		services := unitElements[*unit]
		if len(services) == 0 {
			fmt.Fprintln(os.Stderr, "Unknown unit:", *unit)
			os.Exit(1)
		}
		elements = services
	}

	composeArgs = append(composeArgs, "logs")

	if *follow {
		composeArgs = append(composeArgs, "-f")
	}
	if *tail > 0 {
		composeArgs = append(composeArgs, "--tail", fmt.Sprint(*tail))
	}

	composeArgs = append(composeArgs, elements...)

	if err := docker.ComposeWithEnv(env, composeArgs...); err != nil {
		os.Exit(1)
	}
}

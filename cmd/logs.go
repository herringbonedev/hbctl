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

	profile := fs.String("profile", "", "Compose profile to show logs for (e.g. receiver, database)")
	follow := fs.Bool("follow", false, "Follow log output")
	tail := fs.Int("tail", 0, "Number of lines to show from the end of logs")

	fs.Parse(args)

	services := fs.Args() // optional service names

	// Empty env to silence compose warnings
	env := map[string]string{
		"MONGO_USER":    "",
		"MONGO_PASS":    "",
		"DB_NAME":       "",
		"AUTH_DB":       "",
		"RECEIVER_TYPE": "",
	}

	var composeArgs []string
	composeArgs = append(composeArgs, "-p", composeProject)

	if *profile != "" {
		fmt.Println("[hbctl] Using profile:", *profile)
		composeArgs = append(composeArgs, "--profile", *profile)
	}

	composeArgs = append(composeArgs, "logs")

	if *follow {
		composeArgs = append(composeArgs, "-f")
	}
	if *tail > 0 {
		composeArgs = append(composeArgs, "--tail", fmt.Sprint(*tail))
	}

	// Append service filters if provided
	composeArgs = append(composeArgs, services...)

	if err := docker.ComposeWithEnv(env, composeArgs...); err != nil {
		os.Exit(1)
	}
}

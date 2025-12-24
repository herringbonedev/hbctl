package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/docker"
)

func init() {
	Register("restart", restartCmd)
}

func restartCmd(args []string) {
	fs := flag.NewFlagSet("restart", flag.ExitOnError)
	profile := fs.String("profile", "", "Profile to restart (database, receiver, logs, parser-cardset, parser-enrichment, parser-extractor)")
	fs.Parse(args)

	env := map[string]string{
		"MONGO_ROOT_PASS": "",
		"MONGO_HOST":      "",
		"MONGO_USER":      "",
		"MONGO_PASS":      "",
		"DB_NAME":         "",
		"AUTH_DB":         "",
		"RECEIVER_TYPE":   "",
	}

	if *profile == "" {
		fmt.Println("[hbctl] Restarting all containers...")

		args := []string{"-p", composeProject}
		args = append(args, composeFilesForProfile("receiver")...)
		args = append(args, composeFilesForProfile("logs")...)
		args = append(args, composeFilesForProfile("parser-cardset")...)
		args = append(args, composeFilesForProfile("parser-enrichment")...)
		args = append(args, composeFilesForProfile("parser-extractor")...)
		args = append(args, "restart")

		if err := docker.ComposeWithEnv(env, args...); err != nil {
			os.Exit(1)
		}
		return
	}

	var service string

	switch *profile {
	case "receiver":
		service = "logingestion-receiver"
	case "logs":
		service = "herringbone-logs"
	case "parser-cardset":
		service = "parser-cardset"
	case "parser-enrichment":
		service = "parser-enrichment"
	case "parser-extractor":
		service = "parser-extractor"
	case "database":
		service = "mongodb"
	default:
		fmt.Fprintln(os.Stderr,
			"Error: --profile must be one of: database, receiver, logs, parser-cardset, parser-enrichment, parser-extractor")
		os.Exit(1)
	}

	fmt.Println("[hbctl] Restarting", service, "...")

	composeArgs := []string{"-p", composeProject}
	composeArgs = append(composeArgs, composeFilesForProfile(*profile)...)
	composeArgs = append(composeArgs, "restart", service)

	if err := docker.ComposeWithEnv(env, composeArgs...); err != nil {
		os.Exit(1)
	}
}

package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/docker"
)

func init() {
	Register("stop", stopCmd)
}

func stopCmd(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	profile := fs.String("profile", "", "Profile/service to stop (e.g. mongodb, logingestion-receiver, herringbone-logs)")
	fs.Parse(args)

	env := map[string]string{
		"MONGO_ROOT_PASS": "",
		"MONGO_HOST":      "",
		"MONGO_PORT":     "",
		"MONGO_USER":      "",
		"MONGO_PASS":      "",
		"DB_NAME":         "",
		"AUTH_DB":         "",
		"RECEIVER_TYPE":   "",
		"MATCHER_API":     "",
	}

	composeArgs := []string{
		"-p", composeProject,
	}

	if *profile != "" {
		fmt.Println("[hbctl] Stopping", *profile, "...")
		composeArgs = append(composeArgs, composeFilesForProfile(*profile)...)
		composeArgs = append(composeArgs, "stop", *profile)
	} else {
		fmt.Println("[hbctl] Stopping full Herringbone stack...")
		composeArgs = append(composeArgs, "down")
	}

	if err := docker.ComposeWithEnv(env, composeArgs...); err != nil {
		os.Exit(1)
	}
}

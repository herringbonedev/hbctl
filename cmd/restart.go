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
	profile := fs.String("profile", "", "Profile/service to restart")
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
		fmt.Println("[hbctl] Restarting", *profile, "...")
		composeArgs = append(composeArgs, composeFilesForProfile(*profile)...)
		composeArgs = append(composeArgs, "restart", *profile)
	} else {
		fmt.Println("[hbctl] Restarting full Herringbone stack...")
		composeArgs = append(composeArgs, "restart")
	}

	if err := docker.ComposeWithEnv(env, composeArgs...); err != nil {
		os.Exit(1)
	}
}

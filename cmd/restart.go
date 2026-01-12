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
	element := fs.String("element", "", "Element (service) to restart (e.g. parser-extractor, herringbone-search)")
	fs.Parse(args)

	env := map[string]string{
		"MONGO_ROOT_PASS": "",
		"MONGO_HOST":      "",
		"MONGO_PORT":      "",
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

	if *element != "" {
		fmt.Println("[hbctl] Restarting element:", *element)
		composeArgs = append(composeArgs, composeFilesForElement(*element)...)
		composeArgs = append(composeArgs, "restart", *element)
	} else {
		fmt.Println("[hbctl] Restarting full Herringbone stack...")
		composeArgs = append(composeArgs, "restart")
	}

	if err := docker.ComposeWithEnv(env, composeArgs...); err != nil {
		os.Exit(1)
	}
}

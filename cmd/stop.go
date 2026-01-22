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
	element := fs.String("element", "", "Element (service) to stop (e.g. mongodb, parser-extractor, herringbone-search)")
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
		fmt.Println("[hbctl] Stopping element:", *element)
		composeArgs = append(composeArgs, composeFilesForElement(*element)...)
		composeArgs = append(composeArgs, "stop", *element)
	} else {
		fmt.Println("[hbctl] Stopping full Herringbone stack...")
		composeArgs = append(composeArgs, "down")
	}

	if err := docker.ComposeWithEnv(env, composeArgs...); err != nil {
		os.Exit(1)
	}
}

package local

import (
	"fmt"

	"github.com/herringbonedev/hbctl/internal/docker"
)

type StopOptions struct {
	Project string
	Element string
}

func Stop(opts StopOptions) error {
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
		"-p", opts.Project,
	}

	if opts.Element != "" {
		fmt.Println("[hbctl] Stopping element:", opts.Element)
		composeArgs = append(composeArgs, ComposeFilesForElement(opts.Element)...)
		composeArgs = append(composeArgs, "stop", opts.Element)
		return docker.ComposeWithEnv(env, composeArgs...)
	}

	fmt.Println("[hbctl] Stopping full Herringbone stack...")
	composeArgs = append(composeArgs, "down")
	return docker.ComposeWithEnv(env, composeArgs...)
}

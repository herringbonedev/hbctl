package local

import (
	"fmt"
	"strconv"

	"github.com/herringbonedev/hbctl/internal/docker"
	"github.com/herringbonedev/hbctl/internal/units"
)

type LogsOptions struct {
	Project  string
	Unit     string
	Follow   bool
	Tail     int
	Elements []string
}

func Logs(opts LogsOptions) error {
	env := map[string]string{
		"MONGO_USER":    "",
		"MONGO_PASS":    "",
		"DB_NAME":       "",
		"AUTH_DB":       "",
		"RECEIVER_TYPE": "",
	}

	composeArgs := []string{"-p", opts.Project}

	// If --unit specified, expand to elements
	if opts.Unit != "" {
		fmt.Println("[hbctl] Showing logs for unit:", opts.Unit)
		els := units.UnitElements[opts.Unit]
		if len(els) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}
		for _, e := range els {
			composeArgs = append(composeArgs, ComposeFilesForElement(e)...)
		}
		composeArgs = append(composeArgs, "logs")
		if opts.Follow {
			composeArgs = append(composeArgs, "-f")
		}
		if opts.Tail > 0 {
			composeArgs = append(composeArgs, "--tail", strconv.Itoa(opts.Tail))
		}
		composeArgs = append(composeArgs, els...)
		return docker.ComposeWithEnv(env, composeArgs...)
	}

	// Otherwise, explicit elements from args
	if len(opts.Elements) > 0 {
		for _, e := range opts.Elements {
			composeArgs = append(composeArgs, ComposeFilesForElement(e)...)
		}
		composeArgs = append(composeArgs, "logs")
		if opts.Follow {
			composeArgs = append(composeArgs, "-f")
		}
		if opts.Tail > 0 {
			composeArgs = append(composeArgs, "--tail", strconv.Itoa(opts.Tail))
		}
		composeArgs = append(composeArgs, opts.Elements...)
		return docker.ComposeWithEnv(env, composeArgs...)
	}

	return fmt.Errorf("error: specify --unit or one or more element names")
}

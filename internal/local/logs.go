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
	env := blankLifecycleEnv(true)
	composeArgs := []string{"-p", opts.Project}

	if opts.Unit != "" {
		fmt.Println("[hbctl] Showing logs for unit:", opts.Unit)
		els := units.UnitElements[opts.Unit]
		if len(els) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}
		composeArgs = append(composeArgs, ComposeFilesForElements(els)...)
		composeArgs = append(composeArgs, "logs")
		if opts.Follow {
			composeArgs = append(composeArgs, "-f")
		}
		if opts.Tail > 0 {
			composeArgs = append(composeArgs, "--tail", strconv.Itoa(opts.Tail))
		}
		for _, el := range els {
			composeArgs = append(composeArgs, CanonicalElementName(el))
		}
		return docker.ComposeWithEnv(env, composeArgs...)
	}

	if len(opts.Elements) > 0 {
		canonical := make([]string, 0, len(opts.Elements))
		for _, e := range opts.Elements {
			canonical = append(canonical, CanonicalElementName(e))
		}
		composeArgs = append(composeArgs, ComposeFilesForElements(canonical)...)
		composeArgs = append(composeArgs, "logs")
		if opts.Follow {
			composeArgs = append(composeArgs, "-f")
		}
		if opts.Tail > 0 {
			composeArgs = append(composeArgs, "--tail", strconv.Itoa(opts.Tail))
		}
		composeArgs = append(composeArgs, canonical...)
		return docker.ComposeWithEnv(env, composeArgs...)
	}

	return fmt.Errorf("error: specify --unit or one or more element names")
}

package local

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/docker"
	"github.com/herringbonedev/hbctl/internal/units"
)

type RestartOptions struct {
	Project string
	Element string
	Unit    string
	All     bool
}

func Restart(opts RestartOptions) error {
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
		"HB_ENTERPRISE":   "true",
	}

	if opts.Element != "" {
		return restartElement(opts.Project, env, opts.Element)
	}

	if opts.Unit != "" {
		elements := units.UnitElements[strings.TrimSpace(opts.Unit)]
		if len(elements) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}
		for _, element := range elements {
			if err := restartElement(opts.Project, env, element); err != nil {
				return err
			}
		}
		return nil
	}

	if opts.All {
		fmt.Println("[hbctl] Restarting full Herringbone stack...")
		composeArgs := []string{"-p", opts.Project}
		composeArgs = append(composeArgs, ComposeFilesForFullStack()...)
		composeArgs = append(composeArgs, "restart")
		return docker.ComposeWithEnv(env, composeArgs...)
	}

	return fmt.Errorf("specify --element, --unit, or --all")
}

func restartElement(project string, env map[string]string, element string) error {
	element = CanonicalElementName(element)
	fmt.Println("[hbctl] Restarting element:", element)
	composeArgs := []string{"-p", project}
	composeArgs = append(composeArgs, ComposeFilesForElement(element)...)
	composeArgs = append(composeArgs, "restart", element)
	return docker.ComposeWithEnv(env, composeArgs...)
}

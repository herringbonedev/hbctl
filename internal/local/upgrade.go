package local

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/docker"
	"github.com/herringbonedev/hbctl/internal/units"
)

type UpgradeOptions struct {
	Project       string
	Element       string
	Unit          string
	All           bool
	Pull          bool
	ForceRecreate bool
	Enterprise    bool
}

func Upgrade(opts UpgradeOptions) error {
	env, err := mongoLifecycleEnv(opts.Enterprise)
	if err != nil {
		return err
	}

	if opts.Element != "" {
		return upgradeElement(opts.Project, env, opts.Element, opts.Pull, opts.ForceRecreate)
	}

	if opts.Unit != "" {
		elements := units.UnitElements[strings.TrimSpace(opts.Unit)]
		if len(elements) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}
		for _, element := range elements {
			if err := upgradeElement(opts.Project, env, element, opts.Pull, opts.ForceRecreate); err != nil {
				return err
			}
		}
		return nil
	}

	if opts.All {
		composeArgs := []string{"-p", opts.Project}
		composeArgs = append(composeArgs, ComposeFilesForFullStack()...)
		if len(composeArgs) == 2 {
			return fmt.Errorf("no compose files found for full-stack upgrade")
		}

		if opts.Pull {
			fmt.Println("[hbctl] Pulling latest images for full stack...")
			pullArgs := append([]string{}, composeArgs...)
			pullArgs = append(pullArgs, "pull")
			if err := docker.ComposeWithEnv(env, pullArgs...); err != nil {
				return err
			}
		}

		fmt.Println("[hbctl] Recreating full stack cleanly without compose down...")
		upArgs := append([]string{}, composeArgs...)
		upArgs = append(upArgs, "up", "-d", "--remove-orphans")
		if opts.ForceRecreate {
			upArgs = append(upArgs, "--force-recreate")
		}
		return docker.ComposeWithEnv(env, upArgs...)
	}

	return fmt.Errorf("specify --element, --unit, or --all")
}

func upgradeElement(project string, env map[string]string, element string, pull bool, forceRecreate bool) error {
	element = CanonicalElementName(element)
	composeArgs := []string{"-p", project}
	composeArgs = append(composeArgs, ComposeFilesForElement(element)...)

	if pull {
		fmt.Println("[hbctl] Pulling latest image for", element, "...")
		pullArgs := append([]string{}, composeArgs...)
		pullArgs = append(pullArgs, "pull", element)
		if err := docker.ComposeWithEnv(env, pullArgs...); err != nil {
			return err
		}
	}

	fmt.Println("[hbctl] Recreating", element, "without tearing down dependencies...")
	upArgs := append([]string{}, composeArgs...)
	upArgs = append(upArgs, "up", "-d", "--no-deps")
	if forceRecreate {
		upArgs = append(upArgs, "--force-recreate")
	}
	upArgs = append(upArgs, element)
	return docker.ComposeWithEnv(env, upArgs...)
}

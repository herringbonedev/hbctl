package local

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/herringbonedev/hbctl/internal/docker"
	"github.com/herringbonedev/hbctl/internal/ui"
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
	ui.Header("Herringbone logs")

	if opts.Unit != "" {
		ui.Section("Unit")
		ui.KeyValues([][2]string{{"unit", opts.Unit}, {"follow", ui.Bool(opts.Follow)}, {"tail", fmt.Sprintf("%d", opts.Tail)}})
		els := units.UnitElements[opts.Unit]
		if len(els) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}
		operable, err := operableElements(els)
		if err != nil {
			return err
		}
		if len(operable) == 0 {
			ui.Warn("No available elements found for unit %s", opts.Unit)
			return nil
		}
		files := ComposeFilesForElements(operable)
		services, err := resolveComposeServiceNamesForLogs(files, operable)
		if err != nil {
			return err
		}
		composeArgs = append(composeArgs, files...)
		composeArgs = append(composeArgs, "logs")
		if opts.Follow {
			composeArgs = append(composeArgs, "-f")
		}
		if opts.Tail > 0 {
			composeArgs = append(composeArgs, "--tail", strconv.Itoa(opts.Tail))
		}
		composeArgs = append(composeArgs, services...)
		return docker.ComposeWithEnv(env, composeArgs...)
	}

	if len(opts.Elements) > 0 {
		canonical, err := operableElements(opts.Elements)
		if err != nil {
			return err
		}
		if len(canonical) == 0 {
			ui.Warn("No available elements found")
			return nil
		}
		ui.Section("Elements")
		ui.KeyValues([][2]string{{"elements", fmt.Sprintf("%d", len(canonical))}, {"follow", ui.Bool(opts.Follow)}, {"tail", fmt.Sprintf("%d", opts.Tail)}})
		files := ComposeFilesForElements(canonical)
		services, err := resolveComposeServiceNamesForLogs(files, canonical)
		if err != nil {
			return err
		}
		composeArgs = append(composeArgs, files...)
		composeArgs = append(composeArgs, "logs")
		if opts.Follow {
			composeArgs = append(composeArgs, "-f")
		}
		if opts.Tail > 0 {
			composeArgs = append(composeArgs, "--tail", strconv.Itoa(opts.Tail))
		}
		composeArgs = append(composeArgs, services...)
		return docker.ComposeWithEnv(env, composeArgs...)
	}

	return fmt.Errorf("error: specify --unit or one or more element names")
}

func resolveComposeServiceNamesForLogs(composeFiles []string, elements []string) ([]string, error) {
	services := make([]string, 0, len(elements))
	seen := map[string]bool{}
	for _, element := range elements {
		element = CanonicalElementName(strings.TrimSpace(element))
		if element == "" {
			continue
		}
		service, err := resolveComposeServiceName(composeFiles, element)
		if err != nil {
			return nil, err
		}
		service = strings.TrimSpace(service)
		if service == "" || seen[service] {
			continue
		}
		seen[service] = true
		services = append(services, service)
	}
	return services, nil
}

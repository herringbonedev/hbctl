package local

import (
	"fmt"
	"strconv"

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
		composeArgs = append(composeArgs, ComposeFilesForElements(operable)...)
		composeArgs = append(composeArgs, "logs")
		if opts.Follow {
			composeArgs = append(composeArgs, "-f")
		}
		if opts.Tail > 0 {
			composeArgs = append(composeArgs, "--tail", strconv.Itoa(opts.Tail))
		}
		composeArgs = append(composeArgs, operable...)
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

package local

import "github.com/herringbonedev/hbctl/internal/ui"

type PruneOptions struct {
	Project string
	Core    bool
}

func Prune(opts PruneOptions) error {
	ui.Header("Herringbone prune")
	ui.Plan("Safe prune policy", []string{
		"Only stopped containers are removed.",
		"Docker volumes are never removed.",
		"Running MongoDB, proxy, and auth are never stopped by prune.",
		"Stopped protected core containers are skipped unless --core is set.",
	})
	return pruneStoppedContainers(pruneContainerOptions{
		Project:          opts.Project,
		IncludeProtected: opts.Core,
	})
}

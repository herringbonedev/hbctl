package local

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/docker"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/herringbonedev/hbctl/internal/units"
)

type StopOptions struct {
	Project        string
	Element        string
	Unit           string
	All            bool
	Down           bool
	Proxy          bool
	Mongo          bool
	Auth           bool
	KeepContainers bool
}

func Stop(opts StopOptions) error {
	env := blankLifecycleEnv(true)
	ui.Header("Herringbone stop")

	if opts.Element != "" {
		return stopElement(opts.Project, env, opts.Element, !opts.KeepContainers)
	}

	if opts.Unit != "" {
		elements := units.UnitElements[strings.TrimSpace(opts.Unit)]
		if len(elements) == 0 {
			return fmt.Errorf("unknown unit: %s", opts.Unit)
		}
		ui.Section("Unit")
		ui.KeyValues([][2]string{{"unit", opts.Unit}, {"elements", fmt.Sprintf("%d", len(elements))}})
		operable, err := operableElements(elements)
		if err != nil {
			return err
		}
		for _, element := range operable {
			if err := stopElement(opts.Project, env, element, !opts.KeepContainers); err != nil {
				return err
			}
		}
		ui.Success("Unit %s stopped", opts.Unit)
	}

	if opts.All {
		if err := stopApplicationContainers(opts.Project, env, opts.Down, opts.KeepContainers); err != nil {
			return err
		}
	}

	if opts.Proxy || opts.Mongo || opts.Auth {
		ui.Section("Explicit protected stops")
		if opts.Proxy {
			if err := stopElement(opts.Project, env, "herringbone-proxy", !opts.KeepContainers); err != nil {
				return err
			}
		}
		if opts.Auth {
			if err := stopAuthServices(opts.Project, !opts.KeepContainers); err != nil {
				return err
			}
		}
		if opts.Mongo {
			ui.Warn("MongoDB stop was requested explicitly. Data volumes are not removed.")
			if err := stopElement(opts.Project, env, "mongodb", !opts.KeepContainers); err != nil {
				return err
			}
		}
	}

	if !opts.All && !opts.Proxy && !opts.Mongo && !opts.Auth && opts.Unit == "" {
		return fmt.Errorf("specify --element, --unit, --all, --proxy, --mongo, or --auth")
	}

	return nil
}

func stopAuthServices(project string, prune bool) error {
	services := []string{"herringbone-auth"}
	found := []herringboneContainer{}
	for _, service := range services {
		containers, err := containersForService(project, service, true)
		if err != nil {
			return err
		}
		found = append(found, containers...)
	}

	if len(found) == 0 {
		ui.Skip("auth: no existing auth containers found")
		return nil
	}

	running := []herringboneContainer{}
	for _, container := range found {
		if isRunningContainer(container) {
			running = append(running, container)
		}
	}
	if len(running) > 0 {
		ui.Step("Stopping auth container(s)")
		if err := stopContainers(running); err != nil {
			return err
		}
		ui.Success("Stopped %d auth container(s)", len(running))
	} else {
		ui.Info("Auth container(s) already stopped")
	}

	if prune {
		return pruneStoppedContainers(pruneContainerOptions{Project: project, IncludeProtected: true, Services: services, QuietIfEmpty: true})
	}
	return nil
}

func stopApplicationContainers(project string, env map[string]string, down bool, keepContainers bool) error {
	ui.Section("Application services")
	ui.Plan("Protected stop policy", []string{
		"MongoDB is not stopped by --all.",
		"Proxy is not stopped by --all.",
		"Auth is not stopped by --all.",
		"Use hbctl stop --mongo, --proxy, or --auth when you intentionally want one of those stopped.",
		"Application stop uses Docker container discovery as the final source of truth so orphaned replicas and dedicated receiver projects are stopped too.",
		"Stopped application containers are pruned by default with docker rm. Docker volumes are never removed.",
	})
	if down {
		ui.Warn("--down is ignored for protected --all stops. hbctl will stop and prune app containers without removing volumes.")
	}

	// First try compose-level stops for known application services. These are
	// intentionally best-effort: optional compose files and older alpha service
	// names can drift, but that must never prevent the final container sweep from
	// stopping app containers such as operations-center replicas.
	composeStopWarnings := []string{}
	for _, element := range fullStackStopElements() {
		if err := stopElement(project, env, element, !keepContainers); err != nil {
			msg := fmt.Sprintf("%s: %v", element, err)
			composeStopWarnings = append(composeStopWarnings, msg)
			ui.Warn("Compose stop skipped: %s", msg)
		}
	}

	if len(composeStopWarnings) > 0 {
		ui.Section("Compose stop warnings")
		ui.Plan("Continuing with container discovery", composeStopWarnings)
	}

	if err := stopDiscoveredApplicationContainers(project); err != nil {
		return err
	}

	if keepContainers {
		ui.Info("Stopped containers were kept because --keep-containers was set")
	} else {
		if err := pruneStoppedContainers(pruneContainerOptions{Project: project, IncludeProtected: false}); err != nil {
			return err
		}
	}

	ui.Success("Application stop complete")
	return nil
}

func stopDiscoveredApplicationContainers(project string) error {
	containers, err := listHerringboneContainers(project, false)
	if err != nil {
		return err
	}

	runningApps := []herringboneContainer{}
	protectedRunning := []herringboneContainer{}
	for _, container := range containers {
		if !isRunningContainer(container) {
			continue
		}
		if isProtectedCoreService(container.Service) {
			protectedRunning = append(protectedRunning, container)
			continue
		}
		runningApps = append(runningApps, container)
	}

	if len(runningApps) > 0 {
		ui.Section("Running application containers")
		tableRows := make([][]string, 0, len(runningApps))
		for _, container := range runningApps {
			tableRows = append(tableRows, []string{container.Service, container.Project, container.State, container.Name})
		}
		ui.Table([]string{"SERVICE", "PROJECT", "STATE", "NAME"}, tableRows)
		ui.Step("Stopping discovered application containers")
		if err := stopContainers(runningApps); err != nil {
			return err
		}
		ui.Success("Stopped %d discovered application container(s)", len(runningApps))
	} else {
		ui.Success("No running application containers found")
	}

	if len(protectedRunning) > 0 {
		ui.Section("Protected core left running")
		tableRows := make([][]string, 0, len(protectedRunning))
		for _, container := range protectedRunning {
			tableRows = append(tableRows, []string{container.Service, container.Project, container.State, container.Name})
		}
		ui.Table([]string{"SERVICE", "PROJECT", "STATE", "NAME"}, tableRows)
	}

	return nil
}

func fullStackStopElements() []string {
	return []string{
		"herringbone-logs",
		"herringbone-search",
		"fingerprint-scoreset",
		"fingerprint-identifier",
		"parser-cardset",
		"parser-enrichment",
		"parser-extractor",
		"detectionengine-detector",
		"detectionengine-matcher",
		"detectionengine-ruleset",
		"incidents-incidentset",
		"incidents-correlator",
		"incidents-orchestrator",
		"operations-center",
		"logingestion-receiver",
	}
}

func stopElement(project string, env map[string]string, element string, prune bool) error {
	element = CanonicalElementName(element)
	start, reason, err := shouldStartElement(element, ComposeFilesForElement(element))
	if err != nil {
		return err
	}
	if !start {
		ui.Skip("%s: %s", element, reason)
		return nil
	}
	ui.Step("Stopping %s", element)
	composeArgs := []string{"-p", project}
	composeArgs = append(composeArgs, ComposeFilesForElement(element)...)
	service, err := resolveComposeServiceName(ComposeFilesForElement(element), element)
	if err != nil {
		return err
	}
	composeArgs = append(composeArgs, "stop", service)
	if err := docker.ComposeWithEnv(env, composeArgs...); err != nil {
		return err
	}
	ui.Success("%s stopped", element)
	if prune {
		if err := pruneStoppedContainers(pruneContainerOptions{Project: project, IncludeProtected: true, Services: []string{element}, QuietIfEmpty: true}); err != nil {
			return err
		}
	}
	return nil
}

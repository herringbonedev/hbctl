package local

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/herringbonedev/hbctl/internal/ui"
)

type herringboneContainer struct {
	ID         string
	Image      string
	Name       string
	Service    string
	RawService string
	Project    string
	State      string
	Status     string
	Ports      string
	Labels     map[string]string
}

type dockerContainerRow struct {
	ID     string `json:"ID"`
	Image  string `json:"Image"`
	Names  string `json:"Names"`
	State  string `json:"State"`
	Status string `json:"Status"`
	Ports  string `json:"Ports"`
	Labels string `json:"Labels"`
}

func listHerringboneContainers(project string, includeStopped bool) ([]herringboneContainer, error) {
	project = strings.ToLower(strings.TrimSpace(project))
	if project == "" {
		project = "herringbone"
	}

	args := []string{"ps"}
	if includeStopped {
		args = append(args, "--all")
	}
	args = append(args, "--format", "json")

	cmd := exec.Command("docker", args...)
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			_, _ = os.Stderr.Write(exitError.Stderr)
		}
		return nil, err
	}

	containers := []herringboneContainer{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var row dockerContainerRow
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}

		labels := parseDockerLabels(row.Labels)
		composeProject := strings.ToLower(strings.TrimSpace(labels["com.docker.compose.project"]))
		name := strings.TrimSpace(row.Names)
		lowerName := strings.ToLower(name)
		lowerImage := strings.ToLower(strings.TrimSpace(row.Image))

		managed := false
		switch {
		case composeProject == project:
			managed = true
		case strings.HasPrefix(composeProject, project+"-receiver-"):
			managed = true
		case strings.HasPrefix(lowerName, project+"-"):
			managed = true
		case strings.HasPrefix(lowerName, project+"_"):
			managed = true
		case lowerName == "mongodb":
			managed = true
		case lowerName == "herringbone-proxy":
			managed = true
		case strings.HasPrefix(lowerName, "herringbone-herringbone-auth-"):
			managed = true
		case strings.Contains(lowerImage, "herringbone"):
			managed = true
		}
		if !managed {
			continue
		}

		service := strings.TrimSpace(labels["com.docker.compose.service"])
		if service == "" {
			service = extractServiceName(name)
		}

		containers = append(containers, herringboneContainer{
			ID:         strings.TrimSpace(row.ID),
			Image:      strings.TrimSpace(row.Image),
			Name:       name,
			Service:    CanonicalElementName(service),
			RawService: strings.TrimSpace(service),
			Project:    blankDefault(composeProject, project),
			State:      strings.TrimSpace(row.State),
			Status:     strings.TrimSpace(row.Status),
			Ports:      strings.TrimSpace(row.Ports),
			Labels:     labels,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Slice(containers, func(i, j int) bool {
		if containers[i].Project == containers[j].Project {
			if containers[i].Service == containers[j].Service {
				return containers[i].Name < containers[j].Name
			}
			return containers[i].Service < containers[j].Service
		}
		return containers[i].Project < containers[j].Project
	})

	return containers, nil
}

func containersForService(project string, service string, includeStopped bool) ([]herringboneContainer, error) {
	service = CanonicalElementName(strings.TrimSpace(service))
	containers, err := listHerringboneContainers(project, includeStopped)
	if err != nil {
		return nil, err
	}

	out := []herringboneContainer{}
	for _, container := range containers {
		if CanonicalElementName(container.Service) == service {
			out = append(out, container)
		}
	}
	return out, nil
}

func containersForExactService(project string, service string, includeStopped bool) ([]herringboneContainer, error) {
	service = CanonicalElementName(strings.TrimSpace(service))
	containers, err := listHerringboneContainers(project, includeStopped)
	if err != nil {
		return nil, err
	}

	out := []herringboneContainer{}
	for _, container := range containers {
		raw := strings.TrimSpace(container.RawService)
		if raw == "" {
			raw = strings.TrimSpace(container.Service)
		}
		if raw == service {
			out = append(out, container)
		}
	}
	return out, nil
}

func stoppedContainers(containers []herringboneContainer) []herringboneContainer {
	out := []herringboneContainer{}
	for _, container := range containers {
		if !isRunningContainer(container) {
			out = append(out, container)
		}
	}
	return out
}

func parseDockerLabels(value string) map[string]string {
	labels := map[string]string{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		labels[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return labels
}

func isRunningContainer(container herringboneContainer) bool {
	return strings.EqualFold(strings.TrimSpace(container.State), "running")
}

func isPrunableContainer(container herringboneContainer) bool {
	switch strings.ToLower(strings.TrimSpace(container.State)) {
	case "created", "dead", "exited":
		return true
	default:
		return false
	}
}

func isProtectedCoreService(service string) bool {
	switch CanonicalElementName(strings.TrimSpace(service)) {
	case "mongodb", "herringbone-proxy", "herringbone-auth":
		return true
	default:
		return false
	}
}

func stopContainers(containers []herringboneContainer) error {
	return runContainerCommand("stop", containers)
}

func startContainers(containers []herringboneContainer) error {
	return runContainerCommand("start", containers)
}

func removeContainers(containers []herringboneContainer) error {
	return runContainerCommand("rm", containers)
}

func runContainerCommand(action string, containers []herringboneContainer) error {
	if len(containers) == 0 {
		return nil
	}

	args := []string{action}
	for _, container := range containers {
		if strings.TrimSpace(container.ID) != "" {
			args = append(args, container.ID)
		} else {
			args = append(args, container.Name)
		}
	}

	ui.Command("docker %s", strings.Join(args, " "))
	cmd := exec.Command("docker", args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %s failed: %w", action, err)
	}
	return nil
}

func stoppedDedicatedReceivers(project string) ([]herringboneContainer, error) {
	containers, err := listHerringboneContainers(project, true)
	if err != nil {
		return nil, err
	}

	out := []herringboneContainer{}
	prefix := strings.ToLower(strings.TrimSpace(project))
	if prefix == "" {
		prefix = "herringbone"
	}
	prefix += "-receiver-"

	for _, container := range containers {
		if isRunningContainer(container) {
			continue
		}
		if strings.HasPrefix(strings.ToLower(container.Project), prefix) || strings.HasPrefix(strings.ToLower(container.Name), prefix) {
			out = append(out, container)
		}
	}
	return out, nil
}

type pruneContainerOptions struct {
	Project          string
	IncludeProtected bool
	Services         []string
	QuietIfEmpty     bool
}

func pruneStoppedContainers(opts pruneContainerOptions) error {
	containers, err := listHerringboneContainers(opts.Project, true)
	if err != nil {
		return err
	}

	allowed := map[string]bool{}
	for _, service := range opts.Services {
		allowed[CanonicalElementName(service)] = true
	}

	toRemove := []herringboneContainer{}
	protectedSkipped := []herringboneContainer{}
	for _, container := range containers {
		if !isPrunableContainer(container) {
			continue
		}

		service := CanonicalElementName(container.Service)
		if len(allowed) > 0 && !allowed[service] {
			continue
		}

		if isProtectedCoreService(service) && !opts.IncludeProtected {
			protectedSkipped = append(protectedSkipped, container)
			continue
		}

		toRemove = append(toRemove, container)
	}

	if len(toRemove) == 0 {
		if !opts.QuietIfEmpty {
			ui.Success("No stopped Herringbone containers to prune")
		}
	} else {
		ui.Section("Prune stopped containers")
		ui.Warn("Removing containers only. Docker volumes are not removed.")
		tableRows := make([][]string, 0, len(toRemove))
		for _, container := range toRemove {
			tableRows = append(tableRows, []string{container.Service, container.Project, container.State, container.Name})
		}
		ui.Table([]string{"SERVICE", "PROJECT", "STATE", "NAME"}, tableRows)
		ui.Step("Removing stopped container(s)")
		if err := removeContainers(toRemove); err != nil {
			return err
		}
		ui.Success("Pruned %d stopped container(s)", len(toRemove))
	}

	if len(protectedSkipped) > 0 && !opts.QuietIfEmpty {
		ui.Section("Protected core skipped")
		tableRows := make([][]string, 0, len(protectedSkipped))
		for _, container := range protectedSkipped {
			tableRows = append(tableRows, []string{container.Service, container.Project, container.State, container.Name})
		}
		ui.Table([]string{"SERVICE", "PROJECT", "STATE", "NAME"}, tableRows)
		ui.Info("Use hbctl prune --core to remove stopped protected core containers. Volumes are still never removed by hbctl prune.")
	}

	return nil
}

package local

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/herringbonedev/hbctl/internal/units"
)

type Publisher struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
}

type ContainerStatus struct {
	Name       string      `json:"name"`
	Project    string      `json:"project"`
	Service    string      `json:"service"`
	State      string      `json:"state"`
	Status     string      `json:"status"`
	Protected  bool        `json:"protected"`
	Publishers []Publisher `json:"publishers,omitempty"`
}

type StatusOptions struct {
	Project string
	Unit    string
	JSON    bool
	All     bool
}

func Status(opts StatusOptions) error {
	project := opts.Project
	if project == "" {
		project = "herringbone"
	}

	containers, err := listHerringboneContainers(project, opts.All)
	if err != nil {
		return err
	}

	rows := make([]ContainerStatus, 0, len(containers))
	for _, container := range containers {
		service := CanonicalElementName(container.Service)
		rows = append(rows, ContainerStatus{
			Name:       container.Name,
			Project:    container.Project,
			Service:    service,
			State:      container.State,
			Status:     container.Status,
			Protected:  isProtectedCoreService(service),
			Publishers: parsePorts(container.Ports),
		})
	}

	if opts.Unit != "" {
		allowed := map[string]bool{}
		for _, s := range units.ServiceUnits[opts.Unit] {
			allowed[CanonicalElementName(s)] = true
		}

		var filtered []ContainerStatus
		for _, r := range rows {
			if allowed[CanonicalElementName(r.Service)] {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	running := 0
	protectedRunning := 0
	for _, r := range rows {
		if strings.EqualFold(r.State, "running") {
			running++
			if r.Protected {
				protectedRunning++
			}
		}
	}

	ui.Header("Herringbone status")
	ui.KeyValues([][2]string{
		{"project", project},
		{"containers", statusContainerSummary(len(rows), running, opts.All)},
		{"protected core", fmt.Sprintf("%d running", protectedRunning)},
	})

	if !opts.All {
		ui.Info("Showing active containers only. Use hbctl status --all to include stopped containers.")
	}

	if len(rows) == 0 {
		ui.Success("No matching Herringbone containers found")
		return nil
	}

	summaries := summarizeStatusRows(rows)
	tableRows := make([][]string, 0, len(summaries))
	for _, summary := range summaries {
		tableRows = append(tableRows, []string{
			summary.Service,
			formatContainerState(summary.State),
			summary.Replicas,
			summary.Ports,
		})
	}

	ui.Table([]string{"SERVICE", "STATE", "REPLICAS", "PORTS"}, tableRows)
	return nil
}

type serviceStatusSummary struct {
	Service  string
	State    string
	Replicas string
	Ports    string
}

func summarizeStatusRows(rows []ContainerStatus) []serviceStatusSummary {
	byService := map[string][]ContainerStatus{}
	order := []string{}
	for _, row := range rows {
		key := CanonicalElementName(row.Service)
		if _, ok := byService[key]; !ok {
			order = append(order, key)
		}
		byService[key] = append(byService[key], row)
	}

	out := make([]serviceStatusSummary, 0, len(order))
	for _, service := range order {
		items := byService[service]
		running := 0
		states := map[string]int{}
		ports := map[string]bool{}
		for _, item := range items {
			state := strings.ToLower(strings.TrimSpace(item.State))
			if state == "" {
				state = "unknown"
			}
			states[state]++
			if state == "running" {
				running++
			}
			for _, p := range item.Publishers {
				if p.PublishedPort == 0 || p.TargetPort == 0 {
					continue
				}
				ports[fmt.Sprintf("%d→%d/%s", p.PublishedPort, p.TargetPort, strings.ToLower(p.Protocol))] = true
			}
		}

		state := "unknown"
		switch {
		case running == len(items):
			state = "running"
		case running > 0:
			state = "mixed"
		case len(states) == 1:
			for s := range states {
				state = s
			}
		default:
			state = "stopped"
		}

		portList := make([]string, 0, len(ports))
		for port := range ports {
			portList = append(portList, port)
		}
		sort.Strings(portList)
		portText := strings.Join(portList, ", ")
		if portText == "" {
			portText = "-"
		}

		serviceName := service
		if isProtectedCoreService(service) {
			serviceName += " " + ui.Yellow("core")
		}
		out = append(out, serviceStatusSummary{
			Service:  serviceName,
			State:    state,
			Replicas: fmt.Sprintf("%d/%d", running, len(items)),
			Ports:    portText,
		})
	}
	return out
}

func statusContainerSummary(total, running int, includeStopped bool) string {
	if includeStopped {
		return fmt.Sprintf("%d total / %d running", total, running)
	}
	return fmt.Sprintf("%d active", total)
}

func extractServiceName(container string) string {
	// herringbone-parser-cardset-1 -> parser-cardset
	parts := strings.Split(container, "-")
	if len(parts) >= 3 {
		return strings.Join(parts[1:len(parts)-1], "-")
	}
	return container
}

func parsePorts(s string) []Publisher {
	if s == "" {
		return nil
	}

	var pubs []Publisher
	parts := strings.Split(s, ",")

	for _, p := range parts {
		p = strings.TrimSpace(p)

		// Ignore IPv6 bindings
		if strings.HasPrefix(p, "[") {
			continue
		}

		if !strings.Contains(p, "->") {
			continue
		}

		lr := strings.Split(p, "->")
		if len(lr) != 2 {
			continue
		}

		hostPart := lr[0]
		containerPart := lr[1]

		var host string
		var hostPort int

		hp := strings.Split(hostPart, ":")
		if len(hp) == 2 {
			host = hp[0]
			fmt.Sscanf(hp[1], "%d", &hostPort)
		}

		var containerPort int
		var proto string
		fmt.Sscanf(containerPart, "%d/%s", &containerPort, &proto)

		pubs = append(pubs, Publisher{
			URL:           host,
			PublishedPort: hostPort,
			TargetPort:    containerPort,
			Protocol:      proto,
		})
	}

	return pubs
}

func formatContainerState(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running":
		return ui.Green(state)
	case "exited", "dead", "restarting":
		return ui.Red(state)
	default:
		return ui.Yellow(state)
	}
}

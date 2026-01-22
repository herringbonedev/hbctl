package cmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
)

func init() {
	Register("status", statusCmd)
}

type Publisher struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
}

type ContainerStatus struct {
	Name       string      `json:"Name"`
	Service    string      `json:"Service"`
	State      string      `json:"State"`
	Status     string      `json:"Status"`
	Publishers []Publisher `json:"Publishers"`
}

var serviceUnits = map[string][]string{
	"logs":      {"herringbone-logs", "herringbone-search", "mongodb"},
	"search":    {"herringbone-search", "mongodb"},
	"receiver":  {"logingestion-receiver", "mongodb"},
	"parser":    {"parser-cardset", "parser-enrichment", "parser-extractor"},
	"detection": {"detectionengine-detector", "detectionengine-matcher", "detectionengine-ruleset"},
	"incidents": {"incidents-incidentset", "incidents-correlator", "incidents-orchestrator", "mongodb"},
	"database":  {"mongodb"},
}

func statusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	element := fs.String("element", "", "Filter by exact element (service) name")
	unit := fs.String("unit", "", "Filter by unit (subsystem)")
	fs.Parse(args)

	if *element != "" && *unit != "" {
		fmt.Fprintln(os.Stderr, "Error: use either --element or --unit, not both")
		os.Exit(1)
	}

	cmd := exec.Command("docker", "compose", "-p", composeProject, "ps", "--format", "json")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "docker compose ps failed:", err)
		os.Exit(1)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var rows []ContainerStatus

	for _, line := range lines {
		if line == "" {
			continue
		}
		var c ContainerStatus
		_ = json.Unmarshal([]byte(line), &c)
		rows = append(rows, c)
	}

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSERVICE\tSTATE\tPORTS")

	for _, c := range rows {
		if !allowedService(*element, *unit, c.Service) {
			continue
		}

		var ports []string
		for _, p := range c.Publishers {
			if p.PublishedPort > 0 {
				ports = append(ports,
					fmt.Sprintf("%s:%d->%d/%s",
						p.URL, p.PublishedPort, p.TargetPort, strings.ToLower(p.Protocol)),
				)
			}
		}

		portStr := "-"
		if len(ports) > 0 {
			portStr = strings.Join(ports, ", ")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			c.Name, c.Service, c.State, portStr)
	}

	w.Flush()
}

func allowedService(element, unit, svc string) bool {
	if element != "" {
		return svc == element
	}

	if unit != "" {
		return allowedUnit(unit, svc)
	}

	return true
}

func allowedUnit(unit, svc string) bool {
	allowedSvcs := serviceUnits[unit]
	for _, s := range allowedSvcs {
		if s == svc {
			return true
		}
	}
	return false
}

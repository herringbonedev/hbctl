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

var profileServices = map[string][]string{
	"logs":           {"herringbone-logs", "mongodb"},
	"receiver":       {"logingestion-receiver", "mongodb"},
	"parser-cardset": {"parser-cardset", "mongodb"},
	"database":       {"mongodb"},
}

func statusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	profile := fs.String("profile", "", "Filter by profile (logs, receiver, parser-cardset, database)")
	fs.Parse(args)

	cmd := exec.Command("docker", "compose", "-p", composeProject, "ps", "--format", "json")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to run docker compose ps:", err)
		os.Exit(1)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var rows []ContainerStatus

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var c ContainerStatus
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse docker output:", err)
			continue
		}
		rows = append(rows, c)
	}

	if len(rows) == 0 {
		fmt.Println("No containers found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)

	fmt.Fprintln(w, "NAME\tSERVICE\tSTATE\tPORTS")
	fmt.Fprintln(w, "---------------------------------------------------------------------------------------------------------")

	for _, c := range rows {
		if !allowed(*profile, c.Service) {
			continue
		}

		var ports []string
		for _, p := range c.Publishers {
			if p.PublishedPort > 0 {
				ports = append(ports,
					fmt.Sprintf("%s:%d->%d/%s", p.URL, p.PublishedPort, p.TargetPort, strings.ToLower(p.Protocol)),
				)
			}
		}

		portStr := strings.Join(ports, ", ")
		if portStr == "" {
			portStr = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			c.Name,
			c.Service,
			c.State,
			portStr,
		)
	}

	w.Flush()
}

func allowed(profile, svc string) bool {
	if profile == "" {
		return true
	}
	allowedSvcs, ok := profileServices[profile]
	if !ok {
		return true
	}
	for _, s := range allowedSvcs {
		if s == svc {
			return true
		}
	}
	return false
}

package local

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/herringbonedev/hbctl/internal/units"
)

type Publisher struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
}

type ContainerStatus struct {
	Name       string
	Service    string
	State      string
	Status     string
	Publishers []Publisher
}

type dockerPSRow struct {
	Names  string `json:"Names"`
	State  string `json:"State"`
	Status string `json:"Status"`
	Ports  string `json:"Ports"`
}

type StatusOptions struct {
	Project string
	Unit    string
	JSON    bool
}

func Status(opts StatusOptions) error {
	project := opts.Project
	if project == "" {
		project = "herringbone"
	}

	cmd := exec.Command(
		"docker", "ps",
		"--filter", fmt.Sprintf("label=com.docker.compose.project=%s", project),
		"--format", "json",
	)
	cmd.Env = os.Environ()

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(ee.Stderr)
		}
		return err
	}

	var rows []ContainerStatus

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		var r dockerPSRow
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			return err
		}

		service := extractServiceName(r.Names)

		rows = append(rows, ContainerStatus{
			Name:       r.Names,
			Service:    service,
			State:      r.State,
			Status:     r.Status,
			Publishers: parsePorts(r.Ports),
		})
	}

	if opts.Unit != "" {
		allowed := map[string]bool{}
		for _, s := range units.ServiceUnits[opts.Unit] {
			allowed[s] = true
		}

		var filtered []ContainerStatus
		for _, r := range rows {
			if allowed[r.Service] {
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tSTATE\tSTATUS\tPORTS")

	for _, r := range rows {
		ports := ""
		if len(r.Publishers) > 0 {
			var parts []string
			for _, p := range r.Publishers {
				host := p.URL
				if host == "" {
					host = "0.0.0.0"
				}
				parts = append(parts,
					fmt.Sprintf("%s:%d->%d/%s",
						host,
						p.PublishedPort,
						p.TargetPort,
						strings.ToLower(p.Protocol),
					),
				)
			}
			ports = strings.Join(parts, ", ")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Service, r.State, r.Status, ports)
	}

	w.Flush()
	return nil
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

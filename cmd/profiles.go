package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

type profileInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group"`
}

func init() {
	Register("profiles", profilesCmd)
}

func profilesCmd(args []string) {
	fs := flag.NewFlagSet("profiles", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "Output profiles as JSON")
	namesOnly := fs.Bool("names", false, "Output only profile names")
	filter := fs.String("filter", "", "Filter profiles by name or description")
	wide := fs.Bool("wide", false, "Show wide table with groups")

	_ = fs.Parse(args)

	var out []profileInfo
	if *filter == "" {
		out = allProfiles
	} else {
		f := strings.ToLower(*filter)
		for _, p := range allProfiles {
			if strings.Contains(strings.ToLower(p.Name), f) ||
				strings.Contains(strings.ToLower(p.Description), f) ||
				strings.Contains(strings.ToLower(p.Group), f) {
				out = append(out, p)
			}
		}
	}

	// ---- JSON OUTPUT ----
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return
	}

	// ---- NAMES ONLY ----
	if *namesOnly {
		for _, p := range out {
			fmt.Println(p.Name)
		}
		return
	}

	// ---- WIDE OUTPUT ----
	if *wide {
		if len(out) == 0 {
			fmt.Println("No profiles found.")
			return
		}

		fmt.Println("Herringbone Profiles (wide)")
		fmt.Println("==========================")
		fmt.Printf("%-3s %-26s %-12s %s\n", "#", "NAME", "GROUP", "DESCRIPTION")
		fmt.Printf("%-3s %-26s %-12s %s\n", "-", "--------------------------", "------------", "------------------------------")

		for i, p := range out {
			fmt.Printf("%-3d %-26s %-12s %s\n", i+1, p.Name, p.Group, p.Description)
		}

		fmt.Println()
		fmt.Printf("Total profiles: %d\n", len(out))
		fmt.Println("Use: hbctl start --profile <name>")
		return
	}

	// ---- DEFAULT GROUPED OUTPUT ----
	if len(out) == 0 {
		fmt.Println("No profiles found.")
		return
	}

	grouped := map[string][]profileInfo{}
	for _, p := range out {
		grouped[p.Group] = append(grouped[p.Group], p)
	}

	var groups []string
	for g := range grouped {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	fmt.Println("Herringbone Profiles")
	fmt.Println("====================")

	total := 0
	for _, g := range groups {
		fmt.Printf("\n[%s]\n", g)
		for _, p := range grouped[g] {
			fmt.Printf("  %-26s %s\n", p.Name, p.Description)
			total++
		}
	}

	fmt.Println()
	fmt.Printf("Total profiles: %d\n", total)
	fmt.Println("Use: hbctl start --profile <name>")
	fmt.Println("Tip: hbctl profiles --wide")
}

var allProfiles = []profileInfo{
	{
		Name:        "logingestion-receiver",
		Description: "UDP/TCP/HTTP log ingestion receiver",
		Group:       "Ingestion",
	},
	{
		Name:        "herringbone-logs",
		Description: "Logs API",
		Group:       "Core",
	},
	{
		Name:        "parser-cardset",
		Description: "Cardset metadata parser service",
		Group:       "Parser",
	},
	{
		Name:        "parser-enrichment",
		Description: "Log enrichment parser service",
		Group:       "Parser",
	},
	{
		Name:        "parser-extractor",
		Description: "Regex/JSONPath extractor service",
		Group:       "Parser",
	},
	{
		Name:        "detectionengine-detector",
		Description: "Detection engine detector service",
		Group:       "Detection",
	},
	{
		Name:        "detectionengine-matcher",
		Description: "Detection engine matcher service",
		Group:       "Detection",
	},
	{
		Name:        "detectionengine-ruleset",
		Description: "Detection engine ruleset service",
		Group:       "Detection",
	},
	{
		Name:        "operations-center",
		Description: "Operations Center UI",
		Group:       "Ops",
	},
}

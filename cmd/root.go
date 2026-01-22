package cmd

import (
	"fmt"
	"os"
	"sort"
)

type Command func(args []string)

var commands = map[string]Command{}

func Register(name string, fn Command) {
	commands[name] = fn
}

func Execute() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	cmd := os.Args[1]
	if fn, ok := commands[cmd]; ok {
		fn(os.Args[2:])
		return
	}

	fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
	printHelp()
	os.Exit(1)
}

func printHelp() {
	fmt.Println()
	fmt.Printf("%s%s hbctl — Herringbone Control CLI %s\n", colorBold, colorBlue, colorReset)
	fmt.Println("=================================")
	fmt.Println()
	fmt.Println("Control and manage your local Herringbone stack.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  hbctl <command> [options]")
	fmt.Println()

	fmt.Printf("%sCommands:%s\n", colorBold, colorReset)

	var names []string
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Printf("  %s%-12s%s %s\n", colorGreen, name, colorReset, commandDesc(name))
	}

	fmt.Println()
	fmt.Println("Common examples:")
	fmt.Println("  hbctl profiles")
	fmt.Println("  hbctl groups")
	fmt.Println("  hbctl start --element logingestion-receiver --type UDP")
	fmt.Println("  hbctl start --element herringbone-logs")
	fmt.Println("  hbctl status")
	fmt.Println("  hbctl status --unit detection")
	fmt.Println("  hbctl status --element parser-extractor")
	fmt.Println("  hbctl stop --element herringbone-search")
	fmt.Println()
}

func commandDesc(name string) string {
	switch name {
	case "version":
		return "Show hbctl version"
	case "status":
		return "Show status of running services"
	case "profiles":
		return "List available elements (services)"
	case "groups":
		return "List available units (service groups)"
	case "start":
		return "Start an element or unit"
	case "stop":
		return "Stop an element or entire stack"
	case "restart":
		return "Restart an element or unit"
	case "login":
		return "Store encrypted service credentials"
	default:
		return ""
	}
}

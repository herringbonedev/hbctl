package cmd

import (
	"fmt"
	"os"
)

func init() {
	Register("help", helpCmd)
}

func helpCmd(args []string) {
	printHeader()

	fmt.Println(colorBold + "Usage:" + colorReset)
	fmt.Println("  hbctl <command> [options]")
	fmt.Println()

	fmt.Println(colorBold + "Commands:" + colorReset)
	fmt.Printf("  %-12s %s\n", "start", "Start an element or unit")
	fmt.Printf("  %-12s %s\n", "stop", "Stop an element or the full stack")
	fmt.Printf("  %-12s %s\n", "restart", "Restart an element or unit")
	fmt.Printf("  %-12s %s\n", "status", "Show running elements")
	fmt.Printf("  %-12s %s\n", "elements", "List available elements (services)")
	fmt.Printf("  %-12s %s\n", "units", "List available units (subsystems)")
	fmt.Printf("  %-12s %s\n", "logs", "View logs for elements or units")
	fmt.Printf("  %-12s %s\n", "login", "Store encrypted credentials")
	fmt.Printf("  %-12s %s\n", "version", "Show version")
	fmt.Printf("  %-12s %s\n", "help", "Show this help")
	fmt.Println()

	fmt.Println(colorBold + "Elements:" + colorReset)
	for _, e := range allElements {
		fmt.Printf("  %-28s %s\n", e.Name, e.Description)
	}
	fmt.Println()

	fmt.Println(colorBold + "Units:" + colorReset)
	seen := map[string]bool{}
	for _, e := range allElements {
		if !seen[e.Unit] {
			fmt.Printf("  %s\n", e.Unit)
			seen[e.Unit] = true
		}
	}
	fmt.Println()

	fmt.Println(colorBold + "Examples:" + colorReset)
	fmt.Println("  hbctl elements")
	fmt.Println("  hbctl units")
	fmt.Println("  hbctl start --element logingestion-receiver --type UDP")
	fmt.Println("  hbctl start --element detectionengine-detector")
	fmt.Println("  hbctl stop --element parser-cardset")
	fmt.Println("  hbctl status")
	fmt.Println("  hbctl status --unit detection")
	fmt.Println("  hbctl logs --unit parser --follow")
	fmt.Println()

	os.Exit(0)
}

func printHeader() {
	fmt.Println(colorBlue + colorBold + "hbctl" + colorReset + " - Herringbone Control CLI")
	fmt.Println()
}

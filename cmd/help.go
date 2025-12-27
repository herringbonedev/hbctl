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
	fmt.Printf("  %-12s %s\n", "start", "Start one or more profiles")
	fmt.Printf("  %-12s %s\n", "stop", "Stop one or more profiles")
	fmt.Printf("  %-12s %s\n", "restart", "Restart one or more profiles")
	fmt.Printf("  %-12s %s\n", "status", "Show running services")
	fmt.Printf("  %-12s %s\n", "profiles", "List available profiles")
	fmt.Printf("  %-12s %s\n", "login", "Store encrypted credentials")
	fmt.Printf("  %-12s %s\n", "version", "Show version")
	fmt.Printf("  %-12s %s\n", "help", "Show this help")
	fmt.Println()

	fmt.Println(colorBold + "Profiles:" + colorReset)
	for _, p := range allProfiles {
		fmt.Printf("  %-28s %s\n", p.Name, p.Description)
	}
	fmt.Println()

	fmt.Println(colorBold + "Examples:" + colorReset)
	fmt.Println("  hbctl start")
	fmt.Println("  hbctl start --profile logingestion-receiver --type UDP")
	fmt.Println("  hbctl start --profile detectionengine-detector")
	fmt.Println("  hbctl stop --profile parser-cardset")
	fmt.Println("  hbctl restart")
	fmt.Println("  hbctl status")
	fmt.Println()

	os.Exit(0)
}

func printHeader() {
	fmt.Println(colorBlue + colorBold + "hbctl" + colorReset + " - Herringbone Control CLI")
	fmt.Println()
}

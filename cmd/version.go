package cmd

import "fmt"

var Version = "alpha-0.1.0"

func init() {
	Register("version", versionCmd)
}

func versionCmd(args []string) {
	fmt.Println("hbctl version", Version)
}

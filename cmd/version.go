package cmd

import "fmt"

var Version = "alpha-0.4.0"

func init() {
	Register("version", versionCmd)
}

func versionCmd(args []string) {
	fmt.Println("hbctl version", Version)
}

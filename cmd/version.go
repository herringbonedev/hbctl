package cmd

import "fmt"

var Version = "0.0.1-a"

func init() {
	Register("version", versionCmd)
}

func versionCmd(args []string) {
	fmt.Println("hbctl version", Version)
}

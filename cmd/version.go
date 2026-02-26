package cmd

import "fmt"

<<<<<<< HEAD
var Version = "alpha-0.1.0"
=======
var Version = "alpha-0.2.0"
>>>>>>> 758a020 (include herringbone-proxy)

func init() {
	Register("version", versionCmd)
}

func versionCmd(args []string) {
	fmt.Println("hbctl version", Version)
}

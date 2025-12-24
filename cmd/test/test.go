package test

import (
	"fmt"
	"os"
)

type RegisterFunc func(name string, fn func([]string))

func Init(register RegisterFunc) {
	register("test", testCmd)
}

func testCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: hbctl test <target>")
		fmt.Fprintln(os.Stderr, "Available targets: compose")
		os.Exit(1)
	}

	switch args[0] {
	case "compose":
		testCompose()
	default:
		fmt.Fprintln(os.Stderr, "Unknown test target:", args[0])
		os.Exit(1)
	}
}

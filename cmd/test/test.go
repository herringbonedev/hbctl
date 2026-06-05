package test

import (
	"os"

	"github.com/herringbonedev/hbctl/internal/ui"
)

type RegisterFunc func(name string, fn func([]string))

func Init(register RegisterFunc) {
	register("test", testCmd)
}

func testCmd(args []string) {
	if len(args) < 1 {
		ui.FError(os.Stderr, "Usage: hbctl test <target>")
		ui.FInfo(os.Stderr, "Available targets: compose")
		os.Exit(1)
	}

	switch args[0] {
	case "compose":
		testCompose()
	default:
		ui.FError(os.Stderr, "Unknown test target: %s", args[0])
		os.Exit(1)
	}
}

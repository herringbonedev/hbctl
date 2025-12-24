package cmd

import (
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/cmd/login"
	"github.com/herringbonedev/hbctl/cmd/test"
)

type Command = func(args []string)

var commands = map[string]Command{}

func Register(name string, fn Command) {
	commands[name] = fn
}

func init() {
	login.Init(Register)
	test.Init(Register)
}

func Execute() {
	if len(os.Args) < 2 {
		Usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	if fn, ok := commands[cmd]; ok {
		fn(os.Args[2:])
		return
	}

	fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
	Usage()
	os.Exit(1)
}

func Usage() {
	fmt.Println(`hbctl - Herringbone control CLI

Usage:
  hbctl <command>

Commands:
  version
  status
  start
  stop
  restart
  login
`)
}

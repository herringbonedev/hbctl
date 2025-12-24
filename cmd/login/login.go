package login

import (
	"fmt"
	"os"
)

type RegisterFunc func(name string, fn func([]string))

func Init(register RegisterFunc) {
	register("login", loginCmd)
}

func loginCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: hbctl login <backend>")
		fmt.Fprintln(os.Stderr, "Available backends: mongodb")
		os.Exit(1)
	}

	switch args[0] {
	case "mongodb":
		loginMongo(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "Unknown backend:", args[0])
		os.Exit(1)
	}
}

package test

import (
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/secrets"
)

func testCompose() {
	sec, err := secrets.LoadMongo()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to load MongoDB secret:", err)
		os.Exit(1)
	}

	fmt.Println("[hbctl] Loaded MongoDB secret")
	fmt.Printf("MONGO_USER=%s\n", sec.User)
	fmt.Printf("MONGO_PASSWORD=%s\n", mask(sec.Password))
	fmt.Printf("MONGO_HOST=%s\n", sec.Host)
	fmt.Printf("MONGO_PORT=%d\n", sec.Port)
	fmt.Println()

	fmt.Println("Mock command:")
	fmt.Println("docker compose up -d")
}

func mask(s string) string {
	if len(s) <= 2 {
		return "**"
	}
	return s[:1] + "****" + s[len(s)-1:]
}

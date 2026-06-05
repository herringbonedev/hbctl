package test

import (
	"fmt"
	"os"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
)

func testCompose() {
	sec, err := secrets.LoadMongo()
	if err != nil {
		ui.FError(os.Stderr, "Failed to load MongoDB secret: %v", err)
		os.Exit(1)
	}

	ui.Header("hbctl compose test")
	ui.Success("Loaded MongoDB secret")
	ui.KeyValues([][2]string{
		{"MONGO_USER", sec.User},
		{"MONGO_PASSWORD", mask(sec.Password)},
		{"MONGO_HOST", sec.Host},
		{"MONGO_PORT", fmt.Sprintf("%d", sec.Port)},
	})
	ui.Section("Mock command")
	ui.Command("docker compose up -d")
}

func mask(s string) string {
	if len(s) <= 2 {
		return "**"
	}
	return s[:1] + "****" + s[len(s)-1:]
}

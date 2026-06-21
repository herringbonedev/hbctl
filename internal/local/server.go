package local

import (
	"os"
	"strings"

	"github.com/herringbonedev/hbctl/internal/secrets"
)

func configuredServerURL() string {
	if v := strings.TrimSpace(os.Getenv("HBCTL_AUTH_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("HBCTL_SERVER_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if cfg, err := secrets.LoadServerConfig(); err == nil && strings.TrimSpace(cfg.BaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	}
	return "http://localhost:8080"
}

func serverURLPath(path string) string {
	base := strings.TrimRight(configuredServerURL(), "/")
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	return base + path
}

package cmd

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

func serverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Save or show the Herringbone API server location",
		Long:  "Save the Herringbone API server base URL used by login, whoami, service token bootstrap, and other auth API calls.",
	}

	cmd.AddCommand(serverSetCommand())
	cmd.AddCommand(serverShowCommand())
	cmd.AddCommand(serverClearCommand())
	return cmd
}

func serverSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <url>",
		Short: "Save the Herringbone API server base URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseURL, err := normalizeServerURL(args[0])
			if err != nil {
				return err
			}

			savedAt := time.Now().UTC().Format(time.RFC3339)
			if err := secrets.SaveServerConfig(&secrets.ServerConfig{BaseURL: baseURL, SavedAt: savedAt}); err != nil {
				return fmt.Errorf("failed to save server location: %w", err)
			}

			ui.FHeader(cmd.OutOrStdout(), "Herringbone server")
			ui.FSuccess(cmd.OutOrStdout(), "Server location saved")
			ui.FKeyValues(cmd.OutOrStdout(), [][2]string{{"server", baseURL}, {"saved at", savedAt}})
			return nil
		},
	}
}

func serverShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the saved Herringbone API server base URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := secrets.LoadServerConfig()
			if err != nil {
				return fmt.Errorf("no server location saved. Run: hbctl server set <url>")
			}
			ui.FHeader(cmd.OutOrStdout(), "Herringbone server")
			ui.FKeyValues(cmd.OutOrStdout(), [][2]string{{"server", cfg.BaseURL}, {"saved at", cfg.SavedAt}})
			return nil
		},
	}
}

func serverClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove the saved Herringbone API server location",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := secrets.ClearServerConfig(); err != nil {
				return fmt.Errorf("failed to clear server location: %w", err)
			}
			ui.FSuccess(cmd.OutOrStdout(), "Server location cleared")
			return nil
		},
	}
}

func normalizeServerURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", fmt.Errorf("server URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid server URL %q; include scheme, for example http://localhost:8080", raw)
	}
	return u.String(), nil
}

func resolveCommandServerURL(explicit string, fallback string) string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		if normalized, err := normalizeServerURL(explicit); err == nil {
			return strings.TrimRight(normalized, "/")
		}
		return strings.TrimRight(explicit, "/")
	}

	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		if normalized, err := normalizeServerURL(fallback); err == nil {
			return strings.TrimRight(normalized, "/")
		}
		return strings.TrimRight(fallback, "/")
	}

	return defaultAuthURL()
}

func mapBool(value bool, whenTrue string, whenFalse string) string {
	if value {
		return whenTrue
	}
	return whenFalse
}

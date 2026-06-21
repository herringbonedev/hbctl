package cmd

import (
	"fmt"

	"github.com/herringbonedev/hbctl/internal/secrets"
	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

func logoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear the stored hbctl session token",
		Long:  "Clear the local hbctl session token file. This does not change encrypted hbctl configuration or server settings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := secrets.SessionPath()
			if err := secrets.ClearAuthSession(); err != nil {
				return fmt.Errorf("failed to clear hbctl session: %w", err)
			}
			ui.FSuccess(cmd.OutOrStdout(), "hbctl session cleared")
			if path != "" {
				ui.FKeyValues(cmd.OutOrStdout(), [][2]string{{"session", path}})
			}
			return nil
		},
	}
}

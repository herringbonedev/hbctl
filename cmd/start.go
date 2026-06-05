package cmd

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func startCommand() *cobra.Command {
	var element string
	var unit string
	var all bool
	var receiverType string
	var tokenCreate bool
	var noTokenCreate bool
	var bootstrapTokens bool
	var enterprise bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start an element, a unit, or the full stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && strings.TrimSpace(element) == "" && strings.TrimSpace(unit) == "" {
				return fmt.Errorf("specify --element, --unit, or --all")
			}

			if strings.TrimSpace(receiverType) != "" {
				normalized, err := normalizeReceiverType(receiverType)
				if err != nil {
					return err
				}
				receiverType = strings.ToUpper(normalized)
			}

			return local.Start(local.StartOptions{
				Project:         projectName,
				SecretsDir:      secretsDirOverride,
				Element:         strings.TrimSpace(element),
				Unit:            strings.TrimSpace(unit),
				All:             all,
				RecvType:        receiverType,
				TokenCreate:     tokenCreate,
				NoTokenCreate:   noTokenCreate,
				BootstrapTokens: bootstrapTokens,
				Enterprise:      enterprise,
			})
		},
	}

	cmd.Flags().StringVar(&element, "element", "", "Element to start")
	cmd.Flags().StringVar(&unit, "unit", "", "Unit to start")
	cmd.Flags().BoolVar(&all, "all", false, "Start the full stack")
	cmd.Flags().StringVar(&receiverType, "type", "", "Receiver type for logingestion-receiver: http, tcp, udp, or remote")
	cmd.Flags().BoolVar(&tokenCreate, "token-create", false, "Create or refresh admin/service tokens after auth is reachable")
	cmd.Flags().BoolVar(&bootstrapTokens, "bootstrap-tokens", false, "Deprecated alias for --token-create")
	cmd.Flags().BoolVar(&noTokenCreate, "no-token-create", false, "Deprecated no-op. Token creation is opt-in with --token-create")
	_ = cmd.Flags().MarkHidden("bootstrap-tokens")
	_ = cmd.Flags().MarkHidden("no-token-create")
	cmd.Flags().BoolVar(&enterprise, "enterprise", false, "Start enterprise services and set HB_ENTERPRISE=true")
	return cmd
}

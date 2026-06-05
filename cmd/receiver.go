package cmd

import (
	"fmt"
	"strings"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func receiverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "receiver",
		Short: "Manage dedicated receiver instances",
	}

	cmd.AddCommand(receiverStartCommand())
	cmd.AddCommand(receiverStopCommand())
	cmd.AddCommand(receiverRestartCommand())
	cmd.AddCommand(receiverListCommand())
	cmd.AddCommand(receiverLogsCommand())
	return cmd
}

func receiverStartCommand() *cobra.Command {
	var receiverType string
	var hostPort int
	var containerPort int
	var forwardRoute string
	var composeFile string
	var ingestionKey string
	var ingestionKeyFile string
	var enterprise bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a receiver instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedType, err := normalizeReceiverType(receiverType)
			if err != nil {
				return err
			}

			forwardRoute = strings.TrimSpace(forwardRoute)
			ingestionKey = strings.TrimSpace(ingestionKey)
			ingestionKeyFile = strings.TrimSpace(ingestionKeyFile)

			if normalizedType == "remote" && forwardRoute != "" {
				return fmt.Errorf("remote receivers cannot be forwarders")
			}

			if ingestionKey != "" && ingestionKeyFile != "" {
				return fmt.Errorf("use either --ingestion-key or --ingestion-key-file, not both")
			}

			if forwardRoute != "" && ingestionKey == "" && ingestionKeyFile == "" {
				return fmt.Errorf("forwarding requires --ingestion-key or --ingestion-key-file")
			}

			mode := "local"
			if forwardRoute != "" {
				mode = "forward"
			}

			return local.StartReceiver(local.ReceiverStartOptions{
				Project:          projectName,
				ReceiverType:     strings.ToUpper(normalizedType),
				Mode:             mode,
				HostPort:         hostPort,
				ContainerPort:    containerPort,
				ForwardRoute:     forwardRoute,
				ComposeFile:      strings.TrimSpace(composeFile),
				IngestionKey:     ingestionKey,
				IngestionKeyFile: ingestionKeyFile,
				Enterprise:       enterprise,
			})
		},
	}

	cmd.Flags().StringVar(&receiverType, "type", "http", "Receiver type: http, tcp, udp, or remote")
	cmd.Flags().IntVar(&hostPort, "port", 0, "Host port; auto-allocates when omitted")
	cmd.Flags().IntVar(&containerPort, "container-port", 7004, "Internal container port")
	cmd.Flags().StringVar(&forwardRoute, "forward-route", "", "Forward target (enables forwarding)")
	cmd.Flags().StringVar(&composeFile, "compose-file", "", "Override the receiver compose file path")
	cmd.Flags().StringVar(&ingestionKey, "ingestion-key", "", "Existing ingestion key")
	cmd.Flags().StringVar(&ingestionKeyFile, "ingestion-key-file", "", "Path to ingestion key file")
	cmd.Flags().BoolVar(&enterprise, "enterprise", false, "Start receiver in enterprise mode by setting HB_ENTERPRISE=true")
	return cmd
}

func receiverStopCommand() *cobra.Command {
	var receiverType string
	var hostPort int
	var composeFile string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a receiver instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			if hostPort <= 0 {
				return fmt.Errorf("--port is required")
			}

			normalizedType, err := normalizeReceiverType(receiverType)
			if err != nil {
				return err
			}

			return local.StopReceiver(local.ReceiverStopOptions{
				Project:      projectName,
				ReceiverType: normalizedType,
				HostPort:     hostPort,
				ComposeFile:  strings.TrimSpace(composeFile),
			})
		},
	}

	cmd.Flags().StringVar(&receiverType, "type", "http", "Receiver type: http, tcp, udp, or remote")
	cmd.Flags().IntVar(&hostPort, "port", 0, "Host port")
	cmd.Flags().StringVar(&composeFile, "compose-file", "", "Override the receiver compose file path")
	return cmd
}

func receiverRestartCommand() *cobra.Command {
	var receiverType string
	var hostPort int
	var composeFile string

	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart a receiver instance gracefully",
		RunE: func(cmd *cobra.Command, args []string) error {
			if hostPort <= 0 {
				return fmt.Errorf("--port is required")
			}

			normalizedType, err := normalizeReceiverType(receiverType)
			if err != nil {
				return err
			}

			return local.RestartReceiver(local.ReceiverRestartOptions{
				Project:      projectName,
				ReceiverType: normalizedType,
				HostPort:     hostPort,
				ComposeFile:  strings.TrimSpace(composeFile),
			})
		},
	}

	cmd.Flags().StringVar(&receiverType, "type", "http", "Receiver type: http, tcp, udp, or remote")
	cmd.Flags().IntVar(&hostPort, "port", 0, "Host port")
	cmd.Flags().StringVar(&composeFile, "compose-file", "", "Override the receiver compose file path")
	return cmd
}

func receiverListCommand() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List running receiver instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			return local.ListReceivers(local.ReceiverListOptions{
				Project: projectName,
				JSON:    asJSON,
			})
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func receiverLogsCommand() *cobra.Command {
	var receiverType string
	var hostPort int
	var follow bool
	var tail int
	var composeFile string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show logs for a receiver instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			if hostPort <= 0 {
				return fmt.Errorf("--port is required")
			}

			normalizedType, err := normalizeReceiverType(receiverType)
			if err != nil {
				return err
			}

			return local.LogsReceiver(local.ReceiverLogsOptions{
				Project:      projectName,
				ReceiverType: normalizedType,
				HostPort:     hostPort,
				ComposeFile:  strings.TrimSpace(composeFile),
				Follow:       follow,
				Tail:         tail,
			})
		},
	}

	cmd.Flags().StringVar(&receiverType, "type", "http", "Receiver type: http, tcp, udp, or remote")
	cmd.Flags().IntVar(&hostPort, "port", 0, "Host port")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVar(&tail, "tail", 200, "Number of lines from the end of logs")
	cmd.Flags().StringVar(&composeFile, "compose-file", "", "Override the receiver compose file path")
	return cmd
}

func normalizeReceiverType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "http", "tcp", "udp", "remote":
		return strings.ToLower(strings.TrimSpace(value)), nil
	default:
		return "", fmt.Errorf("invalid receiver type %q: expected http, tcp, udp, or remote", value)
	}
}

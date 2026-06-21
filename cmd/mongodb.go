package cmd

import (
	"strings"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func mongodbCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mongodb",
		Aliases: []string{"mongo"},
		Short:   "Manage MongoDB seed and migration tasks",
	}

	cmd.AddCommand(mongodbInitCommand())
	return cmd
}

func mongodbInitCommand() *cobra.Command {
	var scriptPath string
	var enterprise bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Replay init-mongo.js against the running MongoDB container",
		Long: "Replay the Herringbone MongoDB init script idempotently. " +
			"By default hbctl looks for init-mongo.js in the current directory and then docker/init-mongo.js from the repo root.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return local.RunMongoInit(local.MongoInitOptions{
				Project:    projectName,
				ScriptPath: strings.TrimSpace(scriptPath),
				Enterprise: enterprise,
				DryRun:     dryRun,
			})
		},
	}

	cmd.Flags().StringVarP(&scriptPath, "file", "f", "", "Mongo init JavaScript file; defaults to init-mongo.js or docker/init-mongo.js")
	cmd.Flags().BoolVar(&enterprise, "enterprise", false, "Also ensure enterprise platform/org seed data after replaying init-mongo.js")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the init plan without changing MongoDB")
	return cmd
}

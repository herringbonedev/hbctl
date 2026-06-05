package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/herringbonedev/hbctl/internal/local"
	"github.com/spf13/cobra"
)

func upgradeCommand() *cobra.Command {
	var element string
	var unit string
	var all bool
	var noPull bool
	var forceRecreate bool
	var enterprise bool
	var dryRun bool
	var listReleases bool
	var releaseTag string
	var applyNow bool
	var limit int
	var asJSON bool
	var includeDrafts bool
	var timeoutSeconds int
	var assetName string

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "List releases, stage release files, or safely refresh platform services",
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeoutSeconds <= 0 {
				return fmt.Errorf("--timeout must be greater than zero")
			}

			if listReleases {
				if limit <= 0 {
					return fmt.Errorf("--limit must be greater than zero")
				}
				releases, err := fetchHerringboneReleases(defaultReleasesURL, limit, includeDrafts, time.Duration(timeoutSeconds)*time.Second)
				if err != nil {
					return err
				}
				if asJSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(releases)
				}
				printReleaseList(cmd.OutOrStdout(), releases)
				return nil
			}

			if strings.TrimSpace(releaseTag) != "" {
				if dryRun {
					return fmt.Errorf("--dry-run is only supported for service refresh operations, not release staging")
				}
				if err := stageReleaseFromGitHub(strings.TrimSpace(releaseTag), strings.TrimSpace(assetName), time.Duration(timeoutSeconds)*time.Second); err != nil {
					return err
				}
				if !applyNow {
					return nil
				}
				all = true
			}

			if !all && strings.TrimSpace(element) == "" && strings.TrimSpace(unit) == "" {
				return fmt.Errorf("specify --list-releases, --release-tag, --element, --unit, or --all")
			}
			return local.Upgrade(local.UpgradeOptions{
				Project:       projectName,
				Element:       strings.TrimSpace(element),
				Unit:          strings.TrimSpace(unit),
				All:           all,
				Pull:          !noPull,
				ForceRecreate: forceRecreate,
				Enterprise:    enterprise,
				DryRun:        dryRun,
			})
		},
	}

	cmd.Flags().BoolVar(&listReleases, "list-releases", false, "List available GitHub releases")
	cmd.Flags().StringVar(&releaseTag, "release-tag", "", "Download and stage a specific release tag")
	cmd.Flags().BoolVar(&applyNow, "now", false, "After staging --release-tag, immediately run the safe upgrade refresh")
	cmd.Flags().StringVar(&assetName, "asset", "", "Specific release asset name to download instead of auto-selecting a tarball")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of releases to list")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output release list as JSON")
	cmd.Flags().BoolVar(&includeDrafts, "include-drafts", false, "Include draft releases if the GitHub API token can see them")
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 30, "GitHub API/download timeout in seconds")

	cmd.Flags().StringVar(&element, "element", "", "Element to refresh")
	cmd.Flags().StringVar(&unit, "unit", "", "Unit to refresh")
	cmd.Flags().BoolVar(&all, "all", false, "Run the safe local refresh for the full stack")
	cmd.Flags().BoolVar(&noPull, "no-pull", false, "Skip docker compose pull before recreating services")
	cmd.Flags().BoolVar(&forceRecreate, "force-recreate", true, "Force container recreation during upgrade")
	cmd.Flags().BoolVar(&enterprise, "enterprise", false, "Include enterprise services and set HB_ENTERPRISE=true")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the upgrade plan and docker compose commands without running them")
	return cmd
}

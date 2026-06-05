package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"

	"github.com/herringbonedev/hbctl/internal/ui"
	"github.com/spf13/cobra"
)

var (
	Version = "alpha-0.6.0"

	// Revision is intentionally opaque. Release/build scripts should inject a
	// random hex value with:
	//
	//   go build -ldflags="-X github.com/herringbonedev/hbctl/cmd.Revision=rev-$(openssl rand -hex 8)"
	//
	// If Revision is not injected, hbctl falls back to a short hash of the
	// running binary so plain `go build` still produces a non-descriptive rev.
	Revision = ""
)

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show hbctl version",
		Run: func(cmd *cobra.Command, args []string) {
			ui.FHeader(cmd.OutOrStdout(), "hbctl")
			ui.FKeyValues(cmd.OutOrStdout(), [][2]string{
				{"version", Version},
				{"rev", displayRevision()},
			})
		},
	}
}

func displayRevision() string {
	rev := strings.TrimSpace(Revision)
	if rev != "" {
		return normalizeRevision(rev)
	}

	exe, err := os.Executable()
	if err != nil {
		return "rev-unknown"
	}

	data, err := os.ReadFile(exe)
	if err != nil {
		return "rev-unknown"
	}

	sum := sha256.Sum256(data)
	return "rev-" + hex.EncodeToString(sum[:])[:12]
}

func normalizeRevision(rev string) string {
	if strings.HasPrefix(rev, "rev-") {
		return rev
	}
	return "rev-" + rev
}

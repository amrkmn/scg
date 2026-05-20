package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/ui"
)

// NewCatCommand creates the cat subcommand.
func NewCatCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cat <app>",
		Short: "Show the raw manifest JSON for an app",
		Long:  "Display the raw JSON manifest file for an installed or available app.",
		Args:  cobra.ExactArgs(1),
		Example: `scg cat git
scg cat extras/enso`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			input := args[0]

			// Find the manifest file.
			all := ctx.Services.Manifests.FindAllManifests(input)
			if len(all) == 0 {
				ctx.GetLogger().Error(fmt.Sprintf("manifest for %q not found", input))
				return os.ErrNotExist
			}

			// If a specific bucket was requested, prefer that result.
			requestedBucket := ""
			if parts := strings.SplitN(input, "/", 2); len(parts) == 2 {
				requestedBucket = parts[0]
			}

			// Prefer installed manifest, then bucket manifest.
			var chosen *struct {
				FilePath string
				Bucket   string
				Source   string
			}
			for i := range all {
				fm := &all[i]
				if requestedBucket != "" && !strings.EqualFold(fm.Bucket, requestedBucket) {
					continue
				}
				chosen = &struct {
					FilePath string
					Bucket   string
					Source   string
				}{fm.FilePath, fm.Bucket, fm.Source}
				if fm.Source == "installed" {
					break
				}
			}

			if chosen == nil {
				// No exact bucket match; fall back to first result.
				chosen = &struct {
					FilePath string
					Bucket   string
					Source   string
				}{all[0].FilePath, all[0].Bucket, all[0].Source}
			}

			// Read raw file.
			data, err := os.ReadFile(chosen.FilePath)
			if err != nil {
				return fmt.Errorf("reading manifest: %w", err)
			}

			// Header showing source.
			if chosen.Source == "installed" {
				_, _ = fmt.Fprintf(os.Stdout, "%s %s\n",
					ui.Dim("Manifest from installed app"),
					ui.Dim("("+chosen.Bucket+")"),
				)
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "%s %s\n",
					ui.Dim("Manifest from bucket"),
					ui.Cyan(chosen.Bucket),
				)
			}
			_, _ = fmt.Fprintln(os.Stdout)

			// Output raw file content to preserve property order.
			_, _ = fmt.Fprintln(os.Stdout, strings.TrimSpace(string(data)))
			return nil
		},
	}
}

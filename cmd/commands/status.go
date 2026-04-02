package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// NewStatusCommand creates the status subcommand.
func NewStatusCommand() *cobra.Command {
	var flagLocal, flagVerbose bool

	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Show update status for installed apps",
		Args:    cobra.NoArgs,
		Example: "scg status\nscg status --local\nscg status --verbose",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			apps, err := ctx.Services.Apps.ListInstalled("")
			if err != nil {
				return err
			}
			if len(apps) == 0 {
				fmt.Fprintln(os.Stdout, ui.Dim("No apps installed."))
				return nil
			}

			var checkApps []service.InstalledApp
			for _, a := range apps {
				if a.Name != "scoop" {
					checkApps = append(checkApps, a)
				}
			}

			scoopOutdated, _ := ctx.Services.Buckets.CheckScoopStatus(flagLocal)
			if scoopOutdated {
				fmt.Fprintf(os.Stdout, "%s Scoop has updates available. Run 'scoop update' to update.\n", ui.Warning("!"))
			} else {
				fmt.Fprintf(os.Stdout, "%s Scoop is up to date.\n", ui.Success("✓"))
			}

			buckets, _ := ctx.Services.Buckets.List("")
			bucketsOutdated, _ := ctx.Services.Buckets.CheckBucketsStatus(flagLocal, buckets)

			if bucketsOutdated {
				fmt.Fprintf(os.Stdout, "%s Some buckets have updates available. Run 'scg bucket update' to update.\n", ui.Warning("!"))
			} else {
				fmt.Fprintf(os.Stdout, "%s All buckets are up to date.\n", ui.Success("✓"))
			}
			fmt.Fprintln(os.Stdout)

			appResults := ctx.Services.Status.CheckStatus(checkApps, buckets, nil)

			// In verbose mode, show all apps; otherwise filter to those needing attention.
			var display []service.AppStatusResult
			if flagVerbose {
				display = appResults
			} else {
				for _, r := range appResults {
					if r.Outdated || r.Failed || len(r.MissingDeps) > 0 {
						display = append(display, r)
					}
				}
			}

			if len(display) == 0 {
				fmt.Fprintf(os.Stdout, "%s All installed apps are up to date.\n", ui.Success("✓"))
				return nil
			}

			sort.Slice(display, func(i, j int) bool {
				return display[i].Name < display[j].Name
			})

			header := []string{
				ui.BoldGreen("Name"),
				ui.BoldGreen("Installed"),
				ui.BoldGreen("Latest"),
				ui.BoldGreen("Missing Deps"),
				ui.BoldGreen("Info"),
			}
			rows := [][]string{header}

			for _, r := range display {
				name := ui.Cyan(r.Name)

				latest := r.Latest
				if r.Outdated {
					latest = ui.Yellow("* " + r.Latest)
				}

				info := ""
				if r.Failed {
					info = ui.Red("Failed")
				} else if r.Held {
					info = ui.Yellow("Held")
				}

				// In verbose mode, show status for up-to-date apps
				if flagVerbose && !r.Outdated && !r.Failed && len(r.MissingDeps) == 0 && !r.Held {
					info = ui.Dim("up-to-date")
				}

				missingDeps := strings.Join(r.MissingDeps, ", ")

				rows = append(rows, []string{name, r.Installed, latest, missingDeps, info})
			}

			fmt.Fprintln(os.Stdout, ui.FormatLineColumns(rows, []float64{2.0, 1.0, 1.0, 1.0, 1.5}))

			if flagVerbose {
				var outdated, failed int
				for _, r := range display {
					if r.Outdated {
						outdated++
					}
					if r.Failed {
						failed++
					}
				}
				fmt.Fprintf(os.Stdout, "\n%s %d total, %s%d outdated%s, %s%d failed%s\n",
					ui.Dim("("), len(display), ui.Yellow(""), outdated, ui.Dim(""), ui.Red(""), failed, ui.Dim(")"))
			} else {
				fmt.Fprintf(os.Stdout, "\n%s\n", ui.Dim(fmt.Sprintf("%d app(s) need attention", len(display))))
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagLocal, "local", "l", false, "Skip git fetch, use local state only")
	cmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show detailed output")
	return cmd
}

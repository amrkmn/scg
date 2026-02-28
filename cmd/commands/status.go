package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

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
		Example: "scg status\nscg status --local",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)
			if ctx == nil {
				return fmt.Errorf("context unavailable")
			}

			// Get installed apps.
			apps, err := ctx.Services.Apps.ListInstalled("")
			if err != nil {
				return err
			}
			if len(apps) == 0 {
				fmt.Fprintln(os.Stdout, ui.Dim("No apps installed."))
				return nil
			}

			// Filter out "scoop" itself from per-app checks.
			var checkApps []service.InstalledApp
			for _, a := range apps {
				if a.Name != "scoop" {
					checkApps = append(checkApps, a)
				}
			}

			// Progress bar: 1 scoop + 1 buckets + N apps.
			total := 2 + len(checkApps)
			var done atomic.Int32
			bar := ui.NewProgressBar(total, "Checking status...")
			bar.Start()

			advance := func(label string) {
				n := int(done.Add(1))
				bar.SetProgress(n, label)
			}

			// Phase 1 (parallel): scoop status, bucket list, apps check.
			// Phase 2 (depends on bucket list): bucket status + per-app status.
			//
			// We kick off:
			//   goroutine A – CheckScoopStatus  (git fetch scoop)
			//   goroutine B – List("")           (fast, local FS)
			//     └─ goroutine C – CheckBucketsStatus (git fetch per bucket)
			//     └─ goroutine D – CheckStatus (per-app concurrent)
			// A, C, D all run concurrently.

			var (
				scoopOutdated   bool
				bucketsOutdated bool
				allBuckets      []service.BucketInfo
				appResults      []service.AppStatusResult

				wg sync.WaitGroup
			)

			// Goroutine A: scoop update check.
			wg.Add(1)
			go func() {
				defer wg.Done()
				scoopOutdated, _ = ctx.Services.Buckets.CheckScoopStatus(flagLocal)
				advance("Checking scoop...")
			}()

			// Goroutine B: list buckets then fan out buckets + apps checks.
			wg.Add(1)
			go func() {
				defer wg.Done()

				buckets, _ := ctx.Services.Buckets.List("")
				allBuckets = buckets

				var inner sync.WaitGroup

				// Goroutine C: bucket update checks (uses pre-fetched list).
				inner.Add(1)
				go func() {
					defer inner.Done()
					bucketsOutdated, _ = ctx.Services.Buckets.CheckBucketsStatus(flagLocal, buckets)
					advance("Checking buckets...")
				}()

				// Goroutine D: per-app status checks.
				inner.Add(1)
				go func() {
					defer inner.Done()
					appResults = ctx.Services.Status.CheckStatus(checkApps, buckets, func() {
						n := int(done.Add(1))
						appsDone := n - 2
						if appsDone < 0 {
							appsDone = 0
						}
						bar.SetProgress(n, fmt.Sprintf("Checking apps (%d/%d)...", appsDone, len(checkApps)))
					})
				}()

				inner.Wait()
			}()

			wg.Wait()
			bar.Stop()
			_ = allBuckets // used inside goroutine D above

			// Print scoop/buckets status.
			if scoopOutdated {
				fmt.Fprintf(os.Stdout, "%s Scoop has updates available. Run 'scoop update' to update.\n", ui.Warning("!"))
			} else {
				fmt.Fprintf(os.Stdout, "%s Scoop is up to date.\n", ui.Success("✓"))
			}

			if bucketsOutdated {
				fmt.Fprintf(os.Stdout, "%s Some buckets have updates available. Run 'scg bucket update' to update.\n", ui.Warning("!"))
			} else {
				fmt.Fprintf(os.Stdout, "%s All buckets are up to date.\n", ui.Success("✓"))
			}
			fmt.Fprintln(os.Stdout)

			// Filter: only show outdated, failed, or has missing deps.
			var filtered []service.AppStatusResult
			for _, r := range appResults {
				if r.Outdated || r.Failed || len(r.MissingDeps) > 0 {
					filtered = append(filtered, r)
				}
			}

			if len(filtered) == 0 {
				fmt.Fprintf(os.Stdout, "%s All installed apps are up to date.\n", ui.Success("✓"))
				return nil
			}

			// Sort by name.
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].Name < filtered[j].Name
			})

			// Table header.
			header := []string{
				ui.BoldGreen("Name"),
				ui.BoldGreen("Installed"),
				ui.BoldGreen("Latest"),
				ui.BoldGreen("Missing Deps"),
				ui.BoldGreen("Info"),
			}
			rows := [][]string{header}

			for _, r := range filtered {
				name := ui.Cyan(r.Name)

				latest := r.Latest
				if r.Outdated {
					latest = "* " + r.Latest
				}

				info := ""
				if r.Failed {
					info = ui.Red("Failed")
				}
				if r.Held {
					if info != "" {
						info += " "
					}
					info += ui.Yellow("Held")
				}

				missingDeps := strings.Join(r.MissingDeps, ", ")

				rows = append(rows, []string{name, r.Installed, latest, missingDeps, info})
			}

			_ = flagVerbose
			fmt.Fprintln(os.Stdout, ui.FormatLineColumns(rows, []float64{2.0, 1.0, 1.0, 1.0, 1.5}))
			fmt.Fprintf(os.Stdout, "\n%s\n", ui.Dim(fmt.Sprintf("%d app(s) need attention", len(filtered))))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagLocal, "local", "l", false, "Skip git fetch, use local state only")
	cmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show detailed output")
	return cmd
}

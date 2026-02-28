package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/amrkmn/scg/internal/cmdctx"
	"github.com/amrkmn/scg/internal/service"
	"github.com/amrkmn/scg/internal/ui"
	"github.com/spf13/cobra"
)

// NewSearchCommand creates the search subcommand.
func NewSearchCommand() *cobra.Command {
	var flagGlobal, flagVerbose, flagInstalled bool
	var flagBucket string

	cmd := &cobra.Command{
		Use:     "search <query>",
		Short:   "Search for apps in buckets",
		Args:    cobra.ExactArgs(1),
		Example: "scg search git\nscg search -b main python\nscg search --installed git",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)
			if ctx == nil {
				return fmt.Errorf("context unavailable")
			}
			query := args[0]

			spinner := ui.NewSpinner(fmt.Sprintf("Searching for '%s'...", query))
			spinner.Start()

			// Run ListInstalled and SearchBuckets concurrently.
			var (
				installed []service.InstalledApp
				results   []service.SearchResult
				wg        sync.WaitGroup
			)

			wg.Add(1)
			go func() {
				defer wg.Done()
				installed, _ = ctx.Services.Apps.ListInstalled("")
			}()

			// SearchBuckets itself fans out one goroutine per bucket, each of
			// which now uses an intra-bucket worker pool — so this blocks until
			// all buckets are done, but all buckets run in parallel.
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Pass a nil InstalledApps for now; we'll mark installed below
				// once both goroutines finish.
				results = ctx.Services.Search.SearchBuckets(query, service.SearchOptions{
					Bucket:        flagBucket,
					CaseSensitive: false,
					GlobalOnly:    flagGlobal,
					InstalledOnly: false, // filter after merge
				})
			}()

			wg.Wait()
			spinner.Stop()

			// Build installed map now that both are done.
			installedMap := make(map[string]*service.InstalledApp, len(installed))
			for i := range installed {
				a := &installed[i]
				installedMap[strings.ToLower(a.Name)] = a
			}

			// Apply installed marking and installed-only filter post-search.
			var filtered []service.SearchResult
			for i := range results {
				r := &results[i]
				if app, ok := installedMap[strings.ToLower(r.Name)]; ok {
					if app.Bucket == "" || strings.EqualFold(app.Bucket, r.Bucket) {
						r.IsInstalled = true
					}
				}
				if flagInstalled && !r.IsInstalled {
					continue
				}
				filtered = append(filtered, *r)
			}

			if len(filtered) == 0 {
				fmt.Fprintf(os.Stdout, "%s No results found for '%s'.\n", ui.Warning("!"), query)
				return nil
			}

			// Group by bucket.
			bucketMap := make(map[string][]service.SearchResult)
			bucketOrder := []string{}
			for _, r := range filtered {
				key := r.Bucket
				if _, ok := bucketMap[key]; !ok {
					bucketOrder = append(bucketOrder, key)
				}
				bucketMap[key] = append(bucketMap[key], r)
			}
			sort.Strings(bucketOrder)

			for _, bucketName := range bucketOrder {
				items := bucketMap[bucketName]

				// Sort: installed first, then alphabetical.
				sort.Slice(items, func(i, j int) bool {
					if items[i].IsInstalled != items[j].IsInstalled {
						return items[i].IsInstalled
					}
					return items[i].Name < items[j].Name
				})

				fmt.Fprintf(os.Stdout, "%s\n", ui.Bold(bucketName+":"))
				for _, r := range items {
					installedTag := ""
					if r.IsInstalled {
						installedTag = " " + ui.DimGreen("[installed]")
					}
					line := fmt.Sprintf("  %s %s%s",
						ui.BoldCyan(r.Name),
						ui.DimGreen("("+r.Version+")"),
						installedTag,
					)
					fmt.Fprintln(os.Stdout, line)
					if flagVerbose {
						if r.Description != "" {
							fmt.Fprintf(os.Stdout, "    %s\n", ui.Dim(r.Description))
						}
						for _, b := range r.Binaries {
							fmt.Fprintf(os.Stdout, "    %s %s\n", ui.Dim("-->"), b)
						}
					}
				}
				fmt.Fprintln(os.Stdout)
			}

			fmt.Fprintf(os.Stdout, "%s\n", ui.Blue(fmt.Sprintf("Found %d package(s).", len(filtered))))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Search in global scope only")
	cmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show description and binaries")
	cmd.Flags().BoolVarP(&flagInstalled, "installed", "i", false, "Show only installed apps")
	cmd.Flags().StringVarP(&flagBucket, "bucket", "b", "", "Filter by bucket name")
	return cmd
}

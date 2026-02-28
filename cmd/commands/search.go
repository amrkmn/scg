package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
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

			results := ctx.Services.Search.SearchBuckets(query, service.SearchOptions{
				Bucket:        flagBucket,
				CaseSensitive: false,
				GlobalOnly:    flagGlobal,
				InstalledOnly: flagInstalled,
			})

			if len(results) == 0 {
				fmt.Fprintf(os.Stdout, "%s No results found for '%s'.\n", ui.Warning("!"), query)
				return nil
			}

			// Group by bucket.
			bucketMap := make(map[string][]service.SearchResult)
			bucketOrder := []string{}
			for _, r := range results {
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

			fmt.Fprintf(os.Stdout, "%s\n", ui.Blue(fmt.Sprintf("Found %d package(s).", len(results))))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Search in global scope only")
	cmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show description and binaries")
	cmd.Flags().BoolVarP(&flagInstalled, "installed", "i", false, "Show only installed apps")
	cmd.Flags().StringVarP(&flagBucket, "bucket", "b", "", "Filter by bucket name")
	return cmd
}

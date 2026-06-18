package commands

import (
	"fmt"
	"sort"
	"strings"

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
		Use:     "search [query]",
		Short:   "Search for apps in buckets",
		Args:    cobra.MaximumNArgs(1),
		Example: "scg search git\nscg search -b main python\nscg search --installed git\nscg search",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			installedApps, err := searchInstalledApps(ctx.Services.Apps, flagInstalled || flagVerbose)
			if err != nil {
				return err
			}

			results := ctx.Services.Search.SearchBuckets(query, service.SearchOptions{
				Bucket:        flagBucket,
				CaseSensitive: false,
				GlobalOnly:    flagGlobal,
				InstalledOnly: flagInstalled,
				InstalledApps: installedApps,
			})

			if len(results) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Skip("search", fmt.Sprintf("no results for %s", query)))
				return nil
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), renderSearchResults(results, flagVerbose))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Search in global scope only")
	cmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show description and binaries")
	cmd.Flags().BoolVarP(&flagInstalled, "installed", "i", false, "Show only installed apps")
	cmd.Flags().StringVarP(&flagBucket, "bucket", "b", "", "Filter by bucket name")
	return cmd
}

func renderSearchResults(results []service.SearchResult, verbose bool) string {
	var out strings.Builder
	out.WriteString(ui.Heading("Search results"))
	out.WriteString("\n")

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
		sort.Slice(items, func(i, j int) bool {
			if items[i].IsInstalled != items[j].IsInstalled {
				return items[i].IsInstalled
			}
			return items[i].Name < items[j].Name
		})

		out.WriteString(ui.Bold(bucketName + ":"))
		out.WriteString("\n")
		out.WriteString(renderSearchBucketTable(items, verbose))
		out.WriteString("\n\n")
	}

	out.WriteString(ui.Done("search", fmt.Sprintf("%d package(s)", len(results))))
	return out.String()
}

func renderSearchBucketTable(items []service.SearchResult, verbose bool) string {
	rows := make([][]string, 0, len(items))
	for _, r := range items {
		status := ""
		if r.IsInstalled {
			status = ui.Green("installed")
		}
		if verbose {
			details := r.Description
			if len(r.Binaries) > 0 {
				bins := strings.Join(r.Binaries, ", ")
				if details != "" {
					details += " | "
				}
				details += "bin: " + bins
			}
			rows = append(rows, []string{ui.BoldCyan(r.Name), r.Version, status, details})
			continue
		}
		rows = append(rows, []string{ui.BoldCyan(r.Name), r.Version, status})
	}

	headers := []string{"Package", "Version", "Status"}
	weights := []float64{0.45, 0.2, 0.35}
	if verbose {
		headers = []string{"Package", "Version", "Status", "Details"}
		weights = []float64{0.25, 0.15, 0.15, 0.45}
	}

	return ui.RenderTable(headers, rows, weights, "")
}

func searchInstalledApps(apps *service.AppsService, needed bool) (map[string]*service.InstalledApp, error) {
	if !needed {
		return nil, nil
	}
	installed, err := apps.ListInstalled("")
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*service.InstalledApp, len(installed))
	for i := range installed {
		app := installed[i]
		byName[strings.ToLower(app.Name)] = &app
	}
	return byName, nil
}

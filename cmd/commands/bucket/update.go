package bucket

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

func NewUpdateCommand() *cobra.Command {
	var global bool
	var changelog bool

	cmd := &cobra.Command{
		Use:   "update [name...]",
		Short: "Update buckets",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)

			scope := scoop.ScopeUser
			if global {
				scope = scoop.ScopeGlobal
			}

			names := args
			if len(names) == 0 {
				allBuckets, err := ctx.Services.Buckets.List(scope)
				if err != nil {
					_, _ = fmt.Fprintln(os.Stderr, ui.FailLine(fmt.Sprintf("failed to list buckets: %v", err)))
					os.Exit(1)
				}
				for _, b := range allBuckets {
					names = append(names, b.Name)
				}
				sort.Strings(names)
			}

			if len(names) == 0 {
				_, _ = fmt.Println(ui.Skip("buckets", "none to update"))
				return nil
			}

			_, _ = fmt.Println(ui.Heading(fmt.Sprintf("Updating %d bucket(s)", len(names))))

			results := ctx.Services.Buckets.UpdateBuckets(cmd.Context(), names, scope, changelog, nil, nil)

			updated := 0
			upToDate := 0
			failed := 0
			for _, r := range results {
				switch r.Status {
				case "updated":
					updated++
					_, _ = fmt.Println(ui.Done(r.Name, "updated"))
				case "up-to-date":
					upToDate++
					_, _ = fmt.Println(ui.Skip(r.Name, "up-to-date"))
				case "failed":
					failed++
					msg := "failed"
					if r.Error != nil {
						msg = fmt.Sprintf("failed: %v", r.Error)
					}
					_, _ = fmt.Fprintln(os.Stderr, ui.FailLine(r.Name+": "+msg))
				}
			}

			kind := ui.StatusDone
			detail := fmt.Sprintf("%d updated, %d up-to-date", updated, upToDate)
			if failed > 0 {
				detail = fmt.Sprintf("%d updated, %d up-to-date, %d failed", updated, upToDate, failed)
			}
			_, _ = fmt.Println()
			_, _ = fmt.Println(ui.RenderStatusSummary(kind, "bucket", detail))

			if changelog && updated > 0 {
				_, _ = fmt.Println()
				_, _ = fmt.Println(ui.Heading("Changelog"))
				sortedResults := make([]service.UpdateResult, len(results))
				copy(sortedResults, results)
				sort.Slice(sortedResults, func(i, j int) bool {
					return sortedResults[i].Name < sortedResults[j].Name
				})
				for _, r := range sortedResults {
					if r.Status == "updated" && len(r.Commits) > 0 {
						fmt.Printf("  %s:\n", ui.Cyan(ui.Bold(r.Name)))
						for _, commit := range r.Commits {
							fmt.Printf("    %s\n", commit)
						}
					}
				}
			}

			if failed > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Update buckets in global Scoop installation")
	cmd.Flags().BoolVarP(&changelog, "changelog", "c", false, "Show new commits after update")
	return cmd
}

package bucket

import (
	"fmt"
	"os"
	"sort"

	"github.com/amrkmn/scg/internal/cmdctx"
	"github.com/amrkmn/scg/internal/ui"
	"github.com/spf13/cobra"
)

// NewListCommand creates the bucket list subcommand.
func NewListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed buckets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)
			if ctx == nil {
				return fmt.Errorf("context unavailable")
			}

			buckets, err := ctx.Services.Buckets.List("")
			if err != nil {
				return err
			}

			if len(buckets) == 0 {
				fmt.Fprintln(os.Stdout, ui.Dim("No buckets installed."))
				return nil
			}

			sort.Slice(buckets, func(i, j int) bool {
				return buckets[i].Name < buckets[j].Name
			})

			header := []string{
				ui.BoldGreen("Name"),
				ui.BoldGreen("Source"),
				ui.BoldGreen("Updated"),
				ui.BoldGreen("Manifests"),
			}
			rows := [][]string{header}

			for _, b := range buckets {
				updated := ""
				if !b.Updated.IsZero() {
					updated = b.Updated.Format("2006-01-02 15:04:05")
				}
				manifests := ""
				if b.Manifests > 0 {
					manifests = fmt.Sprintf("%d", b.Manifests)
				}
				rows = append(rows, []string{
					ui.Cyan(b.Name),
					b.Source,
					updated,
					manifests,
				})
			}

			fmt.Fprintln(os.Stdout, ui.FormatLineColumns(rows, []float64{1.0, 3.0, 1.5, 0.5}))
			fmt.Fprintf(os.Stdout, "\n%s\n", ui.Dim(fmt.Sprintf("%d bucket(s) installed", len(buckets))))
			return nil
		},
	}
}

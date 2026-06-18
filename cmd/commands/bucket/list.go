package bucket

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/ui"
)

// NewListCommand creates the bucket list subcommand.
func NewListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed buckets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			buckets, err := ctx.Services.Buckets.List("")
			if err != nil {
				return err
			}

			if len(buckets) == 0 {
				ctx.GetLogger().Skip("buckets", "none installed")
				return nil
			}

			sort.Slice(buckets, func(i, j int) bool {
				return buckets[i].Name < buckets[j].Name
			})

			rows := make([][]string, 0, len(buckets))

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
					ui.BoldCyan(b.Name),
					b.Source,
					updated,
					manifests,
				})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderTable(
				[]string{"Name", "Source", "Updated", "Manifests"},
				rows,
				[]float64{1.0, 3.0, 1.5, 0.5},
				fmt.Sprintf("%d bucket(s) installed", len(buckets)),
			))
			return nil
		},
	}
}

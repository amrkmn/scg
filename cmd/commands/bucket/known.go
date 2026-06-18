package bucket

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/known"
	"go.noz.one/scg/internal/ui"
)

func NewKnownCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "known",
		Short: "List known buckets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			buckets := known.GetAllKnownBuckets()

			rows := make([][]string, 0, len(buckets))
			for _, b := range buckets {
				rows = append(rows, []string{
					ui.BoldCyan(b.Name),
					b.Source,
				})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderTable(nil, rows, []float64{0.25, 0.75}, ""))

			return nil
		},
	}
}

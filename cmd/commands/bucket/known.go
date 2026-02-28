package bucket

import (
	"fmt"
	"strings"

	"github.com/amrkmn/scg/internal/known"
	"github.com/amrkmn/scg/internal/ui"
	"github.com/spf13/cobra"
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
					ui.Bold(b.Name),
					b.Source,
				})
			}

			output := ui.FormatLineColumns(rows, []float64{0.25, 0.75})
			// FormatLineColumns returns a single newline-separated string
			for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
				fmt.Println(line)
			}

			return nil
		},
	}
}

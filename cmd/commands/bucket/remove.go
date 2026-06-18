package bucket

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

func NewRemoveCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a bucket",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)
			logger := ctx.GetLogger()

			name := args[0]
			scope := scoop.ScopeUser
			if global {
				scope = scoop.ScopeGlobal
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Heading(formatBucketHeading("Removing", name, scope)))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.StatusWithOptions(ui.StatusRunning, "remove", ui.BoldCyan(name), ui.StatusOptions{}))

			err := ctx.Services.Buckets.Remove(name, scope)

			if err != nil {
				logger.Error(fmt.Sprintf("failed to remove bucket %q: %v", name, err))
				os.Exit(1)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Done("bucket", ui.BoldCyan(name)))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDone, "bucket", "1 removed, 0 skipped"))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Remove bucket from global Scoop installation")
	return cmd
}

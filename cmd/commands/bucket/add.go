package bucket

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/known"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

func NewAddCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "add <name> [url]",
		Short: "Add a bucket",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)
			logger := ctx.GetLogger()

			name := args[0]
			var url string

			if len(args) == 2 {
				url = args[1]
			} else {
				url = known.GetKnownBucket(name)
				if url == "" {
					logger.Error(fmt.Sprintf("bucket %q not found in known buckets and no URL provided", name))
					os.Exit(1)
				}
			}

			scope := scoop.ScopeUser
			if global {
				scope = scoop.ScopeGlobal
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Heading(formatBucketHeading("Adding", name, scope)))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.StatusWithOptions(ui.StatusRunning, "clone", ui.Dim(url), ui.StatusOptions{}))

			err := ctx.Services.Buckets.Add(name, url, scope, func(current, total int) {})

			if err != nil {
				logger.Error(fmt.Sprintf("failed to add bucket %q: %v", name, err))
				os.Exit(1)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Done("bucket", ui.BoldCyan(name)))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDone, "bucket", "1 added, 0 skipped"))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Add bucket to global Scoop installation")
	return cmd
}

func formatBucketHeading(action, name string, scope scoop.InstallScope) string {
	if scope == scoop.ScopeGlobal {
		return fmt.Sprintf("%s bucket %s [global]", action, name)
	}
	return fmt.Sprintf("%s bucket %s", action, name)
}

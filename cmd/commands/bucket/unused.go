package bucket

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

func NewUnusedCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "unused",
		Short: "List buckets not providing any installed app",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)
			logger := ctx.GetLogger()

			var scope scoop.InstallScope
			if global {
				scope = scoop.ScopeGlobal
			} else {
				scope = scoop.ScopeUser
			}

			buckets, err := ctx.Services.Buckets.List(scope)
			if err != nil {
				logger.Error(fmt.Sprintf("failed to list buckets: %v", err))
				os.Exit(1)
			}

			installedApps, err := ctx.Services.Apps.ListInstalled("")
			if err != nil {
				logger.Error(fmt.Sprintf("failed to list installed apps: %v", err))
				os.Exit(1)
			}

			// Build set of buckets that provide at least one installed app
			usedBuckets := make(map[string]struct{})
			for _, app := range installedApps {
				if app.Bucket != "" {
					usedBuckets[app.Bucket] = struct{}{}
				}
			}

			unused := make([]service.BucketInfo, 0)
			for _, b := range buckets {
				if _, used := usedBuckets[b.Name]; !used {
					unused = append(unused, b)
				}
			}

			if len(unused) == 0 {
				logger.Skip("buckets", "none unused")
				return nil
			}

			logger.Header("Unused buckets")

			for _, b := range unused {
				fmt.Printf("%s  %s\n", ui.Bold(b.Name), ui.Dim(b.Source))
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Check global Scoop installation")
	return cmd
}

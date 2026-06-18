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

			var scope scoop.InstallScope
			if global {
				scope = scoop.ScopeGlobal
			} else {
				scope = scoop.ScopeUser
			}

			buckets, err := ctx.Services.Buckets.List(scope)
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, ui.FailLine("failed to list buckets: "+err.Error()))
				os.Exit(1)
			}

			installedApps, err := ctx.Services.Apps.ListInstalled("")
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, ui.FailLine("failed to list installed apps: "+err.Error()))
				os.Exit(1)
			}

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

			_, _ = fmt.Println(ui.Heading("Unused buckets"))

			if len(unused) == 0 {
				_, _ = fmt.Println(ui.Skip("buckets", "none unused"))
			} else {
				for _, b := range unused {
					_, _ = fmt.Printf("  %s  %s\n", ui.Bold(b.Name), ui.Dim(b.Source))
				}
			}

			_, _ = fmt.Println()
			_, _ = fmt.Println(ui.RenderStatusSummary(ui.StatusDone, "bucket", fmt.Sprintf("%d unused", len(unused))))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Check global Scoop installation")
	return cmd
}

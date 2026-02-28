package bucket

import (
	"fmt"
	"os"

	"github.com/amrkmn/scg/internal/cmdctx"
	"github.com/amrkmn/scg/internal/scoop"
	"github.com/amrkmn/scg/internal/service"
	"github.com/amrkmn/scg/internal/ui"
	"github.com/spf13/cobra"
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
				fmt.Fprintf(os.Stderr, "%s Failed to list buckets: %v\n", ui.Error("error:"), err)
				os.Exit(1)
			}

			installedApps, err := ctx.Services.Apps.ListInstalled("")
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s Failed to list installed apps: %v\n", ui.Error("error:"), err)
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
				fmt.Println("No unused buckets found.")
				return nil
			}

			for _, b := range unused {
				fmt.Printf("%s  %s\n", ui.Bold(b.Name), ui.Dim(b.Source))
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Check global Scoop installation")
	return cmd
}

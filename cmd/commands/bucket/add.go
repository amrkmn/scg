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

			name := args[0]
			var url string

			if len(args) == 2 {
				url = args[1]
			} else {
				url = known.GetKnownBucket(name)
				if url == "" {
					fmt.Fprintf(os.Stderr, "%s bucket '%s' not found in known buckets and no URL provided\n",
						ui.Error("error:"), name)
					os.Exit(1)
				}
			}

			scope := scoop.ScopeUser
			if global {
				scope = scoop.ScopeGlobal
			}

			spinner := ui.NewSpinner(fmt.Sprintf("Adding bucket '%s'...", name))
			spinner.Start()

			err := ctx.Services.Buckets.Add(name, url, scope, func(current, total int) {})

			spinner.Stop()

			if err != nil {
				fmt.Fprintf(os.Stderr, "%s Failed to add bucket '%s': %v\n", ui.Error("error:"), name, err)
				os.Exit(1)
			}

			fmt.Printf("%s Bucket '%s' added successfully\n", ui.Success("success:"), name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Add bucket to global Scoop installation")
	return cmd
}

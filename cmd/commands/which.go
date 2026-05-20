package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
)

// NewWhichCommand creates the which subcommand.
func NewWhichCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "which <command>",
		Short:   "Show the path to an executable managed by Scoop",
		Args:    cobra.ExactArgs(1),
		Example: "scg which git\nscg which python",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			paths, err := ctx.Services.Shims.FindExecutable(args[0])
			if err != nil || len(paths) == 0 {
				ctx.GetLogger().Error(fmt.Sprintf("command %q not found", args[0]))
				return os.ErrNotExist
			}
			for _, p := range paths {
				fmt.Println(p)
			}
			return nil
		},
	}
}

package commands

import (
	"fmt"
	"os"

	"github.com/amrkmn/scg/internal/cmdctx"
	"github.com/spf13/cobra"
)

// NewWhichCommand creates the which subcommand.
func NewWhichCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "which <command>",
		Short:   "Show the path to an executable managed by Scoop",
		Args:    cobra.ExactArgs(1),
		Example: "scg which git\nscg which python",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)
			if ctx == nil {
				return fmt.Errorf("context unavailable")
			}
			paths, err := ctx.Services.Shims.FindExecutable(args[0])
			if err != nil || len(paths) == 0 {
				fmt.Fprintf(os.Stderr, "Command '%s' not found\n", args[0])
				return os.ErrNotExist
			}
			for _, p := range paths {
				fmt.Println(p)
			}
			return nil
		},
	}
}

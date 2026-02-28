package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
)

// NewPrefixCommand creates the prefix subcommand.
func NewPrefixCommand() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:     "prefix <app>",
		Short:   "Show the install prefix of an app",
		Args:    cobra.ExactArgs(1),
		Example: "scg prefix git\nscg prefix -g git",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)
			if ctx == nil {
				return fmt.Errorf("context unavailable")
			}
			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}
			path, err := ctx.Services.Apps.GetAppPrefix(args[0], scope)
			if err != nil {
				fmt.Fprintf(os.Stderr, "App '%s' not found in %s scope\n", args[0], scope)
				return err
			}
			fmt.Println(path)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Search in global scope")
	return cmd
}

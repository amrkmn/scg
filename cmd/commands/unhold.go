package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

// NewUnholdCommand creates the unhold subcommand.
func NewUnholdCommand() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:     "unhold <app>",
		Short:   "Unhold an app to allow updates",
		Args:    cobra.ExactArgs(1),
		Example: "scg unhold git\nscg unhold -g git",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			app := args[0]
			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}

			if app == "scoop" {
				fmt.Printf("%s scoop is no longer held and can be updated.\n", ui.Success("+"))
				return nil
			}

			installed, err := ctx.Services.Apps.GetInstalledApp(app, scope)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s '%s' is not installed in %s scope\n", ui.Error("-"), app, scope)
				return err
			}

			if !installed.Held {
				fmt.Printf("%s '%s' is not held.\n", ui.Info("i"), app)
				return nil
			}

			if err := ctx.Services.Apps.SetHold(app, scope, false); err != nil {
				return fmt.Errorf("failed to unhold '%s': %w", app, err)
			}

			fmt.Printf("%s '%s' is no longer held and can be updated.\n", ui.Success("+"), app)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Unhold a globally installed app")
	return cmd
}

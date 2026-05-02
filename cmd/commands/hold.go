package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

// NewHoldCommand creates the hold subcommand.
func NewHoldCommand() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:     "hold <app>",
		Short:   "Hold an app to prevent updates",
		Args:    cobra.ExactArgs(1),
		Example: "scg hold git\nscg hold -g git",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			app := args[0]
			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}

			if app == "scoop" {
				fmt.Printf("%s scoop is now held and will not be updated.\n", ui.Success("+"))
				return nil
			}

			installed, err := ctx.Services.Apps.GetInstalledApp(app, scope)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s '%s' is not installed in %s scope\n", ui.Error("-"), app, scope)
				return err
			}

			if installed.Held {
				fmt.Printf("%s '%s' is already held.\n", ui.Info("i"), app)
				return nil
			}

			if err := ctx.Services.Apps.SetHold(app, scope, true); err != nil {
				return fmt.Errorf("failed to hold '%s': %w", app, err)
			}

			fmt.Printf("%s '%s' is now held and will not be updated.\n", ui.Success("+"), app)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Hold a globally installed app")
	return cmd
}

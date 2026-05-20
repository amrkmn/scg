package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
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
				ctx.GetLogger().Done("scoop", "held")
				return nil
			}

			installed, err := ctx.Services.Apps.GetInstalledApp(app, scope)
			if err != nil {
				ctx.GetLogger().Error(fmt.Sprintf("%s is not installed in %s scope", app, scope))
				return err
			}

			if installed.Held {
				ctx.GetLogger().Skip(app, "already held")
				return nil
			}

			if err := ctx.Services.Apps.SetHold(app, scope, true); err != nil {
				return fmt.Errorf("failed to hold '%s': %w", app, err)
			}

			ctx.GetLogger().Done(app, "held")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Hold a globally installed app")
	return cmd
}

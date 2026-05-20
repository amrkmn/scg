package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
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
				ctx.GetLogger().Done("scoop", "unheld")
				return nil
			}

			installed, err := ctx.Services.Apps.GetInstalledApp(app, scope)
			if err != nil {
				ctx.GetLogger().Error(fmt.Sprintf("%s is not installed in %s scope", app, scope))
				return err
			}

			if !installed.Held {
				ctx.GetLogger().Skip(app, "not held")
				return nil
			}

			if err := ctx.Services.Apps.SetHold(app, scope, false); err != nil {
				return fmt.Errorf("failed to unhold '%s': %w", app, err)
			}

			ctx.GetLogger().Done(app, "unheld")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Unhold a globally installed app")
	return cmd
}

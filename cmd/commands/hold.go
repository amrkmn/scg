package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

// NewHoldCommand creates the hold subcommand.
func NewHoldCommand() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:     "hold <app> [app...]",
		Short:   "Hold apps to prevent updates",
		Args:    cobra.MinimumNArgs(1),
		Example: "scg hold git\nscg hold -g git\nscg hold git nodejs",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}

			var held, skipped, failed int
			for _, app := range args {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Heading(formatHoldHeading("Holding", app, scope)))
				r := holdOne(ctx, app, scope)
			switch r {
			case 0:
				held++
			case 1:
				skipped++
			default:
				failed++
			}
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDone, "hold", fmt.Sprintf("%d held, %d skipped, %d failed", held, skipped, failed)))

			if failed > 0 {
				return fmt.Errorf("%d app(s) failed to hold", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Hold globally installed apps")
	return cmd
}

// holdOne holds a single app. Returns 0 for held, 1 for skipped, 2 for failed.
func holdOne(ctx *app.Context, appName string, scope scoop.InstallScope) int {
	if appName == "scoop" {
		fmt.Println()
		fmt.Println(ui.Done("hold", ui.BoldCyan("scoop")))
		return 0
	}

	installed, err := ctx.Services.Apps.GetInstalledApp(appName, scope)
	if err != nil {
		ctx.GetLogger().Error(fmt.Sprintf("%s is not installed in %s scope", appName, scope))
		return 2
	}

	if installed.Held {
		fmt.Println(ui.Skip("hold", ui.BoldCyan(appName)+" already held"))
		return 1
	}

	if err := ctx.Services.Apps.SetHold(appName, scope, true); err != nil {
		ctx.GetLogger().Error(fmt.Sprintf("failed to hold '%s': %v", appName, err))
		return 2
	}

	fmt.Println(ui.Done("hold", ui.BoldCyan(appName)))
	return 0
}

func formatHoldHeading(action, app string, scope scoop.InstallScope) string {
	if scope == scoop.ScopeGlobal {
		return fmt.Sprintf("%s %s [global]", action, app)
	}
	return fmt.Sprintf("%s %s", action, app)
}

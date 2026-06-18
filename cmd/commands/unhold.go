package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

// NewUnholdCommand creates the unhold subcommand.
func NewUnholdCommand() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:     "unhold <app> [app...]",
		Short:   "Unhold apps to allow updates",
		Args:    cobra.MinimumNArgs(1),
		Example: "scg unhold git\nscg unhold -g git\nscg unhold git nodejs",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}

			var unheld, skipped, failed int
			for _, a := range args {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Heading(formatHoldHeading("Unholding", a, scope)))
				r := unholdOne(ctx, a, scope)
				switch {
				case r == 0:
					unheld++
				case r == 1:
					skipped++
				default:
					failed++
				}
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDone, "unhold", fmt.Sprintf("%d unheld, %d skipped, %d failed", unheld, skipped, failed)))

			if failed > 0 {
				return fmt.Errorf("%d app(s) failed to unhold", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Unhold globally installed apps")
	return cmd
}

// unholdOne unholds a single app. Returns 0 for unheld, 1 for skipped, 2 for failed.
func unholdOne(ctx *app.Context, appName string, scope scoop.InstallScope) int {
	if appName == "scoop" {
		fmt.Println()
		fmt.Println(ui.Done("unhold", ui.BoldCyan("scoop")))
		return 0
	}

	installed, err := ctx.Services.Apps.GetInstalledApp(appName, scope)
	if err != nil {
		ctx.GetLogger().Error(fmt.Sprintf("%s is not installed in %s scope", appName, scope))
		return 2
	}

	if !installed.Held {
		fmt.Println(ui.Skip("unhold", ui.BoldCyan(appName)+" not held"))
		return 1
	}

	if err := ctx.Services.Apps.SetHold(appName, scope, false); err != nil {
		ctx.GetLogger().Error(fmt.Sprintf("failed to unhold '%s': %v", appName, err))
		return 2
	}

	fmt.Println(ui.Done("unhold", ui.BoldCyan(appName)))
	return 0
}

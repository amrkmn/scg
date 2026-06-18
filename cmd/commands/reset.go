package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// NewResetCommand creates the reset subcommand.
func NewResetCommand() *cobra.Command {
	var flagAll, flagGlobal bool

	cmd := &cobra.Command{
		Use:     "reset <app> [app...]",
		Short:   "Reset apps to resolve shim and environment conflicts",
		Long:    "Reset installed apps by relinking current, recreating shims and shortcuts, and refreshing PATH, environment, and persist links.",
		Example: "scg reset python\nscg reset python@3.12.0\nscg reset --all",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !flagAll && len(args) == 0 {
				return fmt.Errorf("specify apps to reset or use --all")
			}

			ctx := cmdctx.MustFromCmd(cmd)
			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Heading(formatResetHeading(args, scope, flagAll)))

			results := ctx.Services.Resetter.Reset(args, service.ResetOptions{
				Scope:          scope,
				GlobalExplicit: flagGlobal,
				All:            flagAll,
			})

			var reset, skipped, failed int
			for _, r := range results {
				if r.Success {
					reset++
				} else if r.Skipped {
					skipped++
				} else {
					failed++
				}
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDone, "reset", fmt.Sprintf("%d reset, %d skipped, %d failed", reset, skipped, failed)))

			if failed > 0 {
				return fmt.Errorf("%d app(s) failed to reset", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagAll, "all", "a", false, "Reset all installed apps")
	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Reset global apps")
	return cmd
}

func formatResetHeading(apps []string, scope scoop.InstallScope, all bool) string {
	scopeTag := ""
	if scope == scoop.ScopeGlobal {
		scopeTag = " [global]"
	}
	if all {
		return fmt.Sprintf("Resetting all apps%s", scopeTag)
	}
	if len(apps) == 1 {
		return fmt.Sprintf("Resetting %s%s", apps[0], scopeTag)
	}
	return fmt.Sprintf("Resetting %d apps%s", len(apps), scopeTag)
}

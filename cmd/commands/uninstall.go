package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

func NewUninstallCommand() *cobra.Command {
	var flagGlobal, flagPurge bool

	cmd := &cobra.Command{
		Use:     "uninstall <app> [app...]",
		Short:   "Uninstall apps",
		Long:    `Uninstall one or more Scoop apps, removing shims, env vars, and start menu shortcuts.`,
		Args:    cobra.MinimumNArgs(1),
		Example: "scg uninstall git\nscg uninstall nodejs python --global\nscg uninstall nrfutil --purge",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Heading(formatUninstallHeading(args, scope)))

			opts := service.UninstallOptions{
				Scope: scope,
				Purge: flagPurge,
			}

			results := ctx.Services.Uninstaller.Uninstall(args, opts)

			var uninstalled, failed int
			for _, r := range results {
				if r.Success {
					uninstalled++
				} else {
					failed++
				}
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDone, "uninstall", fmt.Sprintf("%d uninstalled, %d failed", uninstalled, failed)))

			if failed > 0 {
				return fmt.Errorf("%d app(s) failed to uninstall", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Uninstall globally")
	cmd.Flags().BoolVarP(&flagPurge, "purge", "p", false, "Remove persisted data")

	return cmd
}

func formatUninstallHeading(apps []string, scope scoop.InstallScope) string {
	scopeTag := ""
	if scope == scoop.ScopeGlobal {
		scopeTag = " [global]"
	}
	if len(apps) == 1 {
		return fmt.Sprintf("Uninstalling %s%s", apps[0], scopeTag)
	}
	return fmt.Sprintf("Uninstalling %d apps%s", len(apps), scopeTag)
}

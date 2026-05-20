package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
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

			results := ctx.Services.Resetter.Reset(args, service.ResetOptions{
				Scope:          scope,
				GlobalExplicit: flagGlobal,
				All:            flagAll,
			})

			var failed int
			for _, r := range results {
				if !r.Success && !r.Skipped {
					failed++
				}
			}
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

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// NewUpdateCommand creates the update subcommand.
func NewUpdateCommand() *cobra.Command {
	var (
		flagGlobal, flagIndependent, flagNoCache, flagSkipHash, flagForce, flagAll, flagDryRun, flagQuiet bool
		flagArch, flagProxy                                                                           string
	)

	cmd := &cobra.Command{
		Use:   "update <app> [app...]",
		Short: "Update installed apps",
		Long: `Update one or more installed apps to their latest available versions.

Use --all or '*' to update all outdated apps. Unlike Scoop, 'scg update'
without arguments requires explicit targets; it does not self-update Scoop.`,
		Example: "scg update git\nscg update --all\nscg update git --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			if err := install.EnsureScoopInstalled(); err != nil {
				ctx.GetLogger().Warn(err.Error())
				return err
			}

			if !flagAll && len(args) == 0 {
				return fmt.Errorf("specify apps to update or use --all")
			}

			args, flagAll = normalizeUpdateArgs(args, flagAll)

			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}

			opts := service.UpdateOptions{
				Scope:       scope,
				Independent: flagIndependent,
				NoCache:     flagNoCache,
				SkipHash:    flagSkipHash,
				Arch:        flagArch,
				Proxy:       flagProxy,
				Force:       flagForce,
				All:         flagAll,
				DryRun:      flagDryRun,
				Quiet:       flagQuiet,
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Heading(formatUpdateHeading(args, scope)))

			results := ctx.Services.Updater.Update(args, opts)

			var updated, skipped, failed int
			for _, r := range results {
				if r.Skipped {
					skipped++
				} else if r.Success {
					updated++
				} else {
					failed++
				}
			}

			if !flagQuiet {
				if flagDryRun {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDry, "update", fmt.Sprintf("%d would update, %d skipped, %d failed", updated, skipped, failed)))
				} else {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDone, "update", fmt.Sprintf("%d updated, %d skipped, %d failed", updated, skipped, failed)))
				}
			}

			if failed > 0 {
				return fmt.Errorf("%d app(s) failed to update", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Update global apps")
	cmd.Flags().BoolVarP(&flagIndependent, "independent", "i", false, "Don't install dependencies automatically")
	cmd.Flags().BoolVarP(&flagNoCache, "no-cache", "k", false, "Don't use the download cache")
	cmd.Flags().BoolVarP(&flagSkipHash, "skip-hash-check", "s", false, "Skip hash validation")
	cmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Reinstall even if same version")
	cmd.Flags().BoolVarP(&flagAll, "all", "a", false, "Update all outdated apps")
	cmd.Flags().BoolVarP(&flagDryRun, "dry-run", "d", false, "Simulate updates without making changes")
	cmd.Flags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress non-essential output")
	cmd.Flags().StringVar(&flagArch, "arch", "", "Use the specified architecture (64bit, 32bit, arm64)")
	cmd.Flags().StringVar(&flagProxy, "proxy", "", "Download via proxy URL")

	return cmd
}

func normalizeUpdateArgs(args []string, all bool) ([]string, bool) {
	if len(args) == 1 && args[0] == "*" {
		return nil, true
	}
	return args, all
}

func formatUpdateHeading(apps []string, scope scoop.InstallScope) string {
	scopeTag := ""
	if scope == scoop.ScopeGlobal {
		scopeTag = " [global]"
	}
	if len(apps) == 1 {
		return fmt.Sprintf("Updating %s%s", apps[0], scopeTag)
	}
	if len(apps) == 0 {
		return fmt.Sprintf("Updating all apps%s", scopeTag)
	}
	return fmt.Sprintf("Updating %d apps%s", len(apps), scopeTag)
}

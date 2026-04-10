package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// NewInstallCommand creates the install subcommand.
func NewInstallCommand() *cobra.Command {
	var flagGlobal, flagIndependent, flagNoCache, flagSkip bool
	var flagArch, flagProxy string

	cmd := &cobra.Command{
		Use:     "install <app> [app...]",
		Short:   "Install apps",
		Long:    `Install one or more apps from Scoop buckets.`,
		Args:    cobra.MinimumNArgs(1),
		Example: "scg install git\nscg install main/git\nscg install git nodejs --global",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			// Verify scoop is installed.
			if err := install.EnsureScoopInstalled(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s %v\n", ui.Warning("!"), err)
				return err
			}

			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}

			opts := service.InstallOptions{
				Scope:       scope,
				Independent: flagIndependent,
				NoCache:     flagNoCache,
				SkipHash:    flagSkip,
				Arch:        flagArch,
				Proxy:       flagProxy,
			}

			results := ctx.Services.Installer.Install(args, opts)

			// Summary.
			var installed, skipped, failed int
			for _, r := range results {
				if r.Skipped {
					skipped++
				} else if r.Success {
					installed++
				} else {
					failed++
				}
			}

			if failed > 0 {
				return fmt.Errorf("%d app(s) failed to install", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Install globally")
	cmd.Flags().BoolVarP(&flagIndependent, "independent", "i", false, "Don't install dependencies automatically")
	cmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Don't use the download cache")
	cmd.Flags().BoolVarP(&flagSkip, "skip", "s", false, "Skip hash validation")
	cmd.Flags().StringVar(&flagArch, "arch", "", "Use the specified architecture (64bit, 32bit, arm64)")
	cmd.Flags().StringVar(&flagProxy, "proxy", "", "Download via proxy URL")

	return cmd
}

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// NewInstallCommand creates the install subcommand.
func NewInstallCommand() *cobra.Command {
	var flagGlobal, flagIndependent, flagNoCache, flagSkipHash bool
	var flagArch, flagProxy string

	cmd := &cobra.Command{
		Use:   "install <app> [app...] [url] [file.json] [app@version]",
		Short: "Install apps",
		Long: `Install one or more apps from Scoop buckets.

Supports app names, bucket-scoped names, URLs, local manifest files, and version pinning:
  scg install git
  scg install main/git
  scg install https://example.com/app.json
  scg install path\to\manifest.json
  scg install git@2.45.0`,
		Args:    cobra.MinimumNArgs(1),
		Example: "scg install git\nscg install main/git\nscg install git nodejs --global\nscg install https://raw.githubusercontent.com/org/bucket/main/app.json\nscg install git@2.45.0",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			// Verify scoop is installed.
			if err := install.EnsureScoopInstalled(); err != nil {
				ctx.GetLogger().Warn(err.Error())
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
				SkipHash:    flagSkipHash,
				Arch:        flagArch,
				Proxy:       flagProxy,
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Heading(formatInstallHeading(args, scope)))

			results := dispatchInstall(ctx, args, opts)

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

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDone, "install", fmt.Sprintf("%d installed, %d skipped, %d failed", installed, skipped, failed)))

			if failed > 0 {
				return fmt.Errorf("%d app(s) failed to install", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Install globally")
	cmd.Flags().BoolVarP(&flagIndependent, "independent", "i", false, "Don't install dependencies automatically")
	cmd.Flags().BoolVarP(&flagNoCache, "no-cache", "k", false, "Don't use the download cache")
	cmd.Flags().BoolVarP(&flagSkipHash, "skip-hash-check", "s", false, "Skip hash validation")
	cmd.Flags().StringVar(&flagArch, "arch", "", "Use the specified architecture (64bit, 32bit, arm64)")
	cmd.Flags().StringVar(&flagProxy, "proxy", "", "Download via proxy URL")

	return cmd
}

// dispatchInstall routes each input to the appropriate install method based on its type.
func dispatchInstall(ctx *app.Context, args []string, opts service.InstallOptions) []service.InstallResult {
	var results []service.InstallResult
	for _, arg := range args {
		switch classifyInstallInput(arg) {
		case "url":
			results = append(results, ctx.Services.Installer.InstallFromURL(arg, opts))
		case "file":
			results = append(results, ctx.Services.Installer.InstallFromFile(arg, opts))
		case "version":
			name, version := splitVersionInput(arg)
			results = append(results, ctx.Services.Installer.InstallWithVersion(name, version, opts))
		default:
			// Normal app name — delegate to the Install method (handles deps, etc.)
			dummy := []string{arg}
			results = append(results, ctx.Services.Installer.Install(dummy, opts)...)
		}
	}
	return results
}

// classifyInstallInput detects the type of install input.
func classifyInstallInput(input string) string {
	if strings.Contains(input, "://") {
		return "url"
	}
	if strings.HasSuffix(strings.ToLower(input), ".json") {
		if _, err := os.Stat(input); err == nil {
			return "file"
		}
		// URL that ends in .json
		if strings.Contains(input, "/") || strings.Contains(input, `\`) {
			return "file"
		}
	}
	if strings.Contains(input, "@") && !strings.Contains(input, "://") {
		return "version"
	}
	return "app"
}

// splitVersionInput splits "app@version" into ("app", "version").
func splitVersionInput(input string) (string, string) {
	idx := strings.Index(input, "@")
	if idx < 0 {
		return input, ""
	}
	return input[:idx], input[idx+1:]
}

func formatInstallHeading(apps []string, scope scoop.InstallScope) string {
	scopeTag := ""
	if scope == scoop.ScopeGlobal {
		scopeTag = " [global]"
	}
	if len(apps) == 1 {
		return fmt.Sprintf("Installing %s%s", apps[0], scopeTag)
	}
	return fmt.Sprintf("Installing %d apps%s", len(apps), scopeTag)
}

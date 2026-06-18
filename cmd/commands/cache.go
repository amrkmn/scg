package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// NewCacheCommand creates the cache subcommand.
func NewCacheCommand() *cobra.Command {
	var flagAll, flagGlobal, flagDryRun bool

	cmd := &cobra.Command{
		Use:   "cache [subcommand] [app...]",
		Short: "Show or remove cached download files",
		Long: `Show or remove cached download files from Scoop's cache.

  scg cache show         Show all cached files (default)
  scg cache show <app>   Show cached files for specific app(s)
  scg cache rm <app>     Remove cached files for specific app(s)
  scg cache rm *         Remove all cached files

Scoop caches downloaded installer files so you don't need to download
the same files again when you uninstall and reinstall the same version
of an app.`,
		Example: `  scg cache show
  scg cache show git
  scg cache rm git
  scg cache rm *
  scg cache rm -a`,
		Aliases: []string{"cc"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			// No args: show all
			if len(args) == 0 {
				return showAllCache(ctx)
			}

			// Handle subcommand
			subcmd := strings.ToLower(args[0])

			switch subcmd {
			case "show", "ls", "list":
				apps := args[1:]
				if len(apps) == 0 {
					apps = nil // Show all
				}
				return showCache(ctx, apps)

			case "rm", "remove", "delete", "del":
				if len(args) < 2 && !flagAll {
					ctx.GetLogger().Error("specify app name(s) or '*'")
					return nil
				}
				apps := args[1:]
				scope := scoop.ScopeUser
				if flagGlobal {
					scope = scoop.ScopeGlobal
				}
				return removeCache(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), apps, flagAll, scope, flagDryRun)

			case "*":
				scope := scoop.ScopeUser
				if flagGlobal {
					scope = scoop.ScopeGlobal
				}
				return removeCache(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), []string{"*"}, flagAll, scope, flagDryRun)

			default:
				// Treat first arg as app name and show it
				return showCache(ctx, args)
			}
		},
	}

	cmd.Flags().BoolVarP(&flagAll, "all", "a", false, "Remove all cached files")
	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Operate on global scope")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Show what would be removed without removing")

	return cmd
}

func showAllCache(ctx *app.Context) error {
	result, err := ctx.Services.Cache.ListCache(nil)
	if err != nil {
		return err
	}

	displayCacheList(os.Stdout, os.Stderr, result, ctx.GetVerbose())
	return nil
}

func showCache(ctx *app.Context, apps []string) error {
	result, err := ctx.Services.Cache.ListCache(apps)
	if err != nil {
		return err
	}

	displayCacheList(os.Stdout, os.Stderr, result, ctx.GetVerbose())
	return nil
}

func removeCache(ctx *app.Context, stdout, stderr io.Writer, apps []string, removeAll bool, scope scoop.InstallScope, dryRun bool) error {
	if removeAll {
		apps = []string{"*"}
	} else {
		for _, a := range apps {
			if a == "*" || a == "-a" || a == "--all" {
				apps = []string{"*"}
				break
			}
		}
	}

	result, err := ctx.Services.Cache.RemoveCache(apps, scope, dryRun)
	if err != nil {
		return err
	}

	displayCacheRemove(stdout, stderr, result, scope, dryRun)
	return nil
}

func displayCacheList(stdout, stderr io.Writer, result service.CacheResult, verbose bool) {
	if len(result.Entries) == 0 {
		_, _ = fmt.Fprintln(stdout, ui.Skip("cache", "empty"))
		return
	}

	_, _ = fmt.Fprintln(stdout, ui.Heading("Cache"))

	// Calculate column widths based on content
	maxApp := ui.VisualLength("Name")
	maxVer := ui.VisualLength("Version")
	maxSize := ui.VisualLength("Size")
	for _, e := range result.Entries {
		if ui.VisualLength(e.App) > maxApp {
			maxApp = ui.VisualLength(e.App)
		}
		if ui.VisualLength(e.Version) > maxVer {
			maxVer = ui.VisualLength(e.Version)
		}
		size := ui.FormatSize(e.Size)
		if verbose && e.Scope == "global" {
			size += " [global]"
		}
		if ui.VisualLength(size) > maxSize {
			maxSize = ui.VisualLength(size)
		}
	}

	// Print header
	_, _ = fmt.Fprintf(stdout, "%s  %s  %s\n",
		ui.PadRight(ui.BoldGreen("Name"), maxApp),
		ui.PadRight(ui.BoldGreen("Version"), maxVer),
		ui.BoldGreen("Size"))

	// Print rows
	for _, e := range result.Entries {
		size := ui.FormatSize(e.Size)
		if verbose && e.Scope == "global" {
			size += " " + ui.Dim("[global]")
		}
		_, _ = fmt.Fprintf(stdout, "%s  %s  %s\n",
			ui.PadRight(ui.Cyan(e.App), maxApp),
			ui.PadRight(e.Version, maxVer),
			size)
	}

	// Footer
	_, _ = fmt.Fprintf(stdout, "\n%s\n", ui.Dim(fmt.Sprintf("%d file(s), %s",
		len(result.Entries), ui.FormatSize(result.TotalSize))))

	// Print errors if any
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			_, _ = fmt.Fprintln(stderr, ui.WarnLine(err))
		}
	}
}

func displayCacheRemove(stdout, stderr io.Writer, result service.CacheResult, scope scoop.InstallScope, dryRun bool) {
	_, _ = fmt.Fprintln(stdout, ui.Heading(cacheRemoveHeading(scope)))

	kind := ui.StatusDone
	if dryRun {
		kind = ui.StatusDry
	}

	if result.FilesRemoved == 0 {
		_, _ = fmt.Fprintln(stdout, ui.RenderStatusSummary(ui.StatusSkip, "cache", "nothing to remove"))
		for _, err := range result.Errors {
			_, _ = fmt.Fprintln(stderr, ui.WarnLine(err))
		}
		return
	}

	// Show what was removed or would be removed.
	if dryRun || result.FilesRemoved <= 5 {
		for _, e := range result.Entries {
			detail := fmt.Sprintf("%s %s", ui.Dim(e.Name), ui.Dim(ui.FormatSize(e.Size)))
			_, _ = fmt.Fprintln(stdout, ui.StatusWithOptions(kind, "remove", detail, ui.StatusOptions{}))
		}
	}

	_, _ = fmt.Fprintln(stdout, ui.RenderStatusSummary(kind, "cache", cacheRemoveSummaryDetail(result, dryRun)))

	// Show errors
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			_, _ = fmt.Fprintln(stderr, ui.WarnLine(err))
		}
	}
}

func cacheRemoveHeading(scope scoop.InstallScope) string {
	if scope == scoop.ScopeGlobal {
		return "Removing cache [global]"
	}
	return "Removing cache"
}

func cacheRemoveSummaryDetail(result service.CacheResult, dryRun bool) string {
	if dryRun {
		return fmt.Sprintf("%d file(s), %s would be freed", result.FilesRemoved, ui.FormatSize(result.BytesFreed))
	}
	return fmt.Sprintf("%d file(s), %s freed", result.FilesRemoved, ui.FormatSize(result.BytesFreed))
}

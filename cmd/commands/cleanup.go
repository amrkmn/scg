package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// NewCleanupCommand creates the cleanup subcommand.
func NewCleanupCommand() *cobra.Command {
	var flagAll, flagCache, flagGlobal, flagVerbose, flagDryRun bool

	cmd := &cobra.Command{
		Use:   "cleanup [app]",
		Short: "Remove old versions of installed apps",
		Long: `Remove old app versions and optionally cache files.

  scg cleanup <app>    Clean a specific app
  scg cleanup --all    Clean all installed apps
  scg cleanup --cache  Also remove cached installers`,
		Args:    cobra.MaximumNArgs(1),
		Example: "scg cleanup git\nscg cleanup --all --cache\nscg cleanup --all --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			opts := service.CleanupOptions{
				Cache:   flagCache,
				DryRun:  flagDryRun,
				Verbose: flagVerbose,
			}

			// Determine which apps to clean.
			var targets []service.InstalledApp

			if flagAll || (len(args) > 0 && args[0] == "*") {
				all, err := ctx.Services.Apps.ListInstalled("")
				if err != nil {
					return err
				}
				targets = all
			} else if len(args) > 0 {
				all, err := ctx.Services.Apps.ListInstalled(args[0])
				if err != nil {
					return err
				}
				for _, a := range all {
					if a.Name == args[0] {
						targets = append(targets, a)
					}
				}
				if len(targets) == 0 {
					ctx.GetLogger().Error(fmt.Sprintf("%s not found", args[0]))
					return os.ErrNotExist
				}
			} else {
				ctx.GetLogger().Error("specify an app name or use --all")
				return nil
			}

			// Filter by scope: without --global, only clean user-scope apps.
			// With --global, clean both user and global apps (matching Scoop behavior).
			if !flagGlobal {
				var userOnly []service.InstalledApp
				for _, t := range targets {
					if t.Scope == scoop.ScopeUser {
						userOnly = append(userOnly, t)
					}
				}
				targets = userOnly
			}

			if flagDryRun {
				ctx.GetLogger().Dry("cleanup", "no files will be removed")
			}

			if flagAll || (len(args) > 0 && args[0] == "*") {
				ctx.GetLogger().Header("Cleaning installed apps")
			} else if len(args) > 0 {
				ctx.GetLogger().Header(fmt.Sprintf("Cleaning %s", args[0]))
			}

			var results []service.CleanupResult
			for _, t := range targets {
				result := ctx.Services.Cleanup.CleanupApp(t.Name, t.Scope, opts)
				results = append(results, result)
			}

			// Find max app name length only from apps that have something to clean.
			maxLen := 0
			for _, r := range results {
				if len(r.OldVersions) > 0 || len(r.FailedVersions) > 0 || len(r.CacheFiles) > 0 {
					if len(r.App) > maxLen {
						maxLen = len(r.App)
					}
				}
			}

			for _, r := range results {
				displayCleanupResult(r, maxLen)
			}

			// Clean up any remaining .download temp files.
			if flagCache && !flagDryRun {
				_ = ctx.Services.Cleanup.CleanupAll(scoop.ScopeUser)
				if flagGlobal {
					_ = ctx.Services.Cleanup.CleanupAll(scoop.ScopeGlobal)
				}
			}

			displayCleanupSummary(results)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagAll, "all", "a", false, "Clean all installed apps")
	cmd.Flags().BoolVarP(&flagCache, "cache", "k", false, "Also remove cached installers")
	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Clean global scope apps")
	cmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show detailed output")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Show what would be removed without removing")
	return cmd
}

func displayCleanupResult(r service.CleanupResult, maxNameLen int) {
	if len(r.OldVersions) == 0 && len(r.FailedVersions) == 0 && len(r.CacheFiles) == 0 {
		return
	}

	var totalSize int64
	versionNames := make([]string, 0, len(r.OldVersions))
	for _, v := range r.OldVersions {
		versionNames = append(versionNames, v.Version)
		totalSize += v.Size
	}
	for _, c := range r.CacheFiles {
		totalSize += c.Size
	}

	name := r.App
	for len(name) < maxNameLen {
		name += " "
	}

	detail := ""
	if len(versionNames) > 0 {
		detail = fmt.Sprintf("%s (%s)", joinStrings(versionNames), ui.Dim(ui.FormatSize(totalSize)))
	}
	cacheDetail := ""
	if len(r.CacheFiles) > 0 {
		cacheDetail = fmt.Sprintf(" +%d cache file(s)", len(r.CacheFiles))
	}

	scopeTag := ui.Dim("[" + string(r.Scope) + "]")
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", ui.Detail(fmt.Sprintf("%s %s%s %s", ui.Cyan(name), detail, cacheDetail, scopeTag)))

	for _, f := range r.FailedVersions {
		_, _ = fmt.Fprintln(os.Stdout, ui.WarnLine(fmt.Sprintf("could not remove %s: %v", f.Version, f.Error)))
	}
}

func displayCleanupSummary(results []service.CleanupResult) {
	var totalVersions, totalCache int
	var totalSize int64
	var hasLocked bool

	for _, r := range results {
		totalVersions += len(r.OldVersions)
		totalCache += len(r.CacheFiles)
		for _, v := range r.OldVersions {
			totalSize += v.Size
		}
		for _, c := range r.CacheFiles {
			totalSize += c.Size
		}
		if len(r.FailedVersions) > 0 {
			hasLocked = true
		}
	}

	if totalVersions == 0 && totalCache == 0 {
		_, _ = fmt.Fprintln(os.Stdout, ui.Skip("cleanup", "nothing to remove"))
		return
	}

	parts := []string{}
	if totalVersions > 0 {
		parts = append(parts, fmt.Sprintf("%d old version(s) removed", totalVersions))
	}
	if totalCache > 0 {
		parts = append(parts, fmt.Sprintf("%d cache file(s) removed", totalCache))
	}
	parts = append(parts, fmt.Sprintf("%s freed", ui.FormatSize(totalSize)))

	_, _ = fmt.Fprintln(os.Stdout, ui.Heading("Summary"))
	_, _ = fmt.Fprintln(os.Stdout, ui.Done("cleanup", joinStrings(parts)))

	if hasLocked {
		_, _ = fmt.Fprintf(os.Stdout, "\n%s\n", ui.WarnLine("some versions could not be removed; files may be in use"))
		_, _ = fmt.Fprintln(os.Stdout, ui.Detail(ui.Dim("close running applications and try again")))
	}
}

func joinStrings(parts []string) string {
	var result strings.Builder
	for i, p := range parts {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(p)
	}
	return result.String()
}

package commands

import (
	"fmt"
	"os"

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
			ctx := cmdctx.FromCmd(cmd)
			if ctx == nil {
				return fmt.Errorf("context unavailable")
			}

			opts := service.CleanupOptions{
				Cache:   flagCache,
				DryRun:  flagDryRun,
				Verbose: flagVerbose,
			}

			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
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
					fmt.Fprintf(os.Stderr, "App '%s' not found\n", args[0])
					return os.ErrNotExist
				}
			} else {
				fmt.Fprintln(os.Stderr, "Specify an app name or use --all")
				return nil
			}

			// Filter by scope.
			var scoped []service.InstalledApp
			for _, t := range targets {
				if flagGlobal && t.Scope != scoop.ScopeGlobal {
					continue
				}
				scoped = append(scoped, t)
			}
			if len(scoped) > 0 {
				targets = scoped
			}

			if flagDryRun {
				fmt.Fprintln(os.Stdout, ui.Dim("(dry run — no files will be removed)"))
			}

			// Find max app name length for padding.
			maxLen := 0
			for _, t := range targets {
				if len(t.Name) > maxLen {
					maxLen = len(t.Name)
				}
			}

			var results []service.CleanupResult
			for _, t := range targets {
				result := ctx.Services.Cleanup.CleanupApp(t.Name, scope, opts)
				results = append(results, result)
				displayCleanupResult(result, maxLen)
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
		detail = fmt.Sprintf("%s (%s)", joinStrings(versionNames), formatSize(totalSize))
	}
	cacheDetail := ""
	if len(r.CacheFiles) > 0 {
		cacheDetail = fmt.Sprintf(" +%d cache file(s)", len(r.CacheFiles))
	}

	scopeTag := ui.Dim("[" + string(r.Scope) + "]")
	fmt.Fprintf(os.Stdout, "%s : %s%s %s\n", ui.Cyan(name), detail, cacheDetail, scopeTag)

	for _, f := range r.FailedVersions {
		fmt.Fprintf(os.Stdout, "  %s Could not remove %s: %v\n", ui.Warning("!"), f.Version, f.Error)
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
		fmt.Fprintln(os.Stdout, ui.Dim("Nothing to clean up."))
		return
	}

	parts := []string{}
	if totalVersions > 0 {
		parts = append(parts, fmt.Sprintf("%d old version(s) removed", totalVersions))
	}
	if totalCache > 0 {
		parts = append(parts, fmt.Sprintf("%d cache file(s) removed", totalCache))
	}
	parts = append(parts, fmt.Sprintf("%s freed", formatSize(totalSize)))

	fmt.Fprintf(os.Stdout, "%s %s\n", ui.Success("✓"), joinStrings(parts))

	if hasLocked {
		fmt.Fprintf(os.Stdout, "\n%s Some versions could not be removed (files may be in use).\n", ui.Warning("!"))
		fmt.Fprintln(os.Stdout, ui.Dim("  Tip: close any running applications and try again."))
	}
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func joinStrings(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/amrkmn/scg/internal/cmdctx"
	"github.com/amrkmn/scg/internal/scoop"
	"github.com/amrkmn/scg/internal/service"
	"github.com/amrkmn/scg/internal/ui"
	"github.com/spf13/cobra"
)

// NewInfoCommand creates the info subcommand.
func NewInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "info <app>",
		Short:   "Show information about an app",
		Args:    cobra.ExactArgs(1),
		Example: "scg info git\nscg info extras/git",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)
			if ctx == nil {
				return fmt.Errorf("context unavailable")
			}

			all := ctx.Services.Manifests.FindAllManifests(args[0])
			if len(all) == 0 {
				fmt.Fprintf(os.Stderr, "App '%s' not found in any installed bucket\n", args[0])
				return os.ErrNotExist
			}

			installed, bucket := ctx.Services.Manifests.FindManifestPair(args[0])

			// If a specific bucket was requested (e.g. "extras/opencode") and the
			// app is installed from a *different* bucket, treat it as not installed
			// for this query — the user is asking about the extras version.
			if installed != nil && bucket != nil &&
				!strings.EqualFold(installed.Bucket, bucket.Bucket) {
				installed = nil
			}

			// If multiple bucket results and not installed, list options.
			bucketResults := 0
			for _, fm := range all {
				if fm.Source == "bucket" {
					bucketResults++
				}
			}
			if installed == nil && bucketResults > 1 {
				fmt.Fprintf(os.Stdout, "%s Found in multiple buckets:\n", ui.Info("i"))
				for _, fm := range all {
					if fm.Source == "bucket" {
						fmt.Fprintf(os.Stdout, "  %s/%s\n", ui.Cyan(fm.Bucket), fm.App)
					}
				}
				fmt.Fprintln(os.Stdout)
			}

			fields := ctx.Services.Manifests.ReadManifestPair(args[0], installed, bucket)

			// Use the bucket manifest for display when a specific bucket was
			// explicitly requested, otherwise fall back to installed manifest.
			var m *scoop.Manifest
			if bucket != nil {
				m = bucket.Manifest
			} else if installed != nil {
				m = installed.Manifest
			}

			if m == nil {
				fmt.Fprintf(os.Stderr, "Could not read manifest for '%s'\n", args[0])
				return os.ErrNotExist
			}

			printAppInfo(fields, m, installed, bucket)
			return nil
		},
	}
}

const infoLabelWidth = 20

func infoLine(label, value string) {
	if value == "" {
		return
	}
	pad := infoLabelWidth - len(label)
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(os.Stdout, "%s%s : %s\n", ui.Bold(label), strings.Repeat(" ", pad), value)
}

func printAppInfo(fields service.InfoFields, m *scoop.Manifest, installed, bucket *service.FoundManifest) {
	infoLine("Name", fields.Name)
	infoLine("Description", fields.Description)

	// Version display.
	if fields.InstalledVersion != "" && fields.LatestVersion != "" {
		if fields.UpdateAvailable {
			infoLine("Version", fmt.Sprintf("%s -> %s %s",
				fields.InstalledVersion,
				fields.LatestVersion,
				ui.Yellow("(update available)"),
			))
		} else {
			infoLine("Version", fmt.Sprintf("%s %s",
				fields.InstalledVersion,
				ui.Dim("(up to date)"),
			))
		}
	} else if fields.InstalledVersion != "" {
		infoLine("Version", fields.InstalledVersion)
	} else if fields.LatestVersion != "" {
		infoLine("Version", fields.LatestVersion)
	} else {
		infoLine("Version", fields.Version)
	}

	infoLine("Homepage", fields.Homepage)
	infoLine("License", fields.License)

	// Installed status.
	if installed != nil {
		infoLine("Installed", fmt.Sprintf("Yes %s", ui.Dim("("+string(installed.Scope)+")")))
		if installed.Bucket != "" {
			infoLine("Bucket", installed.Bucket)
		}
	} else {
		infoLine("Installed", "No")
		if bucket != nil {
			infoLine("Bucket", bucket.Bucket+" "+ui.Dim("("+string(bucket.Scope)+")"))
		}
	}

	// Architecture.
	if len(m.Architecture) > 0 {
		keys := make([]string, 0, len(m.Architecture))
		for k := range m.Architecture {
			keys = append(keys, k)
		}
		infoLine("Architecture", strings.Join(keys, ", "))
	}

	// Dependencies.
	deps := toInfoStringSlice(m.Depends)
	if len(deps) > 0 {
		infoLine("Dependencies", strings.Join(deps, ", "))
	}

	// Suggestions.
	if len(m.Suggest) > 0 {
		suggests := make([]string, 0, len(m.Suggest))
		for k := range m.Suggest {
			suggests = append(suggests, k)
		}
		infoLine("Suggestions", strings.Join(suggests, ", "))
	}

	// Binaries.
	bins := service.ExtractBinaries(m.Bin)
	if len(bins) > 0 {
		infoLine("Binaries", strings.Join(bins, ", "))
	}

	// Adds to PATH.
	pathAdditions := toInfoStringSlice(m.EnvAddPath)
	if len(pathAdditions) > 0 {
		infoLine("Adds to PATH", strings.Join(pathAdditions, ", "))
	}

	// Environment variables.
	if len(m.EnvSet) > 0 {
		parts := make([]string, 0, len(m.EnvSet))
		for k, v := range m.EnvSet {
			parts = append(parts, k+"="+v)
		}
		infoLine("Environment", strings.Join(parts, ", "))
	}

	// Shortcuts (display the alias/name column, index 1).
	if len(m.Shortcuts) > 0 {
		names := make([]string, 0, len(m.Shortcuts))
		for _, s := range m.Shortcuts {
			if arr, ok := s.([]any); ok && len(arr) >= 2 {
				if name, ok := arr[1].(string); ok {
					names = append(names, name)
				}
			}
		}
		if len(names) > 0 {
			infoLine("Creates shortcuts", strings.Join(names, ", "))
		}
	}

	// Persist.
	persisted := toInfoStringSlice(m.Persist)
	if len(persisted) > 0 {
		infoLine("Persisted data", strings.Join(persisted, ", "))
	}

	// Notes.
	notes := toInfoStringSlice(m.Notes)
	if len(notes) > 0 {
		infoLine("Notes", strings.Join(notes, " | "))
	}

	// Deprecated.
	if fields.Deprecated {
		msg := "Yes"
		if fields.ReplacedBy != "" {
			msg = fmt.Sprintf("Yes (replaced by %s)", ui.Cyan(fields.ReplacedBy))
		}
		infoLine("DEPRECATED", ui.Yellow(msg))
	}

	// Comments (##).
	if m.Comments != nil {
		comments := toInfoStringSlice(m.Comments)
		if len(comments) > 0 {
			infoLine("Comments", strings.Join(comments, " | "))
		}
	}

	// Install date.
	if !fields.InstallDate.IsZero() {
		infoLine("Install date", fields.InstallDate.Format("2006-01-02 15:04:05"))
	}
}

// toInfoStringSlice converts any (string or []any) to []string for display.
func toInfoStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		return []string{val}
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			switch s := item.(type) {
			case string:
				out = append(out, s)
			default:
				out = append(out, fmt.Sprintf("%v", s))
			}
		}
		return out
	case []string:
		return val
	}
	return []string{fmt.Sprintf("%v", v)}
}

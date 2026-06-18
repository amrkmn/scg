package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// NewInfoCommand creates the info subcommand.
func NewInfoCommand() *cobra.Command {
	var flagVerbose bool

	cmd := &cobra.Command{
		Use:     "info <app>",
		Short:   "Show information about an app",
		Args:    cobra.ExactArgs(1),
		Example: "scg info git\nscg info extras/git\nscg info git --verbose",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			all := ctx.Services.Manifests.FindAllManifests(args[0])
			if len(all) == 0 {
				ctx.GetLogger().Error(fmt.Sprintf("app %q not found in any installed bucket", args[0]))
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
				ctx.GetLogger().Header("Found in multiple buckets")
				for _, fm := range all {
					if fm.Source == "bucket" {
						ctx.GetLogger().Detail(fmt.Sprintf("%s/%s", fm.Bucket, fm.App))
					}
				}
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
				ctx.GetLogger().Error(fmt.Sprintf("could not read manifest for %q", args[0]))
				return os.ErrNotExist
			}

			printAppInfo(cmd.OutOrStdout(), fields, m, installed, bucket, flagVerbose)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show full paths and URLs")
	return cmd
}

func printAppInfo(w interface{ Write([]byte) (int, error) }, fields service.InfoFields, m *scoop.Manifest, installed, bucket *service.FoundManifest, verbose bool) {
	_, _ = fmt.Fprintln(w, ui.RenderKeyValueBlock("", buildInfoPairs(fields, m, installed, bucket, verbose)))
}

func buildInfoPairs(fields service.InfoFields, m *scoop.Manifest, installed, bucket *service.FoundManifest, verbose bool) []ui.KeyValue {
	pairs := make([]ui.KeyValue, 0, 16)
	appendPair := func(key, value string) {
		if value == "" {
			return
		}
		pairs = append(pairs, ui.KeyValue{Key: key, Value: value})
	}

	appendPair("Name", fields.Name)
	appendPair("Description", fields.Description)

	if fields.InstalledVersion != "" && fields.LatestVersion != "" {
		if fields.UpdateAvailable {
			appendPair("Version", fmt.Sprintf("%s -> %s %s",
				fields.InstalledVersion,
				fields.LatestVersion,
				ui.Yellow("(update available)"),
			))
		} else {
			appendPair("Version", fmt.Sprintf("%s %s",
				fields.InstalledVersion,
				ui.Dim("(up to date)"),
			))
		}
	} else if fields.InstalledVersion != "" {
		appendPair("Version", fields.InstalledVersion)
	} else if fields.LatestVersion != "" {
		appendPair("Version", fields.LatestVersion)
	} else {
		appendPair("Version", fields.Version)
	}

	appendPair("Homepage", fields.Homepage)
	appendPair("License", fields.License)

	if installed != nil {
		appendPair("Installed", fmt.Sprintf("Yes %s", ui.Dim("("+string(installed.Scope)+")")))
		if installed.Bucket != "" {
			appendPair("Bucket", installed.Bucket)
		}
	} else {
		appendPair("Installed", "No")
		if bucket != nil {
			appendPair("Bucket", bucket.Bucket+" "+ui.Dim("("+string(bucket.Scope)+")"))
		}
	}

	if len(m.Architecture) > 0 {
		keys := make([]string, 0, len(m.Architecture))
		for k := range m.Architecture {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		appendPair("Architecture", strings.Join(keys, ", "))
	}

	deps := toInfoStringSlice(m.Depends)
	if len(deps) > 0 {
		appendPair("Dependencies", strings.Join(deps, ", "))
	}

	if len(m.Suggest) > 0 {
		suggests := make([]string, 0, len(m.Suggest))
		for k := range m.Suggest {
			suggests = append(suggests, k)
		}
		sort.Strings(suggests)
		appendPair("Suggestions", strings.Join(suggests, ", "))
	}

	bins := service.ExtractBinaries(m.Bin)
	if len(bins) > 0 {
		appendPair("Binaries", strings.Join(bins, ", "))
	}

	pathAdditions := toInfoStringSlice(m.EnvAddPath)
	if len(pathAdditions) > 0 {
		appendPair("Adds to PATH", strings.Join(pathAdditions, ", "))
	}

	if len(m.EnvSet) > 0 {
		keys := make([]string, 0, len(m.EnvSet))
		for k := range m.EnvSet {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(m.EnvSet))
		for _, k := range keys {
			v := m.EnvSet[k]
			parts = append(parts, k+"="+v)
		}
		appendPair("Environment", strings.Join(parts, ", "))
	}

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
			appendPair("Creates shortcuts", strings.Join(names, ", "))
		}
	}

	persisted := toInfoStringSlice(m.Persist)
	if len(persisted) > 0 {
		appendPair("Persisted data", strings.Join(persisted, ", "))
	}

	notes := toInfoStringSlice(m.Notes)
	if len(notes) > 0 {
		appendPair("Notes", strings.Join(notes, " | "))
	}

	if verbose {
		if installed != nil {
			paths := scoop.ResolvePaths(installed.Scope)
			installDir, _ := scoop.ResolveCurrentDir(installed.App, installed.Scope)
			if installDir != "" {
				appendPair("Install dir", installDir)
			}
			persistDir := filepath.Join(paths.Root, "persist", installed.App)
			if _, err := os.Stat(persistDir); err == nil {
				appendPair("Persist dir", persistDir)
			}
		} else if bucket != nil {
			paths := scoop.ResolvePaths(bucket.Scope)
			cacheDir := paths.Cache
			if cacheDir != "" {
				appendPair("Cache dir", cacheDir)
			}
		}
		if installed != nil && installed.FilePath != "" {
			appendPair("Manifest", installed.FilePath)
		} else if bucket != nil && bucket.FilePath != "" {
			appendPair("Manifest", bucket.FilePath)
		}
	}

	if fields.Deprecated {
		msg := "Yes"
		if fields.ReplacedBy != "" {
			msg = fmt.Sprintf("Yes (replaced by %s)", ui.Cyan(fields.ReplacedBy))
		}
		appendPair("DEPRECATED", ui.Yellow(msg))
	}

	if m.Comments != nil {
		comments := toInfoStringSlice(m.Comments)
		if len(comments) > 0 {
			appendPair("Comments", strings.Join(comments, " | "))
		}
	}

	if !fields.InstallDate.IsZero() {
		appendPair("Install date", fields.InstallDate.Format("2006-01-02 15:04:05"))
	}

	return pairs
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

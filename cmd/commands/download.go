package commands

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

// NewDownloadCommand creates the download subcommand.
func NewDownloadCommand() *cobra.Command {
	var flagForce, flagSkipHash, flagGlobal bool
	var flagArch, flagProxy string

	cmd := &cobra.Command{
		Use:   "download <app> [app...]",
		Short: "Download apps to the cache",
		Long:  "Download one or more apps from Scoop buckets into the cache and verify hashes without installing them.",
		Args:  cobra.MinimumNArgs(1),
		Example: `scg download git
scg download main/bun --force
scg download nodejs --arch 64bit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			if err := install.EnsureScoopInstalled(); err != nil {
				ctx.GetLogger().Warn(err.Error())
				return err
			}

			arch := flagArch
			if arch == "" {
				arch = defaultDownloadArch()
			}

			scope := scoop.ScopeUser
			if flagGlobal {
				scope = scoop.ScopeGlobal
			}
			dm := install.NewDownloadManager(scope, ctx.GetVerbose())

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.Heading(formatDownloadHeading(args, scope)))

			var succeeded, failed int
			for _, input := range args {
				if err := downloadOne(ctx, dm, input, arch, !flagForce, flagSkipHash, flagProxy); err != nil {
					failed++
					ctx.GetLogger().Error(fmt.Sprintf("%s: %v", input, err))
				} else {
					succeeded++
				}
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), ui.RenderStatusSummary(ui.StatusDone, "download", fmt.Sprintf("%d downloaded, %d failed", succeeded, failed)))

			if failed > 0 {
				return fmt.Errorf("%d app(s) failed to download", failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Force download and overwrite cached files")
	cmd.Flags().BoolVarP(&flagSkipHash, "skip-hash-check", "s", false, "Skip hash validation")
	cmd.Flags().BoolVar(&flagGlobal, "global", false, "Use the global Scoop cache")
	cmd.Flags().StringVarP(&flagArch, "arch", "a", "", "Use the specified architecture (64bit, 32bit, arm64)")
	cmd.Flags().StringVar(&flagProxy, "proxy", "", "Download via proxy URL")

	return cmd
}

func downloadOne(ctx *app.Context, dm *install.DownloadManager, input, arch string, useCache, skipHash bool, proxy string) error {
	_, bucket := ctx.Services.Manifests.FindManifestPair(input)
	if bucket == nil {
		return fmt.Errorf("app not found in any bucket")
	}

	appName := appNameFromInput(input)
	manifest := bucket.Manifest
	if manifest.Version == "" {
		return fmt.Errorf("manifest doesn't specify a version")
	}

	urls, err := resolveDownloadURLs(manifest, arch)
	if err != nil {
		return err
	}
	hashes := resolveDownloadHashes(manifest, arch)

	ctx.GetLogger().Header(fmt.Sprintf("Downloading %s %s [%s] from %s", appName, manifest.Version, arch, bucket.Bucket))
	for i, dlURL := range urls {
		res, err := dm.Download(appName, manifest.Version, dlURL, useCache, proxy)
		if err != nil {
			return err
		}

		if res.Downloaded {
			ctx.GetLogger().Done("download", install.DownloadFileName(appName, dlURL))
		} else {
			ctx.GetLogger().Done("cache", install.DownloadFileName(appName, dlURL))
		}

		if skipHash {
			ctx.GetLogger().Skip("hash", "verification disabled")
			continue
		}

		hash := hashAt(hashes, i)
		if hash == "" {
			continue
		}
		format, err := install.ParseHash(hash)
		if err != nil {
			return fmt.Errorf("failed to parse hash for %s: %w", install.DownloadFileName(appName, dlURL), err)
		}
		if format == nil {
			continue
		}
		ctx.GetLogger().Header("Checking hash")
		if err := install.VerifyHash(res.CachePath, format); err != nil {
			_ = os.Remove(res.CachePath)
			return fmt.Errorf("hash verification failed for %s: %w", install.DownloadFileName(appName, dlURL), err)
		}
		ctx.GetLogger().Done("hash", "sha256")
	}

	return nil
}

func defaultDownloadArch() string {
	switch runtime.GOARCH {
	case "386":
		return "32bit"
	case "arm64":
		return "arm64"
	default:
		return "64bit"
	}
}

func appNameFromInput(input string) string {
	input = strings.TrimSuffix(input, ".json")
	input = strings.ReplaceAll(input, `\`, "/")
	if i := strings.LastIndex(input, "/"); i >= 0 {
		return input[i+1:]
	}
	return input
}

func resolveDownloadURLs(m *scoop.Manifest, arch string) ([]string, error) {
	if v, ok := archManifestField(m, arch, "url"); ok {
		if urls := stringList(v); len(urls) > 0 {
			return urls, nil
		}
	}
	if urls := stringList(m.URL); len(urls) > 0 {
		return urls, nil
	}
	return nil, fmt.Errorf("no download URL found in manifest for architecture %s", arch)
}

func resolveDownloadHashes(m *scoop.Manifest, arch string) []string {
	if v, ok := archManifestField(m, arch, "hash"); ok {
		if hashes := stringList(v); len(hashes) > 0 {
			return hashes
		}
	}
	return stringList(m.Hash)
}

func archManifestField(m *scoop.Manifest, arch, field string) (any, bool) {
	if m.Architecture == nil {
		return nil, false
	}
	section, ok := m.Architecture[arch].(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := section[field]
	return v, ok
}

func stringList(v any) []string {
	switch val := v.(type) {
	case string:
		if val != "" {
			return []string{val}
		}
	case []string:
		return val
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func hashAt(hashes []string, i int) string {
	if len(hashes) == 0 {
		return ""
	}
	if i < len(hashes) {
		return hashes[i]
	}
	if len(hashes) == 1 {
		return hashes[0]
	}
	return ""
}

func formatDownloadHeading(apps []string, scope scoop.InstallScope) string {
	scopeTag := ""
	if scope == scoop.ScopeGlobal {
		scopeTag = " [global]"
	}
	if len(apps) == 1 {
		return fmt.Sprintf("Downloading %s%s", apps[0], scopeTag)
	}
	return fmt.Sprintf("Downloading %d apps%s", len(apps), scopeTag)
}

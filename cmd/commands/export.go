package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
)

func NewExportCommand() *cobra.Command {
	var flagConfig bool

	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export installed apps, buckets, and optionally config to JSON",
		Long:    "Exports the list of installed apps and buckets (and optionally the config) in JSON format.\nOutput can be piped to a file and later imported with 'scg import'.",
		Example: "  scg export > scoopfile.json\n  scg export --config > scoopfile.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			export := make(map[string]any)

			if flagConfig {
				config, err := ctx.Services.Config.Load()
				if err == nil {
					cleaned := make(map[string]any)
					for k, v := range config {
						switch strings.ToLower(k) {
						case "last_update", "root_path", "global_path", "cache_path", "alias":
						default:
							cleaned[k] = v
						}
					}
					export["config"] = cleaned
				}
			}

			buckets, err := ctx.Services.Buckets.List("")
			if err != nil {
				ctx.GetLogger().Warn(fmt.Sprintf("failed to list buckets: %v", err))
			} else {
				type bucketExport struct {
					Name   string `json:"name"`
					Source string `json:"source"`
				}
				var bucketList []bucketExport
				for _, b := range buckets {
					bucketList = append(bucketList, bucketExport{
						Name:   b.Name,
						Source: b.Source,
					})
				}
				export["buckets"] = bucketList
			}

			installed, err := ctx.Services.Apps.ListInstalled("")
			if err != nil {
				ctx.GetLogger().Warn(fmt.Sprintf("failed to list installed apps: %v", err))
			} else {
				type appExport struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Source  string `json:"source"`
					Global  bool   `json:"global,omitempty"`
					Held    bool   `json:"held,omitempty"`
				}
				var appList []appExport
				for _, a := range installed {
					appList = append(appList, appExport{
						Name:    a.Name,
						Version: a.Version,
						Source:  a.Bucket,
						Global:  a.Scope == scoop.ScopeGlobal,
						Held:    a.Held,
					})
				}
				export["apps"] = appList
			}

			data, err := json.MarshalIndent(export, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal export data: %w", err)
			}

			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagConfig, "config", "c", false, "Include Scoop configuration in export")
	return cmd
}

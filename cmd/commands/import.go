package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
)

func NewImportCommand() *cobra.Command {
	var flagGlobal bool

	cmd := &cobra.Command{
		Use:     "import <file>",
		Short:   "Install apps from an exported JSON file",
		Long:    "Installs apps and buckets listed in a JSON file (as produced by 'scg export').\nIf the file is '-', reads from stdin.",
		Example: "  scg import scoopfile.json\n  scg export | scg import -",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			filePath := args[0]

			var data []byte
			var err error
			if filePath == "-" {
				data, err = readFromStdin()
			} else {
				data, err = os.ReadFile(filePath)
			}
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", filePath, err)
			}

			var exportData map[string]any
			if err := json.Unmarshal(data, &exportData); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}

			if err := importBuckets(ctx, exportData); err != nil {
				ctx.GetLogger().Warn(err.Error())
			}

			if err := importConfig(ctx, exportData); err != nil {
				ctx.GetLogger().Warn(err.Error())
			}

			if err := importApps(ctx, exportData, flagGlobal); err != nil {
				ctx.GetLogger().Warn(err.Error())
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagGlobal, "global", "g", false, "Install apps globally")
	return cmd
}

func importBuckets(ctx *app.Context, data map[string]any) error {
	buckets, ok := data["buckets"]
	if !ok {
		return nil
	}

	bucketList, ok := buckets.([]any)
	if !ok {
		return fmt.Errorf("invalid buckets format in import file")
	}

	ctx.GetLogger().Header(fmt.Sprintf("Importing %d bucket(s)", len(bucketList)))

	for _, b := range bucketList {
		bm, ok := b.(map[string]any)
		if !ok {
			continue
		}
		name, _ := bm["name"].(string)
		source, _ := bm["source"].(string)

		if name == "" {
			continue
		}

		existing, _ := ctx.Services.Buckets.List("")
		found := false
		for _, eb := range existing {
			if strings.EqualFold(eb.Name, name) {
				found = true
				break
			}
		}

		if found {
			ctx.GetLogger().Skip(name, "bucket already added")
			continue
		}

		if source == "" {
			ctx.GetLogger().Warn(fmt.Sprintf("bucket %s has no source URL; skipping", name))
			continue
		}

		ctx.GetLogger().Header(fmt.Sprintf("Adding bucket %s", name))
		ctx.GetLogger().Detail(source)
		if err := ctx.Services.Buckets.Add(name, source, scoop.ScopeUser, nil); err != nil {
			ctx.GetLogger().Error(fmt.Sprintf("failed to add bucket %s: %v", name, err))
		} else {
			ctx.GetLogger().Done(name, "bucket added")
		}
	}

	return nil
}

func importApps(ctx *app.Context, data map[string]any, forceGlobal bool) error {
	apps, ok := data["apps"]
	if !ok {
		return nil
	}

	appList, ok := apps.([]any)
	if !ok {
		return fmt.Errorf("invalid apps format in import file")
	}

	ctx.GetLogger().Header(fmt.Sprintf("Importing %d app(s)", len(appList)))

	for _, a := range appList {
		am, ok := a.(map[string]any)
		if !ok {
			continue
		}
		name, _ := am["name"].(string)
		if name == "" {
			continue
		}

		source, _ := am["source"].(string)

		global := forceGlobal
		if g, ok := am["global"].(bool); ok && g {
			global = true
		}

		held := false
		if h, ok := am["held"].(bool); ok {
			held = h
		}

		scope := scoop.ScopeUser
		if global {
			scope = scoop.ScopeGlobal
		}

		plainName := name
		installed, _ := ctx.Services.Apps.GetInstalledApp(plainName, scope)
		if installed != nil {
			if held && !installed.Held {
				if err := ctx.Services.Apps.SetHold(plainName, scope, true); err != nil {
					ctx.GetLogger().Warn(fmt.Sprintf("failed to hold %s: %v", plainName, err))
				} else {
					ctx.GetLogger().Done(plainName, "held")
				}
			}
			ctx.GetLogger().Skip(plainName, "already installed")
			continue
		}

		installName := name
		if source != "" {
			installName = source + "/" + name
		}

		ctx.GetLogger().Header(fmt.Sprintf("Installing %s", installName))

		if global {
			ctx.GetLogger().Detail("global scope")
		}

		result := ctx.Services.Installer.InstallSingle(installName, service.InstallOptions{
			Scope: scope,
		})
		if !result.Success && !result.Skipped {
			ctx.GetLogger().Error(fmt.Sprintf("failed to install %s: %v", installName, result.Error))
		} else if result.Skipped {
			ctx.GetLogger().Skip(installName, "already installed")
		} else {
			if held {
				if err := ctx.Services.Apps.SetHold(plainName, scope, true); err != nil {
					ctx.GetLogger().Warn(fmt.Sprintf("failed to hold %s: %v", plainName, err))
				} else {
					ctx.GetLogger().Done(plainName, "held")
				}
			}
			ctx.GetLogger().Done(name, "installed")
		}
	}

	return nil
}

func importConfig(ctx *app.Context, data map[string]any) error {
	configRaw, ok := data["config"]
	if !ok {
		return nil
	}

	configMap, ok := configRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid config format in import file")
	}

	if len(configMap) == 0 {
		return nil
	}

	ctx.GetLogger().Header(fmt.Sprintf("Importing %d config key(s)", len(configMap)))

	currentConfig, err := ctx.Services.Config.Load()
	if err != nil {
		currentConfig = make(map[string]any)
	}

	for k, v := range configMap {
		currentConfig[k] = v
		ctx.GetLogger().Done(k, "config set")
	}

	if err := ctx.Services.Config.Save(currentConfig); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func readFromStdin() ([]byte, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, fmt.Errorf("no input on stdin")
	}
	return io.ReadAll(os.Stdin)
}

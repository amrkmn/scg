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
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
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
				_, _ = fmt.Fprintf(os.Stderr, "%s %v\n", ui.Warning("!"), err)
			}

			if err := importConfig(ctx, exportData); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s %v\n", ui.Warning("!"), err)
			}

			if err := importApps(ctx, exportData, flagGlobal); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%s %v\n", ui.Warning("!"), err)
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

	fmt.Printf("Importing %d bucket(s)...\n", len(bucketList))

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
			fmt.Printf("  %s Bucket '%s' already added\n", ui.Success("✓"), name)
			continue
		}

		if source == "" {
			_, _ = fmt.Fprintf(os.Stderr, "  %s Bucket '%s' has no source URL, skipping\n", ui.Warning("!"), name)
			continue
		}

		fmt.Printf("  Adding bucket %s from %s...\n", ui.Cyan(name), source)
		if err := ctx.Services.Buckets.Add(name, source, scoop.ScopeUser, nil); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "  %s Failed to add bucket '%s': %v\n", ui.Error("-"), name, err)
		} else {
			fmt.Printf("  %s Bucket '%s' added\n", ui.Success("+"), name)
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

	fmt.Printf("Importing %d app(s)...\n", len(appList))

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
					_, _ = fmt.Fprintf(os.Stderr, "  %s Failed to hold '%s': %v\n", ui.Warning("!"), plainName, err)
				} else {
					fmt.Printf("  %s '%s' held\n", ui.Success("+"), plainName)
				}
			}
			fmt.Printf("  %s '%s' is already installed\n", ui.Info("i"), plainName)
			continue
		}

		installName := name
		if source != "" {
			installName = source + "/" + name
		}

		fmt.Printf("  Installing %s...\n", ui.Cyan(installName))

		if global {
			fmt.Printf("  (global scope)\n")
		}

		result := ctx.Services.Installer.InstallSingle(installName, service.InstallOptions{
			Scope: scope,
		})
		if !result.Success && !result.Skipped {
			_, _ = fmt.Fprintf(os.Stderr, "  %s Failed to install '%s': %v\n", ui.Error("-"), installName, result.Error)
		} else if result.Skipped {
			fmt.Printf("  %s '%s' is already installed\n", ui.Info("i"), installName)
		} else {
			if held {
				if err := ctx.Services.Apps.SetHold(plainName, scope, true); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "  %s Failed to hold '%s': %v\n", ui.Warning("!"), plainName, err)
				} else {
					fmt.Printf("  %s '%s' held\n", ui.Success("+"), plainName)
				}
			}
			fmt.Printf("  %s '%s' installed\n", ui.Success("+"), name)
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

	fmt.Printf("Importing %d config key(s)...\n", len(configMap))

	currentConfig, err := ctx.Services.Config.Load()
	if err != nil {
		currentConfig = make(map[string]any)
	}

	for k, v := range configMap {
		currentConfig[k] = v
		fmt.Printf("  %s Config '%s' set\n", ui.Success("+"), k)
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
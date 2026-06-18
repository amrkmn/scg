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

// NewAliasCommand creates the alias subcommand.
func NewAliasCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias <subcommand>",
		Short: "Manage custom Scoop aliases",
		Long:  "Manage custom Scoop aliases stored in Scoop config and user shims.",
	}

	cmd.AddCommand(newAliasAddCommand(), newAliasRemoveCommand(), newAliasListCommand())
	return cmd
}

func newAliasAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "add <name> <command> [description]",
		Short:   "Add an alias",
		Args:    cobra.RangeArgs(2, 3),
		Example: `scg alias add rm "scg uninstall $args[0]" "Uninstall an app"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			name, command := args[0], args[1]
			description := ""
			if len(args) > 2 {
				description = args[2]
			}

			aliases, config, err := loadAliases(ctx.Services.Config)
			if err != nil {
				return err
			}
			if _, ok := aliases[name]; ok {
				return fmt.Errorf("alias %q already exists", name)
			}

			paths := scoop.ResolvePaths(scoop.ScopeUser)
			if err := os.MkdirAll(paths.Shims, 0o755); err != nil {
				return fmt.Errorf("creating shims directory: %w", err)
			}
			scriptName := "scoop-" + name
			scriptPath := filepath.Join(paths.Shims, scriptName+".ps1")
			if _, err := os.Stat(scriptPath); err == nil {
				return fmt.Errorf("file %q already exists in shims directory", filepath.Base(scriptPath))
			}

			script := fmt.Sprintf("# Summary: %s\n%s\n", description, command)
			if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
				return fmt.Errorf("writing alias script: %w", err)
			}

			aliases[name] = scriptName
			config[aliasConfigKey] = aliases
			delete(config, legacyAliasConfigKey)
			if err := ctx.Services.Config.Save(config); err != nil {
				return err
			}
			ctx.GetLogger().Success(fmt.Sprintf("Alias '%s' added.", name))
			return nil
		},
	}
}

func newAliasRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <name>",
		Aliases: []string{"remove", "delete", "del"},
		Short:   "Remove an alias",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			name := args[0]
			aliases, config, err := loadAliases(ctx.Services.Config)
			if err != nil {
				return err
			}
			if _, ok := aliases[name]; !ok {
				return fmt.Errorf("alias %q doesn't exist", name)
			}

			paths := scoop.ResolvePaths(scoop.ScopeUser)
			_ = os.Remove(filepath.Join(paths.Shims, aliasScriptName(aliases, name)+".ps1"))
			delete(aliases, name)
			config[aliasConfigKey] = aliases
			delete(config, legacyAliasConfigKey)
			if err := ctx.Services.Config.Save(config); err != nil {
				return err
			}
			ctx.GetLogger().Success(fmt.Sprintf("Alias '%s' removed.", name))
			return nil
		},
	}
}

func newAliasListCommand() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List aliases",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			aliases, _, err := loadAliases(ctx.Services.Config)
			if err != nil {
				return err
			}
			if len(aliases) == 0 {
				ctx.GetLogger().Info("No alias found.")
				return nil
			}

			names := make([]string, 0, len(aliases))
			for name := range aliases {
				names = append(names, name)
			}
			sort.Strings(names)

			paths := scoop.ResolvePaths(scoop.ScopeUser)
			rows := make([][]string, 0, len(names))
			for _, name := range names {
				command, summary := readAliasScript(filepath.Join(paths.Shims, aliasScriptName(aliases, name)+".ps1"))
				if command == "" {
					command = "<BROKEN>"
				}
				if verbose {
					rows = append(rows, []string{ui.BoldCyan(name), command, summary})
				} else {
					rows = append(rows, []string{ui.BoldCyan(name), command})
				}
			}

			headers := []string{"Name", "Command"}
			weights := []float64{0.25, 0.75}
			if verbose {
				headers = []string{"Name", "Command", "Summary"}
				weights = []float64{0.2, 0.55, 0.25}
			}
			fmt.Fprintln(cmd.OutOrStdout(), ui.RenderTable(headers, rows, weights, fmt.Sprintf("%d alias(es)", len(rows))))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show alias descriptions and headers")
	return cmd
}

const (
	aliasConfigKey       = "alias"
	legacyAliasConfigKey = "ALIAS"
)

func loadAliases(cfg *service.ConfigService) (map[string]any, map[string]any, error) {
	config, err := cfg.Load()
	if err != nil {
		return nil, nil, err
	}
	aliases := map[string]any{}
	if raw, ok := config[aliasConfigKey]; ok {
		if m, ok := raw.(map[string]any); ok {
			aliases = m
		}
	} else if raw, ok := config[legacyAliasConfigKey]; ok {
		if m, ok := raw.(map[string]any); ok {
			aliases = m
		}
	}
	return aliases, config, nil
}

func aliasScriptName(aliases map[string]any, name string) string {
	if scriptName, ok := aliases[name].(string); ok && scriptName != "" {
		return scriptName
	}
	return "scoop-" + name
}

func readAliasScript(path string) (command string, summary string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > 0 {
		summary = strings.TrimSpace(strings.TrimPrefix(lines[0], "# Summary:"))
	}
	if len(lines) > 1 {
		command = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}
	return command, summary
}

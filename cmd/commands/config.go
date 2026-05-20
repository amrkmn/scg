package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// NewConfigCommand creates the config subcommand.
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config [name] [value]",
		Short: "Get, set, or delete configuration values",
		Long: `Manage scg configuration values.

  scg config                   List all config values
  scg config <name>            Get a specific value
  scg config <name> <value>    Set a value (coerced to bool/number/string)
  scg config rm <key>          Delete a key`,
		Args:    cobra.RangeArgs(0, 2),
		Example: "scg config\nscg config proxy http://proxy:8080\nscg config rm proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)
			svc := ctx.Services.Config

			switch len(args) {
			case 0:
				// List all.
				config, err := svc.Load()
				if err != nil {
					return err
				}
				if len(config) == 0 {
					ctx.GetLogger().Skip("config", "no values set")
					return nil
				}
				ctx.GetLogger().Header("Config")
				keys := make([]string, 0, len(config))
				for k := range config {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					_, _ = fmt.Fprintf(os.Stdout, "%s: %s\n", ui.Green(k), formatConfigListValue(config[k]))
				}
			case 1:
				// Get.
				val, ok := svc.Get(args[0])
				if !ok {
					ctx.GetLogger().Skip(args[0], "key not found")
					return nil
				}
				printConfigValue(val)
			case 2:
				// Delete or set.
				if args[0] == "rm" {
					if err := svc.Delete(args[1]); err != nil {
						return err
					}
					ctx.GetLogger().Done(args[1], "deleted")
				} else {
					coerced := service.CoerceValue(args[1])
					if err := svc.Set(args[0], coerced); err != nil {
						return err
					}
					ctx.GetLogger().Done(args[0], fmt.Sprintf("set to %v", coerced))
				}
			}
			return nil
		},
	}
	return cmd
}

func printConfigValue(value any) {
	if m, ok := value.(map[string]any); ok {
		printConfigMap(m)
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, value)
}

func printConfigMap(m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, strings.Join(keys, "\t"))
	separators := make([]string, 0, len(keys))
	for _, key := range keys {
		separators = append(separators, strings.Repeat("-", len(key)))
	}
	_, _ = fmt.Fprintln(w, strings.Join(separators, "\t"))
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, fmt.Sprint(m[key]))
	}
	_, _ = fmt.Fprintln(w, strings.Join(values, "\t"))
	_ = w.Flush()
}

func formatConfigListValue(value any) string {
	m, ok := value.(map[string]any)
	if !ok {
		return fmt.Sprint(value)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, m[key]))
	}
	return "@{" + strings.Join(parts, "; ") + "}"
}

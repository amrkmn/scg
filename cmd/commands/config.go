package commands

import (
	"fmt"
	"os"
	"sort"

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
			ctx := cmdctx.FromCmd(cmd)
			if ctx == nil {
				return fmt.Errorf("context unavailable")
			}
			svc := ctx.Services.Config

			switch len(args) {
			case 0:
				// List all.
				config, err := svc.Load()
				if err != nil {
					return err
				}
				if len(config) == 0 {
					fmt.Fprintln(os.Stdout, ui.Dim("(no config values set)"))
					return nil
				}
				keys := make([]string, 0, len(config))
				for k := range config {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Fprintf(os.Stdout, "%s: %v\n", ui.Green(k), config[k])
				}
			case 1:
				// Get.
				val, ok := svc.Get(args[0])
				if !ok {
					fmt.Fprintf(os.Stderr, "Key '%s' not found\n", args[0])
					return nil
				}
				fmt.Fprintln(os.Stdout, val)
			case 2:
				// Delete or set.
				if args[0] == "rm" {
					if err := svc.Delete(args[1]); err != nil {
						return err
					}
					fmt.Fprintf(os.Stdout, "%s Deleted key '%s'\n", ui.Success("✓"), args[1])
				} else {
					coerced := service.CoerceValue(args[1])
					if err := svc.Set(args[0], coerced); err != nil {
						return err
					}
					fmt.Fprintf(os.Stdout, "%s Set %s = %v\n", ui.Success("✓"), ui.Green(args[0]), coerced)
				}
			}
			return nil
		},
	}
	return cmd
}

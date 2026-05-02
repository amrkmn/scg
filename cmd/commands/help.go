package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/ui"
)

// NewHelpCommand creates the help subcommand.
func NewHelpCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "help [command]",
		Short:   "Show help for a command",
		Long:    "Show help for a specific command or list all available commands.",
		Args:    cobra.MaximumNArgs(1),
		Example: "scg help\nscg help install",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				printHelp(root)
				return nil
			}

			target, _, err := root.Find([]string{args[0]})
			if err != nil || target == nil {
				return fmt.Errorf(ui.Warning("no such command: '%s'"), args[0])
			}

			helpFunc := target.HelpFunc()
			if helpFunc != nil {
				helpFunc(target, args[1:])
			}
			return nil
		},
	}

	return cmd
}

func printHelp(root *cobra.Command) {
	fmt.Println("Usage: scg <command> [<args>]")
	fmt.Println()
	fmt.Println("Available commands are listed below.")
	fmt.Println()
	fmt.Println("Type 'scg help <command>' to get more help for a specific command.")
	fmt.Println()

	commands := root.Commands()
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name() < commands[j].Name()
	})

	maxLen := 0
	for _, c := range commands {
		if len(c.Name()) > maxLen {
			maxLen = len(c.Name())
		}
	}

	for _, c := range commands {
		if c.Hidden {
			continue
		}
		name := c.Name()
		short := c.Short
		if short == "" {
			short = "(no description)"
		}
		short = strings.SplitN(short, "\n", 2)[0]
		padding := strings.Repeat(" ", maxLen-len(name)+2)
		fmt.Printf("  %s%s%s\n", ui.Bold(name), padding, short)
	}
}

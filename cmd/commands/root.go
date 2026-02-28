package commands

import (
	"github.com/amrkmn/scg/cmd/commands/bucket"
	"github.com/amrkmn/scg/internal/app"
	"github.com/amrkmn/scg/internal/cmdctx"
	"github.com/spf13/cobra"
)

// NewRootCommand constructs the root cobra command and wires all subcommands.
func NewRootCommand(version string) *cobra.Command {
	var verbose bool

	root := &cobra.Command{
		Use:   "scg",
		Short: "SCoop in Go — a fast, native Scoop-compatible package manager",
		Long: `scg is a fast, native Scoop-compatible CLI for Windows.
It wraps the Scoop package manager with parallel operations and a clean interface.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Disable cobra's built-in multi-shell completion command.
	root.CompletionOptions.DisableDefaultCmd = true

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// Inject app context into cobra context on every command.
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		appCtx := app.NewContext(version, verbose)
		cmd.SetContext(cmdctx.Inject(cmd.Context(), appCtx))
		return nil
	}

	// Register all top-level commands.
	root.AddCommand(
		NewVersionCommand(version),
		NewPrefixCommand(),
		NewWhichCommand(),
		NewConfigCommand(),
		NewListCommand(),
		NewSearchCommand(),
		NewInfoCommand(),
		NewCleanupCommand(),
		NewStatusCommand(),
		bucket.NewBucketCommand(),
		newCompletionCommand(root),
	)

	return root
}

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewVersionCommand creates the version subcommand.
func NewVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the scg version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("v%s\n", version)
		},
	}
}

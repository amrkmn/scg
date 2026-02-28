package bucket

import (
	"github.com/spf13/cobra"
)

// NewBucketCommand creates the bucket subcommand group.
func NewBucketCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bucket",
		Short: "Manage Scoop buckets",
		Long:  `Add, remove, list, and update Scoop buckets.`,
	}

	cmd.AddCommand(
		NewListCommand(),
		NewAddCommand(),
		NewRemoveCommand(),
		NewKnownCommand(),
		NewUpdateCommand(),
		NewUnusedCommand(),
	)

	return cmd
}

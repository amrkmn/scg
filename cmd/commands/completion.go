package commands

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Generate PowerShell autocompletion script",
		Long: "Generate a PowerShell autocompletion script for scg.\n\n" +
			"To load completions in your current session:\n" +
			"  scg completion | Out-String | Invoke-Expression\n\n" +
			"To load completions for every new session, add the following line to your PowerShell profile:\n" +
			"  Invoke-Expression (scg completion | Out-String)\n\n" +
			"To write it to your profile automatically:\n" +
			"  Add-Content $PROFILE \"`nInvoke-Expression (scg completion | Out-String)\"",
		Args:               cobra.NoArgs,
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenPowerShellCompletionWithDesc(os.Stdout)
		},
	}
}

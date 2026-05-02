package commands

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
)

// NewHomeCommand creates the home subcommand.
func NewHomeCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "home <app>",
		Short:   "Open the homepage for an app",
		Args:    cobra.ExactArgs(1),
		Example: "scg home git\nscg home extras/vscode",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			_, manifest := ctx.Services.Manifests.FindManifestPair(args[0])
			if manifest == nil {
				_, _ = fmt.Fprintf(os.Stderr, "Could not find manifest for '%s'\n", args[0])
				return os.ErrNotExist
			}

			if manifest.Manifest.Homepage == "" {
				_, _ = fmt.Fprintf(os.Stderr, "Could not find homepage in manifest for '%s'\n", args[0])
				return fmt.Errorf("no homepage defined")
			}

			var openCmd *exec.Cmd
			switch runtime.GOOS {
			case "windows":
				openCmd = exec.Command("cmd", "/c", "start", manifest.Manifest.Homepage)
			default:
				openCmd = exec.Command("open", manifest.Manifest.Homepage)
			}
			openCmd.Stdout = os.Stdout
			openCmd.Stderr = os.Stderr
			if err := openCmd.Run(); err != nil {
				return fmt.Errorf("failed to open homepage: %w", err)
			}
			return nil
		},
	}
}

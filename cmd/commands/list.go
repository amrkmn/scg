package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/ui"
)

// NewListCommand creates the list subcommand.
func NewListCommand() *cobra.Command {
	var flagJSON bool

	cmd := &cobra.Command{
		Use:     "list [query]",
		Short:   "List installed apps",
		Args:    cobra.MaximumNArgs(1),
		Example: "scg list\nscg list git\nscg list --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.MustFromCmd(cmd)

			filter := ""
			if len(args) > 0 {
				filter = args[0]
			}

			apps, err := ctx.Services.Apps.ListInstalled(filter)
			if err != nil {
				return err
			}

			if flagJSON {
				type jsonApp struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Bucket  string `json:"bucket"`
					Updated string `json:"updated"`
					Held    bool   `json:"held"`
					Scope   string `json:"scope"`
				}
				out := make([]jsonApp, len(apps))
				for i, a := range apps {
					out[i] = jsonApp{
						Name:    a.Name,
						Version: a.Version,
						Bucket:  a.Bucket,
						Updated: a.Updated.Format(time.RFC3339),
						Held:    a.Held,
						Scope:   string(a.Scope),
					}
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				_, _ = fmt.Fprintln(os.Stdout, string(data))
				return nil
			}

			if len(apps) == 0 {
				if filter != "" {
					_, _ = fmt.Fprintln(os.Stdout, ui.Skip("apps", fmt.Sprintf("no matches for %s", filter)))
				} else {
					_, _ = fmt.Fprintln(os.Stdout, ui.Skip("apps", "none installed"))
				}
				return nil
			}

			// Table: Name | Version | Source | Updated | Info
			// Weights: 2.0, 1.0, 1.0, 0.5, 1.5
			header := []string{
				ui.BoldGreen("Name"),
				ui.BoldGreen("Version"),
				ui.BoldGreen("Source"),
				ui.BoldGreen("Updated"),
				ui.BoldGreen("Info"),
			}
			rows := [][]string{header}

			for _, a := range apps {
				info := ""
				if a.Held {
					info = ui.Yellow("Held")
				}
				if a.Scope == "global" {
					if info != "" {
						info += " "
					}
					info += ui.Dim("Global")
				}

				updated := ""
				if !a.Updated.IsZero() {
					updated = a.Updated.Format("2006-01-02")
				}

				rows = append(rows, []string{
					ui.Cyan(a.Name),
					a.Version,
					a.Bucket,
					updated,
					info,
				})
			}

			_, _ = fmt.Fprintln(os.Stdout, ui.FormatLineColumns(rows, []float64{2.0, 1.0, 1.0, 0.5, 1.5}))

			// Footer
			suffix := ""
			if filter != "" {
				suffix = fmt.Sprintf(" matching '%s'", filter)
			}
			_, _ = fmt.Fprintf(os.Stdout, "\n%s\n", ui.Dim(fmt.Sprintf("%d app(s) installed%s", len(apps), suffix)))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&flagJSON, "json", "j", false, "Output as JSON")
	return cmd
}

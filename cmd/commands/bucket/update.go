package bucket

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/amrkmn/scg/internal/cmdctx"
	"github.com/amrkmn/scg/internal/scoop"
	"github.com/amrkmn/scg/internal/service"
	"github.com/amrkmn/scg/internal/ui"
	"github.com/spf13/cobra"
)

// updateState tracks per-bucket display state for the animated UI
type updateState struct {
	name   string
	status string // "updating" | "updated" | "up-to-date" | "failed"
	err    error
}

func NewUpdateCommand() *cobra.Command {
	var global bool
	var changelog bool

	cmd := &cobra.Command{
		Use:   "update [name...]",
		Short: "Update buckets",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdctx.FromCmd(cmd)

			scope := scoop.ScopeUser
			if global {
				scope = scoop.ScopeGlobal
			}

			// Determine which buckets to update
			names := args
			if len(names) == 0 {
				allBuckets, err := ctx.Services.Buckets.List(scope)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s Failed to list buckets: %v\n", ui.Error("error:"), err)
					os.Exit(1)
				}
				for _, b := range allBuckets {
					names = append(names, b.Name)
				}
			}

			if len(names) == 0 {
				fmt.Println("No buckets to update.")
				return nil
			}

			// Initialize display state for each bucket
			var mu sync.Mutex
			states := make([]updateState, len(names))
			nameIndex := make(map[string]int, len(names))
			for i, name := range names {
				states[i] = updateState{name: name, status: "updating"}
				nameIndex[name] = i
			}

			// Print initial state (all "updating...")
			printStates(states)

			onStart := func(name string) {
				mu.Lock()
				defer mu.Unlock()
				if i, ok := nameIndex[name]; ok {
					states[i].status = "updating"
				}
				// Move cursor up and reprint
				reprintStates(states, len(names))
			}

			onComplete := func(result service.UpdateResult) {
				mu.Lock()
				defer mu.Unlock()
				if i, ok := nameIndex[result.Name]; ok {
					states[i].status = result.Status
					states[i].err = result.Error
				}
				// Move cursor up and reprint
				reprintStates(states, len(names))

				// Print changelog commits if requested
				if changelog && result.Status == "updated" && len(result.Commits) > 0 {
					fmt.Printf("\n  %s changelog:\n", ui.Bold(result.Name))
					for _, commit := range result.Commits {
						fmt.Printf("    %s\n", commit)
					}
				}
			}

			results := ctx.Services.Buckets.UpdateBuckets(names, scope, 4, changelog, onStart, onComplete)

			// Final summary
			updated := 0
			upToDate := 0
			failed := 0
			for _, r := range results {
				switch r.Status {
				case "updated":
					updated++
				case "up-to-date":
					upToDate++
				case "failed":
					failed++
				}
			}

			fmt.Println()
			parts := []string{}
			if updated > 0 {
				parts = append(parts, fmt.Sprintf("%s %d updated", ui.Success("✓"), updated))
			}
			if upToDate > 0 {
				parts = append(parts, fmt.Sprintf("%d up-to-date", upToDate))
			}
			if failed > 0 {
				parts = append(parts, fmt.Sprintf("%s %d failed", ui.Error("✗"), failed))
			}
			fmt.Println(strings.Join(parts, "  "))

			if failed > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Update buckets in global Scoop installation")
	cmd.Flags().BoolVar(&changelog, "changelog", false, "Show new commits after update")
	return cmd
}

// printStates prints all bucket states for the first time
func printStates(states []updateState) {
	for _, s := range states {
		fmt.Println(formatBucketLine(s))
	}
}

// reprintStates moves cursor up N lines and reprints
func reprintStates(states []updateState, n int) {
	// Move cursor up n lines
	fmt.Printf("\x1b[%dA", n)
	for _, s := range states {
		// Clear line and reprint
		fmt.Printf("\r\x1b[2K%s\n", formatBucketLine(s))
	}
}

// formatBucketLine returns a single formatted line for a bucket update state
func formatBucketLine(s updateState) string {
	switch s.status {
	case "updating":
		return fmt.Sprintf("  %s  %s", ui.Dim("..."), ui.Bold(s.name))
	case "updated":
		return fmt.Sprintf("  %s  %s  %s", ui.Success("✓"), ui.Bold(s.name), ui.Dim("updated"))
	case "up-to-date":
		return fmt.Sprintf("  %s  %s  %s", ui.Dim("–"), ui.Bold(s.name), ui.Dim("up-to-date"))
	case "failed":
		msg := "failed"
		if s.err != nil {
			msg = fmt.Sprintf("failed: %v", s.err)
		}
		return fmt.Sprintf("  %s  %s  %s", ui.Error("✗"), ui.Bold(s.name), ui.Error(msg))
	default:
		return fmt.Sprintf("  %s  %s", ui.Dim("?"), ui.Bold(s.name))
	}
}

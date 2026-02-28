package bucket

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// updateState tracks per-bucket display state for the animated UI
type updateState struct {
	name      string
	status    string // "updating" | "updated" | "up-to-date" | "failed"
	err       error
	changelog []string // commit lines to display after the status line
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
			var frame int
			states := make([]updateState, len(names))
			nameIndex := make(map[string]int, len(names))
			for i, name := range names {
				states[i] = updateState{name: name, status: "updating"}
				nameIndex[name] = i
			}

			// Print initial state (all "updating...")
			printStates(states, 0)

			// Background ticker: animate the dots for buckets still "updating"
			ticker := time.NewTicker(ui.SpinnerInterval)
			tickerDone := make(chan struct{})
			go func() {
				for {
					select {
					case <-tickerDone:
						return
					case <-ticker.C:
						mu.Lock()
						anyUpdating := false
						for _, s := range states {
							if s.status == "updating" {
								anyUpdating = true
								break
							}
						}
						if anyUpdating {
							frame++
							reprintStates(states, frame)
						}
						mu.Unlock()
					}
				}
			}()

			onStart := func(name string) {
				mu.Lock()
				defer mu.Unlock()
				if i, ok := nameIndex[name]; ok {
					states[i].status = "updating"
				}
				reprintStates(states, frame)
			}

			onComplete := func(result service.UpdateResult) {
				mu.Lock()
				defer mu.Unlock()
				if i, ok := nameIndex[result.Name]; ok {
					states[i].status = result.Status
					states[i].err = result.Error
					// Store changelog lines in state so reprintStates can account for them
					if changelog && result.Status == "updated" && len(result.Commits) > 0 {
						lines := make([]string, 0, len(result.Commits)+1)
						lines = append(lines, fmt.Sprintf("  %s changelog:", ui.Bold(result.Name)))
						for _, commit := range result.Commits {
							lines = append(lines, fmt.Sprintf("    %s", commit))
						}
						states[i].changelog = lines
					}
				}
				reprintStates(states, frame)
			}

			results := ctx.Services.Buckets.UpdateBuckets(names, scope, changelog, onStart, onComplete)

			// Stop the animation ticker and wait for the goroutine to exit
			ticker.Stop()
			// Drain any pending tick that fired between Stop() and close()
			select {
			case <-ticker.C:
			default:
			}
			close(tickerDone)

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

// totalLines returns the total number of lines occupied by all states (status + changelog)
func totalLines(states []updateState) int {
	n := len(states)
	for _, s := range states {
		n += len(s.changelog)
	}
	return n
}

// printStates prints all bucket states for the first time
func printStates(states []updateState, frame int) {
	for _, s := range states {
		fmt.Println(formatBucketLine(s, frame))
		for _, line := range s.changelog {
			fmt.Println(line)
		}
	}
}

// reprintStates moves cursor up to the top of the state block and reprints everything
func reprintStates(states []updateState, frame int) {
	n := totalLines(states)
	fmt.Printf("\x1b[%dA", n)
	for _, s := range states {
		fmt.Printf("\r\x1b[2K%s\n", formatBucketLine(s, frame))
		for _, line := range s.changelog {
			fmt.Printf("\r\x1b[2K%s\n", line)
		}
	}
}

var dotFrames = ui.SpinnerFrames

// formatBucketLine returns a single formatted line for a bucket update state
func formatBucketLine(s updateState, frame int) string {
	switch s.status {
	case "updating":
		dots := dotFrames[frame%len(dotFrames)]
		return fmt.Sprintf("  %s  %s", ui.Dim(dots), ui.Bold(s.name))
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

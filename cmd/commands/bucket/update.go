package bucket

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/cmdctx"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

// updateState tracks per-bucket display state for the animated UI.
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
					_, _ = fmt.Fprintln(os.Stderr, ui.FailLine(fmt.Sprintf("failed to list buckets: %v", err)))
					os.Exit(1)
				}
				for _, b := range allBuckets {
					names = append(names, b.Name)
				}
				sort.Strings(names)
			}

			if len(names) == 0 {
				fmt.Println(ui.Skip("buckets", "none to update"))
				return nil
			}

			fmt.Println(ui.Heading(fmt.Sprintf("Updating %d bucket(s)", len(names))))

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

			// Background ticker: animate wait frames for buckets still updating.
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
				}
				reprintStates(states, frame)
			}

			results := ctx.Services.Buckets.UpdateBuckets(cmd.Context(), names, scope, changelog, onStart, onComplete)

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
			fmt.Println(ui.Heading("Summary"))
			parts := []string{}
			if updated > 0 {
				parts = append(parts, ui.Green(fmt.Sprintf("%d updated", updated)))
			}
			if upToDate > 0 {
				parts = append(parts, ui.Dim(fmt.Sprintf("%d up-to-date", upToDate)))
			}
			if failed > 0 {
				parts = append(parts, ui.Red(fmt.Sprintf("%d failed", failed)))
			}
			fmt.Println(ui.Detail(strings.Join(parts, ", ")))

			// Print changelogs for updated buckets at the bottom
			if changelog && updated > 0 {
				fmt.Println()
				fmt.Println(ui.Heading("Changelog"))
				sortedResults := make([]service.UpdateResult, len(results))
				copy(sortedResults, results)
				sort.Slice(sortedResults, func(i, j int) bool {
					return sortedResults[i].Name < sortedResults[j].Name
				})
				for _, r := range sortedResults {
					if r.Status == "updated" && len(r.Commits) > 0 {
						fmt.Printf("  %s:\n", ui.Cyan(ui.Bold(r.Name)))
						for _, commit := range r.Commits {
							fmt.Printf("    %s\n", commit)
						}
					}
				}
			}

			if failed > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Update buckets in global Scoop installation")
	cmd.Flags().BoolVarP(&changelog, "changelog", "c", false, "Show new commits after update")
	return cmd
}

// printStates prints all bucket states for the first time
func printStates(states []updateState, frame int) {
	nameWidth := bucketNameWidth(states)
	for _, s := range states {
		fmt.Println(formatBucketLine(s, frame, nameWidth))
	}
}

// reprintStates moves cursor up to the top of the state block and reprints everything
func reprintStates(states []updateState, frame int) {
	nameWidth := bucketNameWidth(states)
	fmt.Printf("\x1b[%dA", len(states))
	for _, s := range states {
		fmt.Printf("\r\x1b[2K%s\n", formatBucketLine(s, frame, nameWidth))
	}
}

var waitFrames = ui.SpinnerFrames

func bucketNameWidth(states []updateState) int {
	width := 0
	for _, s := range states {
		if len(s.name) > width {
			width = len(s.name)
		}
	}
	return width
}

// formatBucketLine returns a single formatted line for a bucket update state.
func formatBucketLine(s updateState, frame, nameWidth int) string {
	name := ui.PadRight(s.name, nameWidth)
	switch s.status {
	case "updating":
		spinner := waitFrames[frame%len(waitFrames)]
		return ui.Detail(fmt.Sprintf("%s %s %s", ui.Cyan(spinner), ui.Bold(name), ui.Cyan("updating")))
	case "updated":
		return ui.Detail(fmt.Sprintf("  %s %s", ui.Green(name), ui.Green("updated")))
	case "up-to-date":
		return ui.Detail(ui.Dim(fmt.Sprintf("  %s up-to-date", name)))
	case "failed":
		msg := "failed"
		if s.err != nil {
			msg = fmt.Sprintf("failed: %v", s.err)
		}
		return ui.Detail(fmt.Sprintf("  %s %s", ui.Red(name), ui.Red(msg)))
	default:
		return ui.Detail(ui.Dim("  " + name))
	}
}

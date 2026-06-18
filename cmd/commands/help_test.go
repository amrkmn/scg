package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPrintHelp(t *testing.T) {
	root := &cobra.Command{Use: "scg"}
	root.AddCommand(
		&cobra.Command{Use: "list", Short: "List installed apps"},
		&cobra.Command{Use: "search", Short: "Search for apps in buckets"},
		&cobra.Command{Use: "hidden", Short: "Hidden command", Hidden: true},
	)

	var out bytes.Buffer
	printHelp(&out, root)

	rendered := out.String()
	checks := []string{
		"Usage: scg <command> [<args>]",
		"Available commands are listed below.",
		"Type 'scg help <command>' to get more help for a specific command.",
		"Command",
		"Description",
		"list",
		"List installed apps",
		"search",
		"Search for apps in buckets",
		"2 command(s)",
	}
	for _, want := range checks {
		if !strings.Contains(rendered, want) {
			t.Fatalf("printHelp() missing %q\nrendered:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "hidden") {
		t.Fatalf("printHelp() rendered hidden command\nrendered:\n%s", rendered)
	}
}

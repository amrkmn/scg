package commands

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestSplitAliasCommand(t *testing.T) {
	got, err := splitAliasCommand(`scg update --all "some app"`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"scg", "update", "--all", "some app"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitAliasCommand() = %#v, want %#v", got, want)
	}
}

func TestExpandAliasPlaceholders(t *testing.T) {
	got, err := expandAliasPlaceholders([]string{"uninstall", "$args[0]"}, []string{"git"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"uninstall", "git"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandAliasPlaceholders() = %#v, want %#v", got, want)
	}
}

func TestExpandAliasPlaceholdersAppendsArgs(t *testing.T) {
	got, err := expandAliasPlaceholders([]string{"update", "--all"}, []string{"--quiet"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"update", "--all", "--quiet"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandAliasPlaceholders() = %#v, want %#v", got, want)
	}
}

func TestCommandExists(t *testing.T) {
	root := &cobra.Command{Use: "scg"}
	root.AddCommand(&cobra.Command{Use: "update", Aliases: []string{"up"}})
	if !commandExists(root, "update") {
		t.Fatal("expected direct command to exist")
	}
	if !commandExists(root, "up") {
		t.Fatal("expected command alias to exist")
	}
	if commandExists(root, "missing") {
		t.Fatal("did not expect missing command to exist")
	}
}

func TestAliasScriptName(t *testing.T) {
	aliases := map[string]any{"up": "scoop-upgrade"}
	if got := aliasScriptName(aliases, "up"); got != "scoop-upgrade" {
		t.Fatalf("aliasScriptName() = %q, want %q", got, "scoop-upgrade")
	}
	if got := aliasScriptName(aliases, "missing"); got != "scoop-missing" {
		t.Fatalf("aliasScriptName() = %q, want %q", got, "scoop-missing")
	}
}

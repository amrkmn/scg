package commands

import (
	"strings"
	"testing"

	"go.noz.one/scg/internal/ui"
)

func TestBuildShimInfoPairs(t *testing.T) {
	pairs := buildShimInfoPairs(
		"git",
		`C:\Users\me\scoop\apps\git\current\bin\git.exe`,
		"--version",
		"git",
		"local",
		"",
		[]string{"mingit", "portablegit"},
	)

	block := ui.RenderKeyValueBlock("Shim git", pairs)
	for _, want := range []string{
		"Shim git",
		"Name",
		"git",
		"Args",
		"--version",
		"Source",
		"Scope",
		"Alternatives",
		"mingit, portablegit",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("shim info block missing %q\n%s", want, block)
		}
	}
}

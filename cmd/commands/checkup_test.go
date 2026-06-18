package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderCheckupOutput(t *testing.T) {
	var out bytes.Buffer
	renderCheckupOutput(&out, []checkResult{
		{ok: true, label: "Git is installed"},
		{ok: false, label: "Main bucket is installed", detail: "Run: scg bucket add main"},
	}, 1)

	rendered := out.String()
	checks := []string{
		"==> Checking Scoop environment",
		"Git is installed",
		"Main bucket is installed",
		"Run: scg bucket add main",
		"==> Summary",
		"found 1 potential problem",
	}
	for _, want := range checks {
		if !strings.Contains(rendered, want) {
			t.Fatalf("renderCheckupOutput() missing %q\nrendered:\n%s", want, rendered)
		}
	}
}

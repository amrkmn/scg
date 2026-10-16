package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptShimFiles is pure (no filesystem writes); verify exact wrapper content.
func TestScriptShimFilesPS1(t *testing.T) {
	resolved := `C:\Users\me\scoop\apps\cowsay\current\cowsay.ps1`
	shimDir := `C:\Users\me\scoop\shims`
	files := scriptShimFiles("cowsay", resolved, "", shimDir)

	if len(files) != 3 {
		t.Fatalf("expected 3 wrapper files, got %d: %v", len(files), files)
	}

	ps1, ok := files["cowsay.ps1"]
	if !ok {
		t.Fatal("missing cowsay.ps1 proxy")
	}
	if !strings.Contains(ps1, "$path = Join-Path $PSScriptRoot") {
		t.Errorf("ps1 proxy should reference target relative to shim dir, got:\n%s", ps1)
	}
	if !strings.Contains(ps1, "exit $LASTEXITCODE") {
		t.Errorf("ps1 proxy should forward exit code, got:\n%s", ps1)
	}
	if !strings.Contains(ps1, "\r\n") {
		t.Error("ps1 proxy should use CRLF")
	}

	cmd, ok := files["cowsay.cmd"]
	if !ok {
		t.Fatal("missing cowsay.cmd wrapper")
	}
	for _, want := range []string{
		`@rem ` + resolved,
		`pwsh -noprofile -ex unrestricted -file "` + resolved + `"`,
		`powershell -noprofile -ex unrestricted -file "` + resolved + `"`,
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("cmd wrapper missing %q, got:\n%s", want, cmd)
		}
	}

	sh, ok := files["cowsay"]
	if !ok {
		t.Fatal("missing extensionless sh wrapper")
	}
	if !strings.HasPrefix(sh, "#!/bin/sh\n") {
		t.Errorf("sh wrapper should start with shebang, got:\n%s", sh)
	}
	if strings.HasSuffix(sh, "\n") {
		t.Error("sh wrapper should have no trailing newline (like Scoop)")
	}
	if strings.Contains(sh, "\r") {
		t.Error("sh wrapper should use LF only")
	}
}

func TestScriptShimFilesBat(t *testing.T) {
	resolved := `C:\apps\tool\run.bat`
	files := scriptShimFiles("run", resolved, "--flag", `C:\shims`)

	if len(files) != 2 {
		t.Fatalf("expected 2 wrapper files, got %d", len(files))
	}
	if cmd := files["run.cmd"]; !strings.Contains(cmd, `@"`+resolved+`" --flag %*`) {
		t.Errorf("unexpected cmd wrapper:\n%s", cmd)
	}
	if sh := files["run"]; !strings.Contains(sh, `MSYS2_ARG_CONV_EXCL=/C cmd.exe /C`) {
		t.Errorf("unexpected sh wrapper:\n%s", sh)
	}
}

func TestScriptShimFilesPyAndJar(t *testing.T) {
	py := scriptShimFiles("tool", `C:\apps\tool\tool.py`, "", `C:\shims`)
	if cmd := py["tool.cmd"]; !strings.Contains(cmd, `@python "C:\apps\tool\tool.py"`) {
		t.Errorf("unexpected py cmd wrapper:\n%s", cmd)
	}

	jar := scriptShimFiles("app", `C:\apps\app\app.jar`, "", `C:\shims`)
	if cmd := jar["app.cmd"]; !strings.Contains(cmd, `@java -jar "C:\apps\app\app.jar"`) {
		t.Errorf("unexpected jar cmd wrapper:\n%s", cmd)
	}
	if sh := jar["app"]; !strings.Contains(sh, "wslpath") || !strings.Contains(sh, "cygpath") {
		t.Errorf("unexpected jar sh wrapper:\n%s", sh)
	}
}

// createSingleShim with a .ps1 target must write wrappers, not an .exe+.shim pair.
func TestCreateSingleShimPS1WritesWrappers(t *testing.T) {
	appDir := t.TempDir()
	shimDir := t.TempDir()

	target := filepath.Join(appDir, "cowsay.ps1")
	if err := os.WriteFile(target, []byte("Write-Host hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	def := ShimDef{Target: "cowsay.ps1", Name: "cowsay"}
	if err := createSingleShim(def, appDir, shimDir); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"cowsay.ps1", "cowsay.cmd", "cowsay"} {
		if _, err := os.Stat(filepath.Join(shimDir, want)); err != nil {
			t.Errorf("expected wrapper %s to exist: %v", want, err)
		}
	}
	for _, mustNotExist := range []string{"cowsay.exe", "cowsay.shim"} {
		if _, err := os.Stat(filepath.Join(shimDir, mustNotExist)); !os.IsNotExist(err) {
			t.Errorf("expected no %s for a .ps1 target", mustNotExist)
		}
	}
}

// createSingleShim with an .exe target keeps the classic .exe+.shim pair.
func TestCreateSingleShimExeKeepsPair(t *testing.T) {
	appDir := t.TempDir()
	shimDir := t.TempDir()

	target := filepath.Join(appDir, "tool.exe")
	if err := os.WriteFile(target, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}

	def := ShimDef{Target: "tool.exe", Name: "tool"}
	if err := createSingleShim(def, appDir, shimDir); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"tool.exe", "tool.shim"} {
		if _, err := os.Stat(filepath.Join(shimDir, want)); err != nil {
			t.Errorf("expected %s to exist: %v", want, err)
		}
	}
}

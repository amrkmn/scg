package install

import "testing"

func TestInstallerArgs(t *testing.T) {
	vars := map[string]string{
		"dir": `C:\apps\tool\current`,
	}

	got, err := installerArgs([]any{"--path", "$dir"}, vars)
	if err != nil {
		t.Fatalf("installerArgs returned error: %v", err)
	}
	if len(got) != 2 || got[0] != "--path" || got[1] != `C:\apps\tool\current` {
		t.Fatalf("installerArgs result = %#v", got)
	}
}

func TestRunInstallerHookUnsupportedMap(t *testing.T) {
	err := RunInstallerHook("installer", map[string]any{"keep": true}, ".", nil)
	if err == nil {
		t.Fatal("expected error for unsupported installer map, got nil")
	}
}

func TestRunInstallerHookFileMissing(t *testing.T) {
	err := RunInstallerHook("installer", map[string]any{"file": "missing.exe"}, ".", nil)
	if err == nil {
		t.Fatal("expected missing file error, got nil")
	}
}

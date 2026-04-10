package install

import "testing"

func TestParseShortcutsField(t *testing.T) {
	in := []any{
		[]any{"app.exe", "App"},
		[]any{"bin/tool.exe", "Tool", "--help"},
		[]any{"bin/ui.exe", "UI", "", "bin/ui.ico"},
		[]any{"", "Bad"},
		"invalid",
	}

	got := ParseShortcutsField(in)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Target != "app.exe" || got[0].Name != "App" {
		t.Fatalf("unexpected first shortcut: %+v", got[0])
	}
	if got[1].Args != "--help" {
		t.Fatalf("unexpected args: %+v", got[1])
	}
	if got[2].Icon != "bin/ui.ico" {
		t.Fatalf("unexpected icon: %+v", got[2])
	}
}

package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvAddPaths(t *testing.T) {
	appDir := `C:\Users\test\scoop\apps\myapp\current`

	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{"nil", nil, nil},
		{"string", "bin", []string{filepath.Join(appDir, "bin")}},
		{"array", []any{"bin", "scripts"}, []string{
			filepath.Join(appDir, "bin"),
			filepath.Join(appDir, "scripts"),
		}},
		{"empty array", []any{}, nil},
		{"non-string items filtered", []any{"bin", 123, "scripts"}, []string{
			filepath.Join(appDir, "bin"),
			filepath.Join(appDir, "scripts"),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnvAddPaths(tt.input, appDir)
			if len(got) != len(tt.want) {
				t.Fatalf("EnvAddPaths() = %v, want %v", got, tt.want)
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("result[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestEnvSetVars(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		want  map[string]string
	}{
		{"nil", nil, nil},
		{
			"simple",
			map[string]string{"FOO": "bar", "BAZ": "qux"},
			map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnvSetVars(tt.input)
			if tt.input == nil {
				if got != nil {
					t.Errorf("EnvSetVars(nil) = %v, want nil", got)
				}
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("EnvSetVars()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"single path", `C:\test\bin`, []string{`C:\test\bin`}},
		{"multiple paths", `C:\a;D:\b;E:\c`, []string{`C:\a`, `D:\b`, `E:\c`}},
		{"trailing semicolons", `C:\a;;D:\b;`, []string{`C:\a`, `D:\b`}},
		{"leading spaces", ` C:\a ; D:\b `, []string{`C:\a`, `D:\b`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitPath(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitPath(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("result[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestEscapePS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"it's", "it''s"},
		{"don't stop", "don''t stop"},
		{"", ""},
		{"no quotes", "no quotes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapePS(tt.input)
			if got != tt.want {
				t.Errorf("escapePS(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSetupHookEnvVars(t *testing.T) {
	t.Run("user scope", func(t *testing.T) {
		got := SetupHookEnvVars(
			`C:\scoop\apps\git\current`,
			`C:\scoop\apps\git\2.43.0`,
			"2.43.0", "64bit", "git",
			`C:\scoop\persist\git`,
			`C:\scoop`, "", false,
		)
		if got["dir"] != `C:\scoop\apps\git\current` {
			t.Errorf("dir = %q, want %q", got["dir"], `C:\scoop\apps\git\current`)
		}
		if got["version"] != "2.43.0" {
			t.Errorf("version = %q, want %q", got["version"], "2.43.0")
		}
		if got["architecture"] != "64bit" {
			t.Errorf("architecture = %q, want %q", got["architecture"], "64bit")
		}
		if got["app"] != "git" {
			t.Errorf("app = %q, want %q", got["app"], "git")
		}
		if got["global"] != "" {
			t.Errorf("global = %q, want empty string for user scope", got["global"])
		}
	})

	t.Run("global scope", func(t *testing.T) {
		got := SetupHookEnvVars(
			`C:\ProgramData\scoop\apps\git\current`,
			`C:\ProgramData\scoop\apps\git\2.43.0`,
			"2.43.0", "64bit", "git",
			`C:\ProgramData\scoop\persist\git`,
			`C:\ProgramData\scoop`, "", true,
		)
		if got["global"] != "true" {
			t.Errorf("global = %q, want %q", got["global"], "true")
		}
	})
}

func TestBuildHookPrelude(t *testing.T) {
	prelude := buildHookPrelude(map[string]string{
		"dir":    `C:\scoop\apps\git\current`,
		"app":    "git",
		"global": "true",
		"quote":  "it's",
	})

	wantSnippets := []string{
		"$env:dir = 'C:\\scoop\\apps\\git\\current'",
		"Set-Variable -Name 'dir' -Value 'C:\\scoop\\apps\\git\\current'",
		"$env:app = 'git'",
		"Set-Variable -Name 'app' -Value 'git'",
		"$env:global = 'true'",
		"Set-Variable -Name 'global' -Value 'true'",
		"$env:quote = 'it''s'",
		"Set-Variable -Name 'quote' -Value 'it''s'",
	}

	for _, snippet := range wantSnippets {
		if !strings.Contains(prelude, snippet) {
			t.Fatalf("buildHookPrelude() missing snippet: %q\nprelude:\n%s", snippet, prelude)
		}
	}
}

func TestExpandEnvSetVars(t *testing.T) {
	input := map[string]string{
		"APP_HOME": "$dir",
		"DATA_DIR": "$persist_dir\\data",
		"MIXED":    "${scoopdir}\\apps\\$app\\$version",
		"STATIC":   "hello",
	}
	vars := map[string]string{
		"dir":         `C:\scoop\apps\myapp\current`,
		"persist_dir": `C:\scoop\persist\myapp`,
		"scoopdir":    `C:\scoop`,
		"app":         "myapp",
		"version":     "1.2.3",
	}

	got := ExpandEnvSetVars(input, vars)

	if got["APP_HOME"] != `C:\scoop\apps\myapp\current` {
		t.Fatalf("APP_HOME = %q", got["APP_HOME"])
	}
	if got["DATA_DIR"] != `C:\scoop\persist\myapp\data` {
		t.Fatalf("DATA_DIR = %q", got["DATA_DIR"])
	}
	if got["MIXED"] != `C:\scoop\apps\myapp\1.2.3` {
		t.Fatalf("MIXED = %q", got["MIXED"])
	}
	if got["STATIC"] != "hello" {
		t.Fatalf("STATIC = %q", got["STATIC"])
	}
}

func TestExpandEnvSetVarsPrefixCollision(t *testing.T) {
	input := map[string]string{
		"ARCH_TEXT": "$architecture",
	}
	vars := map[string]string{
		"arch":         "64bit",
		"architecture": "x64",
	}

	got := ExpandEnvSetVars(input, vars)
	if got["ARCH_TEXT"] != "x64" {
		t.Fatalf("ARCH_TEXT = %q, want %q", got["ARCH_TEXT"], "x64")
	}
}

func TestPathAdditions(t *testing.T) {
	input := []string{
		`C:\apps\a\bin`,
		`C:\apps\b\bin`,
		`C:\apps\a\bin`,
	}
	current := []string{
		`C:\apps\b\bin`,
	}

	got := pathAdditions(input, current)
	if len(got) != 1 {
		t.Fatalf("pathAdditions length = %d, want 1; got=%v", len(got), got)
	}
	if got[0] != `C:\apps\a\bin` {
		t.Fatalf("pathAdditions[0] = %q, want %q", got[0], `C:\apps\a\bin`)
	}
}

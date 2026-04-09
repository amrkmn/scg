package install

import (
	"path/filepath"
	"testing"
)

func TestParseBinField(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []ShimDef
	}{
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
		{
			name:  "string input",
			input: "bin/app.exe",
			want:  []ShimDef{{Target: "bin/app.exe", Name: "app.exe"}},
		},
		{
			name:  "string with path",
			input: "bin/git/cmd/git.exe",
			want:  []ShimDef{{Target: "bin/git/cmd/git.exe", Name: "git.exe"}},
		},
		{
			name:  "array of strings",
			input: []any{"bin/app.exe", "bin/helper.exe"},
			want: []ShimDef{
				{Target: "bin/app.exe", Name: "app.exe"},
				{Target: "bin/helper.exe", Name: "helper.exe"},
			},
		},
		{
			name:  "array with alias",
			input: []any{[]any{"bin/app.exe", "myapp"}},
			want: []ShimDef{
				{Target: "bin/app.exe", Name: "myapp"},
			},
		},
		{
			name:  "array with alias and args",
			input: []any{[]any{"bin/app.exe", "myapp", "--flag"}},
			want: []ShimDef{
				{Target: "bin/app.exe", Name: "myapp", Args: "--flag"},
			},
		},
		{
			name:  "map input",
			input: map[string]any{"bin/app.exe": "myapp"},
			want: []ShimDef{
				{Target: "bin/app.exe", Name: "myapp"},
			},
		},
		{
			name:  "map with empty alias uses basename",
			input: map[string]any{"bin/app.exe": ""},
			want: []ShimDef{
				{Target: "bin/app.exe", Name: "app.exe"},
			},
		},
		{
			name: "mixed array",
			input: []any{
				"cmd/git.exe",
				[]any{"cmd/git-gui.exe", "git-gui", "--gui"},
			},
			want: []ShimDef{
				{Target: "cmd/git.exe", Name: "git.exe"},
				{Target: "cmd/git-gui.exe", Name: "git-gui", Args: "--gui"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBinField(tt.input, "64bit")
			if len(got) != len(tt.want) {
				t.Fatalf("ParseBinField() = %d results, want %d", len(got), len(tt.want))
			}
			for i, g := range got {
				w := tt.want[i]
				if g.Target != w.Target {
					t.Errorf("result[%d].Target = %q, want %q", i, g.Target, w.Target)
				}
				if g.Name != w.Name {
					t.Errorf("result[%d].Name = %q, want %q", i, g.Name, w.Name)
				}
				if g.Args != w.Args {
					t.Errorf("result[%d].Args = %q, want %q", i, g.Args, w.Args)
				}
			}
		})
	}
}

func TestParseArchBinField(t *testing.T) {
	archData := map[string]any{
		"64bit": map[string]any{
			"bin": "bin/app64.exe",
		},
		"32bit": map[string]any{
			"bin": "bin/app32.exe",
		},
	}

	t.Run("64bit arch", func(t *testing.T) {
		got := ParseArchBinField(archData, "64bit")
		if len(got) != 1 {
			t.Fatalf("ParseArchBinField 64bit = %d results, want 1", len(got))
		}
		if got[0].Target != "bin/app64.exe" {
			t.Errorf("ParseArchBinField 64bit target = %q, want %q", got[0].Target, "bin/app64.exe")
		}
	})

	t.Run("32bit arch", func(t *testing.T) {
		got := ParseArchBinField(archData, "32bit")
		if len(got) != 1 {
			t.Fatalf("ParseArchBinField 32bit = %d results, want 1", len(got))
		}
		if got[0].Target != "bin/app32.exe" {
			t.Errorf("ParseArchBinField 32bit target = %q, want %q", got[0].Target, "bin/app32.exe")
		}
	})

	t.Run("missing arch", func(t *testing.T) {
		got := ParseArchBinField(archData, "arm64")
		if got != nil {
			t.Errorf("ParseArchBinField arm64 = %v, want nil", got)
		}
	})

	t.Run("nil data", func(t *testing.T) {
		got := ParseArchBinField(nil, "64bit")
		if got != nil {
			t.Errorf("ParseArchBinField nil = %v, want nil", got)
		}
	})
}

func TestParseBinFieldBasename(t *testing.T) {
	got := ParseBinField("deep/nested/path/myapp.exe", "64bit")
	if len(got) != 1 {
		t.Fatal("expected 1 result")
	}
	if got[0].Name != "myapp.exe" {
		t.Errorf("Name = %q, want %q", got[0].Name, "myapp.exe")
	}
}

func TestParseBinFieldEmptyArray(t *testing.T) {
	got := ParseBinField([]any{}, "64bit")
	if got != nil {
		t.Errorf("ParseBinField empty array = %v, want nil", got)
	}
}

func TestParseBinFieldSingleItemArray(t *testing.T) {
	got := ParseBinField([]any{"app.exe"}, "64bit")
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Target != "app.exe" {
		t.Errorf("Target = %q, want %q", got[0].Target, "app.exe")
	}
}

func TestShimDefPathJoin(t *testing.T) {
	def := ShimDef{Target: filepath.Join("bin", "app.exe"), Name: "app"}
	absPath := filepath.Join("C:", "Users", "test", "scoop", "apps", "myapp", "current", def.Target)
	if filepath.Base(absPath) != "app.exe" {
		t.Errorf("filepath.Base of joined path = %q, want %q", filepath.Base(absPath), "app.exe")
	}
}

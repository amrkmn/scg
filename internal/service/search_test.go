package service

import "testing"

func TestExtractBinaries(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
		{
			name:  "single string",
			input: "app.exe",
			want:  []string{"app.exe"},
		},
		{
			name:  "string with path",
			input: "bin/app.exe",
			want:  []string{"app.exe"},
		},
		{
			name:  "string with windows path",
			input: "bin\\app.exe",
			want:  []string{"app.exe"},
		},
		{
			name:  "array of strings",
			input: []any{"app1.exe", "app2.exe"},
			want:  []string{"app1.exe", "app2.exe"},
		},
		{
			name:  "array with arrays (aliased binaries)",
			input: []any{[]any{"bin/app.exe", "app"}, []any{"bin/helper.exe", "helper"}},
			want:  []string{"app", "helper"},
		},
		{
			name:  "array with mixed types",
			input: []any{"standalone.exe", []any{"bin/nested.exe", "nested"}},
			want:  []string{"standalone.exe", "nested"},
		},
		{
			name:  "map format (aliases)",
			input: map[string]any{"app": "bin/app.exe", "helper": "bin/helper.exe"},
			want:  []string{"app", "helper"}, // Map keys become the binary names
		},
		{
			name:  "empty array",
			input: []any{},
			want:  nil,
		},
		{
			name:  "array with no alias fallback",
			input: []any{[]any{"bin/app.exe"}},
			want:  []string{"app.exe"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractBinaries(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractBinaries() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractBinaries()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
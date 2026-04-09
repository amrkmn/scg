package install

import "testing"

func TestParsePersistField(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{"nil", nil, nil},
		{"string", "data", []string{"data"}},
		{"array of strings", []any{"data", "config"}, []string{"data", "config"}},
		{"array with rename", []any{[]any{"logs", "app-logs"}}, []string{"logs"}},
		{"mixed", []any{"data", []any{"conf", "app-conf"}, "state"}, []string{"data", "conf", "state"}},
		{"empty array", []any{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePersistField(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ParsePersistField() = %v, want %v", got, tt.want)
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("result[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

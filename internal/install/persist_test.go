package install

import "testing"

func TestEnsurePersistACLUserScopeNoop(t *testing.T) {
	if err := EnsurePersistACL("user"); err != nil {
		t.Fatalf("EnsurePersistACL(user) = %v, want nil", err)
	}
}

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

func TestParsePersistItems(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []PersistItem
	}{
		{"nil", nil, nil},
		{"string", "data", []PersistItem{{Source: "data", Target: "data"}}},
		{"array of strings", []any{"data", "config"}, []PersistItem{{Source: "data", Target: "data"}, {Source: "config", Target: "config"}}},
		{"array with rename", []any{[]any{"logs", "app-logs"}}, []PersistItem{{Source: "logs", Target: "app-logs"}}},
		{"mixed", []any{"data", []any{"conf", "app-conf"}, "state"}, []PersistItem{{Source: "data", Target: "data"}, {Source: "conf", Target: "app-conf"}, {Source: "state", Target: "state"}}},
		{"empty array", []any{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePersistItems(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ParsePersistItems() = %v, want %v", got, tt.want)
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("result[%d] = %v, want %v", i, v, tt.want[i])
				}
			}
		})
	}
}

package service

import "testing"

func TestParseVersionString(t *testing.T) {
	tests := []struct {
		input string
		want  [4]int
	}{
		{"1.0.0", [4]int{1, 0, 0, 0}},
		{"1.2.3", [4]int{1, 2, 3, 0}},
		{"2.0", [4]int{2, 0, 0, 0}},
		{"1.0.0.0", [4]int{1, 0, 0, 0}},
		{"10.20.30", [4]int{10, 20, 30, 0}},
		{"1.0.0-beta", [4]int{1, 0, 0, 0}},
		{"1.2.3-rc1", [4]int{1, 2, 3, 0}},
		{"1.2.3+build", [4]int{1, 2, 3, 0}},
		{"2024.01.02", [4]int{2024, 1, 2, 0}},
		{"0.0.0", [4]int{0, 0, 0, 0}},
		{"", [4]int{0, 0, 0, 0}},
		{"1", [4]int{1, 0, 0, 0}},
		{"1.2.3.4.5", [4]int{1, 2, 3, 4}}, // Takes only first 4 parts
	}

	for _, tt := range tests {
		got := parseVersionString(tt.input)
		if got != tt.want {
			t.Errorf("parseVersionString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestCompareVersionArrays(t *testing.T) {
	tests := []struct {
		a, b [4]int
		want int
	}{
		{[4]int{1, 0, 0, 0}, [4]int{1, 0, 0, 0}, 0},  // equal
		{[4]int{1, 0, 0, 0}, [4]int{2, 0, 0, 0}, -1}, // a < b
		{[4]int{2, 0, 0, 0}, [4]int{1, 0, 0, 0}, 1},  // a > b
		{[4]int{1, 2, 0, 0}, [4]int{1, 3, 0, 0}, -1}, // minor version
		{[4]int{1, 2, 3, 0}, [4]int{1, 2, 4, 0}, -1}, // patch version
		{[4]int{1, 2, 3, 4}, [4]int{1, 2, 3, 5}, -1}, // build number
		{[4]int{10, 0, 0, 0}, [4]int{9, 9, 9, 9}, 1}, // major wins
		{[4]int{0, 0, 0, 0}, [4]int{0, 0, 0, 0}, 0},  // zeros
	}

	for _, tt := range tests {
		got := compareVersionArrays(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareVersionArrays(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestLeadingInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"123", 123},
		{"456abc", 456},
		{"0", 0},
		{"999", 999},
		{"abc", 0}, // No leading digits
		{"", 0},
		{"1.2", 1}, // Stops at non-digit
	}

	for _, tt := range tests {
		got := leadingInt(tt.input)
		if got != tt.want {
			t.Errorf("leadingInt(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestToStringSlice(t *testing.T) {
	tests := []struct {
		input any
		want  []string
	}{
		{nil, nil},
		{"single", []string{"single"}},
		{"", nil}, // Empty string returns nil
		{[]any{"a", "b", "c"}, []string{"a", "b", "c"}},
		{[]string{"x", "y"}, []string{"x", "y"}},
		{123, nil},                             // Non-string types return nil
		{[]any{123, "test"}, []string{"test"}}, // Non-strings in array are skipped
	}

	for _, tt := range tests {
		got := toStringSlice(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("toStringSlice(%v) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("toStringSlice(%v) = %v, want %v", tt.input, got, tt.want)
				break
			}
		}
	}
}

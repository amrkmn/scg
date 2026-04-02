package utils

import "testing"

func TestContainsFold(t *testing.T) {
	tests := []struct {
		s, substr string
		want      bool
	}{
		{"Hello World", "world", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "hello", true},
		{"Hello World", "lo wo", true},
		{"Hello World", "xyz", false},
		{"", "", true},
		{"Hello", "", true},
		{"", "x", false},
		{"JavaScript", "script", true},
		{"TypeScript", "typescript", true},
		{"GoLang", "lang", true},
	}

	for _, tt := range tests {
		got := ContainsFold(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("ContainsFold(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestContainsFoldLower(t *testing.T) {
	tests := []struct {
		s, lowerSubstr string
		want           bool
	}{
		{"Hello World", "world", true},
		{"Hello World", "hello", true},
		{"Hello World", "lo wo", true},
		{"Hello World", "xyz", false},
		{"", "", true},
		{"Hello", "", true},
		{"", "x", false},
		{"GoLang", "lang", true},
	}

	for _, tt := range tests {
		got := ContainsFoldLower(tt.s, tt.lowerSubstr)
		if got != tt.want {
			t.Errorf("ContainsFoldLower(%q, %q) = %v, want %v", tt.s, tt.lowerSubstr, got, tt.want)
		}
	}
}

func TestContainsFoldFast(t *testing.T) {
	tests := []struct {
		s, lowerSubstr string
		want           bool
	}{
		{"Hello World", "world", true},
		{"Hello World", "hello", true},
		{"HELLO WORLD", "hello", true},
		{"hello world", "world", true},
		{"Hello World", "lo wo", true},
		{"Hello World", "xyz", false},
		{"", "", true},
		{"Hello", "", true},
		{"", "x", false},
		{"GoLang", "lang", true},
		{"GOLANG", "lang", true},
		{"golang", "lang", true}, // lowerSubstr must be pre-lowered
	}

	for _, tt := range tests {
		got := ContainsFoldFast(tt.s, tt.lowerSubstr)
		if got != tt.want {
			t.Errorf("ContainsFoldFast(%q, %q) = %v, want %v", tt.s, tt.lowerSubstr, got, tt.want)
		}
	}
}

func TestContainsFoldFastPreLowered(t *testing.T) {
	tests := []struct {
		s, lowerSubstr string
		want           bool
	}{
		{"Hello World", "world", true},
		{"HELLO", "hello", true},
		{"GoLang", "gol", true},
		{"TypeScript", "script", true},
	}

	for _, tt := range tests {
		got := ContainsFoldFast(tt.s, tt.lowerSubstr)
		if got != tt.want {
			t.Errorf("ContainsFoldFast(%q, %q) = %v, want %v", tt.s, tt.lowerSubstr, got, tt.want)
		}
	}
}

// Benchmark to compare performance
func BenchmarkContainsFold(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ContainsFold("Hello World This Is A Test String", "test")
	}
}

func BenchmarkContainsFoldLower(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ContainsFoldLower("Hello World This Is A Test String", "test")
	}
}

func BenchmarkContainsFoldFast(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ContainsFoldFast("Hello World This Is A Test String", "test")
	}
}
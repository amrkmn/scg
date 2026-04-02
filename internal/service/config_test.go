package service

import "testing"

func TestCoerceValue(t *testing.T) {
	tests := []struct {
		input string
		want  any
	}{
		{"true", true},
		{"false", false},
		{"TRUE", true},   // case insensitive
		{"FALSE", false},
		{"null", nil},
		{"undefined", nil},
		{"123", int64(123)},
		{"-456", int64(-456)},
		{"3.14", 3.14},
		{"-2.5", -2.5},
		{"hello", "hello"},
		{"", ""},
		{"0", int64(0)},
		{"1.0", 1.0},
		{"9999999999", int64(9999999999)}, // Large int
	}

	for _, tt := range tests {
		got := CoerceValue(tt.input)
		if got != tt.want {
			t.Errorf("CoerceValue(%q) = %v (%T), want %v (%T)", tt.input, got, got, tt.want, tt.want)
		}
	}
}

func TestCoerceValueTypes(t *testing.T) {
	// Test that integer detection works correctly
	intResult := CoerceValue("42")
	if _, ok := intResult.(int64); !ok {
		t.Errorf("CoerceValue(\"42\") should be int64, got %T", intResult)
	}

	// Test that float detection works correctly
	floatResult := CoerceValue("3.14")
	if _, ok := floatResult.(float64); !ok {
		t.Errorf("CoerceValue(\"3.14\") should be float64, got %T", floatResult)
	}

	// Test that bool detection works correctly
	boolResult := CoerceValue("true")
	if _, ok := boolResult.(bool); !ok {
		t.Errorf("CoerceValue(\"true\") should be bool, got %T", boolResult)
	}

	// Test that string passthrough works
	stringResult := CoerceValue("not-a-number")
	if _, ok := stringResult.(string); !ok {
		t.Errorf("CoerceValue(\"not-a-number\") should be string, got %T", stringResult)
	}
}
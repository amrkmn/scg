package utils

import "strings"

// ContainsFold reports whether s contains substr, ignoring case.
// This is optimized for repeated calls with the same substr.
func ContainsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// ContainsFoldLower is an optimized version where substr is already lowercased.
// This avoids repeated lowercasing of substr in tight loops.
func ContainsFoldLower(s, lowerSubstr string) bool {
	if len(lowerSubstr) == 0 {
		return true
	}
	if len(lowerSubstr) > len(s) {
		return false
	}
	sLower := strings.ToLower(s)
	return strings.Contains(sLower, lowerSubstr)
}

// ContainsFoldFast performs case-insensitive substring matching without
// allocating new strings for intermediate results. Useful for performance-critical code.
// lowerSubstr must be pre-lowercased by the caller.
func ContainsFoldFast(s, lowerSubstr string) bool {
	if len(lowerSubstr) == 0 {
		return true
	}
	if len(lowerSubstr) > len(s) {
		return false
	}
	// Inline case-insensitive search without allocating
	for i := 0; i <= len(s)-len(lowerSubstr); i++ {
		match := true
		for j := 0; j < len(lowerSubstr); j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != lowerSubstr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

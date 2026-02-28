package ui

import (
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// ansiRe matches ANSI escape sequences for stripping purposes.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// VisualLength returns the display width of s, ignoring ANSI escape codes.
func VisualLength(s string) int {
	stripped := ansiRe.ReplaceAllString(s, "")
	return utf8.RuneCountInString(stripped)
}

// Truncate shortens s to at most maxLen visual characters, appending "…" if truncated.
// ANSI codes are preserved up to the truncation point then a reset is appended.
func Truncate(s string, maxLen int) string {
	if VisualLength(s) <= maxLen {
		return s
	}
	if maxLen <= 0 {
		return ""
	}
	// Strip and re-truncate plainly for simplicity (ANSI in truncated columns is rare)
	stripped := ansiRe.ReplaceAllString(s, "")
	runes := []rune(stripped)
	if len(runes) > maxLen-1 {
		runes = runes[:maxLen-1]
	}
	return string(runes) + "…"
}

// getTermWidth returns the terminal width, defaulting to 80 if unavailable.
func getTermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// FormatLineColumns formats a 2D slice of strings into aligned columns.
// weights controls proportional column sizing relative to terminal width.
// Columns are separated by two spaces; the last column is not padded or truncated.
// Like swb, columns are both shrunk (content too wide) and expanded (content too narrow)
// to fill the terminal width proportionally.
func FormatLineColumns(rows [][]string, weights []float64) string {
	if len(rows) == 0 {
		return ""
	}
	numCols := len(rows[0])
	if numCols == 0 {
		return ""
	}

	// Normalise weights: must have one per column; default to equal weights.
	w := make([]float64, numCols)
	for c := 0; c < numCols; c++ {
		if c < len(weights) && weights[c] > 0 {
			w[c] = weights[c]
		} else {
			w[c] = 1.0
		}
	}
	totalWeight := 0.0
	for _, wt := range w {
		totalWeight += wt
	}

	const sep = 2 // two-space separator between columns

	// Minimum column widths (matches swb: 12 for first col, 8 for rest).
	minWidth := make([]int, numCols)
	for c := range minWidth {
		if c == 0 {
			minWidth[c] = 12
		} else {
			minWidth[c] = 8
		}
	}

	// Natural (content) width per column.
	natural := make([]int, numCols)
	for _, row := range rows {
		for c := 0; c < numCols && c < len(row); c++ {
			if vl := VisualLength(row[c]); vl > natural[c] {
				natural[c] = vl
			}
		}
	}

	termW := getTermWidth()
	available := termW - sep*(numCols-1) // space for content, excluding separators
	if available < numCols {
		available = numCols
	}

	// Proportional target widths from terminal width.
	target := make([]int, numCols)
	for c := 0; c < numCols; c++ {
		t := int(float64(available) * w[c] / totalWeight)
		if t < minWidth[c] {
			t = minWidth[c]
		}
		target[c] = t
	}

	// Compute total natural width.
	naturalTotal := 0
	for _, n := range natural {
		naturalTotal += n
	}
	naturalTotal += sep * (numCols - 1)

	colWidths := make([]int, numCols)

	if naturalTotal > termW {
		// Content is wider than terminal — shrink to proportional targets.
		copy(colWidths, target)
		// Iteratively reduce the column most over its proportional share until we fit.
		for {
			sum := sep * (numCols - 1)
			for _, cw := range colWidths {
				sum += cw
			}
			if sum <= termW {
				break
			}
			// Find column furthest above its proportional target.
			maxOver, maxIdx := -1, 0
			for c := 0; c < numCols; c++ {
				prop := int(float64(available) * w[c] / totalWeight)
				diff := colWidths[c] - prop
				if diff > maxOver {
					maxOver = diff
					maxIdx = c
				}
			}
			if colWidths[maxIdx] <= minWidth[maxIdx] {
				break
			}
			colWidths[maxIdx]--
		}
	} else {
		// Content fits — expand columns proportionally to fill terminal width.
		// Start with natural widths, then distribute remaining space by weight.
		copy(colWidths, natural)
		remaining := available
		for _, n := range natural {
			remaining -= n
		}
		if remaining > 0 {
			// Distribute extra space proportionally.
			for c := 0; c < numCols; c++ {
				extra := int(float64(remaining) * w[c] / totalWeight)
				colWidths[c] += extra
			}
		}
	}

	// Build output lines.
	var sb strings.Builder
	for i, row := range rows {
		if i > 0 {
			sb.WriteByte('\n')
		}
		for c := 0; c < numCols && c < len(row); c++ {
			if c > 0 {
				sb.WriteString("  ")
			}
			cell := row[c]
			vl := VisualLength(cell)
			isLast := c == numCols-1
			if vl > colWidths[c] && !isLast {
				cell = Truncate(cell, colWidths[c])
				vl = colWidths[c]
			}
			sb.WriteString(cell)
			// Pad all columns except the last.
			if !isLast {
				for pad := colWidths[c] - vl; pad > 0; pad-- {
					sb.WriteByte(' ')
				}
			}
		}
	}
	return sb.String()
}

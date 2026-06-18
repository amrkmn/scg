package ui

import (
	"strings"
)

type KeyValue struct {
	Key   string
	Value string
}

func RenderTable(headers []string, rows [][]string, weights []float64, footer string) string {
	formattedRows := make([][]string, 0, len(rows)+1)
	if len(headers) > 0 {
		header := make([]string, 0, len(headers))
		for _, h := range headers {
			header = append(header, BoldCyan(h))
		}
		formattedRows = append(formattedRows, header)
	}
	formattedRows = append(formattedRows, rows...)

	var sb strings.Builder
	if len(formattedRows) > 0 {
		sb.WriteString(FormatLineColumns(formattedRows, weights))
	}
	if footer != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(Dim(footer))
	}
	return sb.String()
}

func RenderKeyValueBlock(title string, pairs []KeyValue) string {
	if len(pairs) == 0 {
		if title == "" {
			return ""
		}
		return Heading(title)
	}

	maxKeyWidth := 0
	for _, pair := range pairs {
		if width := VisualLength(pair.Key); width > maxKeyWidth {
			maxKeyWidth = width
		}
	}

	var sb strings.Builder
	if title != "" {
		sb.WriteString(Heading(title))
		sb.WriteByte('\n')
	}
	for i, pair := range pairs {
		if i > 0 {
			sb.WriteByte('\n')
		}
		key := PadRight(BoldCyan(pair.Key), maxKeyWidth)
		sb.WriteString(key)
		sb.WriteString(" : ")
		sb.WriteString(pair.Value)
	}
	return sb.String()
}

func RenderSummary(lines ...string) string {
	if len(lines) == 0 {
		return Heading("Summary")
	}

	var sb strings.Builder
	sb.WriteString(Heading("Summary"))
	for _, line := range lines {
		sb.WriteByte('\n')
		sb.WriteString(line)
	}
	return sb.String()
}

func RenderStatusSummary(kind StatusKind, subject, detail string) string {
	return RenderSummary(StatusWithOptions(kind, subject, detail, StatusOptions{}))
}

package ui

import "github.com/fatih/color"

// Color functions wrap fatih/color to return plain formatted strings.
// Windows ANSI support is handled automatically by fatih/color.

var (
	redFn       = color.New(color.FgRed).SprintFunc()
	greenFn     = color.New(color.FgGreen).SprintFunc()
	yellowFn    = color.New(color.FgYellow).SprintFunc()
	blueFn      = color.New(color.FgBlue).SprintFunc()
	cyanFn      = color.New(color.FgCyan).SprintFunc()
	magentaFn   = color.New(color.FgMagenta).SprintFunc()
	grayFn      = color.New(color.FgHiBlack).SprintFunc()
	whiteFn     = color.New(color.FgWhite).SprintFunc()
	boldFn      = color.New(color.Bold).SprintFunc()
	dimFn       = color.New(color.Faint).SprintFunc()
	underlineFn = color.New(color.Underline).SprintFunc()

	boldCyanFn  = color.New(color.Bold, color.FgCyan).SprintFunc()
	boldGreenFn = color.New(color.Bold, color.FgGreen).SprintFunc()
	dimGreenFn  = color.New(color.Faint, color.FgGreen).SprintFunc()
)

func Red(s string) string       { return redFn(s) }
func Green(s string) string     { return greenFn(s) }
func Yellow(s string) string    { return yellowFn(s) }
func Blue(s string) string      { return blueFn(s) }
func Cyan(s string) string      { return cyanFn(s) }
func Magenta(s string) string   { return magentaFn(s) }
func Gray(s string) string      { return grayFn(s) }
func White(s string) string     { return whiteFn(s) }
func Bold(s string) string      { return boldFn(s) }
func Dim(s string) string       { return dimFn(s) }
func Underline(s string) string { return underlineFn(s) }

func BoldCyan(s string) string  { return boldCyanFn(s) }
func BoldGreen(s string) string { return boldGreenFn(s) }
func DimGreen(s string) string  { return dimGreenFn(s) }

// Aliases matching swb naming conventions.
func Error(s string) string     { return Red(s) }
func Success(s string) string   { return Green(s) }
func Warning(s string) string   { return Yellow(s) }
func Info(s string) string      { return Blue(s) }
func Highlight(s string) string { return Cyan(s) }

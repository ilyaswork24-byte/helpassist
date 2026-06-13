// Package ui provides small ANSI helpers for colored terminal output.
package ui

import (
	"fmt"
	"os"
	"strings"
)

// ANSI color codes. Disabled automatically when output is not a terminal
// or when NO_COLOR is set.
var enabled = colorEnabled()

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	// Character device => a terminal.
	return fi.Mode()&os.ModeCharDevice != 0
}

func wrap(code, s string) string {
	if !enabled {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// Color helpers.
func Bold(s string) string   { return wrap("1", s) }
func Dim(s string) string    { return wrap("2", s) }
func Red(s string) string    { return wrap("31", s) }
func Green(s string) string  { return wrap("32", s) }
func Yellow(s string) string { return wrap("33", s) }
func Blue(s string) string   { return wrap("34", s) }
func Cyan(s string) string   { return wrap("36", s) }
func Gray(s string) string   { return wrap("90", s) }

// Tool prints a "running tool" marker, e.g.  ⏺ read_file(path: main.go)
func Tool(name, summary string) {
	dot := Green("⏺")
	if summary != "" {
		fmt.Printf("%s %s %s\n", dot, Bold(name), Gray("("+summary+")"))
	} else {
		fmt.Printf("%s %s\n", dot, Bold(name))
	}
}

// ToolResult prints an indented, truncated preview of a tool's output.
func ToolResult(out string) {
	out = strings.TrimRight(out, "\n")
	if out == "" {
		fmt.Println(Gray("  ⎿  (пусто)"))
		return
	}
	lines := strings.Split(out, "\n")
	const max = 8
	for i, ln := range lines {
		if i >= max {
			fmt.Println(Gray(fmt.Sprintf("  ⎿  … ещё %d строк", len(lines)-max)))
			break
		}
		prefix := "     "
		if i == 0 {
			prefix = "  ⎿  "
		}
		fmt.Println(Gray(prefix + ln))
	}
}

// Error prints an error line.
func Error(format string, a ...any) {
	fmt.Fprintln(os.Stderr, Red("✗ ")+fmt.Sprintf(format, a...))
}

// Info prints a dim informational line.
func Info(format string, a ...any) {
	fmt.Println(Gray(fmt.Sprintf(format, a...)))
}

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	blue   = "\033[34m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	cyan   = "\033[36m"
)

func color(code, value string) string {
	if !useColor() || value == "" {
		return value
	}
	return code + value + reset
}

func useColor() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("HBCTL_NO_COLOR") != "" {
		return false
	}
	return os.Getenv("TERM") != "dumb"
}

func Bold(value string) string   { return color(bold, value) }
func Dim(value string) string    { return color(dim, value) }
func Blue(value string) string   { return color(blue, value) }
func Green(value string) string  { return color(green, value) }
func Yellow(value string) string { return color(yellow, value) }
func Red(value string) string    { return color(red, value) }
func Cyan(value string) string   { return color(cyan, value) }

func Header(title string) { FHeader(os.Stdout, title) }

func FHeader(w io.Writer, title string) {
	line := strings.Repeat("─", visibleLen(title)+8)
	fmt.Fprintf(w, "\n%s\n", color(cyan, "╭"+line+"╮"))
	fmt.Fprintf(w, "%s  %s  %s\n", color(cyan, "│"), Bold(title), color(cyan, "│"))
	fmt.Fprintf(w, "%s\n", color(cyan, "╰"+line+"╯"))
}

func Section(title string) { FSection(os.Stdout, title) }

func FSection(w io.Writer, title string) {
	fmt.Fprintf(w, "\n%s %s\n", color(cyan, "━━"), Bold(title))
}

func Info(format string, args ...any) { FInfo(os.Stdout, format, args...) }

func FInfo(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "%s %s\n", color(blue, "INFO"), fmt.Sprintf(format, args...))
}

func Step(format string, args ...any) { FStep(os.Stdout, format, args...) }

func FStep(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "%s %s\n", color(cyan, "RUN "), fmt.Sprintf(format, args...))
}

func Success(format string, args ...any) { FSuccess(os.Stdout, format, args...) }

func FSuccess(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "%s %s\n", color(green, "OK  "), fmt.Sprintf(format, args...))
}

func Warn(format string, args ...any) { FWarn(os.Stdout, format, args...) }

func FWarn(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "%s %s\n", color(yellow, "WARN"), fmt.Sprintf(format, args...))
}

func Skip(format string, args ...any) { FSkip(os.Stdout, format, args...) }

func FSkip(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "%s %s\n", color(yellow, "SKIP"), fmt.Sprintf(format, args...))
}

func Error(format string, args ...any) { FError(os.Stderr, format, args...) }

func FError(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "%s %s\n", color(red, "ERR "), fmt.Sprintf(format, args...))
}

func Command(format string, args ...any) { FCommand(os.Stdout, format, args...) }

func FCommand(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "%s %s\n", color(dim, "$"), fmt.Sprintf(format, args...))
}

func KeyValues(values [][2]string) { FKeyValues(os.Stdout, values) }

func FKeyValues(w io.Writer, values [][2]string) {
	width := 0
	for _, pair := range values {
		if l := visibleLen(pair[0]); l > width {
			width = l
		}
	}
	for _, pair := range values {
		fmt.Fprintf(w, "  %s  %s\n", color(dim, padRight(pair[0], width)), pair[1])
	}
}

func Plan(title string, items []string) { FPlan(os.Stdout, title, items) }

func FPlan(w io.Writer, title string, items []string) {
	Section(title)
	for i, item := range items {
		fmt.Fprintf(w, "  %s %s\n", color(cyan, fmt.Sprintf("%2d.", i+1)), item)
	}
}

func Table(headers []string, rows [][]string) { FTable(os.Stdout, headers, rows) }

func FTable(w io.Writer, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = visibleLen(header)
	}
	for _, row := range rows {
		for i := 0; i < len(headers) && i < len(row); i++ {
			if l := visibleLen(row[i]); l > widths[i] {
				widths[i] = l
			}
		}
	}

	writeBorder := func(left, mid, right string) {
		fmt.Fprint(w, color(cyan, left))
		for i, width := range widths {
			fmt.Fprint(w, color(cyan, strings.Repeat("─", width+2)))
			if i == len(widths)-1 {
				fmt.Fprint(w, color(cyan, right))
			} else {
				fmt.Fprint(w, color(cyan, mid))
			}
		}
		fmt.Fprintln(w)
	}

	writeRow := func(values []string, header bool) {
		fmt.Fprint(w, color(cyan, "│"))
		for i := range headers {
			value := ""
			if i < len(values) {
				value = values[i]
			}
			if header {
				value = Bold(value)
			}
			fmt.Fprintf(w, " %s ", padRight(value, widths[i]))
			fmt.Fprint(w, color(cyan, "│"))
		}
		fmt.Fprintln(w)
	}

	writeBorder("╭", "┬", "╮")
	writeRow(headers, true)
	writeBorder("├", "┼", "┤")
	for _, row := range rows {
		writeRow(row, false)
	}
	writeBorder("╰", "┴", "╯")
}

func Bool(value bool) string {
	if value {
		return Green("true")
	}
	return Yellow("false")
}

func visibleLen(value string) int {
	// hbctl only uses ANSI decorations generated in this package. Strip simple CSI
	// sequences so padded pretty tables stay aligned when color is enabled.
	length := 0
	inEscape := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inEscape {
			if ch == 'm' {
				inEscape = false
			}
			continue
		}
		if ch == '\033' && i+1 < len(value) && value[i+1] == '[' {
			inEscape = true
			i++
			continue
		}
		length++
	}
	return length
}

func padRight(value string, width int) string {
	padding := width - visibleLen(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

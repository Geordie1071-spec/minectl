package cli

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// Terminal styling. Colour is only used on a terminal, and never with
// --no-color, NO_COLOR or TERM=dumb, so pipes, logs and scripts get the same
// plain text as before.

const (
	sgrBold   = "1"
	sgrDim    = "2"
	sgrAccent = "38;5;75" // the blue from the logo, readable on light and dark
	sgrGreen  = "32"
	sgrYellow = "33"
	sgrRed    = "31"
)

func colorFor(f *os.File) bool {
	if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// paint styles s for stdout.
func paint(sgr, s string) string { return paintFor(os.Stdout, sgr, s) }

func paintFor(f *os.File, sgr, s string) string {
	if s == "" || !colorFor(f) {
		return s
	}
	return "\033[" + sgr + "m" + s + "\033[0m"
}

func bold(s string) string   { return paint(sgrBold, s) }
func dim(s string) string    { return paint(sgrDim, s) }
func accent(s string) string { return paint(sgrAccent, s) }

// success prints a finished action with a green check.
func success(format string, a ...any) {
	if quiet {
		return
	}
	msg := fmt.Sprintf(format, a...)
	if colorFor(os.Stdout) {
		msg = paint(sgrGreen, "✓") + " " + msg
	}
	fmt.Println(msg)
}

// warnLabel and errorLabel prefix messages on stderr.
func warnLabel() string { return paintFor(os.Stderr, sgrYellow+";"+sgrBold, "Warning:") }

// FormatError styles a command's error for stderr.
func FormatError(err error) string {
	if !colorFor(os.Stderr) {
		return err.Error()
	}
	first, rest, _ := strings.Cut(err.Error(), "\n")
	out := paintFor(os.Stderr, sgrRed+";"+sgrBold, "✗ "+first)
	if rest != "" {
		out += "\n" + rest
	}
	return out
}

// statusDot is a server status with a coloured marker.
func statusDot(status string) string {
	sgr, mark := sgrDim, "○"
	switch status {
	case "running":
		sgr, mark = sgrGreen, "●"
	case "creating":
		sgr, mark = sgrYellow, "◐"
	case "error":
		sgr, mark = sgrRed, "●"
	}
	if !colorFor(os.Stdout) {
		return status
	}
	return paint(sgr, mark) + " " + status
}

// visibleLen is the width of s on screen, ignoring colour codes.
func visibleLen(s string) int {
	n, esc := 0, false
	for _, r := range s {
		switch {
		case r == '\033':
			esc = true
		case esc:
			if r == 'm' {
				esc = false
			}
		default:
			n++
		}
	}
	return n
}

// pad right-pads s to width visible characters.
func pad(s string, width int) string {
	if n := visibleLen(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// table lays out rows in columns, with a dim header row.
func table(header []string, rows [][]string) string {
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if n := visibleLen(c); n > widths[i] {
				widths[i] = n
			}
		}
	}
	var b strings.Builder
	line := func(cells []string, style func(string) string) {
		for i, c := range cells {
			if i < len(cells)-1 {
				c = pad(c, widths[i]+2)
			}
			b.WriteString(style(c))
		}
		b.WriteString("\n")
	}
	line(header, dim)
	for _, r := range rows {
		line(r, func(s string) string { return s })
	}
	return b.String()
}

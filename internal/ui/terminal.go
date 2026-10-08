package ui

import (
	"fmt"
	"io"
	"os"
)

// ANSI color escape sequences
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31;1m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

// UI manages CLI diagnostic output, standard output, and ANSI colorization.
type UI struct {
	Stdout     io.Writer
	Stderr     io.Writer
	Color      bool
	Quiet      bool
	Verbose    bool
	PrefixText string
}

// New creates a UI instance configured according to TTY detection and NO_COLOR rules.
func New(stdout, stderr io.Writer, noColorFlag, quietFlag, verboseFlag bool) *UI {
	colorEnabled := true

	if noColorFlag || os.Getenv("NO_COLOR") != "" {
		colorEnabled = false
	} else if file, ok := stderr.(*os.File); ok {
		stat, err := file.Stat()
		if err != nil || (stat.Mode()&os.ModeCharDevice) == 0 {
			colorEnabled = false
		}
	} else {
		colorEnabled = false
	}

	return &UI{
		Stdout:     stdout,
		Stderr:     stderr,
		Color:      colorEnabled,
		Quiet:      quietFlag,
		Verbose:    verboseFlag,
		PrefixText: "[git-brx]",
	}
}

// Log writes an informational message to stderr.
func (u *UI) Log(format string, args ...any) {
	if u.Quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(u.Stderr, "%s %s\n", u.PrefixText, msg)
}

// Warn writes a warning message to stderr.
func (u *UI) Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if u.Color {
		fmt.Fprintf(u.Stderr, "%s %sWarning: %s%s\n", u.PrefixText, colorYellow, msg, colorReset)
	} else {
		fmt.Fprintf(u.Stderr, "%s Warning: %s\n", u.PrefixText, msg)
	}
}

// Error writes an error message to stderr.
func (u *UI) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if u.Color {
		fmt.Fprintf(u.Stderr, "%s %s! %s%s\n", u.PrefixText, colorRed, msg, colorReset)
	} else {
		fmt.Fprintf(u.Stderr, "%s ! %s\n", u.PrefixText, msg)
	}
}

// Hint writes an actionable hint message to stderr.
func (u *UI) Hint(format string, args ...any) {
	if u.Quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if u.Color {
		fmt.Fprintf(u.Stderr, "%s %sHint: %s%s\n", u.PrefixText, colorCyan, msg, colorReset)
	} else {
		fmt.Fprintf(u.Stderr, "%s Hint: %s\n", u.PrefixText, msg)
	}
}

// Debug writes a verbose/debug diagnostic message to stderr.
func (u *UI) Debug(format string, args ...any) {
	if !u.Verbose {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(u.Stderr, "%s [debug] %s\n", u.PrefixText, msg)
}

// Out writes functional data to stdout followed by a newline.
func (u *UI) Out(format string, args ...any) {
	fmt.Fprintf(u.Stdout, format+"\n", args...)
}

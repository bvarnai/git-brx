package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
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
	Stdin      io.Reader
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
		Stdin:      os.Stdin,
		Stdout:     stdout,
		Stderr:     stderr,
		Color:      colorEnabled,
		Quiet:      quietFlag,
		Verbose:    verboseFlag,
		PrefixText: "[git-brx]",
	}
}

// SetStdin overrides standard input reader (e.g. for testing or automation).
func (u *UI) SetStdin(stdin io.Reader) {
	u.Stdin = stdin
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

// Confirm prompts the user for interactive confirmation [y/n] with up to 5 attempts.
func (u *UI) Confirm(prompt string) (bool, error) {
	if u.Stdin == nil {
		u.Stdin = os.Stdin
	}

	reader := bufio.NewReader(u.Stdin)
	for count := 1; count <= 5; count++ {
		fmt.Fprintf(u.Stderr, "%s: ", prompt)
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return false, err
		}
		trimmed := strings.TrimSpace(line)
		switch strings.ToLower(trimmed) {
		case "":
			// If prompt indicates [Y/n] (uppercase Y), default to true on empty Enter
			if strings.Contains(prompt, "[Y/n]") {
				return true, nil
			}
			if strings.Contains(prompt, "[y/N]") {
				return false, nil
			}
			u.Log("Please enter 'y' or 'n'")
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			u.Log("Sorry I don't understand, please try again")
		}
		if count == 5 {
			u.Log("Aborting after 5 tries")
			return false, nil
		}
	}
	return false, nil
}

// Hyperlink formats a URL as an OSC 8 clickable terminal hyperlink if color/styling is enabled.
// If text is empty, the URL itself is used as the link text.
func (u *UI) Hyperlink(url, text string) string {
	if text == "" {
		text = url
	}
	if !u.Color || url == "" {
		return text
	}
	// OSC 8 hyperlink escape sequence: \033]8;;URL\033\TEXT\033]8;;\033\
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

// Width returns the detected terminal width in columns, defaulting to 80 if undetermined.
func (u *UI) Width() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var c int
		if _, err := fmt.Sscanf(cols, "%d", &c); err == nil && c > 20 {
			return c
		}
	}
	return 80
}

// WrapText word-wraps text with hanging indentation to fit within maxCols.
// prefix is prepended to the first line.
// indent is prepended to all subsequent lines.
func (u *UI) WrapText(prefix, indent, text string, maxCols int) []string {
	if maxCols <= 0 {
		maxCols = u.Width()
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{prefix}
	}

	var lines []string
	current := prefix
	currMax := maxCols

	for _, word := range words {
		// If adding word exceeds currMax (and line isn't empty after prefix)
		spaceNeeded := 0
		if current != prefix && current != indent {
			spaceNeeded = 1
		}

		if len(current)+spaceNeeded+len(word) > currMax && (current != prefix && current != indent) {
			lines = append(lines, current)
			current = indent + word
		} else {
			if spaceNeeded > 0 {
				current += " " + word
			} else {
				current += word
			}
		}
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}

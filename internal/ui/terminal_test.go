package ui_test

import (
	"bytes"
	"testing"

	"github.com/bvarnai/git-brx/internal/ui"
	"github.com/stretchr/testify/assert"
)

func TestUIOutputProtocols(t *testing.T) {
	var stdout, stderr bytes.Buffer
	u := ui.New(&stdout, &stderr, true, false, false) // no color, not quiet

	u.Log("hello world")
	assert.Contains(t, stderr.String(), "[git-brx] hello world")
	assert.Empty(t, stdout.String())

	u.Error("something failed")
	assert.Contains(t, stderr.String(), "[git-brx] ! something failed")

	u.Hint("try this")
	assert.Contains(t, stderr.String(), "[git-brx] Hint: try this")

	u.Out("bare-output")
	assert.Equal(t, "bare-output\n", stdout.String())
}

func TestUIQuietMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	u := ui.New(&stdout, &stderr, true, true, false) // quiet = true

	u.Log("informational")
	u.Hint("actionable")
	assert.Empty(t, stderr.String())

	u.Error("fatal error")
	assert.Contains(t, stderr.String(), "[git-brx] ! fatal error")
}

func TestUIWrapText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	u := ui.New(&stdout, &stderr, true, false, false)

	// Short text fits in single line
	lines := u.WrapText("Prefix: ", "        ", "short text", 40)
	assert.Equal(t, []string{"Prefix: short text"}, lines)

	// Long text wraps with hanging indent
	longText := "This is a rather long issue title that definitely exceeds the limited column width"
	lines = u.WrapText("Issue #1: ", "          ", longText, 35)
	assert.Len(t, lines, 4)
	assert.Equal(t, "Issue #1: This is a rather long", lines[0])
	assert.Equal(t, "          issue title that", lines[1])
	assert.Equal(t, "          definitely exceeds the", lines[2])
	assert.Equal(t, "          limited column width", lines[3])
}

func TestUIHyperlink(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// No color mode: plain text
	uNoColor := ui.New(&stdout, &stderr, true, false, false)
	assert.Equal(t, "https://example.com", uNoColor.Hyperlink("https://example.com", ""))

	// Color enabled mode: OSC 8 escape sequences
	uColor := &ui.UI{Color: true}
	link := uColor.Hyperlink("https://example.com", "my link")
	assert.Equal(t, "\033]8;;https://example.com\033\\my link\033]8;;\033\\", link)
}

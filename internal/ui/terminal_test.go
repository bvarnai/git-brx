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

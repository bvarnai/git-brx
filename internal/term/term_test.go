package term_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/bvarnai/git-brx/internal/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveColorPolicy(t *testing.T) {
	tests := []struct {
		name            string
		noColorFlag     bool
		isTTY           bool
		env             map[string]string
		wantUseColor    bool
		wantInvalidWarn string
	}{
		{
			name:         "no color flag overrides all",
			noColorFlag:  true,
			isTTY:        true,
			env:          map[string]string{"GIT_BRX_COLOR": "always"},
			wantUseColor: false,
		},
		{
			name:         "GIT_BRX_COLOR always forces color on non-TTY",
			noColorFlag:  false,
			isTTY:        false,
			env:          map[string]string{"GIT_BRX_COLOR": "always"},
			wantUseColor: true,
		},
		{
			name:         "GIT_BRX_COLOR never disables color on TTY",
			noColorFlag:  false,
			isTTY:        true,
			env:          map[string]string{"GIT_BRX_COLOR": "never"},
			wantUseColor: false,
		},
		{
			name:         "NO_COLOR disables color on TTY in auto mode",
			noColorFlag:  false,
			isTTY:        true,
			env:          map[string]string{"NO_COLOR": "1"},
			wantUseColor: false,
		},
		{
			name:         "TERM=dumb disables color on TTY in auto mode",
			noColorFlag:  false,
			isTTY:        true,
			env:          map[string]string{"TERM": "dumb"},
			wantUseColor: false,
		},
		{
			name:            "invalid GIT_BRX_COLOR value triggers warning and falls back to auto",
			noColorFlag:     false,
			isTTY:           true,
			env:             map[string]string{"GIT_BRX_COLOR": "invalid-mode"},
			wantUseColor:    true,
			wantInvalidWarn: "invalid-mode",
		},
		{
			name:         "auto mode on non-TTY disables color",
			noColorFlag:  false,
			isTTY:        false,
			env:          nil,
			wantUseColor: false,
		},
		{
			name:         "auto mode on TTY enables color",
			noColorFlag:  false,
			isTTY:        true,
			env:          nil,
			wantUseColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envLookup := func(key string) (string, bool) {
				if tt.env == nil {
					return "", false
				}
				v, ok := tt.env[key]
				return v, ok
			}
			useColor, invalidWarn := term.ResolveColorPolicy(tt.noColorFlag, tt.isTTY, envLookup)
			assert.Equal(t, tt.wantUseColor, useColor)
			assert.Equal(t, tt.wantInvalidWarn, invalidWarn)
		})
	}
}

func TestLogger_Outputs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logger := term.New(term.Options{
		Stdout:      &stdout,
		Stderr:      &stderr,
		NoColorFlag: true,
		Quiet:       false,
		Verbose:     true,
	})

	logger.Info("info message: %s", "hello")
	logger.Debug("debug message: %d", 42)
	logger.Warn("warn message")
	logger.Hint("hint message")
	logger.Error("error message")
	logger.Out("stdout message: %s", "world")

	assert.Contains(t, stderr.String(), "[git-brx] info message: hello\n")
	assert.Contains(t, stderr.String(), "[git-brx] Debug: debug message: 42\n")
	assert.Contains(t, stderr.String(), "[git-brx] Warning: warn message\n")
	assert.Contains(t, stderr.String(), "[git-brx] Hint: hint message\n")
	assert.Contains(t, stderr.String(), "[git-brx] Error: error message\n")
	assert.Equal(t, "stdout message: world\n", stdout.String())
}

func TestLogger_Confirm(t *testing.T) {
	var stderr bytes.Buffer
	logger := term.New(term.Options{
		Stderr: &stderr,
	})

	// Yes response
	logger.SetStdin(strings.NewReader("y\n"))
	ok, err := logger.Confirm("Proceed [Y/n]?")
	require.NoError(t, err)
	assert.True(t, ok)

	// No response
	logger.SetStdin(strings.NewReader("n\n"))
	ok, err = logger.Confirm("Proceed [Y/n]?")
	require.NoError(t, err)
	assert.False(t, ok)

	// Default Enter on [Y/n]
	logger.SetStdin(strings.NewReader("\n"))
	ok, err = logger.Confirm("Proceed [Y/n]?")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestLogger_WrapText(t *testing.T) {
	var stderr bytes.Buffer
	logger := term.New(term.Options{
		Stderr: &stderr,
	})

	lines := logger.WrapText("Issue #101: ", "            ", "This is a long issue description that should wrap properly across lines", 40)
	assert.Greater(t, len(lines), 1)
	assert.True(t, strings.HasPrefix(lines[0], "Issue #101: "))
	assert.True(t, strings.HasPrefix(lines[1], "            "))
}

func TestLogger_LogError(t *testing.T) {
	var stderr bytes.Buffer
	logger := term.New(term.Options{
		Stderr:      &stderr,
		NoColorFlag: true,
		Quiet:       true,
	})

	logger.LogError(nil)
	assert.Empty(t, stderr.String())

	err := errcode.NewError(errcode.ErrConflict, "Merge conflict encountered").
		WithHint("Use 'git-brx resolve' to resolve conflicts")
	logger.LogError(err)

	assert.Contains(t, stderr.String(), "[git-brx] Error: Merge conflict encountered\n")
	assert.Contains(t, stderr.String(), "[git-brx] Hint: Use 'git-brx resolve' to resolve conflicts\n")
}

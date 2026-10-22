package git_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bvarnai/git-brx/internal/command"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/bvarnai/git-brx/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizedEnv(t *testing.T) {
	base := []string{
		"PATH=/bin:/usr/bin",
		"LC_ALL=en_US.UTF-8",
		"LANG=fr_FR.UTF-8",
		"GIT_TERMINAL_PROMPT=1",
		"GIT_ADVICE=1",
		"CUSTOM_VAR=value",
	}

	env := command.SanitizedEnv(base, []string{"EXTRA=1"})

	// Check overrides
	assert.Contains(t, env, "LC_ALL=C")
	assert.Contains(t, env, "LANG=C")
	assert.Contains(t, env, "GIT_TERMINAL_PROMPT=0")
	assert.Contains(t, env, "GIT_ADVICE=0")
	assert.Contains(t, env, "GIT_CONFIG_COUNT=1")
	assert.Contains(t, env, "GIT_CONFIG_KEY_0=gc.auto")
	assert.Contains(t, env, "GIT_CONFIG_VALUE_0=0")
	assert.Contains(t, env, "PATH=/bin:/usr/bin")
	assert.Contains(t, env, "CUSTOM_VAR=value")
	assert.Contains(t, env, "EXTRA=1")

	// Ensure old conflicting values were filtered
	assert.NotContains(t, env, "LC_ALL=en_US.UTF-8")
	assert.NotContains(t, env, "LANG=fr_FR.UTF-8")
	assert.NotContains(t, env, "GIT_TERMINAL_PROMPT=1")
	assert.NotContains(t, env, "GIT_ADVICE=1")
}

func TestExecRunner_RunAndCancel(t *testing.T) {
	runner := git.NewExecRunner()
	ctx := context.Background()

	// Simple version command
	out, err := runner.Run(ctx, "", "--version")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, "git version"))

	// Context cancellation
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = runner.Run(cancelCtx, "", "status")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errcode.ErrInterrupted))

	// RunLines
	lines, err := runner.RunLines(ctx, "", "--version")
	require.NoError(t, err)
	assert.Len(t, lines, 1)

	// RunLines on empty output
	emptyLines, err := runner.RunLines(ctx, "", "status", "--porcelain")
	require.NoError(t, err)
	// clean repo or empty porcelain returns nil
	if len(emptyLines) == 0 {
		assert.Nil(t, emptyLines)
	}

	// Timeout
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer timeoutCancel()
	time.Sleep(5 * time.Millisecond)
	_, err = runner.Run(timeoutCtx, "", "status")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errcode.ErrInterrupted))
}

func TestExecRunner_MissingBinary(t *testing.T) {
	runner := &git.ExecRunner{
		BinaryPath: "nonexistent-git-binary-xyz",
	}
	ctx := context.Background()
	_, err := runner.Run(ctx, "", "--version")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errcode.ErrTransport))
	assert.Contains(t, err.Error(), `Required binary "nonexistent-git-binary-xyz" not found in $PATH`)
}

package cli_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestCliRunSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx := context.Background()

	code := cli.Run(ctx, []string{"--help"}, &stdout, &stderr)
	assert.Equal(t, int(domain.ExitSuccess), code)
	assert.Contains(t, stdout.String(), "git-brx provides a simple, opinionated Git workflow")
	assert.Empty(t, stderr.String())
}

func TestCliRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx := context.Background()

	code := cli.Run(ctx, []string{"--version"}, &stdout, &stderr)
	assert.Equal(t, int(domain.ExitSuccess), code)
	assert.Contains(t, stdout.String(), "git-brx version")
	assert.Empty(t, stderr.String())
}

func TestCliRunUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx := context.Background()

	code := cli.Run(ctx, []string{"--unknown-flag"}, &stdout, &stderr)
	assert.Equal(t, int(domain.ExitUsageError), code)
	assert.Contains(t, stderr.String(), "[git-brx] Error: unknown flag: --unknown-flag")
}

func TestCliRunHelp(t *testing.T) {
	t.Run("top-level help command", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := cli.Run(context.Background(), []string{"help"}, &stdout, &stderr)
		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stdout.String(), "Available Commands:")
		assert.Contains(t, stdout.String(), "help")
		assert.Empty(t, stderr.String())
	})

	t.Run("subcommand help", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := cli.Run(context.Background(), []string{"help", "create"}, &stdout, &stderr)
		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stdout.String(), "git-brx create [flags] <branch>")
		assert.Empty(t, stderr.String())
	})
}


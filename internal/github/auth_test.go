package github_test

import (
	"context"
	"testing"

	"github.com/bvarnai/git-brx/internal/github"
	"github.com/stretchr/testify/assert"
)

func TestResolveToken_EnvVars(t *testing.T) {
	t.Run("GITHUB_TOKEN has precedence", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "token-from-github-env")
		t.Setenv("GH_TOKEN", "token-from-gh-env")

		token := github.ResolveToken(context.Background(), nil, "github.com")
		assert.Equal(t, "token-from-github-env", token)
	})

	t.Run("GH_TOKEN fallback", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "token-from-gh-env")

		token := github.ResolveToken(context.Background(), nil, "github.com")
		assert.Equal(t, "token-from-gh-env", token)
	})
}

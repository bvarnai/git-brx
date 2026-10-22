package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigIntegration(t *testing.T) {
	t.Run("ZeroConfigRemoteOriginAutoDiscovered", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("README.md", "# test", "initial commit")

		// Point remote to a mock github repo
		h.Git("remote", "add", "origin", "git@github.com:acme-org/test-service.git")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"config"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())

		out := stdout.String()
		assert.Contains(t, out, "# Source hierarchy:")
		assert.Contains(t, out, "Tier 4 (Remote origin [git remote origin]):")
		assert.Contains(t, out, "git@github.com:acme-org/test-service.git")
		assert.Contains(t, out, "platform: github # auto-discovered from remote origin")
		assert.Contains(t, out, "owner: acme-org # auto-discovered from remote origin")
		assert.Contains(t, out, "repo: test-service # auto-discovered from remote origin")

		// Verify round-trip parsing
		var parsed domain.ProjectConfig
		err := yaml.Unmarshal([]byte(out), &parsed)
		require.NoError(t, err)
		assert.Equal(t, "github", parsed.Platform)
		assert.Equal(t, "acme-org", parsed.SCM.Owner)
		assert.Equal(t, "test-service", parsed.SCM.Repo)
	})

	t.Run("SourcesFlagOnly", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("README.md", "# test", "initial commit")
		h.Git("remote", "add", "origin", "git@github.com:acme-org/test-service.git")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"config", "--sources"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())

		out := stdout.String()
		assert.Contains(t, out, "# Source hierarchy:")
		assert.Contains(t, out, "Tier 4 (Remote origin [git remote origin]):")
		assert.Contains(t, out, "git@github.com:acme-org/test-service.git")
		assert.NotContains(t, out, "platform:")
	})

	t.Run("CascadingWithGIT_BRX_CONFIGAndLocalRepo", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("README.md", "# test", "initial commit")
		h.Git("remote", "add", "origin", "git@github.com:acme-org/test-service.git")

		// Create company config
		companyDir := t.TempDir()
		companyFile := filepath.Join(companyDir, "company-rules.yaml")
		companyContent := `
branch:
  template: ^(story|task)/[0-9]+$
  mapping:
    story: story
`
		require.NoError(t, os.WriteFile(companyFile, []byte(companyContent), 0644))
		t.Setenv("GIT_BRX_CONFIG", companyFile)

		// Create repo config
		repoFile := filepath.Join(h.RepoDir, ".git-brx.yaml")
		repoContent := `
branch:
  mapping:
    spike: research
`
		require.NoError(t, os.WriteFile(repoFile, []byte(repoContent), 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"config"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code, "stderr: %s", stderr.String())

		out := stdout.String()
		assert.Contains(t, out, companyFile)
		assert.Contains(t, out, repoFile)

		var parsed domain.ProjectConfig
		err := yaml.Unmarshal([]byte(out), &parsed)
		require.NoError(t, err)
		assert.Equal(t, "^(story|task)/[0-9]+$", parsed.Branch.Template)
		assert.Equal(t, "story", parsed.Branch.Mapping["story"])
		assert.Equal(t, "research", parsed.Branch.Mapping["spike"])
	})
}

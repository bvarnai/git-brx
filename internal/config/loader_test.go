package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoader_ZeroConfigAutoInference(t *testing.T) {
	tempDir := t.TempDir()
	origin := "git@github.com:my-org/my-service.git"

	cfg, err := config.Load(tempDir, origin)
	require.NoError(t, err)
	assert.Equal(t, "github", cfg.Platform)
	assert.Equal(t, "github", cfg.Tracker.Provider)
	assert.Equal(t, "my-org", cfg.Tracker.Owner)
	assert.Equal(t, "my-service", cfg.Tracker.Repo)
	assert.Equal(t, "github", cfg.SCM.Provider)
	assert.Equal(t, "my-org", cfg.SCM.Owner)
	assert.Equal(t, "my-service", cfg.SCM.Repo)
	assert.Contains(t, cfg.Branch.Template, "issue")
}

func TestLoader_YAMLPlatformPresetWithOverrides(t *testing.T) {
	tempDir := t.TempDir()
	content := `
# Team configuration
platform: github
branch:
  template: ^(issue|feature)/[0-9]+$
review:
  mapping:
    frontend: [alice, bob]
    backend: charlie
    default: lead
`
	err := os.WriteFile(filepath.Join(tempDir, ".git-brx.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tempDir, "git@github.com:my-org/web.git")
	require.NoError(t, err)
	assert.Equal(t, "github", cfg.Platform)
	assert.Equal(t, "^(issue|feature)/[0-9]+$", cfg.Branch.Template)
	assert.Equal(t, "my-org", cfg.SCM.Owner)
	assert.Equal(t, "web", cfg.SCM.Repo)

	// Reviewer resolution
	reviewers := config.ResolveReviewers(&cfg.Review, []string{"frontend"})
	assert.Equal(t, []string{"alice", "bob"}, reviewers)

	reviewersBackend := config.ResolveReviewers(&cfg.Review, []string{"backend"})
	assert.Equal(t, []string{"charlie"}, reviewersBackend)

	reviewersDefault := config.ResolveReviewers(&cfg.Review, []string{"unknown"})
	assert.Equal(t, []string{"lead"}, reviewersDefault)
}

func TestLoader_SplitBackendsJiraAndBitbucket(t *testing.T) {
	tempDir := t.TempDir()
	content := `
# Enterprise setup
tracker:
  provider: jira
  uri: https://jira.company.com
  project: VSB

scm:
  provider: bitbucket
  uri: https://bitbucket.company.com
  project: VSB
  repo: core-app

branch:
  template: ^(issue|feature)/(${projectkey}-[0-9]+)$
`
	err := os.WriteFile(filepath.Join(tempDir, ".git-brx.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tempDir, "")
	require.NoError(t, err)
	assert.Equal(t, "jira", cfg.Tracker.Provider)
	assert.Equal(t, "https://jira.company.com", cfg.Tracker.URI)
	assert.Equal(t, "VSB", cfg.Tracker.Project)
	assert.Equal(t, "bitbucket", cfg.SCM.Provider)
	assert.Equal(t, "https://bitbucket.company.com", cfg.SCM.URI)
	assert.Equal(t, "VSB", cfg.SCM.Project)
	assert.Equal(t, "core-app", cfg.SCM.Repo)
	// Interpolated template
	assert.Equal(t, "^(issue|feature)/(VSB-[0-9]+)$", cfg.Branch.Template)
}

func TestLoader_EnvVarOverride(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "custom-config.yaml")
	content := `
platform: gitlab
`
	err := os.WriteFile(customPath, []byte(content), 0644)
	require.NoError(t, err)

	t.Setenv("GIT_BRX_CONFIG_PATH", customPath)

	cfg, err := config.Load(t.TempDir(), "git@gitlab.com:grp/proj.git")
	require.NoError(t, err)
	assert.Equal(t, "gitlab", cfg.Platform)
	assert.Equal(t, "gitlab", cfg.Tracker.Provider)
	assert.Equal(t, "gitlab", cfg.SCM.Provider)
}

func TestLoader_MissingConfigAndOriginFails(t *testing.T) {
	tempDir := t.TempDir()
	_, err := config.Load(tempDir, "")
	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ExitConfigError, appErr.Code)
	assert.Contains(t, appErr.Message, "No configuration file found")
}

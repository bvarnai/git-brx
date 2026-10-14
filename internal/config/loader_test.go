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
    labels:
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
	reviewers := config.ResolveReviewers(&cfg.Review.Mapping, domain.ReviewerResolutionOptions{Labels: []string{"frontend"}})
	assert.Equal(t, []string{"alice", "bob"}, reviewers)

	reviewersBackend := config.ResolveReviewers(&cfg.Review.Mapping, domain.ReviewerResolutionOptions{Labels: []string{"backend"}})
	assert.Equal(t, []string{"charlie"}, reviewersBackend)

	reviewersDefault := config.ResolveReviewers(&cfg.Review.Mapping, domain.ReviewerResolutionOptions{Labels: []string{"unknown"}})
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

	t.Setenv("GIT_BRX_CONFIG", customPath)

	cfg, err := config.Load(t.TempDir(), "git@gitlab.com:grp/proj.git")
	require.NoError(t, err)
	assert.Equal(t, "gitlab", cfg.Platform)
	assert.Equal(t, "gitlab", cfg.Tracker.Provider)
	assert.Equal(t, "gitlab", cfg.SCM.Provider)
}

func TestLoader_InvalidEnvVarConfigFails(t *testing.T) {
	t.Setenv("GIT_BRX_CONFIG", "/nonexistent/path/to/config.yaml")
	_, err := config.Load(t.TempDir(), "")
	require.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ExitConfigError, appErr.Code)
	assert.Contains(t, appErr.Message, "configuration file specified by GIT_BRX_CONFIG not found")
}

func TestLoader_LayeredCascadeUserCompanyRepo(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Tier 1: User Global (~/.config/git-brx/config.yaml)
	userConfigDir := filepath.Join(fakeHome, ".config", "git-brx")
	require.NoError(t, os.MkdirAll(userConfigDir, 0755))
	userConfigContent := `
platform: github
branch:
  template: ^(issue|feature)/[0-9]+$
  mapping:
    bug: issue
    task: issue
review:
  mapping:
    default: [global-lead]
`
	require.NoError(t, os.WriteFile(filepath.Join(userConfigDir, "config.yaml"), []byte(userConfigContent), 0644))

	// Tier 2: Company Global via GIT_BRX_CONFIG
	companyConfigDir := t.TempDir()
	companyConfigFile := filepath.Join(companyConfigDir, "company-rules.yaml")
	companyConfigContent := `
branch:
  template: ^(story|chore|epic)/[0-9]+$
  mapping:
    story: story
    task: chore
review:
  mapping:
    labels:
      security: [sec-team]
`
	require.NoError(t, os.WriteFile(companyConfigFile, []byte(companyConfigContent), 0644))
	t.Setenv("GIT_BRX_CONFIG", companyConfigFile)

	// Tier 3: Repo Local (.git-brx.yaml)
	repoDir := t.TempDir()
	repoConfigContent := `
branch:
  mapping:
    spike: research
review:
  merge:
    strategy: squash
`
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".git-brx.yaml"), []byte(repoConfigContent), 0644))

	// Load configuration with remote origin for Tier 4 auto-discovery
	cfg, err := config.Load(repoDir, "git@github.com:my-company/service-x.git")
	require.NoError(t, err)

	// Tier 1 Base preserved:
	assert.Equal(t, "github", cfg.Platform)
	assert.Equal(t, []any{"global-lead"}, cfg.Review.Mapping.Default)
	assert.Equal(t, "issue", cfg.Branch.Mapping["bug"])

	// Tier 2 Company Override applied:
	assert.Equal(t, "^(story|chore|epic)/[0-9]+$", cfg.Branch.Template)
	assert.Equal(t, "story", cfg.Branch.Mapping["story"])
	assert.Equal(t, "chore", cfg.Branch.Mapping["task"]) // company overrode user global's task: issue
	assert.NotNil(t, cfg.Review.Mapping.Labels["security"])

	// Tier 3 Repo Override applied:
	assert.Equal(t, "research", cfg.Branch.Mapping["spike"]) // repo added spike
	assert.Equal(t, "squash", cfg.Review.Merge.Strategy)

	// Tier 4 Auto-Discovery applied:
	assert.Equal(t, "my-company", cfg.SCM.Owner)
	assert.Equal(t, "service-x", cfg.SCM.Repo)
}

func TestLoader_UserGlobalDotfileFallback(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Test ~/.git-brx.yaml fallback
	dotfileContent := `
platform: gitlab
`
	require.NoError(t, os.WriteFile(filepath.Join(fakeHome, ".git-brx.yaml"), []byte(dotfileContent), 0644))

	repoDir := t.TempDir()
	cfg, err := config.Load(repoDir, "git@gitlab.com:org/app.git")
	require.NoError(t, err)
	assert.Equal(t, "gitlab", cfg.Platform)
	assert.Equal(t, "gitlab", cfg.Tracker.Provider)
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

func TestLoader_PartialConfigAutoInfersPlatformFromRemote(t *testing.T) {
	tempDir := t.TempDir()
	content := `
branch:
  mapping:
    enhancement: "issue"
`
	err := os.WriteFile(filepath.Join(tempDir, ".git-brx.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tempDir, "git@github.com:my-org/my-service.git")
	require.NoError(t, err)
	assert.Equal(t, "github", cfg.Platform)
	assert.Equal(t, "github", cfg.Tracker.Provider)
	assert.Equal(t, "my-org", cfg.Tracker.Owner)
	assert.Equal(t, "my-service", cfg.Tracker.Repo)
	assert.Equal(t, "github", cfg.SCM.Provider)
	assert.Equal(t, "my-org", cfg.SCM.Owner)
	assert.Equal(t, "my-service", cfg.SCM.Repo)
	assert.Equal(t, "issue", cfg.Branch.Mapping["enhancement"])
}

func TestLoader_ExplicitPlatformNonePreserved(t *testing.T) {
	tempDir := t.TempDir()
	content := `
platform: none
branch:
  mapping:
    enhancement: "issue"
`
	err := os.WriteFile(filepath.Join(tempDir, ".git-brx.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tempDir, "git@github.com:my-org/my-service.git")
	require.NoError(t, err)
	assert.Equal(t, "none", cfg.Platform)
	assert.Equal(t, "", cfg.Tracker.Provider)
	assert.Equal(t, "", cfg.Tracker.Owner)
	assert.Equal(t, "", cfg.Tracker.Repo)
	assert.Equal(t, "", cfg.SCM.Provider)
	assert.Equal(t, "", cfg.SCM.Owner)
	assert.Equal(t, "", cfg.SCM.Repo)
	assert.Equal(t, "issue", cfg.Branch.Mapping["enhancement"])
}

func TestLoader_PartialConfigWithoutRemoteOriginSucceeds(t *testing.T) {
	tempDir := t.TempDir()
	content := `
branch:
  template: ^(issue|feature)/[0-9]+$
`
	err := os.WriteFile(filepath.Join(tempDir, ".git-brx.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tempDir, "")
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Platform)
	assert.Equal(t, "", cfg.Tracker.Provider)
	assert.Equal(t, "", cfg.SCM.Provider)
	assert.Equal(t, "^(issue|feature)/[0-9]+$", cfg.Branch.Template)
}

func TestLoader_ReviewMergeConfig(t *testing.T) {
	tempDir := t.TempDir()
	content := `
platform: github
review:
  merge:
    instructions: false
    strategy: rebase
    strategies:
      sources:
        "epic/*": merge
      targets:
        "release/*": merge
`
	err := os.WriteFile(filepath.Join(tempDir, ".git-brx.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tempDir, "git@github.com:my-org/web.git")
	require.NoError(t, err)
	require.NotNil(t, cfg.Review.Merge.Instructions)
	assert.False(t, *cfg.Review.Merge.Instructions)
	assert.Equal(t, "rebase", cfg.Review.Merge.Strategy)
	assert.Equal(t, "merge", cfg.Review.Merge.Strategies.Sources["epic/*"])
	assert.Equal(t, "merge", cfg.Review.Merge.Strategies.Targets["release/*"])
}

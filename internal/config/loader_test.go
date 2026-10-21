package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/config"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
  template: ^(issue|feature)/(VSB-[0-9]+)$
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
	if resolved, err := filepath.EvalSymlinks(fakeHome); err == nil {
		fakeHome = resolved
	}
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

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
	if resolved, err := filepath.EvalSymlinks(fakeHome); err == nil {
		fakeHome = resolved
	}
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

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

func TestInspect_ZeroConfigAutoInferredComments(t *testing.T) {
	tempDir := t.TempDir()
	origin := "git@github.com:my-org/my-service.git"

	report, err := config.Inspect(tempDir, origin)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "auto-discovered from remote origin", report.InferredFields["platform"])
	assert.Equal(t, "inferred from platform preset", report.InferredFields["tracker.provider"])
	assert.Equal(t, "auto-discovered from remote origin", report.InferredFields["tracker.owner"])
	assert.Equal(t, "auto-discovered from remote origin", report.InferredFields["tracker.repo"])
	assert.Equal(t, "inferred from platform preset", report.InferredFields["scm.provider"])
	assert.Equal(t, "auto-discovered from remote origin", report.InferredFields["scm.owner"])
	assert.Equal(t, "auto-discovered from remote origin", report.InferredFields["scm.repo"])
	assert.Equal(t, "platform default (github)", report.InferredFields["branch.template"])
	assert.Equal(t, "platform default (github)", report.InferredFields["branch.mapping"])

	output, err := config.FormatReport(report, false)
	require.NoError(t, err)

	// Check headers
	assert.Contains(t, output, "# Source hierarchy:")
	assert.Contains(t, output, "Tier 1 (User global [~/.config/git-brx/config.yaml, ~/.git-brx.yaml]):")
	assert.Contains(t, output, "Tier 2 (Shared global [$GIT_BRX_CONFIG]):")
	assert.Contains(t, output, "Tier 3 (Repo local [.git-brx.yaml, .git-brx/config.yaml]):")
	assert.Contains(t, output, "Tier 4 (Remote origin [git remote origin]):")
	assert.Contains(t, output, "git@github.com:my-org/my-service.git")

	// Check annotations
	assert.Contains(t, output, "platform: github # auto-discovered from remote origin")
	assert.Contains(t, output, "provider: github # inferred from platform preset")
	assert.Contains(t, output, "owner: my-org # auto-discovered from remote origin")
	assert.Contains(t, output, "repo: my-service # auto-discovered from remote origin")
	assert.Contains(t, output, "template: ^(issue|feature|epic)/[1-9][0-9]*$ # platform default (github)")
	assert.Contains(t, output, "mapping: # platform default (github)")
	assert.Contains(t, output, "bug: issue")
	assert.Contains(t, output, "feature: feature")
	assert.Contains(t, output, "epic: epic")

	// Verify valid YAML round-trip
	var parsed domain.ProjectConfig
	err = yaml.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "github", parsed.Platform)
	assert.Equal(t, "^(issue|feature|epic)/[1-9][0-9]*$", parsed.Branch.Template)
	assert.Equal(t, "issue", parsed.Branch.Mapping["bug"])
	assert.Equal(t, []string{"issue", "feature"}, config.ParseBranchTargets(parsed.Branch.Mapping["enhancement"]))
	assert.Equal(t, "feature", parsed.Branch.Mapping["feature"])
	assert.Equal(t, "epic", parsed.Branch.Mapping["epic"])
	assert.Equal(t, "my-org", parsed.SCM.Owner)
	assert.Equal(t, "my-service", parsed.SCM.Repo)
}

func TestInspect_SourcesOnly(t *testing.T) {
	fakeHome := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(fakeHome); err == nil {
		fakeHome = resolved
	}
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)
	userConfigDir := filepath.Join(fakeHome, ".config", "git-brx")
	require.NoError(t, os.MkdirAll(userConfigDir, 0755))
	userConfigFile := filepath.Join(userConfigDir, "config.yaml")
	require.NoError(t, os.WriteFile(userConfigFile, []byte("platform: github\n"), 0644))

	companyConfigDir := t.TempDir()
	companyConfigFile := filepath.Join(companyConfigDir, "company.yaml")
	require.NoError(t, os.WriteFile(companyConfigFile, []byte("branch:\n  template: custom\n"), 0644))
	t.Setenv("GIT_BRX_CONFIG", companyConfigFile)

	repoDir := t.TempDir()
	repoConfigFile := filepath.Join(repoDir, ".git-brx.yaml")
	require.NoError(t, os.WriteFile(repoConfigFile, []byte("review:\n  template: pr.md\n"), 0644))

	report, err := config.Inspect(repoDir, "git@github.com:acme/core.git")
	require.NoError(t, err)
	assert.True(t, filepath.Clean(userConfigFile) == filepath.Clean(report.UserConfigPath) || strings.EqualFold(filepath.Clean(userConfigFile), filepath.Clean(report.UserConfigPath)))
	assert.True(t, filepath.Clean(companyConfigFile) == filepath.Clean(report.SharedConfigPath) || strings.EqualFold(filepath.Clean(companyConfigFile), filepath.Clean(report.SharedConfigPath)))
	assert.True(t, filepath.Clean(repoConfigFile) == filepath.Clean(report.RepoConfigPath) || strings.EqualFold(filepath.Clean(repoConfigFile), filepath.Clean(report.RepoConfigPath)))
	assert.Equal(t, "git@github.com:acme/core.git", report.OriginURL)

	sourcesOutput, err := config.FormatReport(report, true)
	require.NoError(t, err)
	assert.Contains(t, sourcesOutput, "# Source hierarchy:")
	assert.Contains(t, sourcesOutput, userConfigFile)
	assert.Contains(t, sourcesOutput, companyConfigFile)
	assert.Contains(t, sourcesOutput, repoConfigFile)
	assert.Contains(t, sourcesOutput, "git@github.com:acme/core.git")
	assert.NotContains(t, sourcesOutput, "platform:")
}

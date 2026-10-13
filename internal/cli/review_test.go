package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
		expectStderr  string
	}{
		{
			name:         "excess positional arguments returns usage error",
			args:         []string{"review", "target1", "target2"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: target2",
		},
		{
			name:         "conflicting merge-instructions flags returns usage error",
			args:         []string{"review", "--merge-instructions", "--no-merge-instructions"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Cannot specify both --merge-instructions and --no-merge-instructions",
		},
		{
			name:         "invalid strategy flag returns usage error",
			args:         []string{"review", "--strategy", "invalid-strat"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unsupported merge strategy: invalid-strat",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"review"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Not a git repository",
		},
		{
			name: "unborn repo without commits returns code 3",
			args: []string{"review"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {err: errors.New("exit status 1")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Repository has no commits yet",
		},
		{
			name: "missing origin remote returns code 3",
			args: []string{"review"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				"remote get-url origin":           {err: errors.New("remote origin not found")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Origin remote is not configured",
		},
		{
			name: "detached HEAD returns code 3",
			args: []string{"review"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				"remote get-url origin":           {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":    {err: errors.New("detached")},
				"rev-parse --short HEAD":          {out: "abc1234"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Cannot create review in detached HEAD state",
		},
		{
			name: "master branch cannot be reviewed",
			args: []string{"review"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                           {out: "true"},
				"rev-parse --show-toplevel":                                 {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                {out: "abc1234"},
				"remote get-url origin":                                     {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                              {out: "master"},
				"rev-parse --short HEAD":                                    {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/master": {out: "origin/master"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "You must be on an 'issue', 'feature', or 'epic' branch to create a review",
		},
		{
			name: "unpublished branch returns code 4 with publish hint",
			args: []string{"review"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                  {out: "true"},
				"rev-parse --show-toplevel":                                        {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                       {out: "abc1234"},
				"remote get-url origin":                                            {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                                     {out: "issue/VSB-101"},
				"rev-parse --short HEAD":                                           {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-101": {out: "origin/issue/VSB-101"},
				"ls-remote --heads origin issue/VSB-101":                           {out: ""},
			},
			expectedCode: int(domain.ExitPreconditionRemote),
			errSubstr:    "Branch 'issue/VSB-101' has not been pushed to remote origin",
		},
		{
			name: "dry run formats planned review without API calls",
			args: []string{"review", "--dry-run"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                             {out: "true"},
				"rev-parse --show-toplevel":                                   {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                  {out: "abc1234"},
				"remote get-url origin":                                       {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                                {out: "issue/42"},
				"rev-parse --short HEAD":                                      {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/42": {out: "origin/issue/42"},
				"ls-remote --heads origin issue/42":                           {out: "abc1234 refs/heads/issue/42\n"},
				"show-ref --verify --quiet refs/heads/master":                 {out: ""},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStderr: "Would create pull request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			runner := &mockGitRunner{responses: tt.mockResponses}
			app := cli.NewApp(&stdout, &stderr, runner)
			app.RootCmd.SetArgs(tt.args)

			err := app.Execute(context.Background())
			if tt.expectedCode == int(domain.ExitSuccess) {
				require.NoError(t, err)
				if tt.expectStderr != "" {
					assert.Contains(t, stderr.String(), tt.expectStderr)
				}
			} else {
				require.Error(t, err)
				var appErr *domain.AppError
				require.True(t, errors.As(err, &appErr), "expected *domain.AppError, got %T: %v", err, err)
				assert.Equal(t, tt.expectedCode, int(appErr.Code))
				if tt.errSubstr != "" {
					assert.Contains(t, appErr.Message, tt.errSubstr)
				}
			}
		})
	}
}

func TestComposeReviewDescription(t *testing.T) {
	app := cli.NewApp(nil, nil, nil)
	scm := &mockSCMProvider{}

	t.Run("scm formats closing issue in default description", func(t *testing.T) {
		cfg := &domain.ProjectConfig{}
		desc := app.ComposeReviewDescription(cfg, scm, "8", "Feature summary", "feature/8", "main")
		assert.Contains(t, desc, "Closes #8")
		assert.Contains(t, desc, "Feature summary")
	})

	t.Run("scm formats closing issue in template", func(t *testing.T) {
		cfg := &domain.ProjectConfig{
			Review: domain.ReviewConfig{
				Template: "## Custom Template\nSome details here.",
			},
		}
		desc := app.ComposeReviewDescription(cfg, scm, "12", "Details", "feature/12", "main")
		assert.Contains(t, desc, "## Custom Template")
		assert.Contains(t, desc, "Closes #12")
	})

	t.Run("scm returning empty string does not inject closing line", func(t *testing.T) {
		emptySCM := &mockSCMProvider{emptyClosing: true}
		cfg := &domain.ProjectConfig{}
		desc := app.ComposeReviewDescription(cfg, emptySCM, "12", "Details", "feature/12", "main")
		assert.False(t, strings.Contains(desc, "Closes #"))
	})

	t.Run("nil scm does not panic and formats default instructions", func(t *testing.T) {
		cfg := &domain.ProjectConfig{}
		desc := app.ComposeReviewDescription(cfg, nil, "12", "Details", "feature/12", "main")
		assert.Contains(t, desc, "Details")
		assert.Contains(t, desc, "# Merge instructions")
	})

	t.Run("merge_instructions false suppresses merge instructions", func(t *testing.T) {
		f := false
		cfg := &domain.ProjectConfig{
			Review: domain.ReviewConfig{
				MergeInstructions: &f,
			},
		}
		desc := app.ComposeReviewDescription(cfg, scm, "12", "Details", "feature/12", "main")
		assert.Contains(t, desc, "Details")
		assert.False(t, strings.Contains(desc, "# Merge instructions"))
	})

	t.Run("legacy instructions false suppresses merge instructions", func(t *testing.T) {
		f := false
		cfg := &domain.ProjectConfig{
			Review: domain.ReviewConfig{
				Instructions: &f,
			},
		}
		desc := app.ComposeReviewDescription(cfg, scm, "12", "Details", "feature/12", "main")
		assert.Contains(t, desc, "Details")
		assert.False(t, strings.Contains(desc, "# Merge instructions"))
	})

	t.Run("epic branch produces merge commit instructions by convention", func(t *testing.T) {
		cfg := &domain.ProjectConfig{}
		desc := app.ComposeReviewDescription(cfg, scm, "1", "Major Epic", "epic/1", "main")
		assert.Contains(t, desc, "Create a merge commit (`--no-ff`)")
	})

	t.Run("targeting epic branch produces merge commit instructions by convention", func(t *testing.T) {
		cfg := &domain.ProjectConfig{}
		desc := app.ComposeReviewDescription(cfg, scm, "10", "Sub task", "feature/10", "epic/1")
		assert.Contains(t, desc, "Create a merge commit (`--no-ff`)")
	})

	t.Run("explicit strategy override produces rebase instructions", func(t *testing.T) {
		cfg := &domain.ProjectConfig{}
		desc := app.ComposeReviewDescription(cfg, scm, "10", "Sub task", "feature/10", "main", "rebase")
		assert.Contains(t, desc, "Rebase & Merge")
	})
}

func TestComposePullRequestTitle(t *testing.T) {
	tests := []struct {
		name        string
		customTitle string
		branch      string
		issueKey    string
		issueTitle  string
		expected    string
	}{
		{
			name:        "custom title takes precedence",
			customTitle: "Explicit PR Title",
			branch:      "feature/12",
			issueKey:    "12",
			issueTitle:  "feat(review): something",
			expected:    "Explicit PR Title",
		},
		{
			name:        "empty issue title falls back to branch",
			customTitle: "",
			branch:      "feature/12",
			issueKey:    "12",
			issueTitle:  "",
			expected:    "feature/12",
		},
		{
			name:        "conventional commit title with numeric key replaces scope with #key",
			customTitle: "",
			branch:      "feature/12",
			issueKey:    "12",
			issueTitle:  "feat(review): align PR title",
			expected:    "feat(#12): align PR title",
		},
		{
			name:        "conventional commit title without scope injects #key",
			customTitle: "",
			branch:      "feature/12",
			issueKey:    "12",
			issueTitle:  "fix: button alignment",
			expected:    "fix(#12): button alignment",
		},
		{
			name:        "conventional commit title with Jira alphanumeric key injects key as scope",
			customTitle: "",
			branch:      "feature/VSB-101",
			issueKey:    "VSB-101",
			issueTitle:  "feat(auth): support OAuth2",
			expected:    "feat(VSB-101): support OAuth2",
		},
		{
			name:        "plain issue title on feature branch infers feat(#key)",
			customTitle: "",
			branch:      "feature/12",
			issueKey:    "12",
			issueTitle:  "Add new dashboard",
			expected:    "feat(#12): Add new dashboard",
		},
		{
			name:        "plain issue title on issue branch infers fix(#key)",
			customTitle: "",
			branch:      "issue/12",
			issueKey:    "12",
			issueTitle:  "Broken login redirect",
			expected:    "fix(#12): Broken login redirect",
		},
		{
			name:        "plain issue title on custom branch uses #key: title",
			customTitle: "",
			branch:      "custom/12",
			issueKey:    "12",
			issueTitle:  "Some custom work",
			expected:    "#12: Some custom work",
		},
		{
			name:        "unlinked branch with conventional title keeps title",
			customTitle: "",
			branch:      "feature/adhoc",
			issueKey:    "",
			issueTitle:  "docs: update readme",
			expected:    "docs: update readme",
		},
		{
			name:        "unlinked branch with plain title prefixes branch",
			customTitle: "",
			branch:      "feature/adhoc",
			issueKey:    "",
			issueTitle:  "update readme",
			expected:    "feature/adhoc: update readme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := cli.ComposePullRequestTitle(tt.customTitle, tt.branch, tt.issueKey, tt.issueTitle)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

type mockSCMProvider struct {
	emptyClosing bool
}

func (m *mockSCMProvider) Name() string { return "mock" }
func (m *mockSCMProvider) CreatePullRequest(ctx context.Context, req domain.PullRequestRequest) (*domain.PullRequestResult, error) {
	return nil, nil
}
func (m *mockSCMProvider) GetPullRequestForBranch(ctx context.Context, branch string) (*domain.PullRequestDetail, error) {
	return nil, nil
}
func (m *mockSCMProvider) ResolveReviewers(components []string) []string { return nil }
func (m *mockSCMProvider) FormatMergeInstructions(opts domain.MergeInstructionOptions) string {
	return domain.FormatDefaultMergeInstructions(opts)
}
func (m *mockSCMProvider) FormatClosingIssue(issueKey string) string {
	if m.emptyClosing || issueKey == "" {
		return ""
	}
	return "Closes #" + strings.TrimPrefix(issueKey, "#")
}

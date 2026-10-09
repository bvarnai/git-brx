package cli_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
		expectStdout  string
		expectStderr  string
	}{
		{
			name:         "excess arguments returns usage error",
			args:         []string{"history", "path1", "path2"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: path2",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"history"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "This is not a git repository",
		},
		{
			name: "unborn repo without commits returns code 3",
			args: []string{"history"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {err: errors.New("exit status 1")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Repository has no commits yet",
		},
		{
			name: "clean history execution streams formatted graph",
			args: []string{"history", "--no-color"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				`log --pretty=format:%h %ad | %s%d [%an] --graph --decorate --date=short --no-color`: {
					out: "* abc1234 2026-10-09 | commit message (HEAD -> master) [Test User]",
				},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "* abc1234 2026-10-09 | commit message (HEAD -> master) [Test User]\n",
		},
		{
			name: "max-count flag appends limit to git log",
			args: []string{"history", "-n", "5", "--no-color"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				`log --pretty=format:%h %ad | %s%d [%an] --graph --decorate --date=short --max-count=5 --no-color`: {
					out: "* abc1234 2026-10-09 | commit message [Test User]",
				},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "* abc1234 2026-10-09 | commit message [Test User]\n",
		},
		{
			name: "limit flag alias appends limit to git log",
			args: []string{"history", "--limit", "3", "--no-color"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				`log --pretty=format:%h %ad | %s%d [%an] --graph --decorate --date=short --max-count=3 --no-color`: {
					out: "* abc1234 2026-10-09 | commit message [Test User]",
				},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "* abc1234 2026-10-09 | commit message [Test User]\n",
		},
		{
			name: "search flag filters commits by keyword",
			args: []string{"history", "-s", "VSB-101", "--no-color"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				`log --pretty=format:%h %ad | %s%d [%an] --graph --decorate --date=short --grep=VSB-101 -i --no-color`: {
					out: "* abc1234 2026-10-09 | VSB-101 fix auth issue [Test User]",
				},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "* abc1234 2026-10-09 | VSB-101 fix auth issue [Test User]\n",
		},
		{
			name: "stat flag appends --stat to git log",
			args: []string{"history", "--stat", "--no-color"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				`log --pretty=format:%h %ad | %s%d [%an] --graph --decorate --date=short --stat --no-color`: {
					out: "* abc1234 2026-10-09 | commit message [Test User]\n 1 file changed, 1 insertion(+)",
				},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "* abc1234 2026-10-09 | commit message [Test User]\n 1 file changed, 1 insertion(+)\n",
		},
		{
			name: "patch flag appends -p to git log",
			args: []string{"history", "-p", "--no-color"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				`log --pretty=format:%h %ad | %s%d [%an] --graph --decorate --date=short -p --no-color`: {
					out: "* abc1234 2026-10-09 | commit message [Test User]\ndiff --git a/f b/f",
				},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "* abc1234 2026-10-09 | commit message [Test User]\ndiff --git a/f b/f\n",
		},
		{
			name: "branch flag filters to current branch delta vs master",
			args: []string{"history", "-b", "--no-color"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                  {out: "true"},
				"rev-parse --show-toplevel":                                        {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                       {out: "abc1234"},
				"symbolic-ref --short -q HEAD":                                     {out: "issue/VSB-101"},
				"rev-parse --short HEAD":                                           {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-101": {out: "origin/issue/VSB-101"},
				"show-ref --verify --quiet refs/heads/master":                      {out: ""},
				`log --pretty=format:%h %ad | %s%d [%an] --graph --decorate --date=short --no-color master..issue/VSB-101`: {
					out: "* abc1234 2026-10-09 | issue commit [Test User]",
				},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "* abc1234 2026-10-09 | issue commit [Test User]\n",
		},
		{
			name: "path scoping appends follow to regular file",
			args: []string{"history", "main.go", "--no-color"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				`log --pretty=format:%h %ad | %s%d [%an] --graph --decorate --date=short --no-color --follow -- main.go`: {
					out: "* abc1234 2026-10-09 | touch main.go [Test User]",
				},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "* abc1234 2026-10-09 | touch main.go [Test User]\n",
		},
		{
			name: "git log failure returns exit code 1",
			args: []string{"history", "--no-color"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				`log --pretty=format:%h %ad | %s%d [%an] --graph --decorate --date=short --no-color`: {
					err: errors.New("broken pipe"),
				},
			},
			expectedCode: int(domain.ExitGeneralError),
			errSubstr:    "Getting history failed (git log failed)",
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
				if tt.expectStdout != "" {
					assert.Equal(t, tt.expectStdout, stdout.String())
				}
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

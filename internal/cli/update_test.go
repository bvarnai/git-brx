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

func TestUpdateCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
	}{
		{
			name:         "excess arguments returns usage error",
			args:         []string{"update", "unexpected-arg"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: unexpected-arg",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"update"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "This is not a git repository",
		},
		{
			name: "detached HEAD returns code 3",
			args: []string{"update"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/repo"},
				"rev-parse --git-dir":             {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":    {err: errors.New("detached")},
				"rev-parse --short HEAD":          {out: "abc1234"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Cannot execute on a detached HEAD",
		},
		{
			name: "missing origin returns code 4",
			args: []string{"update"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/repo"},
				"rev-parse --git-dir":             {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":    {out: "feature/test"},
				"rev-parse --short HEAD":          {out: "abc1234"},
				"remote get-url origin":           {err: errors.New("no origin")},
			},
			expectedCode: int(domain.ExitPreconditionRemote),
			errSubstr:    "Your repository has no remote origin",
		},
		{
			name: "branch not published on origin returns code 4 with publish hint",
			args: []string{"update"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                   {out: "true"},
				"rev-parse --show-toplevel":                                         {out: "/repo"},
				"rev-parse --git-dir":                                               {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":                                      {out: "feature/unpublished"},
				"rev-parse --short HEAD":                                            {out: "abc1234"},
				"remote get-url origin":                                             {out: "https://remote.example.com/repo.git"},
				"show-ref --verify --quiet refs/heads/feature/unpublished":          {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/feature/unpublished": {err: errors.New("not found")},
				"ls-remote --heads origin feature/unpublished":                      {out: ""},
			},
			expectedCode: int(domain.ExitPreconditionRemote),
			errSubstr:    "Branch 'feature/unpublished' has not been published to origin yet",
		},
		{
			name: "dry run logs planned rebase without modifying refs",
			args: []string{"update", "--dry-run"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                             {out: "true"},
				"rev-parse --show-toplevel":                                   {out: "/repo"},
				"rev-parse --git-dir":                                         {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":                                {out: "feature/ready"},
				"rev-parse --short HEAD":                                      {out: "abc1234"},
				"remote get-url origin":                                       {out: "https://remote.example.com/repo.git"},
				"show-ref --verify --quiet refs/heads/feature/ready":          {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/feature/ready": {out: ""},
			},
			expectedCode: int(domain.ExitSuccess),
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

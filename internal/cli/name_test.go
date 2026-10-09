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

func TestNameCommand_UnitTable(t *testing.T) {
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
			args:         []string{"name", "unexpected-arg"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: unexpected-arg",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"name"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "This is not a git repository",
		},
		{
			name: "attached branch emits branch name to stdout",
			args: []string{"name"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                  {out: "true"},
				"rev-parse --show-toplevel":                                        {out: "/repo"},
				"symbolic-ref --short -q HEAD":                                     {out: "issue/VSB-400"},
				"rev-parse --short HEAD":                                           {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-400": {out: "origin/issue/VSB-400"},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "issue/VSB-400\n",
		},
		{
			name: "detached HEAD emits short hash to stdout and warns on stderr",
			args: []string{"name"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/repo"},
				"symbolic-ref --short -q HEAD":    {err: errors.New("detached")},
				"rev-parse --short HEAD":          {out: "deadbeef"},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStdout: "deadbeef\n",
			expectStderr: "Warning: HEAD is detached at deadbeef",
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

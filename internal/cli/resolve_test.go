package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
		expectStdout  string
	}{
		{
			name:         "excess arguments returns usage error",
			args:         []string{"resolve", "unexpected-arg"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: unexpected-arg",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"resolve"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "This is not a git repository",
		},
		{
			name: "no active operation logs notice and exits 0",
			args: []string{"resolve"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/repo"},
				"rev-parse --git-dir":             {out: "/repo/.git"},
			},
			expectedCode: int(domain.ExitSuccess),
		},
	}

	tempDir := t.TempDir()
	gitDir := tempDir + "/.git"
	_ = os.MkdirAll(gitDir, 0755)
	_ = os.WriteFile(gitDir+"/MERGE_HEAD", []byte("sha"), 0644)

	tests = append(tests, struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
		expectStdout  string
	}{
		name: "mergetool crash returns code 5",
		args: []string{"resolve"},
		mockResponses: map[string]mockGitResponse{
			"rev-parse --is-inside-work-tree":  {out: "true"},
			"rev-parse --show-toplevel":        {out: tempDir},
			"rev-parse --git-dir":              {out: gitDir},
			"diff --name-only --diff-filter=U": {out: "file.txt"},
			"mergetool --no-prompt":            {err: errors.New("mergetool crashed")},
		},
		expectedCode: int(domain.ExitConflict),
		errSubstr:    "Merge tool execution failed",
	})

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

package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/errcode"
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
			expectedCode: int(errcode.ExitUsageError),
			errSubstr:    "Unexpected argument: unexpected-arg",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"resolve"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(errcode.ExitPreconditionRepo),
			errSubstr:    "Not a git repository",
		},
		{
			name: "no active operation logs notice and exits 0",
			args: []string{"resolve"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/repo"},
				"rev-parse --git-dir":             {out: "/repo/.git"},
			},
			expectedCode: int(errcode.ExitSuccess),
		},
	}

	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte("sha"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("<<<<<<< HEAD\na\n=======\nb\n>>>>>>> main\n"), 0644))

	cleanDir := t.TempDir()
	cleanGitDir := filepath.Join(cleanDir, ".git")
	require.NoError(t, os.MkdirAll(cleanGitDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cleanGitDir, "MERGE_HEAD"), []byte("sha"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(cleanDir, "file.txt"), []byte("resolved content\n"), 0644))

	deletedDir := t.TempDir()
	deletedGitDir := filepath.Join(deletedDir, ".git")
	require.NoError(t, os.MkdirAll(deletedGitDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(deletedGitDir, "MERGE_HEAD"), []byte("sha"), 0644))

	tests = append(tests,
		struct {
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
				"config --get merge.tool":          {out: "vimdiff"},
				"mergetool --no-prompt":            {err: errors.New("mergetool crashed")},
			},
			expectedCode: int(errcode.ExitConflict),
			errSubstr:    "Merge tool execution failed",
		},
	)

	extraTests := []struct {
		name           string
		args           []string
		mockResponses  map[string]mockGitResponse
		expectedCode   int
		errSubstr      string
		expectStderr   []string
		expectExecuted []string
		forbidExecuted []string
	}{
		{
			name: "marker-free unmerged file skips mergetool and stages file",
			args: []string{"resolve"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":  {out: "true"},
				"rev-parse --show-toplevel":        {out: cleanDir},
				"rev-parse --git-dir":              {out: cleanGitDir},
				"diff --name-only --diff-filter=U": {out: "file.txt"},
				"add -- file.txt":                  {out: ""},
				"commit --no-edit":                 {out: ""},
			},
			expectedCode:   int(errcode.ExitSuccess),
			expectStderr:   []string{"Merge completed"},
			expectExecuted: []string{"add -- file.txt", "commit --no-edit"},
			forbidExecuted: []string{"mergetool --no-prompt"},
		},
		{
			name: "conflicted file with --no-tool exits 5 listing file",
			args: []string{"resolve", "--no-tool"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":  {out: "true"},
				"rev-parse --show-toplevel":        {out: tempDir},
				"rev-parse --git-dir":              {out: gitDir},
				"diff --name-only --diff-filter=U": {out: "file.txt"},
			},
			expectedCode:   int(errcode.ExitConflict),
			errSubstr:      "Conflicts remain unresolved; cannot continue",
			expectStderr:   []string{"file.txt"},
			forbidExecuted: []string{"mergetool --no-prompt"},
		},
		{
			name: "conflicted file without merge.tool configured exits 5 listing file",
			args: []string{"resolve"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":  {out: "true"},
				"rev-parse --show-toplevel":        {out: tempDir},
				"rev-parse --git-dir":              {out: gitDir},
				"diff --name-only --diff-filter=U": {out: "file.txt"},
				"config --get merge.tool":          {err: errors.New("not configured")},
			},
			expectedCode:   int(errcode.ExitConflict),
			errSubstr:      "Conflicts remain unresolved; cannot continue",
			expectStderr:   []string{"file.txt"},
			forbidExecuted: []string{"mergetool --no-prompt"},
		},
		{
			name: "deleted unmerged file with --no-tool is not auto-staged and exits 5",
			args: []string{"resolve", "--no-tool"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":  {out: "true"},
				"rev-parse --show-toplevel":        {out: deletedDir},
				"rev-parse --git-dir":              {out: deletedGitDir},
				"diff --name-only --diff-filter=U": {out: "deleted.txt"},
			},
			expectedCode:   int(errcode.ExitConflict),
			errSubstr:      "Conflicts remain unresolved; cannot continue",
			expectStderr:   []string{"deleted.txt"},
			forbidExecuted: []string{"add -- deleted.txt", "mergetool --no-prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			runner := &mockGitRunner{responses: tt.mockResponses}
			app := cli.NewApp(&stdout, &stderr, runner)
			app.RootCmd.SetArgs(tt.args)

			err := app.Execute(context.Background())
			if tt.expectedCode == int(errcode.ExitSuccess) {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				var appErr *errcode.AppError
				require.True(t, errors.As(err, &appErr), "expected *errcode.AppError, got %T: %v", err, err)
				assert.Equal(t, tt.expectedCode, int(errcode.ExitCode(appErr)))
				if tt.errSubstr != "" {
					assert.Contains(t, appErr.Message, tt.errSubstr)
				}
			}
		})
	}

	for _, tt := range extraTests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			runner := &mockGitRunner{responses: tt.mockResponses}
			app := cli.NewApp(&stdout, &stderr, runner)
			app.RootCmd.SetArgs(tt.args)

			err := app.Execute(context.Background())
			if tt.expectedCode == int(errcode.ExitSuccess) {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				var appErr *errcode.AppError
				require.True(t, errors.As(err, &appErr), "expected *errcode.AppError, got %T: %v", err, err)
				assert.Equal(t, tt.expectedCode, int(errcode.ExitCode(appErr)))
				if tt.errSubstr != "" {
					assert.Contains(t, appErr.Message, tt.errSubstr)
				}
			}
			for _, substr := range tt.expectStderr {
				assert.Contains(t, stderr.String(), substr)
			}
			for _, cmd := range tt.expectExecuted {
				assert.Contains(t, runner.executed, cmd)
			}
			for _, cmd := range tt.forbidExecuted {
				assert.NotContains(t, runner.executed, cmd)
			}
		})
	}
}

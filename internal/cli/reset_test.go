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

func TestResetCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
	}{
		{
			name:         "excess arguments returns usage error",
			args:         []string{"reset", "unexpected-arg"},
			expectedCode: int(errcode.ExitUsageError),
			errSubstr:    "Unexpected argument: unexpected-arg",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"reset"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(errcode.ExitPreconditionRepo),
			errSubstr:    "Not a git repository",
		},
		{
			name: "detached HEAD returns code 3",
			args: []string{"reset"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/repo"},
				"rev-parse --git-dir":             {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":    {err: errors.New("detached")},
				"rev-parse --short HEAD":          {out: "abc1234"},
				"remote get-url origin":           {out: "https://remote.example.com/repo.git"},
			},
			expectedCode: int(errcode.ExitPreconditionRepo),
			errSubstr:    "Cannot execute on a detached HEAD",
		},
		{
			name: "missing origin returns code 4",
			args: []string{"reset"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/repo"},
				"rev-parse --git-dir":             {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":    {out: "feature/test"},
				"rev-parse --short HEAD":          {out: "abc1234"},
				"remote get-url origin":           {err: errors.New("no origin")},
			},
			expectedCode: int(errcode.ExitPreconditionRemote),
			errSubstr:    "Your repository has no remote origin",
		},
		{
			name: "branch not published on origin returns code 4 with publish hint",
			args: []string{"reset"},
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
			expectedCode: int(errcode.ExitPreconditionRemote),
			errSubstr:    "Remote reference 'origin/feature/unpublished' does not exist",
		},
		{
			name: "resets hard to remote ref cleanly",
			args: []string{"reset"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                             {out: "true"},
				"rev-parse --show-toplevel":                                   {out: "/repo"},
				"rev-parse --git-dir":                                         {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":                                {out: "feature/ready"},
				"rev-parse --short HEAD":                                      {out: "abc1234"},
				"remote get-url origin":                                       {out: "https://remote.example.com/repo.git"},
				"show-ref --verify --quiet refs/heads/feature/ready":          {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/feature/ready": {out: ""},
				"reset --hard origin/feature/ready":                           {out: ""},
			},
			expectedCode: int(errcode.ExitSuccess),
		},
	}

	// Active conflict test cases (in-flight rebase / merge)
	rebaseRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rebaseRepo, ".git", "rebase-merge"), 0755))

	mergeRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(mergeRepo, ".git"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mergeRepo, ".git", "MERGE_HEAD"), []byte("deadbeef\n"), 0644))

	activeTests := []struct {
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
			name: "active rebase with --clean returns code 2 without aborting",
			args: []string{"reset", "--clean"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: rebaseRepo},
				"rev-parse --git-dir":             {out: filepath.Join(rebaseRepo, ".git")},
			},
			expectedCode:   int(errcode.ExitUsageError),
			errSubstr:      "Cannot use '--clean' while a rebase is in progress",
			forbidExecuted: []string{"rebase --abort", "clean -fd"},
		},
		{
			name: "active rebase on unpublished branch aborts safely without hard reset",
			args: []string{"reset"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: rebaseRepo},
				"rev-parse --git-dir":             {out: filepath.Join(rebaseRepo, ".git")},
				"rebase --abort":                  {out: ""},
				"symbolic-ref --short -q HEAD":    {out: "issue/unpublished"},
				"rev-parse --short HEAD":          {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/unpublished": {out: ""},
				"show-ref --verify --quiet refs/heads/issue/unpublished":               {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/issue/unpublished":      {err: errors.New("not found")},
			},
			expectedCode:   int(errcode.ExitSuccess),
			expectStderr:   []string{"Aborted in-progress rebase; local commits preserved"},
			expectExecuted: []string{"rebase --abort"},
			forbidExecuted: []string{"reset --hard origin/issue/unpublished"},
		},
		{
			name: "active merge on published branch aborts safely and emits re-run hint",
			args: []string{"reset"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: mergeRepo},
				"rev-parse --git-dir":             {out: filepath.Join(mergeRepo, ".git")},
				"merge --abort":                   {out: ""},
				"symbolic-ref --short -q HEAD":    {out: "feature/test"},
				"rev-parse --short HEAD":          {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/feature/test": {out: "origin/feature/test"},
				"show-ref --verify --quiet refs/heads/feature/test":               {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/feature/test":      {out: ""},
			},
			expectedCode:   int(errcode.ExitSuccess),
			expectStderr:   []string{"Aborted in-progress merge; local commits preserved", "Run 'git-brx reset' again to discard local changes and match 'origin/feature/test'"},
			expectExecuted: []string{"merge --abort"},
			forbidExecuted: []string{"reset --hard origin/feature/test"},
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

	for _, tt := range activeTests {
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

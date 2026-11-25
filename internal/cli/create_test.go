package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		stdin         string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
		outSubstr     string
	}{
		{
			name:         "missing branch name argument returns usage error",
			args:         []string{"create"},
			expectedCode: int(errcode.ExitUsageError),
			errSubstr:    "No branch name specified",
		},
		{
			name:         "excess positional arguments returns usage error",
			args:         []string{"create", "issue/VSB-1", "extra"},
			expectedCode: int(errcode.ExitUsageError),
			errSubstr:    "Unexpected argument: extra",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"create", "issue/VSB-1"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false", err: errors.New("not a git repo")},
			},
			expectedCode: int(errcode.ExitPreconditionRepo),
			errSubstr:    "Not a git repository",
		},
		{
			name: "local branch already exists auto-switches and exits 0",
			args: []string{"create", "issue/VSB-100"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                    {out: "true"},
				"rev-parse --show-toplevel":                          {out: "/repo"},
				"rev-parse --git-dir":                                {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/VSB-100": {out: ""}, // exists locally
				"remote get-url origin":                              {err: errors.New("no origin")},
				"symbolic-ref -q HEAD":                               {out: "refs/heads/master"},
				"symbolic-ref --short -q HEAD":                       {out: "master"},
				"checkout issue/VSB-100":                             {out: ""},
			},
			expectedCode: int(errcode.ExitSuccess),
			outSubstr:    "Branch 'issue/VSB-100' already exists; switched to 'issue/VSB-100'",
		},
		{
			name: "remote branch already exists in online mode fetches, switches, and exits 0",
			args: []string{"create", "issue/VSB-101"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                    {out: "true"},
				"rev-parse --show-toplevel":                                          {out: "/repo"},
				"rev-parse --git-dir":                                                {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/VSB-101":                 {out: "", err: errors.New("not local")},
				"remote get-url origin":                                              {out: "https://github.com/org/repo.git"},
				"ls-remote --heads origin issue/VSB-101":                             {out: "abc1234 refs/heads/issue/VSB-101"},
				"symbolic-ref -q HEAD":                                               {out: "refs/heads/master"},
				"symbolic-ref --short -q HEAD":                                       {out: "master"},
				"fetch origin":                                                       {out: ""},
				"checkout issue/VSB-101":                                             {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/issue/VSB-101":        {out: ""},
				"rev-list --left-right --count issue/VSB-101...origin/issue/VSB-101": {out: "0\t0"},
			},
			expectedCode: int(errcode.ExitSuccess),
			outSubstr:    "Branch 'issue/VSB-101' already exists; switched to 'issue/VSB-101'",
		},
		{
			name: "remote unreachable in online mode returns exit code 4",
			args: []string{"create", "issue/VSB-102"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                    {out: "true"},
				"rev-parse --show-toplevel":                          {out: "/repo"},
				"rev-parse --git-dir":                                {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/VSB-102": {out: "", err: errors.New("not local")},
				"remote get-url origin":                              {out: "https://github.com/org/repo.git"},
				"ls-remote --heads origin issue/VSB-102":             {out: "", err: errors.New("fatal: unable to access")},
			},
			expectedCode: int(errcode.ExitPreconditionRemote),
			errSubstr:    "Unable to reach remote",
		},
		{
			name: "branch name fails naming template returns exit code 8",
			args: []string{"create", "--offline", "invalid-branch-name"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                          {out: "true"},
				"rev-parse --show-toplevel":                                {out: "/repo"},
				"rev-parse --git-dir":                                      {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/invalid-branch-name": {out: "", err: errors.New("not found")},
				"remote get-url origin":                                    {out: "https://github.com/org/repo.git"},
			},
			expectedCode: int(errcode.ExitConfigError),
			errSubstr:    "doesn't match pattern",
		},
		{
			name: "offline mode provisions branch cleanly",
			args: []string{"create", "--offline", "issue/42"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":               {out: "true"},
				"rev-parse --show-toplevel":                     {out: "/repo"},
				"rev-parse --git-dir":                           {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/42": {out: "", err: errors.New("not found")},
				"remote get-url origin":                         {out: "https://github.com/org/repo.git"},
				"checkout -b issue/42":                          {out: "Switched to a new branch 'issue/42'"},
			},
			expectedCode: int(errcode.ExitSuccess),
			outSubstr:    "Creating branch 'issue/42'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockGitRunner{responses: tt.mockResponses}
			var stdout, stderr bytes.Buffer
			app := cli.NewApp(&stdout, &stderr, runner)
			if tt.stdin != "" {
				app.Logger.SetStdin(strings.NewReader(tt.stdin))
			}
			app.RootCmd.SetArgs(tt.args)

			err := app.Execute(context.Background())
			if tt.expectedCode == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				var appErr *errcode.AppError
				require.True(t, errors.As(err, &appErr))
				assert.Equal(t, tt.expectedCode, int(errcode.ExitCode(appErr)))
				if tt.errSubstr != "" {
					assert.Contains(t, appErr.Message, tt.errSubstr)
				}
			}

			if tt.outSubstr != "" {
				combined := stderr.String() + stdout.String()
				assert.Contains(t, combined, tt.outSubstr)
			}
		})
	}
}

func TestCreateCommand_BaseAndGuardrails(t *testing.T) {
	t.Run("missing --base branch returns code 3", func(t *testing.T) {
		runner := &mockGitRunner{
			responses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                        {out: "true"},
				"rev-parse --show-toplevel":                              {out: "/repo"},
				"rev-parse --git-dir":                                    {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/44":          {err: errors.New("not found")},
				"remote get-url origin":                                  {out: "https://github.com/org/repo.git"},
				"show-ref --verify --quiet refs/heads/epic/999":          {err: errors.New("not found")},
				"show-ref --verify --quiet refs/remotes/origin/epic/999": {err: errors.New("not found")},
			},
		}
		var stdout, stderr bytes.Buffer
		app := cli.NewApp(&stdout, &stderr, runner)
		app.RootCmd.SetArgs([]string{"create", "--offline", "--base", "epic/999", "issue/44"})

		err := app.Execute(context.Background())
		require.Error(t, err)
		var appErr *errcode.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, int(errcode.ExitPreconditionRepo), int(errcode.ExitCode(appErr)))
		assert.Contains(t, appErr.Message, "Base branch 'epic/999' not found")
	})

	t.Run("--base epic/10 forks from epic/10 and records brx-base", func(t *testing.T) {
		runner := &mockGitRunner{
			responses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                       {out: "true"},
				"rev-parse --show-toplevel":                             {out: "/repo"},
				"rev-parse --git-dir":                                   {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/44":         {err: errors.New("not found")},
				"remote get-url origin":                                 {out: "https://github.com/org/repo.git"},
				"show-ref --verify --quiet refs/heads/epic/10":          {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/epic/10": {err: errors.New("not remote")},
				"checkout -b issue/44 epic/10":                          {out: ""},
				"config --local branch.issue/44.brx-base epic/10":       {out: ""},
			},
		}
		var stdout, stderr bytes.Buffer
		app := cli.NewApp(&stdout, &stderr, runner)
		app.RootCmd.SetArgs([]string{"create", "--offline", "--base", "epic/10", "issue/44"})

		require.NoError(t, app.Execute(context.Background()))
		assert.Contains(t, runner.executed, "checkout -b issue/44 epic/10")
		assert.Contains(t, runner.executed, "config --local branch.issue/44.brx-base epic/10")
	})

	t.Run("forking from issue branch with dirty worktree emits warnings", func(t *testing.T) {
		runner := &mockGitRunner{
			responses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                  {out: "true"},
				"rev-parse --show-toplevel":                        {out: "/repo"},
				"rev-parse --git-dir":                              {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/45":    {err: errors.New("not found")},
				"remote get-url origin":                            {out: "https://github.com/org/repo.git"},
				"symbolic-ref --short -q HEAD":                     {out: "issue/10"},
				"show-ref --verify --quiet refs/heads/main":        {out: ""},
				"status --porcelain":                               {out: " M dirty.go"},
				"checkout -b issue/45":                             {out: ""},
				"config --local branch.issue/45.brx-base issue/10": {out: ""},
			},
		}
		var stdout, stderr bytes.Buffer
		app := cli.NewApp(&stdout, &stderr, runner)
		app.RootCmd.SetArgs([]string{"create", "--offline", "issue/45"})

		require.NoError(t, app.Execute(context.Background()))
		assert.Contains(t, stderr.String(), "Creating 'issue/45' from 'issue/10' instead of default branch")
		assert.Contains(t, stderr.String(), "Uncommitted local changes will be carried over to 'issue/45'")
	})
}

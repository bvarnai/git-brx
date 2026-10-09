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
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "No branch name specified",
		},
		{
			name:         "excess positional arguments returns usage error",
			args:         []string{"create", "issue/VSB-1", "extra"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: extra",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"create", "issue/VSB-1"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false", err: errors.New("not a git repo")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Awh! This is not a git repository",
		},
		{
			name: "local branch already exists exits 0 with select hint",
			args: []string{"create", "issue/VSB-100"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                    {out: "true"},
				"rev-parse --show-toplevel":                          {out: "/repo"},
				"rev-parse --git-dir":                                {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/VSB-100": {out: ""}, // exists locally
			},
			expectedCode: int(domain.ExitSuccess),
			outSubstr:    "Branch 'issue/VSB-100' found (local)",
		},
		{
			name: "remote branch already exists in online mode exits 0 with select hint",
			args: []string{"create", "issue/VSB-101"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                    {out: "true"},
				"rev-parse --show-toplevel":                          {out: "/repo"},
				"rev-parse --git-dir":                                {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/VSB-101": {out: "", err: errors.New("not local")},
				"remote get-url origin":                              {out: "https://github.com/org/repo.git"},
				"ls-remote --heads origin issue/VSB-101":             {out: "abc1234 refs/heads/issue/VSB-101"},
			},
			expectedCode: int(domain.ExitSuccess),
			outSubstr:    "Branch 'issue/VSB-101' found (remote)",
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
			expectedCode: int(domain.ExitPreconditionRemote),
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
			expectedCode: int(domain.ExitConfigError),
			errSubstr:    "doesn't match pattern",
		},
		{
			name: "offline mode successfully provisions branch with dry-run",
			args: []string{"create", "--offline", "--dry-run", "issue/42"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":               {out: "true"},
				"rev-parse --show-toplevel":                     {out: "/repo"},
				"rev-parse --git-dir":                           {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/issue/42": {out: "", err: errors.New("not found")},
				"remote get-url origin":                         {out: "https://github.com/org/repo.git"},
			},
			expectedCode: int(domain.ExitSuccess),
			outSubstr:    "Would create branch 'issue/42'",
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
			expectedCode: int(domain.ExitSuccess),
			outSubstr:    "Creating branch 'issue/42'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockGitRunner{responses: tt.mockResponses}
			var stdout, stderr bytes.Buffer
			app := cli.NewApp(&stdout, &stderr, runner)
			if tt.stdin != "" {
				app.UI.SetStdin(strings.NewReader(tt.stdin))
			}
			app.RootCmd.SetArgs(tt.args)

			err := app.Execute(context.Background())
			if tt.expectedCode == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				var appErr *domain.AppError
				require.True(t, errors.As(err, &appErr))
				assert.Equal(t, tt.expectedCode, int(appErr.Code))
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

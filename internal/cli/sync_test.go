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

func TestSyncCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
	}{
		{
			name:         "excess arguments returns usage error",
			args:         []string{"sync", "master", "extra-arg"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: extra-arg",
		},
		{
			name:         "conflicting strategies -m and -r returns usage error",
			args:         []string{"sync", "-m", "-r"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Multiple override sync strategies specified",
		},
		{
			name: "autostash on merge strategy returns usage error",
			args: []string{"sync", "-m", "-a"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --git-dir":               {out: "/repo/.git"},
				"rev-parse --is-shallow-repository": {out: "false"},
				"symbolic-ref --short -q HEAD":      {out: "feature/test"},
				"rev-parse --short HEAD":            {out: "abc1234"},
			},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Options 'autostash/interactive' are rebase only",
		},
		{
			name: "interactive on merge strategy returns usage error",
			args: []string{"sync", "-m", "-i"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --git-dir":               {out: "/repo/.git"},
				"rev-parse --is-shallow-repository": {out: "false"},
				"symbolic-ref --short -q HEAD":      {out: "feature/test"},
				"rev-parse --short HEAD":            {out: "abc1234"},
			},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Options 'autostash/interactive' are rebase only",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"sync"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "This is not a git repository",
		},
		{
			name: "shallow repository returns code 3",
			args: []string{"sync"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --git-dir":               {out: "/repo/.git"},
				"rev-parse --is-shallow-repository": {out: "true"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "You are in a shallow repository",
		},
		{
			name: "detached HEAD returns code 3",
			args: []string{"sync"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --git-dir":               {out: "/repo/.git"},
				"rev-parse --is-shallow-repository": {out: "false"},
				"symbolic-ref --short -q HEAD":      {err: errors.New("detached")},
				"rev-parse --short HEAD":            {out: "abc1234"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Cannot execute on a detached HEAD",
		},
		{
			name: "unsupported custom branch type without override returns code 3",
			args: []string{"sync"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --git-dir":               {out: "/repo/.git"},
				"rev-parse --is-shallow-repository": {out: "false"},
				"symbolic-ref --short -q HEAD":      {out: "my-custom-branch"},
				"rev-parse --short HEAD":            {out: "abc1234"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "You must be on 'issue', 'feature' or 'epic' branch",
		},
		{
			name: "syncing current branch onto itself returns code 3",
			args: []string{"sync", "issue/VSB-100"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --git-dir":               {out: "/repo/.git"},
				"rev-parse --is-shallow-repository": {out: "false"},
				"symbolic-ref --short -q HEAD":      {out: "issue/VSB-100"},
				"rev-parse --short HEAD":            {out: "abc1234"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Cannot sync branch 'issue/VSB-100' onto itself",
		},
		{
			name: "base branch does not exist locally returns code 3",
			args: []string{"sync", "non-existent-base"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                 {out: "true"},
				"rev-parse --show-toplevel":                                       {out: "/repo"},
				"rev-parse --git-dir":                                             {out: "/repo/.git"},
				"rev-parse --is-shallow-repository":                               {out: "false"},
				"symbolic-ref --short -q HEAD":                                    {out: "issue/VSB-100"},
				"rev-parse --short HEAD":                                          {out: "abc1234"},
				"show-ref --verify --quiet refs/heads/non-existent-base":          {err: errors.New("not found")},
				"show-ref --verify --quiet refs/remotes/origin/non-existent-base": {err: errors.New("not found")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Base branch 'non-existent-base' does not exist locally",
		},
		{
			name: "diverged base branch from origin returns code 5",
			args: []string{"sync", "master"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                      {out: "true"},
				"rev-parse --show-toplevel":                            {out: "/repo"},
				"rev-parse --git-dir":                                  {out: "/repo/.git"},
				"rev-parse --is-shallow-repository":                    {out: "false"},
				"symbolic-ref --short -q HEAD":                         {out: "issue/VSB-100"},
				"rev-parse --short HEAD":                               {out: "abc1234"},
				"show-ref --verify --quiet refs/heads/master":          {out: ""},
				"remote get-url origin":                                {out: "https://remote.example.com/repo.git"},
				"fetch origin":                                         {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/master": {out: ""},
				"rev-list --left-right --count master...origin/master": {out: "0\t2"},
			},
			expectedCode: int(domain.ExitConflict),
			errSubstr:    "Sync branch 'master' is not up-to-date",
		},
		{
			name: "dry-run on issue branch logs planned rebase command",
			args: []string{"sync", "--dry-run", "master"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                      {out: "true"},
				"rev-parse --show-toplevel":                            {out: "/repo"},
				"rev-parse --git-dir":                                  {out: "/repo/.git"},
				"rev-parse --is-shallow-repository":                    {out: "false"},
				"symbolic-ref --short -q HEAD":                         {out: "issue/VSB-100"},
				"rev-parse --short HEAD":                               {out: "abc1234"},
				"show-ref --verify --quiet refs/heads/master":          {out: ""},
				"remote get-url origin":                                {out: "https://remote.example.com/repo.git"},
				"fetch origin":                                         {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/master": {out: ""},
				"rev-list --left-right --count master...origin/master": {out: "0\t0"},
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

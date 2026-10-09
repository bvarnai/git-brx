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

func TestPublishCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
		outSubstr     string
	}{
		{
			name:         "excess positional arguments returns usage error",
			args:         []string{"publish", "extra-arg"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: extra-arg",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"publish"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false", err: errors.New("not a git repo")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Awh! This is not a git repository",
		},
		{
			name: "shallow repository returns code 3",
			args: []string{"publish"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --is-shallow-repository": {out: "true"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "You are in a shallow repository",
		},
		{
			name: "detached HEAD returns code 3",
			args: []string{"publish"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --is-shallow-repository": {out: "false"},
				"rev-parse --git-dir":               {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":      {out: "abc1234", err: errors.New("detached")},
				"rev-parse --short HEAD":            {out: "abc1234"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Cannot execute on a detached HEAD",
		},
		{
			name: "missing origin remote returns code 4",
			args: []string{"publish"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --is-shallow-repository": {out: "false"},
				"rev-parse --git-dir":               {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":      {out: "issue/VSB-1"},
				"rev-parse --short HEAD":            {out: "abc1234"},
				"remote get-url origin":             {out: "", err: errors.New("no origin")},
			},
			expectedCode: int(domain.ExitPreconditionRemote),
			errSubstr:    "Your repository has no remote origin",
		},
		{
			name: "refuses force-push with lease directly to master",
			args: []string{"publish"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":   {out: "true"},
				"rev-parse --show-toplevel":         {out: "/repo"},
				"rev-parse --is-shallow-repository": {out: "false"},
				"rev-parse --git-dir":               {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":      {out: "master"},
				"rev-parse --short HEAD":            {out: "abc1234"},
				"remote get-url origin":             {out: "https://github.com/org/repo.git"},
			},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Refusing to force-push with lease directly to protected 'master' branch",
		},
		{
			name: "first publish sets upstream and force with lease",
			args: []string{"publish"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                {out: "true"},
				"rev-parse --show-toplevel":                                      {out: "/repo"},
				"rev-parse --is-shallow-repository":                              {out: "false"},
				"rev-parse --git-dir":                                            {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":                                   {out: "issue/VSB-1"},
				"rev-parse --short HEAD":                                         {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-1": {out: ""}, // no upstream yet
				"remote get-url origin":                                          {out: "https://github.com/org/repo.git"},
				"push --set-upstream origin issue/VSB-1 --force-with-lease":      {out: "Everything up-to-date"},
			},
			expectedCode: int(domain.ExitSuccess),
			outSubstr:    "Branch 'issue/VSB-1' successfully published to origin",
		},
		{
			name: "subsequent publish uses origin branch and lease",
			args: []string{"publish"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                {out: "true"},
				"rev-parse --show-toplevel":                                      {out: "/repo"},
				"rev-parse --is-shallow-repository":                              {out: "false"},
				"rev-parse --git-dir":                                            {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":                                   {out: "issue/VSB-1"},
				"rev-parse --short HEAD":                                         {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-1": {out: "origin/issue/VSB-1"},
				"remote get-url origin":                                          {out: "https://github.com/org/repo.git"},
				"push origin issue/VSB-1 --force-with-lease":                     {out: "Everything up-to-date"},
			},
			expectedCode: int(domain.ExitSuccess),
			outSubstr:    "Branch 'issue/VSB-1' successfully published to origin",
		},
		{
			name: "stale lease rejection returns code 4 with update hint",
			args: []string{"publish"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                {out: "true"},
				"rev-parse --show-toplevel":                                      {out: "/repo"},
				"rev-parse --is-shallow-repository":                              {out: "false"},
				"rev-parse --git-dir":                                            {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":                                   {out: "issue/VSB-1"},
				"rev-parse --short HEAD":                                         {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-1": {out: "origin/issue/VSB-1"},
				"remote get-url origin":                                          {out: "https://github.com/org/repo.git"},
				"push origin issue/VSB-1 --force-with-lease":                     {out: "", err: errors.New("! [rejected] (stale info)")},
			},
			expectedCode: int(domain.ExitPreconditionRemote),
			errSubstr:    "Publish rejected: remote has newer commits",
		},
		{
			name: "dry run simulation",
			args: []string{"publish", "--dry-run"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                {out: "true"},
				"rev-parse --show-toplevel":                                      {out: "/repo"},
				"rev-parse --is-shallow-repository":                              {out: "false"},
				"rev-parse --git-dir":                                            {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":                                   {out: "issue/VSB-1"},
				"rev-parse --short HEAD":                                         {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-1": {out: "origin/issue/VSB-1"},
				"remote get-url origin":                                          {out: "https://github.com/org/repo.git"},
				"push --dry-run origin issue/VSB-1 --force-with-lease":           {out: ""},
			},
			expectedCode: int(domain.ExitSuccess),
			outSubstr:    "Would publish branch 'issue/VSB-1' to origin",
		},
		{
			name: "publish with --no-force pushes without lease",
			args: []string{"publish", "--no-force"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                {out: "true"},
				"rev-parse --show-toplevel":                                      {out: "/repo"},
				"rev-parse --is-shallow-repository":                              {out: "false"},
				"rev-parse --git-dir":                                            {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":                                   {out: "issue/VSB-1"},
				"rev-parse --short HEAD":                                         {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-1": {out: "origin/issue/VSB-1"},
				"remote get-url origin":                                          {out: "https://github.com/org/repo.git"},
				"push origin issue/VSB-1":                                        {out: "Everything up-to-date"},
			},
			expectedCode: int(domain.ExitSuccess),
			outSubstr:    "Branch 'issue/VSB-1' successfully published to origin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockGitRunner{responses: tt.mockResponses}
			var stdout, stderr bytes.Buffer
			app := cli.NewApp(&stdout, &stderr, runner)
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

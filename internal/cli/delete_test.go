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

func TestDeleteCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		errSubstr     string
		expectStderr  string
	}{
		{
			name:         "multiple positional arguments return usage error",
			args:         []string{"delete", "branch1", "branch2"},
			expectedCode: int(domain.ExitUsageError),
			errSubstr:    "Unexpected argument: branch2",
		},
		{
			name: "named branch that does not exist locally returns code 3",
			args: []string{"delete", "issue/missing"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                    {out: "true"},
				"rev-parse --show-toplevel":                          {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                         {out: "abc1234"},
				"remote get-url origin":                              {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                       {out: "main"},
				"rev-parse --short HEAD":                             {out: "abc1234"},
				"show-ref --verify --quiet refs/heads/issue/missing": {err: errors.New("not found")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Branch 'issue/missing' does not exist locally",
		},
		{
			name: "named protected branch cannot be deleted",
			args: []string{"delete", "main"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":           {out: "true"},
				"rev-parse --show-toplevel":                 {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                {out: "abc1234"},
				"remote get-url origin":                     {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":              {out: "feature/foo"},
				"rev-parse --short HEAD":                    {out: "abc1234"},
				"show-ref --verify --quiet refs/heads/main": {out: ""},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "You can only delete an 'issue', 'feature', or 'epic' branch",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"delete"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "This is not a git repository",
		},
		{
			name: "unborn repo without commits returns code 3",
			args: []string{"delete"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {err: errors.New("exit status 1")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Repository has no commits yet",
		},
		{
			name: "missing origin remote returns code 3",
			args: []string{"delete"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				"remote get-url origin":           {err: errors.New("remote origin not found")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Origin remote is not configured",
		},
		{
			name: "detached HEAD returns code 3",
			args: []string{"delete"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":      {out: "abc1234"},
				"remote get-url origin":           {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":    {err: errors.New("detached")},
				"rev-parse --short HEAD":          {out: "abc1234"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Cannot delete branch in detached HEAD state",
		},
		{
			name: "master branch cannot be deleted",
			args: []string{"delete"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                           {out: "true"},
				"rev-parse --show-toplevel":                                 {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                {out: "abc1234"},
				"remote get-url origin":                                     {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                              {out: "master"},
				"rev-parse --short HEAD":                                    {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/master": {out: "origin/master"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "You must be on an 'issue', 'feature', or 'epic' branch to delete it",
		},
		{
			name: "release branch cannot be deleted",
			args: []string{"delete"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                 {out: "true"},
				"rev-parse --show-toplevel":                                       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                      {out: "abc1234"},
				"remote get-url origin":                                           {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                                    {out: "release/v1.0"},
				"rev-parse --short HEAD":                                          {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/release/v1.0": {out: "origin/release/v1.0"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "You must be on an 'issue', 'feature', or 'epic' branch to delete it",
		},
		{
			name: "remote check offline failure returns code 4",
			args: []string{"delete"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                  {out: "true"},
				"rev-parse --show-toplevel":                                        {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                       {out: "abc1234"},
				"remote get-url origin":                                            {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                                     {out: "issue/VSB-101"},
				"rev-parse --short HEAD":                                           {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-101": {out: "origin/issue/VSB-101"},
				"ls-remote --heads origin issue/VSB-101":                           {err: errors.New("Could not resolve host")},
			},
			expectedCode: int(domain.ExitPreconditionRemote),
			errSubstr:    "Unable to reach remote; are you offline?",
		},
		{
			name: "branch still on remote is rejected with hint",
			args: []string{"delete"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                  {out: "true"},
				"rev-parse --show-toplevel":                                        {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                       {out: "abc1234"},
				"remote get-url origin":                                            {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                                     {out: "issue/VSB-101"},
				"rev-parse --short HEAD":                                           {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-101": {out: "origin/issue/VSB-101"},
				"ls-remote --heads origin issue/VSB-101":                           {out: "abc1234 refs/heads/issue/VSB-101\n"},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			errSubstr:    "Branch must be deleted on remote first (e.g., after merging pull request)",
		},
		{
			name: "force flag bypasses remote check and cleans up branch",
			args: []string{"delete", "--force"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                  {out: "true"},
				"rev-parse --show-toplevel":                                        {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                       {out: "abc1234"},
				"remote get-url origin":                                            {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                                     {out: "issue/VSB-101"},
				"rev-parse --short HEAD":                                           {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-101": {out: "origin/issue/VSB-101"},
				"show-ref --verify --quiet refs/heads/master":                      {out: ""},
				"checkout master":                                                  {out: "Switched to branch 'master'"},
				"branch -D issue/VSB-101":                                          {out: "Deleted branch issue/VSB-101"},
				"remote prune origin":                                              {out: ""},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStderr: "Deleted local branch 'issue/VSB-101' and pruned origin",
		},
		{
			name: "dry run logs planned actions without deleting",
			args: []string{"delete", "--dry-run", "--force"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                  {out: "true"},
				"rev-parse --show-toplevel":                                        {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                       {out: "abc1234"},
				"remote get-url origin":                                            {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                                     {out: "feature/login"},
				"rev-parse --short HEAD":                                           {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/feature/login": {out: "origin/feature/login"},
				"show-ref --verify --quiet refs/heads/master":                      {out: ""},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStderr: "Would checkout branch 'master'",
		},
		{
			name: "clean deletion when branch already absent on remote",
			args: []string{"delete"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                 {out: "true"},
				"rev-parse --show-toplevel":                                       {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                      {out: "abc1234"},
				"remote get-url origin":                                           {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                                    {out: "epic/billing"},
				"rev-parse --short HEAD":                                          {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/epic/billing": {out: "origin/epic/billing"},
				"ls-remote --heads origin epic/billing":                           {out: ""},
				"show-ref --verify --quiet refs/heads/master":                     {out: ""},
				"checkout master":                                                 {out: "Switched to branch 'master'"},
				"branch -D epic/billing":                                          {out: "Deleted branch epic/billing"},
				"remote prune origin":                                             {out: ""},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStderr: "Deleted local branch 'epic/billing' and pruned origin",
		},
		{
			name: "checkout failure returns exit code 5",
			args: []string{"delete", "-f"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                                  {out: "true"},
				"rev-parse --show-toplevel":                                        {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                                       {out: "abc1234"},
				"remote get-url origin":                                            {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                                     {out: "issue/VSB-101"},
				"rev-parse --short HEAD":                                           {out: "abc1234"},
				"for-each-ref --format=%(upstream:short) refs/heads/issue/VSB-101": {out: "origin/issue/VSB-101"},
				"show-ref --verify --quiet refs/heads/master":                      {out: ""},
				"checkout master":                                                  {err: errors.New("local changes would be overwritten")},
			},
			expectedCode: int(domain.ExitConflict),
			errSubstr:    "Unable to switch to 'master' branch",
		},
		{
			name: "deleting named branch while currently on master deletes without checkout",
			args: []string{"delete", "issue/VSB-101"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                    {out: "true"},
				"rev-parse --show-toplevel":                          {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                         {out: "abc1234"},
				"remote get-url origin":                              {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                       {out: "master"},
				"rev-parse --short HEAD":                             {out: "abc1234"},
				"show-ref --verify --quiet refs/heads/issue/VSB-101": {out: ""},
				"ls-remote --heads origin issue/VSB-101":             {out: ""},
				"show-ref --verify --quiet refs/heads/master":        {out: ""},
				"branch -D issue/VSB-101":                            {out: "Deleted branch issue/VSB-101"},
				"remote prune origin":                                {out: ""},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStderr: "Deleted local branch 'issue/VSB-101' and pruned origin",
		},
		{
			name: "deleting named branch while currently on target branch checks out master first",
			args: []string{"delete", "feature/login", "-f"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                    {out: "true"},
				"rev-parse --show-toplevel":                          {out: "/mock/repo"},
				"rev-parse --verify -q HEAD":                         {out: "abc1234"},
				"remote get-url origin":                              {out: "git@github.com:org/repo.git"},
				"symbolic-ref --short -q HEAD":                       {out: "feature/login"},
				"rev-parse --short HEAD":                             {out: "abc1234"},
				"show-ref --verify --quiet refs/heads/feature/login": {out: ""},
				"show-ref --verify --quiet refs/heads/master":        {out: ""},
				"checkout master":                                    {out: "Switched to branch 'master'"},
				"branch -D feature/login":                            {out: "Deleted branch feature/login"},
				"remote prune origin":                                {out: ""},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStderr: "Deleted local branch 'feature/login' and pruned origin",
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

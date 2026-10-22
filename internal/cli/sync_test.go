package cli_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/errcode"
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
		expectStderr  string
	}{
		{
			name:         "excess arguments returns usage error",
			args:         []string{"sync", "master", "extra-arg"},
			expectedCode: int(errcode.ExitUsageError),
			errSubstr:    "Unexpected argument: extra-arg",
		},
		{
			name:         "conflicting strategies -m and -r returns usage error",
			args:         []string{"sync", "-m", "-r"},
			expectedCode: int(errcode.ExitUsageError),
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
			expectedCode: int(errcode.ExitUsageError),
			errSubstr:    "Flags --autostash and --interactive are only supported with rebase strategy",
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
			expectedCode: int(errcode.ExitUsageError),
			errSubstr:    "Flags --autostash and --interactive are only supported with rebase strategy",
		},
		{
			name: "outside worktree returns code 3",
			args: []string{"sync"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "false"},
			},
			expectedCode: int(errcode.ExitPreconditionRepo),
			errSubstr:    "Not a git repository",
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
			expectedCode: int(errcode.ExitPreconditionRepo),
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
			expectedCode: int(errcode.ExitPreconditionRepo),
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
			expectedCode: int(errcode.ExitPreconditionRepo),
			errSubstr:    "You must be on an 'issue', 'feature', or 'epic' branch to sync",
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
			expectedCode: int(errcode.ExitPreconditionRepo),
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
			expectedCode: int(errcode.ExitPreconditionRepo),
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
				"rev-list --left-right --count master...origin/master": {out: "1\t1"},
			},
			expectedCode: int(errcode.ExitConflict),
			errSubstr:    "Base branch 'master' is not up-to-date with 'origin/master'",
		},
		{
			name: "base branch with unpushed commits returns code 5",
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
				"rev-list --left-right --count master...origin/master": {out: "2\t0"},
			},
			expectedCode: int(errcode.ExitConflict),
			errSubstr:    "Base branch 'master' is not up-to-date with 'origin/master'",
		},
		{
			name: "clean base branch behind origin with --no-auto-update-base returns code 5",
			args: []string{"sync", "--no-auto-update-base", "master"},
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
			expectedCode: int(errcode.ExitConflict),
			errSubstr:    "Base branch 'master' is not up-to-date with 'origin/master'",
		},
		{
			name: "clean base branch behind origin automatically fast-forwards and rebases",
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
				"fetch origin master:master":                           {out: ""},
				"rebase master":                                        {out: ""},
			},
			expectedCode: int(errcode.ExitSuccess),
			expectStderr: "Fast-forwarded local 'master' to match 'origin/master' (2 commits behind)",
		},
		{
			name: "issue branch default sync logs rebase strategy and rebase success",
			args: []string{"sync"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                      {out: "true"},
				"rev-parse --show-toplevel":                            {out: "/repo"},
				"rev-parse --git-dir":                                  {out: "/repo/.git"},
				"rev-parse --is-shallow-repository":                    {out: "false"},
				"symbolic-ref --short -q HEAD":                         {out: "issue/VSB-100"},
				"rev-parse --short HEAD":                               {out: "abc1234"},
				"symbolic-ref --short refs/remotes/origin/HEAD":        {out: "origin/master"},
				"show-ref --verify --quiet refs/heads/master":          {out: ""},
				"remote get-url origin":                                {out: "https://remote.example.com/repo.git"},
				"fetch origin":                                         {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/master": {out: ""},
				"rev-list --left-right --count master...origin/master": {out: "0\t0"},
				"rebase master":                                        {out: "Successfully rebased"},
			},
			expectedCode: int(errcode.ExitSuccess),
			expectStderr: "Successfully rebased 'issue/VSB-100' onto 'master'",
		},
		{
			name: "feature branch explicit base logs merge strategy and merge success",
			args: []string{"sync", "main"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                       {out: "true"},
				"rev-parse --show-toplevel":                             {out: "/repo"},
				"rev-parse --git-dir":                                   {out: "/repo/.git"},
				"rev-parse --is-shallow-repository":                     {out: "false"},
				"symbolic-ref --short -q HEAD":                          {out: "feature/auth"},
				"rev-parse --short HEAD":                                {out: "abc1234"},
				"show-ref --verify --quiet refs/heads/main":             {out: ""},
				"remote get-url origin":                                 {out: "https://remote.example.com/repo.git"},
				"fetch origin":                                          {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/main":    {out: ""},
				"rev-list --left-right --count main...origin/main":      {out: "0\t0"},
				"merge -m Merge branch 'main' into 'feature/auth' main": {out: "Merge made by 'ort' strategy"},
			},
			expectedCode: int(errcode.ExitSuccess),
			expectStderr: "Successfully merged 'main' into 'feature/auth'",
		},
		{
			name: "issue branch already up to date logs up to date message",
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
				"rev-list --left-right --count master...origin/master": {out: "0\t0"},
				"rebase master":                                        {out: "Current branch issue/VSB-100 is up to date."},
			},
			expectedCode: int(errcode.ExitSuccess),
			expectStderr: "Branch 'issue/VSB-100' is already up to date with 'master'",
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
			if tt.expectStderr != "" {
				assert.Contains(t, stderr.String(), tt.expectStderr)
			}
		})
	}
}

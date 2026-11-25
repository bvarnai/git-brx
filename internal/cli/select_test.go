package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockGitRunner struct {
	responses map[string]mockGitResponse
	executed  []string
}

type mockGitResponse struct {
	out string
	err error
}

func (m *mockGitRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	m.executed = append(m.executed, key)
	if resp, ok := m.responses[key]; ok {
		return resp.out, resp.err
	}
	switch key {
	case "symbolic-ref -q HEAD":
		return "refs/heads/master", nil
	case "symbolic-ref --short -q HEAD":
		return "master", nil
	case "status --porcelain":
		return "", nil
	case "for-each-ref --format=%(refname:short) refs/heads refs/remotes/origin":
		return "", nil
	default:
		return "", errors.New("command not mocked: " + key)
	}
}

func (m *mockGitRunner) RunLines(ctx context.Context, dir string, args ...string) ([]string, error) {
	out, err := m.Run(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

func (m *mockGitRunner) RunWithEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error) {
	return m.Run(ctx, dir, args...)
}

func (m *mockGitRunner) RunStream(ctx context.Context, dir string, stdout, stderr io.Writer, extraEnv []string, args ...string) error {
	out, err := m.RunWithEnv(ctx, dir, extraEnv, args...)
	if out != "" && stdout != nil {
		fmt.Fprintln(stdout, out)
	}
	return err
}

func TestSelectCommand_UnitTable(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		mockResponses map[string]mockGitResponse
		expectedCode  int
		expectStderr  string
		expectStdout  string
	}{
		{
			name: "excess positional arguments returns usage error",
			args: []string{"select", "branch-a", "branch-b"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {out: "true"},
				"rev-parse --show-toplevel":       {out: "/repo"},
			},
			expectedCode: int(errcode.ExitUsageError),
			expectStderr: "[git-brx] Error: Unexpected argument: branch-b",
		},
		{
			name: "outside git repository returns precondition error",
			args: []string{"select"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {err: errors.New("fatal: not a git repo")},
			},
			expectedCode: int(errcode.ExitPreconditionRepo),
			expectStderr: "[git-brx] Error: Not a git repository",
		},
		{
			name: "branch not found locally or remotely returns code 3",
			args: []string{"select", "-o", "non-existent"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                            {out: "true"},
				"rev-parse --show-toplevel":                                  {out: "/repo"},
				"rev-parse --git-dir":                                        {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/non-existent":          {err: errors.New("not found")},
				"show-ref --verify --quiet refs/remotes/origin/non-existent": {err: errors.New("not found")},
			},
			expectedCode: int(errcode.ExitPreconditionRepo),
			expectStderr: "[git-brx] Error: Branch 'non-existent' not found",
		},
		{
			name: "switches to local branch in offline mode",
			args: []string{"select", "-o", "feature/test"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                            {out: "true"},
				"rev-parse --show-toplevel":                                  {out: "/repo"},
				"rev-parse --git-dir":                                        {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/feature/test":          {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/feature/test": {err: errors.New("not found")},
				"checkout feature/test":                                      {out: ""},
			},
			expectedCode: int(errcode.ExitSuccess),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			ctx := context.Background()

			runner := &mockGitRunner{responses: tc.mockResponses}
			app := cli.NewApp(&stdout, &stderr, runner)
			app.RootCmd.SetArgs(tc.args)

			err := app.Execute(ctx)
			code := int(errcode.ExitSuccess)
			if err != nil {
				var appErr *errcode.AppError
				if errors.As(err, &appErr) {
					app.Logger.Error("%s", appErr.Message)
					if appErr.Hint != "" {
						app.Logger.Hint("%s", appErr.Hint)
					}
					code = int(errcode.ExitCode(appErr))
				} else {
					app.Logger.Error("%v", err)
					code = int(errcode.ExitUsageError)
				}
			}

			assert.Equal(t, tc.expectedCode, code)
			if tc.expectStderr != "" {
				assert.Contains(t, stderr.String(), tc.expectStderr)
			}
			if tc.expectStdout != "" {
				assert.Contains(t, stdout.String(), tc.expectStdout)
			}
			if tc.name == "switches to local branch in offline mode" {
				require.Contains(t, runner.executed, "checkout feature/test")
			}
		})
	}
}

func TestSelectCommand_FastForwardAndHints(t *testing.T) {
	t.Run("clean branch behind origin auto-fast-forwards on switch", func(t *testing.T) {
		runner := &mockGitRunner{
			responses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                        {out: "true"},
				"rev-parse --show-toplevel":                              {out: "/repo"},
				"rev-parse --git-dir":                                    {out: "/repo/.git"},
				"remote get-url origin":                                  {out: "https://github.com/org/repo.git"},
				"fetch origin":                                           {out: ""},
				"show-ref --verify --quiet refs/heads/epic/10":           {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/epic/10":  {out: ""},
				"checkout epic/10":                                       {out: ""},
				"rev-list --left-right --count epic/10...origin/epic/10": {out: "0\t2"},
				"merge --ff-only origin/epic/10":                         {out: ""},
			},
		}
		var stdout, stderr bytes.Buffer
		app := cli.NewApp(&stdout, &stderr, runner)
		app.RootCmd.SetArgs([]string{"select", "epic/10"})

		require.NoError(t, app.Execute(context.Background()))
		assert.Contains(t, runner.executed, "merge --ff-only origin/epic/10")
		assert.Contains(t, stderr.String(), "Fast-forwarded 'epic/10' to match 'origin/epic/10' (2 new commits)")
	})

	t.Run("--no-ff skips fast-forward and emits update hint", func(t *testing.T) {
		runner := &mockGitRunner{
			responses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                        {out: "true"},
				"rev-parse --show-toplevel":                              {out: "/repo"},
				"rev-parse --git-dir":                                    {out: "/repo/.git"},
				"remote get-url origin":                                  {out: "https://github.com/org/repo.git"},
				"fetch origin":                                           {out: ""},
				"show-ref --verify --quiet refs/heads/epic/10":           {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/epic/10":  {out: ""},
				"checkout epic/10":                                       {out: ""},
				"rev-list --left-right --count epic/10...origin/epic/10": {out: "0\t2"},
			},
		}
		var stdout, stderr bytes.Buffer
		app := cli.NewApp(&stdout, &stderr, runner)
		app.RootCmd.SetArgs([]string{"select", "--no-ff", "epic/10"})

		require.NoError(t, app.Execute(context.Background()))
		assert.NotContains(t, runner.executed, "merge --ff-only origin/epic/10")
		assert.Contains(t, stderr.String(), "Run 'git-brx update' to update.")
	})

	t.Run("diverged work or epic branch emits update hint instead of sync", func(t *testing.T) {
		runner := &mockGitRunner{
			responses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                        {out: "true"},
				"rev-parse --show-toplevel":                              {out: "/repo"},
				"rev-parse --git-dir":                                    {out: "/repo/.git"},
				"symbolic-ref --short -q HEAD":                           {out: "epic/10"},
				"remote get-url origin":                                  {out: "https://github.com/org/repo.git"},
				"fetch origin":                                           {out: ""},
				"show-ref --verify --quiet refs/heads/epic/10":           {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/epic/10":  {out: ""},
				"rev-list --left-right --count epic/10...origin/epic/10": {out: "1\t1"},
			},
		}
		var stdout, stderr bytes.Buffer
		app := cli.NewApp(&stdout, &stderr, runner)
		app.RootCmd.SetArgs([]string{"select", "epic/10"})

		require.NoError(t, app.Execute(context.Background()))
		assert.Contains(t, stderr.String(), "Run 'git-brx update' to update.")
		assert.NotContains(t, stderr.String(), "git-brx sync")
	})
}

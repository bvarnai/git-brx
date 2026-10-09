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
	"github.com/bvarnai/git-brx/internal/domain"
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
			expectedCode: int(domain.ExitUsageError),
			expectStderr: "[git-brx] ! Unexpected argument: branch-b",
		},
		{
			name: "outside git repository returns precondition error",
			args: []string{"select"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree": {err: errors.New("fatal: not a git repo")},
			},
			expectedCode: int(domain.ExitPreconditionRepo),
			expectStderr: "[git-brx] ! Awh! This is not a git repository",
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
			expectedCode: int(domain.ExitPreconditionRepo),
			expectStderr: "[git-brx] ! Branch 'non-existent' not found",
		},
		{
			name: "dry run logs planned checkout without executing checkout",
			args: []string{"select", "--dry-run", "-o", "feature/test"},
			mockResponses: map[string]mockGitResponse{
				"rev-parse --is-inside-work-tree":                            {out: "true"},
				"rev-parse --show-toplevel":                                  {out: "/repo"},
				"rev-parse --git-dir":                                        {out: "/repo/.git"},
				"show-ref --verify --quiet refs/heads/feature/test":          {out: ""},
				"show-ref --verify --quiet refs/remotes/origin/feature/test": {err: errors.New("not found")},
			},
			expectedCode: int(domain.ExitSuccess),
			expectStderr: "[git-brx] Would checkout branch 'feature/test'",
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
			code := int(domain.ExitSuccess)
			if err != nil {
				var appErr *domain.AppError
				if errors.As(err, &appErr) {
					app.UI.Error("%s", appErr.Message)
					if appErr.Hint != "" {
						app.UI.Hint("%s", appErr.Hint)
					}
					code = int(appErr.Code)
				} else {
					app.UI.Error("%v", err)
					code = int(domain.ExitUsageError)
				}
			}

			assert.Equal(t, tc.expectedCode, code)
			if tc.expectStderr != "" {
				assert.Contains(t, stderr.String(), tc.expectStderr)
			}
			if tc.expectStdout != "" {
				assert.Contains(t, stdout.String(), tc.expectStdout)
			}
			if tc.name == "dry run logs planned checkout without executing checkout" {
				for _, execCmd := range runner.executed {
					require.False(t, strings.HasPrefix(execCmd, "checkout"), "checkout should not be called in dry run")
				}
			}
		})
	}
}

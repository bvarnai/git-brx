package checks_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/git"
	"github.com/bvarnai/git-brx/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	responses map[string]string
	errors    map[string]error
}

func (m *mockRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	if err, ok := m.errors[key]; ok {
		return "", err
	}
	if out, ok := m.responses[key]; ok {
		return out, nil
	}
	return "", errors.New("command not mocked: " + key)
}

func (m *mockRunner) RunLines(ctx context.Context, dir string, args ...string) ([]string, error) {
	out, err := m.Run(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

func (m *mockRunner) RunWithEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error) {
	return m.Run(ctx, dir, args...)
}

func TestCheckInProgress(t *testing.T) {
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	runner := &mockRunner{
		responses: map[string]string{
			"rev-parse --git-dir": gitDir,
		},
	}
	inspector := git.NewInspector(runner)

	// Clean state
	err := checks.CheckInProgress(context.Background(), inspector, tempDir)
	assert.NoError(t, err)

	// In-progress cherry-pick
	cherryPickHead := filepath.Join(gitDir, "CHERRY_PICK_HEAD")
	require.NoError(t, os.WriteFile(cherryPickHead, []byte("commit-sha"), 0644))
	err = checks.CheckInProgress(context.Background(), inspector, tempDir)
	assert.Error(t, err)
	var appErr *domain.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ExitConflict, appErr.Code)
	assert.Contains(t, appErr.Message, "active cherry-pick")
	assert.Contains(t, appErr.Hint, "git cherry-pick --abort")

	// Clean cherry-pick, test merge
	os.Remove(cherryPickHead)
	mergeHead := filepath.Join(gitDir, "MERGE_HEAD")
	require.NoError(t, os.WriteFile(mergeHead, []byte("commit-sha"), 0644))
	err = checks.CheckInProgress(context.Background(), inspector, tempDir)
	assert.Error(t, err)
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ExitConflict, appErr.Code)
	assert.Contains(t, appErr.Message, "active merge")
}

func TestCheckDetachedHead(t *testing.T) {
	t.Run("attached HEAD does not error", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"symbolic-ref -q HEAD": "refs/heads/master",
			},
		}
		inspector := git.NewInspector(runner)
		err := checks.CheckDetachedHead(context.Background(), inspector, "/repo")
		assert.NoError(t, err)
	})

	t.Run("detached HEAD without orphans passes", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"rev-list -n 1 HEAD --not --branches --remotes": "",
			},
			errors: map[string]error{
				"symbolic-ref -q HEAD": errors.New("detached"),
			},
		}
		inspector := git.NewInspector(runner)
		err := checks.CheckDetachedHead(context.Background(), inspector, "/repo")
		assert.NoError(t, err)
	})

	t.Run("detached HEAD with unreferenced commits blocks switch", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"rev-list -n 1 HEAD --not --branches --remotes": "abc1234",
			},
			errors: map[string]error{
				"symbolic-ref -q HEAD": errors.New("detached"),
			},
		}
		inspector := git.NewInspector(runner)
		err := checks.CheckDetachedHead(context.Background(), inspector, "/repo")
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.ExitConflict, appErr.Code)
		assert.Contains(t, appErr.Message, "detached HEAD with unreferenced commits")
		assert.Contains(t, appErr.Hint, "Save your work into a branch first")
	})
}

func TestCheckDirtyWorktree(t *testing.T) {
	var stdout, stderr bytes.Buffer
	u := ui.New(&stdout, &stderr, true, false, false)

	// Switched branches with dirty files
	checks.CheckDirtyWorktree(u, true, "feature/old", "feature/new")
	assert.Contains(t, stderr.String(), "Warning: You have uncommitted local changes that were carried over to 'feature/new'.")
	assert.Contains(t, stderr.String(), "Hint: If this was unintentional, run 'git-brx select feature/old'")

	// Same branch: no warning
	stderr.Reset()
	checks.CheckDirtyWorktree(u, true, "master", "master")
	assert.Empty(t, stderr.String())

	// Clean worktree: no warning
	stderr.Reset()
	checks.CheckDirtyWorktree(u, false, "feature/old", "feature/new")
	assert.Empty(t, stderr.String())
}

func TestCheckUpstreamSync(t *testing.T) {
	var stdout, stderr bytes.Buffer
	u := ui.New(&stdout, &stderr, true, false, false)

	runner := &mockRunner{
		responses: map[string]string{
			"show-ref --verify --quiet refs/heads/master":          "",
			"show-ref --verify --quiet refs/remotes/origin/master": "",
			"rev-list --left-right --count master...origin/master": "0\t3",
		},
	}
	inspector := git.NewInspector(runner)

	checks.CheckUpstreamSync(context.Background(), inspector, u, "/repo", "master")
	assert.Contains(t, stderr.String(), "Hint: Your branch is behind 'origin/master' by 3 commit(s). Run 'git-brx sync' to update.")
}

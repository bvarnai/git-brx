package checks_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/checks"
	"github.com/bvarnai/git-brx/internal/domain"
	"github.com/bvarnai/git-brx/internal/git"
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

func TestInsideWorkTree(t *testing.T) {
	t.Run("valid work tree returns root", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"rev-parse --is-inside-work-tree": "true",
				"rev-parse --show-toplevel":       "/repo/root",
			},
		}
		inspector := git.NewInspector(runner)
		root, err := checks.InsideWorkTree(context.Background(), inspector, "/repo/root/sub")
		require.NoError(t, err)
		assert.Equal(t, "/repo/root", root)
	})

	t.Run("outside work tree returns domain ExitPreconditionRepo error", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"rev-parse --is-inside-work-tree": "false",
			},
		}
		inspector := git.NewInspector(runner)
		_, err := checks.InsideWorkTree(context.Background(), inspector, "/outside")
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.ExitPreconditionRepo, appErr.Code)
		assert.Contains(t, appErr.Message, "Awh! This is not a git repository")
	})
}

func TestNoActiveOperation(t *testing.T) {
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
	err := checks.NoActiveOperation(context.Background(), inspector, tempDir)
	assert.NoError(t, err)

	// In-progress cherry-pick
	cherryPickHead := filepath.Join(gitDir, "CHERRY_PICK_HEAD")
	require.NoError(t, os.WriteFile(cherryPickHead, []byte("commit-sha"), 0644))
	err = checks.NoActiveOperation(context.Background(), inspector, tempDir)
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
	err = checks.NoActiveOperation(context.Background(), inspector, tempDir)
	assert.Error(t, err)
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, domain.ExitConflict, appErr.Code)
	assert.Contains(t, appErr.Message, "active merge")
}

func TestNoOrphanedCommits(t *testing.T) {
	t.Run("attached HEAD does not error", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"symbolic-ref -q HEAD": "refs/heads/master",
			},
		}
		inspector := git.NewInspector(runner)
		err := checks.NoOrphanedCommits(context.Background(), inspector, "/repo")
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
		err := checks.NoOrphanedCommits(context.Background(), inspector, "/repo")
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
		err := checks.NoOrphanedCommits(context.Background(), inspector, "/repo")
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.ExitConflict, appErr.Code)
		assert.Contains(t, appErr.Message, "detached HEAD with unreferenced commits")
		assert.Contains(t, appErr.Hint, "Save your work into a branch first")
	})
}

func TestNotShallow(t *testing.T) {
	t.Run("non-shallow repo passes", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"rev-parse --is-shallow-repository": "false",
			},
		}
		inspector := git.NewInspector(runner)
		err := checks.NotShallow(context.Background(), inspector, "/repo")
		assert.NoError(t, err)
	})

	t.Run("shallow repo returns ExitPreconditionRepo", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"rev-parse --is-shallow-repository": "true",
			},
		}
		inspector := git.NewInspector(runner)
		err := checks.NotShallow(context.Background(), inspector, "/repo")
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.ExitPreconditionRepo, appErr.Code)
		assert.Contains(t, appErr.Message, "shallow repository")
	})
}

func TestAttachedBranch(t *testing.T) {
	t.Run("attached branch returns branch record", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"symbolic-ref --short -q HEAD": "feature/my-feat",
				"rev-parse --short HEAD":       "abc1234",
				"for-each-ref --format=%(upstream:short) refs/heads/feature/my-feat": "origin/feature/my-feat",
			},
		}
		inspector := git.NewInspector(runner)
		rec, err := checks.AttachedBranch(context.Background(), inspector, "/repo")
		require.NoError(t, err)
		assert.Equal(t, "feature/my-feat", rec.Name)
		assert.False(t, rec.IsDetached)
	})

	t.Run("detached HEAD returns ExitPreconditionRepo", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"rev-parse --short HEAD": "abc1234",
			},
			errors: map[string]error{
				"symbolic-ref --short -q HEAD": errors.New("detached"),
			},
		}
		inspector := git.NewInspector(runner)
		rec, err := checks.AttachedBranch(context.Background(), inspector, "/repo")
		assert.Nil(t, rec)
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.ExitPreconditionRepo, appErr.Code)
		assert.Contains(t, appErr.Message, "detached HEAD")
	})
}

func TestHasOrigin(t *testing.T) {
	t.Run("origin configured passes", func(t *testing.T) {
		runner := &mockRunner{
			responses: map[string]string{
				"remote get-url origin": "https://remote.example.com/repo.git",
			},
		}
		inspector := git.NewInspector(runner)
		err := checks.HasOrigin(context.Background(), inspector, "/repo")
		assert.NoError(t, err)
	})

	t.Run("origin missing returns ExitPreconditionRemote", func(t *testing.T) {
		runner := &mockRunner{
			errors: map[string]error{
				"remote get-url origin": errors.New("fatal: No such remote 'origin'"),
			},
		}
		inspector := git.NewInspector(runner)
		err := checks.HasOrigin(context.Background(), inspector, "/repo")
		assert.Error(t, err)
		var appErr *domain.AppError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, domain.ExitPreconditionRemote, appErr.Code)
		assert.Contains(t, appErr.Message, "no remote origin")
	})
}

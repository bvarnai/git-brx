package git_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bvarnai/git-brx/internal/command"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/bvarnai/git-brx/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizedEnv(t *testing.T) {
	base := []string{
		"PATH=/bin:/usr/bin",
		"LC_ALL=en_US.UTF-8",
		"LANG=fr_FR.UTF-8",
		"GIT_TERMINAL_PROMPT=1",
		"GIT_ADVICE=1",
		"CUSTOM_VAR=value",
	}

	env := command.SanitizedEnv(base, []string{"EXTRA=1"})

	// Check overrides
	assert.Contains(t, env, "LC_ALL=C")
	assert.Contains(t, env, "LANG=C")
	assert.Contains(t, env, "GIT_TERMINAL_PROMPT=0")
	assert.Contains(t, env, "GIT_ADVICE=0")
	assert.Contains(t, env, "GIT_CONFIG_COUNT=1")
	assert.Contains(t, env, "GIT_CONFIG_KEY_0=gc.auto")
	assert.Contains(t, env, "GIT_CONFIG_VALUE_0=0")
	assert.Contains(t, env, "PATH=/bin:/usr/bin")
	assert.Contains(t, env, "CUSTOM_VAR=value")
	assert.Contains(t, env, "EXTRA=1")

	// Ensure old conflicting values were filtered
	assert.NotContains(t, env, "LC_ALL=en_US.UTF-8")
	assert.NotContains(t, env, "LANG=fr_FR.UTF-8")
	assert.NotContains(t, env, "GIT_TERMINAL_PROMPT=1")
	assert.NotContains(t, env, "GIT_ADVICE=1")
}

func TestExecRunner_RunAndCancel(t *testing.T) {
	runner := git.NewExecRunner()
	ctx := context.Background()

	// Simple version command
	out, err := runner.Run(ctx, "", "--version")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, "git version"))

	// Context cancellation
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = runner.Run(cancelCtx, "", "status")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errcode.ErrInterrupted))

	// RunLines
	lines, err := runner.RunLines(ctx, "", "--version")
	require.NoError(t, err)
	assert.Len(t, lines, 1)

	// RunLines on empty output
	emptyLines, err := runner.RunLines(ctx, "", "status", "--porcelain")
	require.NoError(t, err)
	// clean repo or empty porcelain returns nil
	if len(emptyLines) == 0 {
		assert.Nil(t, emptyLines)
	}

	// Timeout
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer timeoutCancel()
	time.Sleep(5 * time.Millisecond)
	_, err = runner.Run(timeoutCtx, "", "status")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errcode.ErrInterrupted))
}

func TestExecRunner_MissingBinary(t *testing.T) {
	runner := &git.ExecRunner{
		BinaryPath: "nonexistent-git-binary-xyz",
	}
	ctx := context.Background()
	_, err := runner.Run(ctx, "", "--version")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errcode.ErrTransport))
	assert.Contains(t, err.Error(), `Required binary "nonexistent-git-binary-xyz" not found in $PATH`)
}

type stubRunner struct {
	runFunc func(args ...string) (string, error)
}

func (s *stubRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	return s.runFunc(args...)
}

func (s *stubRunner) RunLines(_ context.Context, _ string, _ ...string) ([]string, error) {
	return nil, nil
}

func (s *stubRunner) RunWithEnv(_ context.Context, _ string, _ []string, args ...string) (string, error) {
	return s.runFunc(args...)
}

func (s *stubRunner) RunStream(_ context.Context, _ string, _, _ any, _ []string, _ ...string) error {
	return nil
}

func TestInspector_HasConflictMarkers(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, root string) string
		wantMarkers   bool
		wantScannable bool
	}{
		{
			name: "clean text file returns false and scannable",
			setup: func(t *testing.T, root string) string {
				require.NoError(t, os.WriteFile(filepath.Join(root, "clean.txt"), []byte("hello\nworld\n"), 0644))
				return "clean.txt"
			},
			wantMarkers:   false,
			wantScannable: true,
		},
		{
			name: "standard conflict markers return true and scannable",
			setup: func(t *testing.T, root string) string {
				content := "before\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> main\nafter\n"
				require.NoError(t, os.WriteFile(filepath.Join(root, "conflict.txt"), []byte(content), 0644))
				return "conflict.txt"
			},
			wantMarkers:   true,
			wantScannable: true,
		},
		{
			name: "diff3 base marker returns true and scannable",
			setup: func(t *testing.T, root string) string {
				content := "<<<<<<< HEAD\nours\n||||||| base\ncommon\n=======\ntheirs\n>>>>>>> main\n"
				require.NoError(t, os.WriteFile(filepath.Join(root, "diff3.txt"), []byte(content), 0644))
				return "diff3.txt"
			},
			wantMarkers:   true,
			wantScannable: true,
		},
		{
			name: "CRLF conflict separator returns true and scannable",
			setup: func(t *testing.T, root string) string {
				content := "<<<<<<< HEAD\r\nours\r\n=======\r\ntheirs\r\n>>>>>>> main\r\n"
				require.NoError(t, os.WriteFile(filepath.Join(root, "crlf.txt"), []byte(content), 0644))
				return "crlf.txt"
			},
			wantMarkers:   true,
			wantScannable: true,
		},
		{
			name: "missing deleted file returns non-scannable",
			setup: func(_ *testing.T, _ string) string {
				return "deleted.txt"
			},
			wantMarkers:   false,
			wantScannable: false,
		},
		{
			name: "binary file with NUL byte returns non-scannable",
			setup: func(t *testing.T, root string) string {
				require.NoError(t, os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0x01, 0x00, 0x02}, 0644))
				return "bin.dat"
			},
			wantMarkers:   false,
			wantScannable: false,
		},
	}

	inspector := git.NewInspector(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			relPath := tt.setup(t, root)
			hasMarkers, scannable, err := inspector.HasConflictMarkers(root, relPath)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMarkers, hasMarkers)
			assert.Equal(t, tt.wantScannable, scannable)
		})
	}
}

func TestInspector_MergeToolConfigured(t *testing.T) {
	t.Run("configured merge tool returns name and true", func(t *testing.T) {
		runner := git.NewExecRunner()
		dir := t.TempDir()
		_, err := runner.Run(context.Background(), dir, "init")
		require.NoError(t, err)
		_, err = runner.Run(context.Background(), dir, "config", "merge.tool", "vimdiff")
		require.NoError(t, err)

		inspector := git.NewInspector(runner)
		tool, ok := inspector.MergeToolConfigured(context.Background(), dir)
		assert.True(t, ok)
		assert.Equal(t, "vimdiff", tool)
	})

	t.Run("unset merge tool returns empty and false", func(t *testing.T) {
		dir := t.TempDir()
		runner := git.NewExecRunner()
		_, err := runner.Run(context.Background(), dir, "init")
		require.NoError(t, err)

		inspector := git.NewInspector(runner)
		// Isolate from host ~/.gitconfig by checking a nonexistent HOME
		t.Setenv("HOME", dir)
		tool, ok := inspector.MergeToolConfigured(context.Background(), dir)
		assert.False(t, ok)
		assert.Empty(t, tool)
	})
}

func TestInspector_BranchBaseAndSetBranchBase(t *testing.T) {
	ctx := context.Background()
	runner := git.NewExecRunner()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	_, err := runner.Run(ctx, dir, "init", "-b", "main")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "config", "user.name", "Test")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "config", "user.email", "test@example.com")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	_, err = runner.Run(ctx, dir, "add", "a.txt")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "commit", "-m", "init")
	require.NoError(t, err)

	ops := git.NewOperations(runner)
	inspector := git.NewInspector(runner)

	require.NoError(t, ops.CreateAndCheckout(ctx, dir, "issue/44"))
	require.NoError(t, ops.SetBranchBase(ctx, dir, "issue/44", "epic/10"))

	assert.Equal(t, "epic/10", inspector.BranchBase(ctx, dir, "issue/44"))
	curr, err := inspector.CurrentBranch(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, "epic/10", curr.BaseBranch)

	// Deleting branch cleans up branch.<name>.brx-base automatically
	require.NoError(t, ops.Checkout(ctx, dir, "main"))
	_, err = runner.Run(ctx, dir, "branch", "-D", "issue/44")
	require.NoError(t, err)
	assert.Equal(t, "", inspector.BranchBase(ctx, dir, "issue/44"))
}

func TestInspector_IsBranchCheckedOutInOtherWorktree(t *testing.T) {
	ctx := context.Background()
	runner := git.NewExecRunner()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	_, err := runner.Run(ctx, dir, "init", "-b", "main")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "config", "user.name", "Test")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "config", "user.email", "test@example.com")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	_, err = runner.Run(ctx, dir, "add", "a.txt")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "commit", "-m", "init")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "branch", "epic/10")
	require.NoError(t, err)

	linkedDir := filepath.Join(t.TempDir(), "wt-epic")
	_, err = runner.Run(ctx, dir, "worktree", "add", linkedDir, "epic/10")
	require.NoError(t, err)

	inspector := git.NewInspector(runner)
	assert.True(t, inspector.IsBranchCheckedOutInOtherWorktree(ctx, dir, "epic/10"))
	assert.False(t, inspector.IsBranchCheckedOutInOtherWorktree(ctx, dir, "main"))
}

func TestInspector_RemoteBranchSHAAndChangedFilesRemoteTarget(t *testing.T) {
	ctx := context.Background()
	runner := git.NewExecRunner()
	dir := t.TempDir()
	remote := t.TempDir()
	t.Setenv("HOME", dir)

	_, err := runner.Run(ctx, remote, "init", "--bare")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "init", "-b", "main")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "config", "user.name", "Test")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "config", "user.email", "test@example.com")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "remote", "add", "origin", remote)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0644))
	_, err = runner.Run(ctx, dir, "add", "init.txt")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "commit", "-m", "init")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "push", "-u", "origin", "main")
	require.NoError(t, err)

	ops := git.NewOperations(runner)
	inspector := git.NewInspector(runner)

	require.NoError(t, ops.CreateAndCheckout(ctx, dir, "epic/remote-10"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "epic.txt"), []byte("epic"), 0644))
	_, err = runner.Run(ctx, dir, "add", "epic.txt")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "commit", "-m", "epic base")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "push", "-u", "origin", "epic/remote-10")
	require.NoError(t, err)

	headSHA, err := inspector.HeadSHA(ctx, dir)
	require.NoError(t, err)
	remoteSHA, err := inspector.RemoteBranchSHA(ctx, dir, "epic/remote-10")
	require.NoError(t, err)
	assert.Equal(t, headSHA, remoteSHA)

	missingSHA, err := inspector.RemoteBranchSHA(ctx, dir, "epic/nonexistent")
	require.NoError(t, err)
	assert.Empty(t, missingSHA)

	// Fork child branch, then delete local epic/remote-10 so only origin/epic/remote-10 remains
	require.NoError(t, ops.CreateAndCheckout(ctx, dir, "issue/child"))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "ui"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal", "ui", "view.go"), []byte("package ui"), 0644))
	_, err = runner.Run(ctx, dir, "add", "internal/ui/view.go")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "commit", "-m", "add ui view")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "branch", "-D", "epic/remote-10")
	require.NoError(t, err)

	changed, err := inspector.ChangedFiles(ctx, dir, "epic/remote-10")
	require.NoError(t, err)
	assert.Equal(t, []string{"internal/ui/view.go"}, changed)
}

func TestOperations_DeleteRemoteBranchAndCheckoutDetach(t *testing.T) {
	ctx := context.Background()
	runner := git.NewExecRunner()
	dir := t.TempDir()
	remote := t.TempDir()
	t.Setenv("HOME", dir)

	_, err := runner.Run(ctx, remote, "init", "--bare")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "init", "-b", "main")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "config", "user.name", "Test")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "config", "user.email", "test@example.com")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "remote", "add", "origin", remote)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0644))
	_, err = runner.Run(ctx, dir, "add", "init.txt")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "commit", "-m", "init")
	require.NoError(t, err)
	_, err = runner.Run(ctx, dir, "push", "-u", "origin", "main")
	require.NoError(t, err)

	ops := git.NewOperations(runner)
	inspector := git.NewInspector(runner)

	require.NoError(t, ops.CreateAndCheckout(ctx, dir, "issue/del-ops"))
	_, err = runner.Run(ctx, dir, "push", "-u", "origin", "issue/del-ops")
	require.NoError(t, err)

	require.NoError(t, ops.CheckoutDetach(ctx, dir, "main"))
	curr, err := inspector.CurrentBranch(ctx, dir)
	require.NoError(t, err)
	assert.True(t, curr.IsDetached)

	require.NoError(t, ops.DeleteRemoteBranch(ctx, dir, "origin", "issue/del-ops"))
	sha, err := inspector.RemoteBranchSHA(ctx, dir, "issue/del-ops")
	require.NoError(t, err)
	assert.Empty(t, sha)
}

package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/errcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectIntegration(t *testing.T) {
	t.Run("DefaultMaster", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("feature/test-default")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Selecting 'master' branch by default")
		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "master", currentBranch)
	})

	t.Run("SwitchExistingLocalBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("feature/alpha")
		h.Git("checkout", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o", "feature/alpha"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "feature/alpha", currentBranch)
	})

	t.Run("RemoteTrackingBranch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Create branch, push to origin, delete local branch
		h.CreateBranch("feature/remote-only")
		h.CommitFile("remote.txt", "remote", "Remote commit")
		h.Git("push", "-u", "origin", "feature/remote-only")
		h.Git("checkout", "master")
		h.Git("branch", "-D", "feature/remote-only")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "feature/remote-only"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "feature/remote-only", currentBranch)
	})

	t.Run("OfflineFlagSkipsFetch", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("feature/offline")
		h.Git("checkout", "master")
		h.Git("remote", "add", "origin", "http://127.0.0.1:65534/dummy.git")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o", "feature/offline"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "feature/offline", currentBranch)
	})

	t.Run("RemoteFetchFallbackToLocal", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("feature/fallback")
		h.Git("checkout", "master")
		h.Git("remote", "add", "origin", "http://127.0.0.1:65534/dummy.git")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "feature/fallback"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Warning: Unable to reach remote; falling back to local references")
		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "feature/fallback", currentBranch)
	})

	t.Run("RemoteFetchFailsAndBranchNotFound", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.Git("remote", "add", "origin", "http://127.0.0.1:65534/dummy.git")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "nonexistent-branch"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRemote), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: Unable to fetch from origin")
	})

	t.Run("BranchNotFound", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o", "nonexistent"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: Branch 'nonexistent' not found")
	})

	t.Run("ActiveRebaseBlocksSelect", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		err := os.MkdirAll(filepath.Join(h.RepoDir, ".git", "rebase-merge"), 0755)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "master"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitConflict), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: Cannot switch branches during an active rebase")
	})

	t.Run("ActiveMergeBlocksSelect", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		err := os.WriteFile(filepath.Join(h.RepoDir, ".git", "MERGE_HEAD"), []byte("deadbeef\n"), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "master"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitConflict), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: Cannot switch branches during an active merge")
	})

	t.Run("DirtyWorkingTreeConflict", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("file.txt", "version 1", "Initial file")
		h.CreateBranch("feature/dirty")
		h.CommitFile("file.txt", "version 2", "Update file on branch")
		h.Git("checkout", "master")

		// Create uncommitted local conflict
		conflictPath := filepath.Join(h.RepoDir, "file.txt")
		err := os.WriteFile(conflictPath, []byte("uncommitted change in master"), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o", "feature/dirty"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitConflict), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: Local modifications would be overwritten by checkout")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: Commit or stash your changes before switching branches")
	})

	t.Run("ExcessArguments", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "arg1", "arg2"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitUsageError), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: Unexpected argument: arg2")
	})

	t.Run("AutoDetectMainDefault", func(t *testing.T) {
		repoDir := t.TempDir()
		h := &Harness{T: t, RepoDir: repoDir}
		h.Git("init", "-b", "main")
		h.Git("config", "user.name", "Test User")
		h.Git("config", "user.email", "test@example.com")
		h.Git("config", "commit.gpgsign", "false")
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("feature/temp")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Selecting 'main' branch by default")
		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "main", currentBranch)
	})

	t.Run("DetachedHeadWithOrphanedCommits", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		// Detach HEAD
		h.Git("checkout", "HEAD~0")
		// Commit file while detached
		h.CommitFile("detached.txt", "detached work", "Commit on detached HEAD")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o", "master"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitConflict), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: You are on a detached HEAD with unreferenced commits")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: Save your work into a branch first: 'git branch <branch-name>'")
	})

	t.Run("InProgressCherryPickAndRevert", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")

		// Active cherry-pick
		cherryPickHead := filepath.Join(h.RepoDir, ".git", "CHERRY_PICK_HEAD")
		require.NoError(t, os.WriteFile(cherryPickHead, []byte("sha\n"), 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o", "master"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitConflict), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: Cannot switch branches during an active cherry-pick")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: Abort with 'git cherry-pick --abort'")

		// Active revert
		require.NoError(t, os.Remove(cherryPickHead))
		revertHead := filepath.Join(h.RepoDir, ".git", "REVERT_HEAD")
		require.NoError(t, os.WriteFile(revertHead, []byte("sha\n"), 0644))

		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o", "master"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitConflict), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: Cannot switch branches during an active revert")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: Abort with 'git revert --abort'")
	})

	t.Run("AlreadyOnBranch_AutoFastForwardsWhenCleanAndNoFFSkips", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteDir := h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Push a new commit from another clone
		secondRepo := t.TempDir()
		h2 := &Harness{T: t, RepoDir: secondRepo}
		h2.Git("clone", remoteDir, secondRepo)
		h2.Git("config", "user.name", "Test User")
		h2.Git("config", "user.email", "test@example.com")
		h2.CommitFile("remote_update.txt", "update", "Commit from second repo")
		h2.Git("push", "origin", "master")
		remoteSha := strings.TrimSpace(h2.Git("rev-parse", "HEAD"))

		// --no-ff skips fast-forward and emits update hint
		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "--no-ff", "master"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Already on 'master'")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: Your branch is behind 'origin/master' by 1 commit(s). Run 'git-brx update' to update.")

		// Default select fast-forwards clean branch automatically
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "master"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Fast-forwarded 'master' to match 'origin/master' (1 new commit)")
		assert.Equal(t, remoteSha, strings.TrimSpace(h.Git("rev-parse", "HEAD")))
	})

	t.Run("DirtyChangesCarriedOverWarning", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("base.txt", "base", "Base commit")
		h.CreateBranch("feature/other")
		h.Git("checkout", "master")

		// Uncommitted local changes that do not collide
		err := os.WriteFile(filepath.Join(h.RepoDir, "uncommitted.txt"), []byte("draft"), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o", "feature/other"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "[git-brx] Warning: You have uncommitted local changes that were carried over to 'feature/other'.")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: If this was unintentional, run 'git-brx select master' and commit or stash first.")
		currentBranch := strings.TrimSpace(h.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "feature/other", currentBranch)
	})

	t.Run("FuzzySuggestionOnTypo", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.CreateBranch("feature/login")
		h.Git("checkout", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "-o", "featrue/login"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRepo), code)
		assert.Contains(t, stderr.String(), "[git-brx] Error: Branch 'featrue/login' not found")
		assert.Contains(t, stderr.String(), "[git-brx] Hint: Did you mean 'feature/login'?")
	})

	t.Run("SwitchAutoFastForwardsAndHandlesDivergedAndDirtyBranches", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		remoteDir := h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("epic/10")
		h.CommitFile("epic1.txt", "1", "Epic 1")
		h.Git("push", "-u", "origin", "epic/10")

		h.CreateBranch("issue/20")
		h.CommitFile("issue1.txt", "1", "Issue 1")
		h.Git("push", "-u", "origin", "issue/20")
		h.Git("checkout", "master")

		// Push ahead on epic/10 and issue/20 from a second clone
		secondRepo := t.TempDir()
		h2 := &Harness{T: t, RepoDir: secondRepo}
		h2.Git("clone", remoteDir, secondRepo)
		h2.Git("config", "user.name", "Test User")
		h2.Git("config", "user.email", "test@example.com")
		h2.Git("checkout", "epic/10")
		h2.CommitFile("epic2.txt", "2", "Epic 2 from remote")
		h2.Git("push", "origin", "epic/10")
		remoteEpicSha := strings.TrimSpace(h2.Git("rev-parse", "HEAD"))

		h2.Git("checkout", "issue/20")
		h2.CommitFile("issue2.txt", "2", "Issue 2 from remote")
		h2.Git("push", "origin", "issue/20")

		// TC-SEL-01: clean switch to epic/10 auto-fast-forwards
		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "epic/10"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Fast-forwarded 'epic/10' to match 'origin/epic/10' (1 new commit)")
		assert.Equal(t, remoteEpicSha, strings.TrimSpace(h.Git("rev-parse", "HEAD")))

		// TC-SEL-04: dirty worktree switching to behind issue/20 skips FF and warns
		require.NoError(t, os.WriteFile(filepath.Join(h.RepoDir, "dirty.txt"), []byte("wip"), 0644))
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"select", "issue/20"}, &stdout, &stderr)
		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "uncommitted local changes that were carried over")
		assert.Contains(t, stderr.String(), "Run 'git-brx update' to update.")
		assert.NotContains(t, stderr.String(), "Fast-forwarded")
	})
}

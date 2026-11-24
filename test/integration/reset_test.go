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

func TestResetIntegration(t *testing.T) {
	t.Run("CleanResetBackToOrigin", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/VSB-300")
		h.CommitFile("published.txt", "pub", "Published commit")
		h.Git("push", "-u", "origin", "issue/VSB-300")
		origSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))

		// Make unwanted local commits
		h.CommitFile("unwanted.txt", "unwanted", "Unwanted commit")
		currSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))
		assert.NotEqual(t, origSha, currSha)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Working tree successfully reset to 'origin/issue/VSB-300'")

		resetSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))
		assert.Equal(t, origSha, resetSha)
	})

	t.Run("TwoStepResetAbortsMergeThenResetsToOrigin", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("shared.txt", "init\n", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/merge-abort")
		h.CommitFile("shared.txt", "branch line\n", "Published branch commit")
		h.Git("push", "-u", "origin", "feature/merge-abort")
		originSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))

		// Add unpushed local commit before triggering merge conflict
		h.CommitFile("local.txt", "unpushed\n", "Unpushed local commit")
		localSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))

		h.Git("checkout", "master")
		h.CommitFile("shared.txt", "master line\n", "Master conflicting commit")

		h.Git("checkout", "feature/merge-abort")
		_, _ = h.GitAllowError("merge", "master")

		mergeHead := filepath.Join(h.RepoDir, ".git", "MERGE_HEAD")
		_, statErr := os.Stat(mergeHead)
		require.NoError(t, statErr, "MERGE_HEAD must exist during conflict")

		// First reset: abort merge only and preserve unpushed local commit
		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Aborting in-progress merge")
		assert.Contains(t, stderr.String(), "Aborted in-progress merge; local commits preserved")
		assert.Contains(t, stderr.String(), "Run 'git-brx reset' again to discard local changes and match 'origin/feature/merge-abort'")

		_, statErr = os.Stat(mergeHead)
		assert.True(t, os.IsNotExist(statErr))
		assert.Equal(t, localSha, strings.TrimSpace(h.Git("rev-parse", "HEAD")))

		// Second reset (now idle): hard-reset to origin/feature/merge-abort
		stdout.Reset()
		stderr.Reset()
		code = cli.RunInDir(context.Background(), h.RepoDir, []string{"reset"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Working tree successfully reset to 'origin/feature/merge-abort'")
		assert.Equal(t, originSha, strings.TrimSpace(h.Git("rev-parse", "HEAD")))
	})

	t.Run("UnpublishedBranchActiveRebasePreservesLocalCommit", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		// Unpublished branch with an unpushed commit
		h.CreateBranch("issue/rebase-abort")
		h.CommitFile("shared.txt", "branch line\n", "Unpublished branch commit")
		unpushedSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))

		// Create conflicting commit on master
		h.Git("checkout", "master")
		h.CommitFile("shared.txt", "master line\n", "Master conflicting commit")

		// Rebase onto master to trigger real in-flight rebase conflict
		h.Git("checkout", "issue/rebase-abort")
		_, _ = h.GitAllowError("rebase", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Aborting in-progress rebase")
		assert.Contains(t, stderr.String(), "Aborted in-progress rebase; local commits preserved")
		assert.Equal(t, unpushedSha, strings.TrimSpace(h.Git("rev-parse", "HEAD")))

		// In-flight rebase should be aborted
		rebaseMerge := filepath.Join(h.RepoDir, ".git", "rebase-merge")
		rebaseApply := filepath.Join(h.RepoDir, ".git", "rebase-apply")
		_, statErr1 := os.Stat(rebaseMerge)
		assert.True(t, os.IsNotExist(statErr1))
		_, statErr2 := os.Stat(rebaseApply)
		assert.True(t, os.IsNotExist(statErr2))
	})

	t.Run("ActiveConflictRejectsCleanFlag", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("shared.txt", "init\n", "Initial commit")

		h.CreateBranch("issue/clean-guard")
		h.CommitFile("shared.txt", "branch line\n", "Branch commit")

		h.Git("checkout", "master")
		h.CommitFile("shared.txt", "master line\n", "Master commit")

		h.Git("checkout", "issue/clean-guard")
		_, _ = h.GitAllowError("rebase", "master")

		untracked := filepath.Join(h.RepoDir, "keep-me.txt")
		require.NoError(t, os.WriteFile(untracked, []byte("untracked"), 0644))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset", "--clean"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitUsageError), code)
		assert.Contains(t, stderr.String(), "Cannot use '--clean' while a rebase is in progress")

		// Rebase must remain active and untracked file untouched
		_, statErr1 := os.Stat(filepath.Join(h.RepoDir, ".git", "rebase-merge"))
		_, statErr2 := os.Stat(filepath.Join(h.RepoDir, ".git", "rebase-apply"))
		assert.True(t, statErr1 == nil || statErr2 == nil)
		_, statUntracked := os.Stat(untracked)
		assert.NoError(t, statUntracked)
	})

	t.Run("CleanFlagRemovesUntrackedFiles", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/clean-test")
		h.CommitFile("feat.txt", "feat", "Feature commit")
		h.Git("push", "-u", "origin", "feature/clean-test")

		// Create untracked file and directory
		untrackedFile := filepath.Join(h.RepoDir, "untracked.txt")
		err := os.WriteFile(untrackedFile, []byte("untracked file"), 0644)
		require.NoError(t, err)

		untrackedSub := filepath.Join(h.RepoDir, "untracked_dir", "sub.txt")
		err = os.MkdirAll(filepath.Dir(untrackedSub), 0755)
		require.NoError(t, err)
		err = os.WriteFile(untrackedSub, []byte("untracked sub"), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset", "--clean"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)

		// Untracked files must be removed
		_, err = os.Stat(untrackedFile)
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(untrackedSub)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("WithoutCleanFlagUntrackedFilesPersist", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/no-clean")
		h.CommitFile("feat.txt", "feat", "Feature commit")
		h.Git("push", "-u", "origin", "feature/no-clean")

		untrackedFile := filepath.Join(h.RepoDir, "persist.txt")
		err := os.WriteFile(untrackedFile, []byte("should persist"), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitSuccess), code)

		// Untracked file must persist
		content, err := os.ReadFile(untrackedFile)
		require.NoError(t, err)
		assert.Equal(t, "should persist", string(content))
	})

	t.Run("UnpublishedBranchFailsWithExit4AndPublishHint", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/unpublished-reset")
		h.CommitFile("local.txt", "local", "Local only")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset"}, &stdout, &stderr)

		assert.Equal(t, int(errcode.ExitPreconditionRemote), code)
		assert.Contains(t, stderr.String(), "Reset failed: remote reference 'origin/feature/unpublished-reset' does not exist")
		assert.Contains(t, stderr.String(), "Use 'git-brx publish' to publish it")
	})
}

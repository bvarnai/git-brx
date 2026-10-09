package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvarnai/git-brx/internal/cli"
	"github.com/bvarnai/git-brx/internal/domain"
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

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Working tree successfully reset to 'origin/issue/VSB-300'")

		resetSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))
		assert.Equal(t, origSha, resetSha)
	})

	t.Run("InFlightMergeAbortedAndReset", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/merge-abort")
		h.CommitFile("feat.txt", "feat", "Feature commit")
		h.Git("push", "-u", "origin", "feature/merge-abort")

		// Simulate in-flight merge
		mergeHead := filepath.Join(h.RepoDir, ".git", "MERGE_HEAD")
		err := os.WriteFile(mergeHead, []byte("deadbeef\n"), 0644)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "You are in the middle of a merge, aborting")
		assert.Contains(t, stderr.String(), "Working tree successfully reset to 'origin/feature/merge-abort'")

		// MERGE_HEAD should no longer exist
		_, statErr := os.Stat(mergeHead)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("InFlightRebaseAbortedAndReset", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("issue/rebase-abort")
		h.CommitFile("shared.txt", "branch line\n", "Branch commit")
		h.Git("push", "-u", "origin", "issue/rebase-abort")

		// Create conflicting commit on master
		h.Git("checkout", "master")
		h.CommitFile("shared.txt", "master line\n", "Master conflicting commit")

		// Rebase onto master to trigger real in-flight rebase conflict
		h.Git("checkout", "issue/rebase-abort")
		_, _ = h.GitAllowError("rebase", "master")

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "You are in the middle of a rebase, aborting")
		assert.Contains(t, stderr.String(), "Working tree successfully reset to 'origin/issue/rebase-abort'")

		// In-flight rebase should be aborted
		rebaseMerge := filepath.Join(h.RepoDir, ".git", "rebase-merge")
		rebaseApply := filepath.Join(h.RepoDir, ".git", "rebase-apply")
		_, statErr1 := os.Stat(rebaseMerge)
		assert.True(t, os.IsNotExist(statErr1))
		_, statErr2 := os.Stat(rebaseApply)
		assert.True(t, os.IsNotExist(statErr2))
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

		assert.Equal(t, int(domain.ExitSuccess), code)

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

		assert.Equal(t, int(domain.ExitSuccess), code)

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

		assert.Equal(t, int(domain.ExitPreconditionRemote), code)
		assert.Contains(t, stderr.String(), "Reset failed: remote reference 'origin/feature/unpublished-reset' does not exist")
		assert.Contains(t, stderr.String(), "Use 'git-brx publish' to publish it")
	})

	t.Run("DryRunModeDoesNotModifyRefsOrFiles", func(t *testing.T) {
		h := NewHarness(t)
		h.CommitFile("init.txt", "init", "Initial commit")
		h.SetupRemoteBare()
		h.Git("push", "-u", "origin", "master")

		h.CreateBranch("feature/dry-run-reset")
		h.CommitFile("feat.txt", "feat", "Feature commit")
		h.Git("push", "-u", "origin", "feature/dry-run-reset")

		// Create local commit
		h.CommitFile("unwanted.txt", "unwanted", "Unwanted commit")
		currSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))

		var stdout, stderr bytes.Buffer
		code := cli.RunInDir(context.Background(), h.RepoDir, []string{"reset", "--dry-run", "-c"}, &stdout, &stderr)

		assert.Equal(t, int(domain.ExitSuccess), code)
		assert.Contains(t, stderr.String(), "Would execute: git reset --hard origin/feature/dry-run-reset")
		assert.Contains(t, stderr.String(), "Would execute: git clean -fd")

		postSha := strings.TrimSpace(h.Git("rev-parse", "HEAD"))
		assert.Equal(t, currSha, postSha, "dry-run must not modify HEAD")
	})
}
